package service

import (
	"context"
	"encoding/json"
	"etsy_dev_v1_202512/internal/api/dto"
	"fmt"
	"sync"
	"time"

	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
)

// ==================== 外部服务依赖 ====================
// 以下接口引用同包中其他服务定义的类型：
// - ScrapedProduct: 定义于 onebound_svc.go
// - TextGenerateResult: 定义于 ai_svc.go

// OneBoundServiceInterface 万邦抓取服务接口
type OneBoundServiceInterface interface {
	ParseURL(url string) (platform, itemID string, err error)
	FetchProduct(ctx context.Context, platform, itemID string) (*ScrapedProduct, error)
}

// AIServiceInterface AI服务接口
type AIServiceInterface interface {
	GenerateProductContent(ctx context.Context, title, styleHint string) (*TextGenerateResult, error)
	GenerateImages(ctx context.Context, prompt, refImageURL string, count int) ([]string, error)
}

// StorageServiceInterface 存储服务接口
type StorageServiceInterface interface {
	SaveBase64(base64Data, prefix string) (url string, err error)
}

// ==================== 服务实现 ====================

// DraftService 草稿服务
type DraftService struct {
	uow     *repository.DraftUnitOfWork
	scraper OneBoundServiceInterface
	ai      AIServiceInterface
	storage StorageServiceInterface

	// 进度订阅管理
	subscribers     map[int64][]chan dto.ProgressEvent
	subscriberMutex sync.RWMutex
}

// NewDraftService 创建草稿服务
func NewDraftService(
	uow *repository.DraftUnitOfWork,
	scraper OneBoundServiceInterface,
	ai AIServiceInterface,
	storage StorageServiceInterface,
) *DraftService {
	return &DraftService{
		uow:         uow,
		scraper:     scraper,
		ai:          ai,
		storage:     storage,
		subscribers: make(map[int64][]chan dto.ProgressEvent),
	}
}

// ==================== 进度订阅 ====================

// Subscribe 订阅任务进度
func (s *DraftService) Subscribe(taskID int64) chan dto.ProgressEvent {
	s.subscriberMutex.Lock()
	defer s.subscriberMutex.Unlock()

	ch := make(chan dto.ProgressEvent, 10)
	s.subscribers[taskID] = append(s.subscribers[taskID], ch)
	return ch
}

// Unsubscribe 取消订阅
func (s *DraftService) Unsubscribe(taskID int64, ch chan dto.ProgressEvent) {
	s.subscriberMutex.Lock()
	defer s.subscriberMutex.Unlock()

	subs := s.subscribers[taskID]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}

	if len(s.subscribers[taskID]) == 0 {
		delete(s.subscribers, taskID)
	}
}

// notifyProgress 通知进度
func (s *DraftService) notifyProgress(taskID int64, event dto.ProgressEvent) {
	s.subscriberMutex.RLock()
	defer s.subscriberMutex.RUnlock()

	for _, ch := range s.subscribers[taskID] {
		select {
		case ch <- event:
		default:
			// channel 已满，跳过
		}
	}
}

// ==================== 创建草稿 ====================

// CreateDraft 创建草稿任务
func (s *DraftService) CreateDraft(ctx context.Context, req *dto.CreateDraftRequest) (*dto.CreateDraftResult, error) {
	// 解析 URL
	platform, itemID, err := s.scraper.ParseURL(req.SourceURL)
	if err != nil {
		return nil, fmt.Errorf("不支持的商品链接: %v", err)
	}

	// 设置默认值
	imageCount := req.ImageCount
	if imageCount <= 0 {
		imageCount = 20
	}
	if imageCount > 20 {
		imageCount = 20
	}

	// 创建任务
	task := &model.DraftTask{
		UserID:         req.UserID,
		SourceURL:      req.SourceURL,
		SourcePlatform: platform,
		SourceItemID:   itemID,
		ImageCount:     imageCount,
		StyleHint:      req.StyleHint,
		ExtraPrompt:    req.ExtraPrompt,
		Status:         model.TaskStatusPending,
		AIStatus:       model.AIStatusPending,
	}

	if err := s.uow.Tasks.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("创建任务失败: %v", err)
	}

	// 异步处理
	go s.processTask(task.ID, req.ShopIDs)

	return &dto.CreateDraftResult{
		TaskID:    task.ID,
		Status:    task.Status,
		CreatedAt: task.CreatedAt,
	}, nil
}

