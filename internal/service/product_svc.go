package service

import (
	"bytes"
	"context"
	"encoding/json"
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/pkg/net"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

type ProductService struct {
	ProductRepo *repository.ProductRepo
	ShopRepo    *repository.ShopRepo
	AIService   *AIService
	Storage     *StorageService
	Dispatcher  net.Dispatcher
}

func NewProductService(
	productRepo *repository.ProductRepo,
	shopRepo *repository.ShopRepo,
	ai *AIService,
	storage *StorageService,
	dispatcher net.Dispatcher,
) *ProductService {
	return &ProductService{
		ProductRepo: productRepo,
		ShopRepo:    shopRepo,
		AIService:   ai,
		Storage:     storage,
		Dispatcher:  dispatcher,
	}
}

// ==================== 查询操作 ====================

// GetShopProducts 获取店铺商品列表
func (s *ProductService) GetShopProducts(shopID int64, page, pageSize int) ([]model.Product, int64, error) {
	ctx := context.Background()
	return s.ProductRepo.ListByShop(ctx, shopID, page, pageSize)
}

// GetProductByID 获取商品详情
func (s *ProductService) GetProductByID(ctx context.Context, id int64) (*model.Product, error) {
	return s.ProductRepo.GetByID(ctx, id)
}

// SearchProducts 搜索商品
func (s *ProductService) SearchProducts(ctx context.Context, shopID int64, keyword string, page, pageSize int) ([]model.Product, int64, error) {
	return s.ProductRepo.SearchByTitle(ctx, shopID, keyword, page, pageSize)
}

// GetProductStats 获取店铺商品统计
func (s *ProductService) GetProductStats(ctx context.Context, shopID int64) (*dto.ProductStatsResp, error) {
	stats, err := s.ProductRepo.CountByShopAndState(ctx, shopID)
	if err != nil {
		return nil, err
	}

	var total int64
	byState := make(map[string]int64)
	for state, count := range stats {
		byState[string(state)] = count
		total += count
	}

	return &dto.ProductStatsResp{
		ShopID:  shopID,
		Total:   total,
		ByState: byState,
	}, nil
}

// ==================== AI 草稿流程 ====================

// GenerateAIDraft 调用 AI 生成商品草稿
func (s *ProductService) GenerateAIDraft(ctx context.Context, req *dto.AIGenerateReq) (*model.Product, error) {
	// 1. 获取店铺信息
	shop, err := s.ShopRepo.GetByID(ctx, req.ShopID)
	if err != nil {
		return nil, fmt.Errorf("店铺不存在: %v", err)
	}

	// 2. 调用 AI 服务生成内容
	aiResult, err := s.AIService.GenerateProductContent(ctx, req.SourceMaterial, req.StyleHint)
	if err != nil {
		return nil, fmt.Errorf("AI 生成失败: %v", err)
	}

	// 3. 构建本地草稿
	product := &model.Product{
		ShopID:         shop.ID,
		Title:          aiResult.Title,
		Description:    aiResult.Description,
		Tags:           aiResult.Tags,
		State:          model.ProductStateDraft,
		SyncStatus:     int(model.ProductSyncStatusLocal),
		EditStatus:     model.EditStatusAIDraft,
		SourceMaterial: req.SourceMaterial,
		WhoMade:        "i_did",
		WhenMade:       "made_to_order",
		CurrencyCode:   shop.CurrencyCode,
		PriceDivisor:   100,
	}

	if req.TargetCategory > 0 {
		product.TaxonomyID = req.TargetCategory
	}

	// 4. 保存到本地数据库
	if err := s.ProductRepo.Create(ctx, product); err != nil {
		return nil, fmt.Errorf("保存草稿失败: %v", err)
	}

	return product, nil
}

// ApproveAIDraft 审核通过 AI 草稿
func (s *ProductService) ApproveAIDraft(ctx context.Context, productID int64) error {
	product, err := s.ProductRepo.GetByID(ctx, productID)
	if err != nil {
		return err
	}

	if product.EditStatus != model.EditStatusAIDraft && product.EditStatus != model.EditStatusReviewing {
		return fmt.Errorf("当前状态不允许审核")
	}

	product.EditStatus = model.EditStatusApproved
	product.SyncStatus = int(model.DraftSyncStatusPending)
	return s.ProductRepo.Update(ctx, product)
}

// ==================== Etsy 同步操作 ====================

// CreateDraftListing 创建 Etsy 草稿并同步
func (s *ProductService) CreateDraftListing(ctx context.Context, req *dto.CreateProductReq) (*model.Product, error) {
	// 1. 获取店铺及鉴权信息 (GetByID 已 Preload Developer & Proxy)
	shop, err := s.ShopRepo.GetByID(ctx, req.ShopID)
	if err != nil {
		return nil, fmt.Errorf("店铺不存在: %v", err)
	}
	if shop.TokenStatus == model.TokenStatusInvalid {
		return nil, fmt.Errorf("授权已失效，请重新授权")
	}

	// 2. 构建 Etsy API 请求体
	priceAmount := int64(math.Round(req.Price * 100))
	currency := req.Currency
	if currency == "" {
		currency = shop.CurrencyCode
		if currency == "" {
			currency = "USD"
		}
	}

	payload := map[string]interface{}{
		"quantity":    req.Quantity,
		"title":       req.Title,
		"description": req.Description,
		"price": map[string]interface{}{
			"amount":        priceAmount,
			"divisor":       100,
			"currency_code": currency,
		},
		"taxonomy_id":         req.TaxonomyID,
		"shipping_profile_id": req.ShippingProfileID,
		"who_made":            defaultString(req.WhoMade, "i_did"),
		"when_made":           defaultString(req.WhenMade, "made_to_order"),
		"is_supply":           req.IsSupply,
	}

	// 可选字段
	if req.ReturnPolicyID > 0 {
		payload["return_policy_id"] = req.ReturnPolicyID
	}
	if len(req.Tags) > 0 {
		payload["tags"] = req.Tags
	}
	if len(req.Materials) > 0 {
		payload["materials"] = req.Materials
	}
	if len(req.Styles) > 0 {
		payload["styles"] = req.Styles
	}
	if len(req.ImageIDs) > 0 {
		payload["image_ids"] = req.ImageIDs
	}
	if req.ShopSectionID > 0 {
		payload["shop_section_id"] = req.ShopSectionID
	}

	// 物理属性
	if req.ItemWeight > 0 {
		payload["item_weight"] = req.ItemWeight
		payload["item_weight_unit"] = defaultString(req.ItemWeightUnit, "oz")
	}
	if req.ItemLength > 0 || req.ItemWidth > 0 || req.ItemHeight > 0 {
		payload["item_length"] = req.ItemLength
		payload["item_width"] = req.ItemWidth
		payload["item_height"] = req.ItemHeight
		payload["item_dimensions_unit"] = defaultString(req.ItemDimensionsUnit, "in")
	}

	// 3. 构建 HTTP 请求
	url := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%d/listings", shop.EtsyShopID)
	bodyBytes, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", shop.Developer.ApiKey)
	httpReq.Header.Set("Authorization", "Bearer "+shop.AccessToken)

	// 4. 通过 Dispatcher 发送 (自动处理代理)
	resp, err := s.Dispatcher.Send(ctx, shop.ID, httpReq)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("ETSY API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	// 5. 解析响应
	var result struct {
		ListingID int64 `json:"listing_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 6. 本地入库
	product := &model.Product{
		ShopID:            shop.ID,
		ListingID:         result.ListingID,
		Title:             req.Title,
		Description:       req.Description,
		Tags:              req.Tags,
		Materials:         req.Materials,
		Styles:            req.Styles,
		State:             model.ProductStateDraft,
		PriceAmount:       priceAmount,
		PriceDivisor:      100,
		CurrencyCode:      currency,
		Quantity:          req.Quantity,
		TaxonomyID:        req.TaxonomyID,
		ShippingProfileID: req.ShippingProfileID,
		ReturnPolicyID:    req.ReturnPolicyID,
		ShopSectionID:     req.ShopSectionID,
		WhoMade:           defaultString(req.WhoMade, "i_did"),
		WhenMade:          defaultString(req.WhenMade, "made_to_order"),
		IsSupply:          req.IsSupply,
		SyncStatus:        int(model.ProductSyncStatusSynced),
		SourceMaterial:    req.SourceMaterial,
	}

	if err := s.ProductRepo.Create(ctx, product); err != nil {
		// 回滚: 删除远程草稿
		s.deleteEtsyListingInternal(ctx, shop, result.ListingID)
		return nil, fmt.Errorf("本地入库失败(已回滚远程草稿): %v", err)
	}

	return product, nil
}

// UpdateListing 更新商品 (先推 Etsy，再更新本地)
func (s *ProductService) UpdateListing(ctx context.Context, req *dto.UpdateProductReq) error {
	// 1. 获取商品
	product, err := s.ProductRepo.GetByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("商品不存在: %v", err)
	}

	// 2. 获取店铺
	shop, err := s.ShopRepo.GetByID(ctx, product.ShopID)
	if err != nil {
		return err
	}

	// 3. 构建更新 payload
	payload := make(map[string]interface{})
	if req.Title != nil {
		payload["title"] = *req.Title
		product.Title = *req.Title
	}
	if req.Description != nil {
		payload["description"] = *req.Description
		product.Description = *req.Description
	}
	if req.Price != nil {
		priceAmount := int64(math.Round(*req.Price * 100))
		payload["price"] = map[string]interface{}{
			"amount":        priceAmount,
			"divisor":       100,
			"currency_code": product.CurrencyCode,
		}
		product.PriceAmount = priceAmount
	}
	if req.Quantity != nil {
		payload["quantity"] = *req.Quantity
		product.Quantity = *req.Quantity
	}
	if len(req.Tags) > 0 {
		payload["tags"] = req.Tags
		product.Tags = req.Tags
	}

	// 4. 如果商品已上传 Etsy，则先推送
	if product.ListingID > 0 && len(payload) > 0 {
		url := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%d/listings/%d",
			shop.EtsyShopID, product.ListingID)

		bodyBytes, _ := json.Marshal(payload)
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(bodyBytes))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", shop.Developer.ApiKey)
		httpReq.Header.Set("Authorization", "Bearer "+shop.AccessToken)

		resp, err := s.Dispatcher.Send(ctx, shop.ID, httpReq)
		if err != nil {
			product.SyncStatus = int(model.DraftSyncStatusFailed)
			product.SyncError = err.Error()
			_ = s.ProductRepo.Update(ctx, product)
			return fmt.Errorf("ETSY 更新失败: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			product.SyncStatus = int(model.DraftSyncStatusFailed)
			product.SyncError = string(respBody)
			_ = s.ProductRepo.Update(ctx, product)
			return fmt.Errorf("ETSY API 错误 [%d]: %s", resp.StatusCode, string(respBody))
		}

		product.SyncStatus = int(model.ProductSyncStatusSynced)
		product.SyncError = ""
	}

	// 5. 更新本地
	return s.ProductRepo.Update(ctx, product)
}

// ActivateListing 上架商品
func (s *ProductService) ActivateListing(ctx context.Context, productID int64) error {
	product, err := s.ProductRepo.GetByID(ctx, productID)
	if err != nil {
		return err
	}
	if product.ListingID == 0 {
		return fmt.Errorf("商品尚未上传到 Etsy")
	}

	shop, err := s.ShopRepo.GetByID(ctx, product.ShopID)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%d/listings/%d",
		shop.EtsyShopID, product.ListingID)

	bodyBytes, _ := json.Marshal(map[string]interface{}{"state": "active"})
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", shop.Developer.ApiKey)
	httpReq.Header.Set("Authorization", "Bearer "+shop.AccessToken)

	resp, err := s.Dispatcher.Send(ctx, shop.ID, httpReq)
	if err != nil {
		return fmt.Errorf("上架失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ETSY API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	product.State = model.ProductStateActive
	product.SyncStatus = int(model.ProductSyncStatusSynced)
	return s.ProductRepo.Update(ctx, product)
}

// DeactivateListing 下架商品
func (s *ProductService) DeactivateListing(ctx context.Context, productID int64) error {
	product, err := s.ProductRepo.GetByID(ctx, productID)
	if err != nil {
		return err
	}
	if product.ListingID == 0 {
		return fmt.Errorf("商品尚未上传到 Etsy")
	}

	shop, err := s.ShopRepo.GetByID(ctx, product.ShopID)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%d/listings/%d",
		shop.EtsyShopID, product.ListingID)

	bodyBytes, _ := json.Marshal(map[string]interface{}{"state": "inactive"})
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", shop.Developer.ApiKey)
	httpReq.Header.Set("Authorization", "Bearer "+shop.AccessToken)

	resp, err := s.Dispatcher.Send(ctx, shop.ID, httpReq)
	if err != nil {
		return fmt.Errorf("下架失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ETSY API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	product.State = model.ProductStateInactive
	return s.ProductRepo.Update(ctx, product)
}

// DeleteListing 删除商品
func (s *ProductService) DeleteListing(ctx context.Context, productID int64) error {
	product, err := s.ProductRepo.GetByID(ctx, productID)
	if err != nil {
		return err
	}

	// 如果已上传 Etsy，先删除远程
	if product.ListingID > 0 {
		shop, err := s.ShopRepo.GetByID(ctx, product.ShopID)
		if err != nil {
			return err
		}

		url := fmt.Sprintf("https://api.etsy.com/v3/application/listings/%d", product.ListingID)
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		httpReq.Header.Set("x-api-key", shop.Developer.ApiKey)
		httpReq.Header.Set("Authorization", "Bearer "+shop.AccessToken)

		resp, err := s.Dispatcher.Send(ctx, shop.ID, httpReq)
		if err != nil {
			return fmt.Errorf("删除远程失败: %v", err)
		}
		defer resp.Body.Close()

		// 404 也视为删除成功
		if resp.StatusCode != 204 && resp.StatusCode != 404 {
			respBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("ETSY API 错误 [%d]: %s", resp.StatusCode, string(respBody))
		}
	}

	// 本地软删除
	return s.ProductRepo.Delete(ctx, productID)
}

// ==================== 批量同步 ====================

// SyncListingsFromEtsy 从 Etsy 全量同步商品
func (s *ProductService) SyncListingsFromEtsy(ctx context.Context, shopID int64) error {
	shop, err := s.ShopRepo.GetByID(ctx, shopID)
	if err != nil {
		return err
	}
	if shop.TokenStatus == model.TokenStatusInvalid {
		return fmt.Errorf("授权已失效")
	}

	var allProducts []model.Product
	limit := 100
	offset := 0

	for {
		url := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%d/listings?limit=%d&offset=%d",
			shop.EtsyShopID, limit, offset)

		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		httpReq.Header.Set("x-api-key", shop.Developer.ApiKey)
		httpReq.Header.Set("Authorization", "Bearer "+shop.AccessToken)

		resp, err := s.Dispatcher.Send(ctx, shop.ID, httpReq)
		if err != nil {
			return fmt.Errorf("同步失败: %v", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("ETSY API 错误 [%d]: %s", resp.StatusCode, string(respBody))
		}

		var result struct {
			Count   int                      `json:"count"`
			Results []map[string]interface{} `json:"results"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return fmt.Errorf("解析响应失败: %v", err)
		}

		for _, item := range result.Results {
			product := s.mapEtsyListingToProduct(shop.ID, item)
			allProducts = append(allProducts, *product)
		}

		if len(result.Results) < limit {
			break
		}
		offset += limit
		time.Sleep(500 * time.Millisecond)
	}

	if len(allProducts) == 0 {
		return nil
	}

	return s.ProductRepo.BatchUpsert(ctx, allProducts)
}

// ==================== 图片操作 ====================

// UploadListingImage 上传图片到 Etsy
func (s *ProductService) UploadListingImage(ctx context.Context, productID int64, imageData []byte, filename string, rank int) (*model.ProductImage, error) {
	// 1. 获取商品
	product, err := s.ProductRepo.GetByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("商品不存在: %v", err)
	}
	if product.ListingID == 0 {
		return nil, fmt.Errorf("商品尚未上传到 Etsy，请先创建 Etsy 草稿")
	}

	// 2. 获取店铺
	shop, err := s.ShopRepo.GetByID(ctx, product.ShopID)
	if err != nil {
		return nil, fmt.Errorf("店铺不存在: %v", err)
	}

	// 3. 构建 multipart 请求
	url := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%d/listings/%d/images",
		shop.EtsyShopID, product.ListingID)

	resp, err := s.Dispatcher.SendMultipart(ctx, shop.ID, &net.MultipartRequest{
		URL: url,
		Headers: map[string]string{
			"x-api-key":     shop.Developer.ApiKey,
			"Authorization": "Bearer " + shop.AccessToken,
		},
		Files: map[string]net.FileData{
			"image": {Data: imageData, Filename: filename},
		},
		Fields: map[string]string{
			"rank": fmt.Sprintf("%d", rank),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("上传请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("ETSY API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	// 4. 解析响应
	var result struct {
		ListingImageID int64  `json:"listing_image_id"`
		ListingID      int64  `json:"listing_id"`
		Rank           int    `json:"rank"`
		UrlFullxfull   string `json:"url_fullxfull"`
		Url570xN       string `json:"url_570xN"`
		UrlThumb       string `json:"url_75x75"`
		FullHeight     int    `json:"full_height"`
		FullWidth      int    `json:"full_width"`
		HexCode        string `json:"hex_code"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 5. 本地入库
	image := &model.ProductImage{
		ProductID:   productID,
		EtsyImageID: result.ListingImageID,
		EtsyUrl:     result.UrlFullxfull,
		Rank:        result.Rank,
		Height:      result.FullHeight,
		Width:       result.FullWidth,
		HexCode:     result.HexCode,
		SyncStatus:  int(model.ProductSyncStatusSynced),
	}

	if err := s.ProductRepo.CreateImage(ctx, image); err != nil {
		return nil, fmt.Errorf("图片入库失败: %v", err)
	}

	return image, nil
}

// ==================== 私有方法 ====================

func (s *ProductService) deleteEtsyListingInternal(ctx context.Context, shop *model.Shop, listingID int64) {
	url := fmt.Sprintf("https://api.etsy.com/v3/application/listings/%d", listingID)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	httpReq.Header.Set("x-api-key", shop.Developer.ApiKey)
	httpReq.Header.Set("Authorization", "Bearer "+shop.AccessToken)

	_, err := s.Dispatcher.Send(ctx, shop.ID, httpReq)
	if err != nil {
		fmt.Printf("[CRITICAL] 回滚失败: Shop=%d, Listing=%d, Err=%v\n", shop.ID, listingID, err)
	}
}

func (s *ProductService) mapEtsyListingToProduct(shopID int64, data map[string]interface{}) *model.Product {
	product := &model.Product{
		ShopID:     shopID,
		SyncStatus: int(model.ProductSyncStatusSynced),
	}

	if v, ok := data["listing_id"].(float64); ok {
		product.ListingID = int64(v)
	}
	if v, ok := data["user_id"].(float64); ok {
		product.UserID = int64(v)
	}
	if v, ok := data["title"].(string); ok {
		product.Title = v
	}
	if v, ok := data["description"].(string); ok {
		product.Description = v
	}
	if v, ok := data["state"].(string); ok {
		product.State = model.ProductState(v)
	}
	if v, ok := data["url"].(string); ok {
		product.Url = v
	}
	if v, ok := data["quantity"].(float64); ok {
		product.Quantity = int(v)
	}
	if v, ok := data["views"].(float64); ok {
		product.Views = int(v)
	}
	if v, ok := data["num_favorers"].(float64); ok {
		product.NumFavorers = int(v)
	}

	// 价格
	if price, ok := data["price"].(map[string]interface{}); ok {
		if v, ok := price["amount"].(float64); ok {
			product.PriceAmount = int64(v)
		}
		if v, ok := price["divisor"].(float64); ok {
			product.PriceDivisor = int64(v)
		}
		if v, ok := price["currency_code"].(string); ok {
			product.CurrencyCode = v
		}
	}

	// 标签
	if tags, ok := data["tags"].([]interface{}); ok {
		for _, t := range tags {
			if str, ok := t.(string); ok {
				product.Tags = append(product.Tags, str)
			}
		}
	}

	// 时间戳
	if v, ok := data["creation_timestamp"].(float64); ok {
		product.EtsyCreationTS = int64(v)
	}
	if v, ok := data["last_modified_timestamp"].(float64); ok {
		product.EtsyLastModifiedTS = int64(v)
	}

	return product
}

func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ==================== DTO 转换方法 ====================

// ToProductResp Model -> DTO
func (s *ProductService) ToProductResp(p *model.Product) dto.ProductResp {
	resp := dto.ProductResp{
		ID:        p.ID,
		ListingID: p.ListingID,
		ShopID:    p.ShopID,

		Title:       p.Title,
		Description: p.Description,
		Tags:        p.Tags,
		Materials:   p.Materials,
		Styles:      p.Styles,
		Url:         p.Url,

		Price:        float64(p.PriceAmount) / float64(p.PriceDivisor),
		CurrencyCode: p.CurrencyCode,
		Quantity:     p.Quantity,

		State:      string(p.State),
		SyncStatus: p.SyncStatus,
		SyncError:  p.SyncError,
		EditStatus: int(p.EditStatus),

		TaxonomyID:        p.TaxonomyID,
		ShippingProfileID: p.ShippingProfileID,
		ReturnPolicyID:    p.ReturnPolicyID,
		ShopSectionID:     p.ShopSectionID,

		WhoMade:  p.WhoMade,
		WhenMade: p.WhenMade,
		IsSupply: p.IsSupply,

		ItemWeight:         p.ItemWeight,
		ItemWeightUnit:     p.ItemWeightUnit,
		ItemLength:         p.ItemLength,
		ItemWidth:          p.ItemWidth,
		ItemHeight:         p.ItemHeight,
		ItemDimensionsUnit: p.ItemDimensionsUnit,

		Views:       p.Views,
		NumFavorers: p.NumFavorers,

		CreatedAt: p.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: p.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	// 转换图片
	resp.Images = make([]dto.ProductImageResp, 0, len(p.Images))
	for _, img := range p.Images {
		resp.Images = append(resp.Images, s.toProductImageResp(&img))
	}

	// 转换变体
	resp.Variants = make([]dto.ProductVariantResp, 0, len(p.Variants))
	for _, v := range p.Variants {
		resp.Variants = append(resp.Variants, s.toProductVariantResp(&v))
	}

	return resp
}

// toProductImageResp 图片 Model -> DTO
func (s *ProductService) toProductImageResp(img *model.ProductImage) dto.ProductImageResp {
	return dto.ProductImageResp{
		ID:          img.ID,
		EtsyImageID: img.EtsyImageID,
		Url:         img.EtsyUrl,
		LocalPath:   img.LocalPath,
		Rank:        img.Rank,
		AltText:     img.AltText,
		Width:       img.Width,
		Height:      img.Height,
	}
}

// toProductVariantResp 变体 Model -> DTO
func (s *ProductService) toProductVariantResp(v *model.ProductVariant) dto.ProductVariantResp {
	props := make(map[string]interface{})
	if v.PropertyValues != nil {
		_ = json.Unmarshal(v.PropertyValues, &props)
	}

	return dto.ProductVariantResp{
		ID:             v.ID,
		EtsyProductID:  v.EtsyProductID,
		EtsyOfferingID: v.EtsyOfferingID,
		PropertyValues: props,
		Price:          float64(v.PriceAmount) / float64(v.PriceDivisor),
		Quantity:       v.Quantity,
		LocalSKU:       v.LocalSKU,
		EtsySKU:        v.EtsySKU,
		IsEnabled:      v.IsEnabled,
	}
}

// ToProductVO 兼容旧方法名
func (s *ProductService) ToProductVO(p *model.Product) dto.ProductResp {
	return s.ToProductResp(p)
}
