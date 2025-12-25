package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"gorm.io/gorm"
)

// ==================== 草稿提交任务 ====================

// DraftSubmitTask 定时扫描已确认的草稿并提交到 Etsy
type DraftSubmitTask struct {
	db         *gorm.DB
	dispatcher Dispatcher
	notifier   Notifier

	running bool
	mutex   sync.Mutex
}

// Dispatcher 网络调度器接口
type Dispatcher interface {
	Send(ctx context.Context, shopID int64, req *http.Request) (*http.Response, error)
	SendMultipart(ctx context.Context, shopID int64, req *MultipartRequest) (*http.Response, error)
}

// MultipartRequest 多部分请求
type MultipartRequest struct {
	URL     string
	Headers map[string]string
	Files   map[string]FileData
	Fields  map[string]string
}

type FileData struct {
	Data     []byte
	Filename string
}

// Notifier 通知接口
type Notifier interface {
	NotifyUser(userID int64, event string, data interface{}) error
}

func NewDraftSubmitTask(db *gorm.DB, dispatcher Dispatcher, notifier Notifier) *DraftSubmitTask {
	return &DraftSubmitTask{
		db:         db,
		dispatcher: dispatcher,
		notifier:   notifier,
	}
}

// Start 启动定时任务
func (t *DraftSubmitTask) Start(interval time.Duration) {
	t.mutex.Lock()
	if t.running {
		t.mutex.Unlock()
		return
	}
	t.running = true
	t.mutex.Unlock()

	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			t.execute()
		}
	}()
}

// execute 执行一次任务
func (t *DraftSubmitTask) execute() {
	ctx := context.Background()

	// 查询待提交的草稿商品
	var drafts []DraftProduct
	err := t.db.
		Preload("Shop").
		Preload("Shop.Developer").
		Where("status = ? AND sync_status = ?", "confirmed", 1).
		Limit(10). // 每次处理10个
		Find(&drafts).Error

	if err != nil {
		fmt.Printf("[DraftSubmitTask] 查询失败: %v\n", err)
		return
	}

	if len(drafts) == 0 {
		return
	}

	fmt.Printf("[DraftSubmitTask] 发现 %d 个待提交草稿\n", len(drafts))

	for _, draft := range drafts {
		t.submitDraft(ctx, &draft)
	}
}

// submitDraft 提交单个草稿到 Etsy
func (t *DraftSubmitTask) submitDraft(ctx context.Context, draft *DraftProduct) {
	// 更新状态为提交中
	t.db.Model(draft).Update("sync_status", 2) // syncing

	// 1. 获取店铺信息
	var shop Shop
	if err := t.db.Preload("Developer").First(&shop, draft.ShopID).Error; err != nil {
		t.markFailed(draft, fmt.Sprintf("店铺不存在: %v", err))
		return
	}

	if shop.TokenStatus != 1 { // TokenStatusActive
		t.markFailed(draft, "店铺授权已失效")
		return
	}

	// 2. 上传图片
	var imageIDs []int64
	var selectedImages []string
	if draft.SelectedImages != nil {
		json.Unmarshal(draft.SelectedImages, &selectedImages)
	}

	for _, imgURL := range selectedImages {
		imageID, err := t.uploadImage(ctx, &shop, imgURL)
		if err != nil {
			fmt.Printf("[DraftSubmitTask] 图片上传失败: %v\n", err)
			continue
		}
		imageIDs = append(imageIDs, imageID)
	}

	// 3. 创建 Etsy 草稿
	listingID, err := t.createEtsyListing(ctx, &shop, draft, imageIDs)
	if err != nil {
		t.markFailed(draft, fmt.Sprintf("创建Listing失败: %v", err))
		return
	}

	// 4. 更新状态
	t.db.Model(draft).Updates(map[string]interface{}{
		"status":      "submitted",
		"sync_status": 0, // synced
		"listing_id":  listingID,
	})

	// 5. 创建正式 Product 记录
	product := &Product{
		ShopID:            draft.ShopID,
		ListingID:         listingID,
		Title:             draft.Title,
		Description:       draft.Description,
		Tags:              draft.Tags,
		State:             "draft",
		PriceAmount:       draft.PriceAmount,
		PriceDivisor:      draft.PriceDivisor,
		CurrencyCode:      draft.CurrencyCode,
		Quantity:          draft.Quantity,
		TaxonomyID:        draft.TaxonomyID,
		ShippingProfileID: draft.ShippingProfileID,
		ReturnPolicyID:    draft.ReturnPolicyID,
		WhoMade:           draft.WhoMade,
		WhenMade:          draft.WhenMade,
		IsSupply:          draft.IsSupply,
		SyncStatus:        0,
	}
	if err := t.db.Create(product).Error; err != nil {
		fmt.Printf("[DraftSubmitTask] Product入库失败: %v\n", err)
	} else {
		t.db.Model(draft).Update("product_id", product.ID)
	}

	// 6. 通知用户
	if t.notifier != nil {
		// 获取任务的用户ID
		var task DraftTask
		t.db.First(&task, draft.TaskID)
		t.notifier.NotifyUser(task.UserID, "draft_submitted", map[string]interface{}{
			"draft_id":   draft.ID,
			"product_id": product.ID,
			"listing_id": listingID,
			"shop_id":    draft.ShopID,
		})
	}

	fmt.Printf("[DraftSubmitTask] 草稿 %d 提交成功, ListingID: %d\n", draft.ID, listingID)
}