// processTask 异步处理任务
func (s *DraftService) processTask(taskID int64, shopIDs []int64) {
	ctx := context.Background()

	// 更新状态为处理中
	s.uow.Tasks.UpdateStatus(ctx, taskID, model.TaskStatusProcessing, model.AIStatusProcessing)

	// 获取任务
	task, err := s.uow.Tasks.GetByID(ctx, taskID)
	if err != nil {
		s.failTask(ctx, taskID, "获取任务失败: "+err.Error())
		return
	}

	// 1. 抓取商品数据
	s.notifyProgress(taskID, dto.ProgressEvent{
		TaskID:   taskID,
		Stage:    "fetching",
		Progress: 10,
		Message:  "正在抓取商品信息...",
	})

	product, err := s.scraper.FetchProduct(ctx, task.SourcePlatform, task.SourceItemID)
	if err != nil {
		s.failTask(ctx, taskID, "抓取商品失败: "+err.Error())
		return
	}

	// 保存抓取数据
	sourceData := map[string]interface{}{
		"platform":    product.Platform,
		"item_id":     product.ItemID,
		"title":       product.Title,
		"price":       product.Price,
		"currency":    product.Currency,
		"images":      product.Images,
		"description": product.Description,
		"attributes":  product.Attributes,
	}
	s.uow.Tasks.UpdateFields(ctx, taskID, map[string]interface{}{
		"source_data": model.JSONMap(sourceData),
	})

	// 2. 生成 AI 文案
	s.notifyProgress(taskID, dto.ProgressEvent{
		TaskID:   taskID,
		Stage:    "generating_text",
		Progress: 30,
		Message:  "正在生成商品文案...",
	})

	textResult, err := s.ai.GenerateProductContent(ctx, product.Title, task.StyleHint)
	if err != nil {
		s.failTask(ctx, taskID, "生成文案失败: "+err.Error())
		return
	}

	// 保存 AI 文案结果
	aiTextResult := map[string]interface{}{
		"title":       textResult.Title,
		"description": textResult.Description,
		"tags":        textResult.Tags,
	}
	s.uow.Tasks.UpdateFields(ctx, taskID, map[string]interface{}{
		"ai_text_result": model.JSONMap(aiTextResult),
	})

	// 3. 生成 AI 图片
	s.notifyProgress(taskID, dto.ProgressEvent{
		TaskID:   taskID,
		Stage:    "generating_images",
		Progress: 50,
		Message:  "正在生成商品图片...",
	})

	var refImageURL string
	if len(product.Images) > 0 {
		refImageURL = product.Images[0]
	}

	imagePrompt := fmt.Sprintf("Product photo of %s, professional e-commerce photography", textResult.Title)
	base64Images, err := s.ai.GenerateImages(ctx, imagePrompt, refImageURL, task.ImageCount)
	if err != nil {
		s.failTask(ctx, taskID, "生成图片失败: "+err.Error())
		return
	}

	// 4. 保存图片
	s.notifyProgress(taskID, dto.ProgressEvent{
		TaskID:   taskID,
		Stage:    "saving",
		Progress: 80,
		Message:  "正在保存图片...",
	})

	var imageURLs []string
	var draftImages []model.DraftImage

	for i, base64Data := range base64Images {
		prefix := fmt.Sprintf("draft/%d/img_%d", taskID, i)
		url, err := s.storage.SaveBase64(base64Data, prefix)
		if err != nil {
			continue // 跳过失败的图片
		}
		imageURLs = append(imageURLs, url)

		draftImages = append(draftImages, model.DraftImage{
			TaskID:     taskID,
			GroupIndex: 0,
			ImageIndex: i,
			StorageURL: url,
			Status:     model.ImageStatusReady,
		})
	}

	if len(draftImages) > 0 {
		s.uow.Images.CreateBatch(ctx, draftImages)
	}

	// 更新任务的 AI 图片
	s.uow.Tasks.UpdateFields(ctx, taskID, map[string]interface{}{
		"ai_images": model.StringSlice(imageURLs),
	})

	// 5. 创建草稿商品
	var draftProducts []model.DraftProduct
	for _, shopID := range shopIDs {
		draftProducts = append(draftProducts, model.DraftProduct{
			TaskID:         taskID,
			ShopID:         shopID,
			Title:          textResult.Title,
			Description:    textResult.Description,
			Tags:           model.StringSlice(textResult.Tags),
			SelectedImages: model.StringSlice(imageURLs),
			CurrencyCode:   "USD",
			Quantity:       1,
			Status:         model.DraftStatusDraft,
			SyncStatus:     model.DraftSyncStatusNone,
		})
	}

	if err := s.uow.Products.CreateBatch(ctx, draftProducts); err != nil {
		s.failTask(ctx, taskID, "创建草稿商品失败: "+err.Error())
		return
	}

	// 6. 完成
	s.uow.Tasks.UpdateStatus(ctx, taskID, model.TaskStatusDraft, model.AIStatusDone)

	s.notifyProgress(taskID, dto.ProgressEvent{
		TaskID:   taskID,
		Stage:    "done",
		Progress: 100,
		Message:  "处理完成",
	})
}

