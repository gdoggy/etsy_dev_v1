package task

import (
	"bytes"
	"context"
	"encoding/json"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/pkg/net"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// ==================== 接口定义 ====================

// Notifier 通知接口
type Notifier interface {
	NotifyUser(userID int64, event string, data interface{}) error
}

// StorageProvider 存储接口
type StorageProvider interface {
	Delete(ctx context.Context, url string) error
}

// ==================== DraftSubmitTask 草稿提交任务 ====================

// DraftSubmitTask 定时扫描已确认的草稿并提交到 Etsy
type DraftSubmitTask struct {
	draftProductRepo repository.DraftProductRepository
	draftTaskRepo    repository.DraftTaskRepository
	draftImageRepo   repository.DraftImageRepository
	productRepo      repository.ProductRepository
	shopRepo         repository.ShopRepository
	dispatcher       net.Dispatcher
	notifier         Notifier
	cron             *cron.Cron

	// 并发控制
	concurrencyLimit int
	sleepTime        time.Duration
}

// NewDraftSubmitTask 创建草稿提交任务
func NewDraftSubmitTask(
	draftProductRepo repository.DraftProductRepository,
	draftTaskRepo repository.DraftTaskRepository,
	draftImageRepo repository.DraftImageRepository,
	productRepo repository.ProductRepository,
	shopRepo repository.ShopRepository,
	dispatcher net.Dispatcher,
	notifier Notifier,
) *DraftSubmitTask {
	return &DraftSubmitTask{
		draftProductRepo: draftProductRepo,
		draftTaskRepo:    draftTaskRepo,
		draftImageRepo:   draftImageRepo,
		productRepo:      productRepo,
		shopRepo:         shopRepo,
		dispatcher:       dispatcher,
		notifier:         notifier,
		cron:             cron.New(cron.WithSeconds()),
		concurrencyLimit: 5,                      // 草稿提交并发上限（API 限制）
		sleepTime:        200 * time.Millisecond, // 协程启动间隔
	}
}

// SetConcurrency 设置并发参数
func (t *DraftSubmitTask) SetConcurrency(limit int, sleep time.Duration) {
	t.concurrencyLimit = limit
	t.sleepTime = sleep
}

// Start 启动定时任务
func (t *DraftSubmitTask) Start() {
	// 定时策略：每分钟执行
	_, err := t.cron.AddFunc("0 * * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		t.execute(ctx)
	})

	if err != nil {
		log.Fatalf("[DraftSubmitTask] 无法启动定时任务: %v", err)
	}

	t.cron.Start()
	log.Println("[DraftSubmitTask] 草稿提交任务已启动 (每分钟检查)")
}

// Stop 停止任务
func (t *DraftSubmitTask) Stop() {
	ctx := t.cron.Stop()
	<-ctx.Done()
	log.Println("[DraftSubmitTask] 已停止")
}

// execute 执行一次任务
func (t *DraftSubmitTask) execute(ctx context.Context) {
	// 查询待提交的草稿商品
	drafts, err := t.draftProductRepo.FindPendingSubmit(ctx, 20)
	if err != nil {
		log.Printf("[DraftSubmitTask] 查询失败: %v", err)
		return
	}

	if len(drafts) == 0 {
		return
	}

	log.Printf("[DraftSubmitTask] 发现 %d 个待提交草稿", len(drafts))

	// 信号量控制并发
	sem := make(chan struct{}, t.concurrencyLimit)
	var wg sync.WaitGroup

	var (
		successCount int
		failCount    int
		mu           sync.Mutex
	)

	for i := range drafts {
		draft := drafts[i]
		select {
		case <-ctx.Done():
			log.Println("[DraftSubmitTask] 任务超时停止")
			wg.Wait()
			return
		default:
		}

		sem <- struct{}{}
		wg.Add(1)
		time.Sleep(t.sleepTime)

		go func(d model.DraftProduct) {
			defer wg.Done()
			defer func() { <-sem }()

			err := t.submitDraft(ctx, &d)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				failCount++
				log.Printf("[DraftSubmitTask] 草稿 %d 提交失败: %v", d.ID, err)
			} else {
				successCount++
				log.Printf("[DraftSubmitTask] 草稿 %d 提交成功", d.ID)
			}
		}(draft)
	}

	wg.Wait()
	log.Printf("[DraftSubmitTask] 本轮完成，成功: %d, 失败: %d", successCount, failCount)
}