// createEtsyListing 调用 Etsy API 创建草稿
func (t *DraftSubmitTask) createEtsyListing(ctx context.Context, shop *Shop, draft *DraftProduct, imageIDs []int64) (int64, error) {
	payload := map[string]interface{}{
		"quantity":    draft.Quantity,
		"title":       draft.Title,
		"description": draft.Description,
		"price": map[string]interface{}{
			"amount":        draft.PriceAmount,
			"divisor":       draft.PriceDivisor,
			"currency_code": draft.CurrencyCode,
		},
		"taxonomy_id":         draft.TaxonomyID,
		"shipping_profile_id": draft.ShippingProfileID,
		"who_made":            draft.WhoMade,
		"when_made":           draft.WhenMade,
		"is_supply":           draft.IsSupply,
	}

	if draft.ReturnPolicyID > 0 {
		payload["return_policy_id"] = draft.ReturnPolicyID
	}
	if len(draft.Tags) > 0 {
		payload["tags"] = draft.Tags
	}
	if len(imageIDs) > 0 {
		payload["image_ids"] = imageIDs
	}

	url := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%d/listings", shop.EtsyShopID)
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", shop.Developer.ApiKey)
	req.Header.Set("Authorization", "Bearer "+shop.AccessToken)

	resp, err := t.dispatcher.Send(ctx, shop.ID, req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return 0, fmt.Errorf("ETSY API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ListingID int64 `json:"listing_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, err
	}

	return result.ListingID, nil
}

// uploadImage 上传图片到 Etsy (预留，需要先创建临时 listing 或使用其他方式)
func (t *DraftSubmitTask) uploadImage(ctx context.Context, shop *Shop, imageURL string) (int64, error) {
	// TODO: 实现图片上传逻辑
	// Etsy 要求先创建 listing 才能上传图片
	// 这里暂时返回0，后续在 createEtsyListing 后补充上传
	return 0, nil
}

func (t *DraftSubmitTask) markFailed(draft *DraftProduct, errMsg string) {
	t.db.Model(draft).Updates(map[string]interface{}{
		"sync_status": 3, // failed
		"sync_error":  errMsg,
	})
	fmt.Printf("[DraftSubmitTask] 草稿 %d 提交失败: %s\n", draft.ID, errMsg)
}

// ==================== 过期清理任务 ====================

// DraftCleanupTask 清理过期草稿
type DraftCleanupTask struct {
	db      *gorm.DB
	storage StorageProvider
}

// StorageProvider 存储接口
type StorageProvider interface {
	Delete(ctx context.Context, url string) error
}

func NewDraftCleanupTask(db *gorm.DB, storage StorageProvider) *DraftCleanupTask {
	return &DraftCleanupTask{
		db:      db,
		storage: storage,
	}
}

// Start 启动定时清理任务
func (t *DraftCleanupTask) Start() {
	// 每小时执行一次
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		// 启动时立即执行一次
		t.execute()

		for range ticker.C {
			t.execute()
		}
	}()
}