// failTask 标记任务失败
func (s *DraftService) failTask(ctx context.Context, taskID int64, errMsg string) {
	s.uow.Tasks.UpdateFields(ctx, taskID, map[string]interface{}{
		"status":           model.TaskStatusFailed,
		"ai_status":        model.AIStatusFailed,
		"ai_error_message": errMsg,
	})

	s.notifyProgress(taskID, dto.ProgressEvent{
		TaskID:   taskID,
		Stage:    "failed",
		Progress: 0,
		Message:  errMsg,
	})
}

// ==================== 查询 ====================

// ListTasks 查询任务列表
func (s *DraftService) ListTasks(ctx context.Context, req *dto.ListDraftTasksRequest) ([]dto.DraftTaskResponse, int64, error) {
	tasks, total, err := s.uow.Tasks.List(ctx, repository.TaskFilter{
		UserID:   req.UserID,
		Status:   req.Status,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.DraftTaskResponse, len(tasks))
	for i, task := range tasks {
		count, _ := s.uow.Products.CountByTaskID(ctx, task.ID)
		result[i] = dto.DraftTaskResponse{
			TaskID:       task.ID,
			Status:       task.Status,
			AIStatus:     task.AIStatus,
			SourceURL:    task.SourceURL,
			Platform:     task.SourcePlatform,
			CreatedAt:    task.CreatedAt.Format(time.RFC3339),
			ProductCount: int(count),
		}
	}

	return result, total, nil
}

// GetTaskDetail 获取任务详情
func (s *DraftService) GetTaskDetail(ctx context.Context, taskID int64) (*dto.DraftDetailResponse, error) {
	task, err := s.uow.Tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("任务不存在")
	}

	products, _ := s.uow.Products.GetByTaskID(ctx, taskID)

	// 转换为 VO
	taskVO := &dto.DraftTaskVO{
		ID:           task.ID,
		Status:       task.Status,
		AIStatus:     task.AIStatus,
		SourceURL:    task.SourceURL,
		Platform:     task.SourcePlatform,
		ItemID:       task.SourceItemID,
		ImageCount:   task.ImageCount,
		ErrorMessage: task.AIErrorMessage,
		CreatedAt:    task.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    task.UpdatedAt.Format(time.RFC3339),
	}

	// 解析源商品数据
	var sourceProduct dto.ScrapedProductVO
	if task.SourceData != nil {
		sourceProduct = dto.ScrapedProductVO{
			Platform:    getMapString(task.SourceData, "platform"),
			ItemID:      getMapString(task.SourceData, "item_id"),
			Title:       getMapString(task.SourceData, "title"),
			Price:       getMapFloat(task.SourceData, "price"),
			Currency:    getMapString(task.SourceData, "currency"),
			Description: getMapString(task.SourceData, "description"),
		}
		if images, ok := task.SourceData["images"].([]interface{}); ok {
			for _, img := range images {
				if s, ok := img.(string); ok {
					sourceProduct.Images = append(sourceProduct.Images, s)
				}
			}
		}
	}

	// 解析 AI 结果
	var aiResult dto.AIGenerateResult
	if task.AITextResult != nil {
		aiResult.Title = getMapString(task.AITextResult, "title")
		aiResult.Description = getMapString(task.AITextResult, "description")
		if tags, ok := task.AITextResult["tags"].([]interface{}); ok {
			for _, tag := range tags {
				if s, ok := tag.(string); ok {
					aiResult.Tags = append(aiResult.Tags, s)
				}
			}
		}
	}
	aiResult.Images = []string(task.AIImages)

	// 转换商品
	productVOs := make([]dto.DraftProductVO, len(products))
	for i, p := range products {
		productVOs[i] = dto.DraftProductVO{
			ID:                p.ID,
			ShopID:            p.ShopID,
			Status:            p.Status,
			SyncStatus:        p.SyncStatus,
			Title:             p.Title,
			Description:       p.Description,
			Tags:              []string(p.Tags),
			Price:             p.GetPrice(),
			CurrencyCode:      p.CurrencyCode,
			Quantity:          p.Quantity,
			TaxonomyID:        p.TaxonomyID,
			ShippingProfileID: p.ShippingProfileID,
			ReturnPolicyID:    p.ReturnPolicyID,
			SelectedImages:    []string(p.SelectedImages),
			ListingID:         p.ListingID,
			SyncError:         p.SyncError,
		}
	}

	return &dto.DraftDetailResponse{
		Task:          taskVO,
		SourceProduct: &sourceProduct,
		AIResult:      &aiResult,
		Products:      productVOs,
	}, nil
}