// submitDraft 提交单个草稿到 Etsy
func (t *DraftSubmitTask) submitDraft(ctx context.Context, draft *model.DraftProduct) error {
	// 更新状态为提交中
	t.draftProductRepo.UpdateSyncStatus(ctx, draft.ID, model.DraftSyncStatusPending)

	// 1. 获取店铺信息
	shop, err := t.shopRepo.GetByID(ctx, draft.ShopID)
	if err != nil {
		t.markFailed(ctx, draft, fmt.Sprintf("店铺不存在: %v", err))
		return err
	}

	if shop.TokenStatus != model.ShopTokenStatusValid {
		errMsg := "店铺授权已失效"
		t.markFailed(ctx, draft, errMsg)
		return fmt.Errorf(errMsg)
	}

	// 2. 获取开发者信息
	developer, err := t.shopRepo.GetDeveloperByShopID(ctx, shop.ID)
	if err != nil {
		t.markFailed(ctx, draft, fmt.Sprintf("开发者不存在: %v", err))
		return err
	}

	// 3. 创建 Etsy 草稿 Listing
	listingID, err := t.createEtsyListing(ctx, shop, developer, draft)
	if err != nil {
		t.markFailed(ctx, draft, fmt.Sprintf("创建Listing失败: %v", err))
		return err
	}

	// 4. 上传图片（需要先有 listing）
	if len(draft.SelectedImages) > 0 {
		for i, imgURL := range draft.SelectedImages {
			_, err := t.uploadImage(ctx, shop, developer, listingID, imgURL, i+1)
			if err != nil {
				log.Printf("[DraftSubmitTask] 图片上传失败: %v", err)
				// 继续处理，不中断
			}
		}
	}

	// 5. 更新草稿状态
	t.draftProductRepo.MarkSubmitted(ctx, draft.ID, listingID)

	// 6. 创建正式 Product 记录
	product := &model.Product{
		ShopID:            draft.ShopID,
		ListingID:         listingID,
		Title:             draft.Title,
		Description:       draft.Description,
		Tags:              draft.Tags,
		State:             model.ProductStateDraft,
		PriceAmount:       draft.PriceAmount,
		PriceDivisor:      draft.PriceDivisor,
		CurrencyCode:      draft.CurrencyCode,
		Quantity:          draft.Quantity,
		TaxonomyID:        draft.TaxonomyID,
		ShippingProfileID: draft.ShippingProfileID,
		ReturnPolicyID:    draft.ReturnPolicyID,
		SyncStatus:        int(model.ProductSyncStatusSynced),
	}
	if err := t.productRepo.Create(ctx, product); err != nil {
		log.Printf("[DraftSubmitTask] Product入库失败: %v", err)
	} else {
		t.draftProductRepo.UpdateProductID(ctx, draft.ID, product.ID)
	}

	// 7. 通知用户
	if t.notifier != nil {
		task, _ := t.draftTaskRepo.GetByID(ctx, draft.TaskID)
		if task != nil {
			t.notifier.NotifyUser(task.UserID, "draft_submitted", map[string]interface{}{
				"draft_id":   draft.ID,
				"product_id": product.ID,
				"listing_id": listingID,
				"shop_id":    draft.ShopID,
			})
		}
	}

	return nil
}

// createEtsyListing 调用 Etsy API 创建草稿
func (t *DraftSubmitTask) createEtsyListing(
	ctx context.Context,
	shop *model.Shop,
	developer *model.Developer,
	draft *model.DraftProduct,
) (int64, error) {
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
		"who_made":            "i_did",
		"when_made":           "made_to_order",
		"is_supply":           false,
	}

	if draft.ReturnPolicyID > 0 {
		payload["return_policy_id"] = draft.ReturnPolicyID
	}
	if len(draft.Tags) > 0 {
		payload["tags"] = []string(draft.Tags)
	}

	apiURL := fmt.Sprintf("https://openapi.etsy.com/v3/application/shops/%d/listings", shop.EtsyShopID)
	bodyBytes, _ := json.Marshal(payload)

	req, err := net.BuildEtsyRequest(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return 0, err
	}

	resp, err := t.dispatcher.Send(ctx, shop.ID, req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("Etsy API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ListingID int64 `json:"listing_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, err
	}

	return result.ListingID, nil
}

