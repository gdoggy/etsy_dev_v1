package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gorm.io/datatypes"

	"etsy_dev_v1_202512/internal/api/dto"
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
	uow      *repository.DraftUnitOfWork
	shopRepo repository.ShopRepository
	scraper  OneBoundServiceInterface
	ai       AIServiceInterface
	storage  StorageServiceInterface

	// 进度订阅管理
	subscribers     map[int64][]chan dto.ProgressEvent
	subscriberMutex sync.RWMutex
}

// NewDraftService 创建草稿服务
func NewDraftService(
	uow *repository.DraftUnitOfWork,
	shopRepo repository.ShopRepository,
	scraper OneBoundServiceInterface,
	ai AIServiceInterface,
	storage StorageServiceInterface,
) *DraftService {
	return &DraftService{
		uow:         uow,
		shopRepo:    shopRepo,
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

	// 验证店铺存在
	for _, shopID := range req.ShopIDs {
		if _, err := s.shopRepo.GetByID(ctx, shopID); err != nil {
			return nil, fmt.Errorf("店铺 %d 不存在", shopID)
		}
	}

	// 设置默认值
	imageCount := req.ImageCount
	if imageCount <= 0 || imageCount > 20 {
		imageCount = 20
	}

	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
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
	go s.processTask(task.ID, req.ShopIDs, quantity)

	return &dto.CreateDraftResult{
		TaskID:    task.ID,
		Status:    task.Status,
		CreatedAt: task.CreatedAt,
	}, nil
}

// shopDraftResult 单个店铺草稿生成结果
type shopDraftResult struct {
	ShopID       int64
	CurrencyCode string
	Title        string
	Description  string
	Tags         []string
	ImageURLs    []string
	Error        error
}

// processTask 异步处理任务
func (s *DraftService) processTask(taskID int64, shopIDs []int64, quantity int) {
	ctx := context.Background()

	// 更新状态为处理中
	s.uow.Tasks.UpdateStatus(ctx, taskID, model.TaskStatusProcessing, model.AIStatusProcessing)

	// 获取任务
	task, err := s.uow.Tasks.GetByID(ctx, taskID)
	if err != nil {
		s.failTask(ctx, taskID, "获取任务失败: "+err.Error())
		return
	}

	// 1. 抓取商品数据（共享）
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

	// 保存抓取数据（使用 datatypes.JSON）
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
	sourceDataBytes, _ := json.Marshal(sourceData)
	s.uow.Tasks.UpdateFields(ctx, taskID, map[string]interface{}{
		"source_data": datatypes.JSON(sourceDataBytes),
	})

	// 获取参考图片
	var refImageURL string
	if len(product.Images) > 0 {
		refImageURL = product.Images[0]
	}

	// 2. 为每个店铺并发生成 AI 内容
	s.notifyProgress(taskID, dto.ProgressEvent{
		TaskID:   taskID,
		Stage:    "generating",
		Progress: 20,
		Message:  fmt.Sprintf("正在为 %d 个店铺生成内容...", len(shopIDs)),
	})

	results := s.generateForShops(ctx, taskID, shopIDs, product.Title, task.StyleHint, task.ExtraPrompt, refImageURL, task.ImageCount)

	// 3. 处理结果，创建草稿商品
	s.notifyProgress(taskID, dto.ProgressEvent{
		TaskID:   taskID,
		Stage:    "saving",
		Progress: 80,
		Message:  "正在保存草稿商品...",
	})

	var successCount int
	var lastError string

	for _, result := range results {
		if result.Error != nil {
			lastError = result.Error.Error()
			continue
		}

		// 保存图片到 DraftImage
		var draftImages []model.DraftImage
		for i, url := range result.ImageURLs {
			draftImages = append(draftImages, model.DraftImage{
				TaskID:     taskID,
				GroupIndex: int(result.ShopID), // 按店铺分组
				ImageIndex: i,
				StorageURL: url,
				Status:     model.ImageStatusReady,
			})
		}
		if len(draftImages) > 0 {
			s.uow.Images.CreateBatch(ctx, draftImages)
		}

		// 创建草稿商品（使用 datatypes.JSONSlice）
		draftProduct := model.DraftProduct{
			TaskID:         taskID,
			ShopID:         result.ShopID,
			Title:          result.Title,
			Description:    result.Description,
			Tags:           datatypes.JSONSlice[string](result.Tags),
			SelectedImages: datatypes.JSONSlice[string](result.ImageURLs),
			CurrencyCode:   result.CurrencyCode,
			Quantity:       quantity,
			Status:         model.DraftStatusDraft,
			SyncStatus:     model.DraftSyncStatusNone,
		}

		if err := s.uow.Products.Create(ctx, &draftProduct); err != nil {
			lastError = err.Error()
			continue
		}

		successCount++
	}

	// 4. 更新任务状态
	if successCount == 0 {
		s.failTask(ctx, taskID, "所有店铺生成均失败: "+lastError)
		return
	}

	// 部分成功也算完成
	s.uow.Tasks.UpdateStatus(ctx, taskID, model.TaskStatusDraft, model.AIStatusDone)

	s.notifyProgress(taskID, dto.ProgressEvent{
		TaskID:   taskID,
		Stage:    "done",
		Progress: 100,
		Message:  fmt.Sprintf("处理完成，成功生成 %d/%d 个店铺草稿", successCount, len(shopIDs)),
	})
}

// generateForShops 并发为多个店铺生成内容
func (s *DraftService) generateForShops(
	ctx context.Context,
	taskID int64,
	shopIDs []int64,
	sourceTitle, styleHint, extraPrompt, refImageURL string,
	imageCount int,
) []shopDraftResult {
	results := make([]shopDraftResult, len(shopIDs))

	// 风格变体，确保每个店铺生成不同的内容
	styleVariants := []string{
		"minimalist modern style",
		"warm cozy aesthetic",
		"elegant premium look",
		"vibrant colorful design",
		"natural organic feel",
	}

	var wg sync.WaitGroup
	for i, shopID := range shopIDs {
		wg.Add(1)
		go func(idx int, sid int64) {
			defer wg.Done()

			result := shopDraftResult{ShopID: sid}

			// 获取店铺货币
			shop, err := s.shopRepo.GetByID(ctx, sid)
			if err != nil {
				result.Error = fmt.Errorf("获取店铺失败: %v", err)
				results[idx] = result
				return
			}
			result.CurrencyCode = shop.CurrencyCode
			if result.CurrencyCode == "" {
				result.CurrencyCode = "USD" // 默认值
			}

			// 组合风格提示（原始 + 变体）
			variantStyle := styleVariants[idx%len(styleVariants)]
			combinedStyle := variantStyle
			if styleHint != "" {
				combinedStyle = styleHint + ", " + variantStyle
			}
			if extraPrompt != "" {
				combinedStyle = combinedStyle + ". " + extraPrompt
			}

			// 生成文案
			textResult, err := s.ai.GenerateProductContent(ctx, sourceTitle, combinedStyle)
			if err != nil {
				result.Error = fmt.Errorf("生成文案失败: %v", err)
				results[idx] = result
				return
			}
			result.Title = textResult.Title
			result.Description = textResult.Description
			result.Tags = textResult.Tags

			// 生成图片
			imagePrompt := fmt.Sprintf("Product photo of %s, %s, professional e-commerce photography",
				textResult.Title, variantStyle)
			base64Images, err := s.ai.GenerateImages(ctx, imagePrompt, refImageURL, imageCount)
			if err != nil {
				result.Error = fmt.Errorf("生成图片失败: %v", err)
				results[idx] = result
				return
			}

			// 保存图片
			var imageURLs []string
			for j, base64Data := range base64Images {
				prefix := fmt.Sprintf("draft/%d/shop_%d/img_%d", taskID, sid, j)
				url, err := s.storage.SaveBase64(base64Data, prefix)
				if err != nil {
					continue // 跳过失败的图片
				}
				imageURLs = append(imageURLs, url)
			}

			if len(imageURLs) == 0 {
				result.Error = fmt.Errorf("所有图片保存失败")
				results[idx] = result
				return
			}

			result.ImageURLs = imageURLs
			results[idx] = result
		}(i, shopID)
	}

	wg.Wait()
	return results
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

	// 解析源商品数据（从 datatypes.JSON）
	var sourceProduct dto.ScrapedProductVO
	if len(task.SourceData) > 0 {
		var sourceMap map[string]interface{}
		if err := json.Unmarshal(task.SourceData, &sourceMap); err == nil {
			sourceProduct = dto.ScrapedProductVO{
				Platform:    getMapString(sourceMap, "platform"),
				ItemID:      getMapString(sourceMap, "item_id"),
				Title:       getMapString(sourceMap, "title"),
				Price:       getMapFloat(sourceMap, "price"),
				Currency:    getMapString(sourceMap, "currency"),
				Description: getMapString(sourceMap, "description"),
			}
			if images, ok := sourceMap["images"].([]interface{}); ok {
				for _, img := range images {
					if str, ok := img.(string); ok {
						sourceProduct.Images = append(sourceProduct.Images, str)
					}
				}
			}
		}
	}

	// 转换商品（每个店铺有独立的 AI 结果）
	productVOs := make([]dto.DraftProductVO, len(products))
	for i, p := range products {
		// 获取店铺名称
		shopName := ""
		if shop, err := s.shopRepo.GetByID(ctx, p.ShopID); err == nil {
			shopName = shop.ShopName
		}

		productVOs[i] = dto.DraftProductVO{
			ID:                p.ID,
			ShopID:            p.ShopID,
			ShopName:          shopName,
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
		AIResult:      nil, // 每个店铺有独立结果，不再有统一的 AIResult
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
		updates["tags"] = datatypes.JSONSlice[string](req.Tags)
	}
	if req.Price != nil {
		updates["price_amount"] = int64(*req.Price * 100)
	}
	if len(req.SelectedImages) > 0 {
		updates["selected_images"] = datatypes.JSONSlice[string](req.SelectedImages)
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