// ==================== 更新 ====================

// UpdateDraftProduct 更新草稿商品
func (s *DraftService) UpdateDraftProduct(ctx context.Context, productID int64, req *dto.UpdateDraftProductRequest) error {
	product, err := s.uow.Products.GetByID(ctx, productID)
	if err != nil {
		return fmt.Errorf("草稿商品不存在")
	}

	if product.Status != model.DraftStatusDraft {
		return fmt.Errorf("只能修改草稿状态的商品")
	}

	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if len(req.Tags) > 0 {
		updates["tags"] = model.StringSlice(req.Tags)
	}
	if req.Price != nil {
		updates["price_amount"] = int64(*req.Price * 100)
	}
	if len(req.SelectedImages) > 0 {
		updates["selected_images"] = model.StringSlice(req.SelectedImages)
	}
	if req.Quantity != nil {
		updates["quantity"] = *req.Quantity
	}
	if req.TaxonomyID != nil {
		updates["taxonomy_id"] = *req.TaxonomyID
	}
	if req.ShippingProfileID != nil {
		updates["shipping_profile_id"] = *req.ShippingProfileID
	}

	if len(updates) == 0 {
		return nil
	}

	return s.uow.Products.UpdateFields(ctx, productID, updates)
}

// ==================== 确认 ====================

// ConfirmDraft 确认单个草稿
func (s *DraftService) ConfirmDraft(ctx context.Context, productID int64) error {
	product, err := s.uow.Products.GetByID(ctx, productID)
	if err != nil {
		return fmt.Errorf("草稿商品不存在")
	}

	if err := product.CanConfirm(); err != nil {
		return err
	}

	return s.uow.Products.UpdateFields(ctx, productID, map[string]interface{}{
		"status":      model.DraftStatusConfirmed,
		"sync_status": model.DraftSyncStatusPending,
	})
}

// ConfirmAllDrafts 确认任务下所有草稿
func (s *DraftService) ConfirmAllDrafts(ctx context.Context, taskID int64) (int64, error) {
	return s.uow.Products.ConfirmAll(ctx, taskID)
}

// ==================== 平台信息 ====================

// GetSupportedPlatforms 获取支持的平台
func (s *DraftService) GetSupportedPlatforms() *dto.SupportedPlatformsResponse {
	return &dto.SupportedPlatformsResponse{
		Platforms: []dto.PlatformInfo{
			{Code: "1688", Name: "1688", URLPatterns: []string{"detail.1688.com", "m.1688.com"}},
			{Code: "taobao", Name: "淘宝", URLPatterns: []string{"item.taobao.com", "detail.tmall.com"}},
			{Code: "tmall", Name: "天猫", URLPatterns: []string{"detail.tmall.com", "detail.tmall.hk"}},
			{Code: "aliexpress", Name: "速卖通", URLPatterns: []string{"aliexpress.com", "aliexpress.ru"}},
			{Code: "amazon", Name: "Amazon", URLPatterns: []string{"amazon.com", "amazon.co.uk", "amazon.de"}},
			{Code: "ebay", Name: "eBay", URLPatterns: []string{"ebay.com", "ebay.co.uk", "ebay.de"}},
		},
	}
}

// ==================== 辅助函数 ====================

func getMapString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getMapFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		case int64:
			return float64(val)
		case json.Number:
			f, _ := val.Float64()
			return f
		}
	}
	return 0
}