// uploadImage 上传图片到 Etsy
func (t *DraftSubmitTask) uploadImage(
	ctx context.Context,
	shop *model.Shop,
	developer *model.Developer,
	listingID int64,
	imageURL string,
	rank int,
) (int64, error) {
	// 1. 下载图片
	imgResp, err := http.Get(imageURL)
	if err != nil {
		return 0, fmt.Errorf("下载图片失败: %v", err)
	}
	defer imgResp.Body.Close()

	imageData, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取图片失败: %v", err)
	}

	// 2. 构建 multipart 请求
	apiURL := fmt.Sprintf("https://openapi.etsy.com/v3/application/shops/%d/listings/%d/images",
		shop.EtsyShopID, listingID)

	multipartReq := &net.MultipartRequest{
		URL: apiURL,
		Headers: map[string]string{
			"x-api-key":     developer.ApiKey,
			"Authorization": "Bearer " + shop.AccessToken,
		},
		Files: map[string]net.FileData{
			"image": {
				Data:     imageData,
				Filename: fmt.Sprintf("image_%d.jpg", rank),
			},
		},
		Fields: map[string]string{
			"rank": fmt.Sprintf("%d", rank),
		},
	}

	resp, err := t.dispatcher.SendMultipart(ctx, shop.ID, multipartReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("上传图片失败 [%d]: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ListingImageID int64 `json:"listing_image_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, err
	}

	return result.ListingImageID, nil
}

// markFailed 标记失败
func (t *DraftSubmitTask) markFailed(ctx context.Context, draft *model.DraftProduct, errMsg string) {
	t.draftProductRepo.MarkFailed(ctx, draft.ID, errMsg)
}

// ==================== DraftCleanupTask 过期清理任务 ====================

// DraftCleanupTask 清理过期草稿
type DraftCleanupTask struct {
	draftTaskRepo    repository.DraftTaskRepository
	draftProductRepo repository.DraftProductRepository
	draftImageRepo   repository.DraftImageRepository
	storage          StorageProvider
	cron             *cron.Cron
}

// NewDraftCleanupTask 创建清理任务
func NewDraftCleanupTask(
	draftTaskRepo repository.DraftTaskRepository,
	draftProductRepo repository.DraftProductRepository,
	draftImageRepo repository.DraftImageRepository,
	storage StorageProvider,
) *DraftCleanupTask {
	return &DraftCleanupTask{
		draftTaskRepo:    draftTaskRepo,
		draftProductRepo: draftProductRepo,
		draftImageRepo:   draftImageRepo,
		storage:          storage,
		cron:             cron.New(cron.WithSeconds()),
	}
}

// Start 启动定时清理任务
func (t *DraftCleanupTask) Start() {
	// 每小时执行一次
	_, err := t.cron.AddFunc("0 0 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		t.execute(ctx)
	})

	if err != nil {
		log.Fatalf("[DraftCleanupTask] 无法启动定时任务: %v", err)
	}

	// 启动时立即执行一次
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		t.execute(ctx)
	}()

	t.cron.Start()
	log.Println("[DraftCleanupTask] 草稿清理任务已启动 (每小时)")
}

// Stop 停止任务
func (t *DraftCleanupTask) Stop() {
	ctx := t.cron.Stop()
	<-ctx.Done()
	log.Println("[DraftCleanupTask] 已停止")
}

// execute 执行清理
func (t *DraftCleanupTask) execute(ctx context.Context) {
	expireTime := time.Now().Add(-24 * time.Hour)

	log.Printf("[DraftCleanupTask] 开始清理 %s 之前的过期草稿", expireTime.Format(time.RFC3339))

	// 查询过期的任务
	tasks, err := t.draftTaskRepo.FindExpired(ctx, expireTime)
	if err != nil {
		log.Printf("[DraftCleanupTask] 查询失败: %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	log.Printf("[DraftCleanupTask] 发现 %d 个过期任务", len(tasks))

	for _, task := range tasks {
		t.cleanupTask(ctx, &task)
	}
}

// cleanupTask 清理单个任务
func (t *DraftCleanupTask) cleanupTask(ctx context.Context, task *model.DraftTask) {
	// 1. 删除关联的图片文件
	images, _ := t.draftImageRepo.GetByTaskID(ctx, task.ID)
	for _, img := range images {
		if img.StorageURL != "" && t.storage != nil {
			if err := t.storage.Delete(ctx, img.StorageURL); err != nil {
				log.Printf("[DraftCleanupTask] 删除图片失败: %v", err)
			}
		}
	}

	// 2. 删除图片记录
	t.draftImageRepo.DeleteByTaskID(ctx, task.ID)

	// 3. 删除草稿商品
	t.draftProductRepo.DeleteByTaskID(ctx, task.ID)

	// 4. 更新任务状态为过期
	t.draftTaskRepo.MarkExpired(ctx, task.ID)

	log.Printf("[DraftCleanupTask] 任务 %d 已清理", task.ID)
}