// execute 执行清理
func (t *DraftCleanupTask) execute() {
	ctx := context.Background()
	expireTime := time.Now().Add(-24 * time.Hour)

	fmt.Printf("[DraftCleanupTask] 开始清理 %s 之前的过期草稿\n", expireTime.Format(time.RFC3339))

	// 1. 查询过期的任务
	var tasks []DraftTask
	err := t.db.
		Where("status = ? AND created_at < ?", "draft", expireTime).
		Find(&tasks).Error

	if err != nil {
		fmt.Printf("[DraftCleanupTask] 查询失败: %v\n", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	fmt.Printf("[DraftCleanupTask] 发现 %d 个过期任务\n", len(tasks))

	for _, task := range tasks {
		t.cleanupTask(ctx, &task)
	}
}

// cleanupTask 清理单个任务
func (t *DraftCleanupTask) cleanupTask(ctx context.Context, task *DraftTask) {
	// 1. 删除关联的图片文件
	var images []DraftImage
	t.db.Where("task_id = ?", task.ID).Find(&images)

	for _, img := range images {
		if img.StorageURL != "" && t.storage != nil {
			if err := t.storage.Delete(ctx, img.StorageURL); err != nil {
				fmt.Printf("[DraftCleanupTask] 删除图片失败: %v\n", err)
			}
		}
	}

	// 2. 删除图片记录
	t.db.Where("task_id = ?", task.ID).Delete(&DraftImage{})

	// 3. 删除草稿商品
	t.db.Where("task_id = ?", task.ID).Delete(&DraftProduct{})

	// 4. 更新任务状态为过期
	t.db.Model(task).Update("status", "expired")

	fmt.Printf("[DraftCleanupTask] 任务 %d 已清理\n", task.ID)
}

// ==================== 类型定义 (引用其他包) ====================

type DraftTask struct {
	ID             int64
	UserID         int64
	Status         string
	CreatedAt      time.Time
	SelectedImages []byte
}

type DraftProduct struct {
	ID                int64
	TaskID            int64
	ShopID            int64
	Shop              *Shop
	Title             string
	Description       string
	Tags              []string
	PriceAmount       int64
	PriceDivisor      int64
	CurrencyCode      string
	SelectedImages    []byte
	Quantity          int
	TaxonomyID        int64
	ShippingProfileID int64
	ReturnPolicyID    int64
	WhoMade           string
	WhenMade          string
	IsSupply          bool
	Status            string
	SyncStatus        int
	SyncError         string
	ListingID         int64
	ProductID         int64
}

type DraftImage struct {
	ID         int64
	TaskID     int64
	StorageURL string
}

type Shop struct {
	ID          int64
	EtsyShopID  int64
	AccessToken string
	TokenStatus int
	Developer   *Developer
}

type Developer struct {
	ApiKey string
}

type Product struct {
	ID                int64
	ShopID            int64
	ListingID         int64
	Title             string
	Description       string
	Tags              []string
	State             string
	PriceAmount       int64
	PriceDivisor      int64
	CurrencyCode      string
	Quantity          int
	TaxonomyID        int64
	ShippingProfileID int64
	ReturnPolicyID    int64
	WhoMade           string
	WhenMade          string
	IsSupply          bool
	SyncStatus        int
}
