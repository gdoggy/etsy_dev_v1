package service

import (
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
)

type ProductService struct {
	ShopRepo  *repository.ShopRepo
	AIService *AIService
	Storage   *StorageService
}

func NewProductService(shopRepo *repository.ShopRepo, ai *AIService, storage *StorageService) *ProductService {
	return &ProductService{
		ShopRepo:  shopRepo,
		AIService: ai,
		Storage:   storage,
	}
}

func (s *ProductService) GetShopProducts(shopID int64, page, pageSize int) ([]model.Product, int64, error) {
	panic("implement me")
}

/*
func (s *ProductService) SyncAndSaveListings(ctx context.Context, shopID int64) error {
	// 1. 获取店铺鉴权信息
	shop, err := s.ShopRepo.GetByID(ctx, shopID)
	if err != nil {
		return fmt.Errorf("店铺不存在: %v", err)
	}

	if shop.TokenStatus == model.TokenStatusInvalid {
		return fmt.Errorf("授权已失效，请在店铺列表重新授权")
	}

	// 2. 循环分页
	limit := 100 // 建议设为 100，效率最高
	offset := 0
	var allProducts []model.Product // 用于暂存所有拉取到的商品

	fmt.Println("开始全量同步商品...")

	for {
		// 3. 动态拼接 URL (带分页参数)
		// 接口文档: /v3/application/shops/{shop_id}/listings/active
		// 参数: limit=100, offset=0
		apiUrl := fmt.Sprintf(
			"https://api.etsy.com/v3/application/shop/%d/listings/state=active?limit=%d&offset=%d",
			shop.EtsyShopID, limit, offset,
		)

		var res etsy.ProductListingsResp
		resp, err := client.R().
			SetHeader("x-api-key", shop.Developer.APIKey).
			SetHeader("Authorization", "Bearer "+shop.AccessToken).
			SetResult(&res). // 解析到 DTO
			Get(apiUrl)

		if err != nil {
			return fmt.Errorf("网络请求失败: %v", err)
		}
		if resp.StatusCode() != 200 {
			return fmt.Errorf("API 异常 [%d]: %s", resp.StatusCode(), resp.String())
		}

		// 4. 数据转换 (DTO -> Model) 并放入切片
		for _, itemDTO := range res.Results {
			// 调用私有方法
			productModel := ToProductModel(itemDTO)
			allProducts = append(allProducts, *productModel)
		}

		fmt.Printf("   >> 本页拉取 %d 条 (Offset: %d)\n", len(res.Results), offset)

		// 5. 循环终止条件
		// 如果当前页拿到的数据少于 limit，说明后面没数据了
		if len(res.Results) < limit {
			break
		}

		// 否则，翻页
		offset += limit
		// 建议休眠一下，防止触发 QPS 限制
		time.Sleep(1 * time.Second)
	}

	if len(allProducts) == 0 {
		fmt.Println("该店铺没有在线商品")
		return nil
	}

	// 6. 批量入库 (UPSERT 逻辑)
	// 这里的逻辑是：如果 listing_id 冲突，就更新后面列出的字段
	err := s.ShopRepo.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "listing_id"}}, // 冲突检测列
		DoUpdates: clause.AssignmentColumns([]string{
			"title", "description", "state", "url",
			"price_amount", "quantity", "views", "num_favorers",
			"tags", "last_modified_tsz", "updated_at",
		}),
	}).Create(&allProducts).Error

	if err != nil {
		return fmt.Errorf("数据库保存失败: %v", err)
	}

	fmt.Printf("同步完成！共更新/插入 %d 个商品\n", len(allProducts))
	return nil
}

func (s *ProductService) GetShopProducts(shopID uint, page, pageSize int) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	// 基础查询
	query := s.ShopRepo.db.Model(&model.Product{}).Where("shop_id = ?", shopID)

	// 1. 查总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 2. 查列表 (分页)
	offset := (page - 1) * pageSize
	err := query.Order("updated_at DESC"). // 按更新时间倒序
						Limit(pageSize).
						Offset(offset).
						Find(&products).Error

	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

// CreateDraftListing 创建 Etsy 草稿商品
func (s *ProductService) CreateDraftListing(shopID int64, req etsy.CreateListingReq) (int64, error) {
	// 1. 获取店铺
	var shop model.Shop
	if err := s.ShopRepo.db.Preload("Proxy").Preload("Developer").First(&shop, shopID).Error; err != nil {
		return 0, fmt.Errorf("店铺不存在")
	}

	client := NewProxiedClient(shop.Proxy)

	// 2. 价格处理 (必须使用 math.Round 修正精度)
	// 变量在这里定义，确保作用域覆盖整个函数
	priceAmount := int64(math.Round(req.Price * 100))

	currency := shop.CurrencyCode
	if currency == "" {
		currency = "USD"
	}

	priceObj := map[string]interface{}{
		"amount":        priceAmount,
		"divisor":       100,
		"currency_code": currency,
	}

	// 3. 构建 Payload
	payload := map[string]interface{}{
		"quantity":            req.Quantity,
		"title":               req.Title,
		"description":         req.Description,
		"price":               priceObj,
		"taxonomy_id":         req.TaxonomyID,
		"shipping_profile_id": req.ShippingProfileID,
		"who_made":            req.WhoMade,
		"when_made":           req.WhenMade,
		"type":                req.Type,
		"state":               "draft",
		"tags":                req.Tags,
	}

	if len(req.ImageIDs) > 0 {
		payload["image_ids"] = req.ImageIDs
	}
	// 防御性添加 readiness_state_id (如果前端传了)
	if req.ReadinessStateID > 0 {
		payload["readiness_state_id"] = req.ReadinessStateID
	}

	// 4. 发起请求
	url := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%d/listings", shop.EtsyShopID)

	type createResp struct {
		ListingID int64 `json:"listing_id"`
	}
	var res createResp
	var errResp map[string]interface{}

	resp, err := client.R().
		SetHeader("x-api-key", shop.Developer.APIKey).
		SetHeader("Authorization", "Bearer "+shop.AccessToken).
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		SetResult(&res).
		SetError(&errResp).
		Post(url)

	if err != nil {
		return 0, fmt.Errorf("网络请求失败: %v", err)
	}

	if resp.StatusCode() != 201 {
		errJson, _ := json.Marshal(errResp)
		return 0, fmt.Errorf("ETSY Error (Code %d): %s", resp.StatusCode(), string(errJson))
	}

	// 5. 入库与事务回滚 (Transaction & Rollback)
	newProduct := model.Product{
		ShopID:       shop.ID,
		ListingID:    res.ListingID,
		Title:        req.Title,
		Description:  req.Description,
		State:        "draft",
		PriceAmount:  priceAmount,
		PriceDivisor: 100,
		CurrencyCode: currency,
		Quantity:     req.Quantity,
		Tags:         req.Tags,
		// 建议补齐关联字段，虽然 GORM 不强制
		ShippingProfileID: req.ShippingProfileID,
		TaxonomyID:        req.TaxonomyID,
	}

	// 严谨处理：如果入库失败，必须回滚远程操作
	if err = s.ShopRepo.db.Create(&newProduct).Error; err != nil {
		s.deleteEtsyListingInternal(client, shop.EtsyShopID, res.ListingID)
		return 0, fmt.Errorf("本地入库失败(已回滚远程草稿): %v", err)
	}

	return res.ListingID, nil
}

// deleteEtsyListingInternal 仅用于创建失败时的回滚
func (s *ProductService) deleteEtsyListingInternal(client *resty.Client, shopEtsyID int64, listingID int64) {
	// DELETE /v3/application/shops/{shop_id}/listings/{listing_id}
	url := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%d/listings/%d", shopEtsyID, listingID)

	// 这是一个补救操作，尽最大努力执行
	_, err := client.R().Delete(url)
	if err != nil {
		// 如果回滚都失败了，这才是真正的灾难，必须记录高危日志 (Sentry/Log)
		fmt.Printf("[CRITICAL] 严重故障：商品入库失败且远程回滚失败！Shop: %d, Listing: %d, Err: %v\n", shopEtsyID, listingID, err)
	} else {
		fmt.Printf("[Rollback] 商品入库失败，已自动删除 Etsy 远程草稿。Listing: %d\n", listingID)
	}
}

// SyncShippingProfiles 同步运费模板
func (s *ProductService) SyncShippingProfiles(shopID uint) error {
	// 1. 准备 Shop & Client
	var shop model.Shop
	if err := s.ShopRepo.db.Preload("Proxy").Preload("Developer").First(&shop, shopID).Error; err != nil {
		return err
	}
	client := NewProxiedClient(shop.Proxy)

	// 2. 请求 Etsy
	url := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%d/shipping-profiles", shop.EtsyShopID)
	var res struct {
		Results []struct {
			ShippingProfileID int64  `json:"shipping_profile_id"`
			Title             string `json:"title"`
			MinProcessingDays int    `json:"min_processing_days"`
			MaxProcessingDays int    `json:"max_processing_days"`
			OriginCountryIso  string `json:"origin_country_iso"`
		} `json:"results"`
	}

	if _, err := client.R().
		SetHeader("x-api-key", shop.Developer.APIKey).
		SetHeader("Authorization", "Bearer "+shop.AccessToken).
		SetResult(&res).
		Get(url); err != nil {
		return err
	}

	if len(res.Results) == 0 {
		return nil
	}

	// 3. 构造数据切片
	var profiles []model.ShippingProfile
	for _, item := range res.Results {
		profiles = append(profiles, model.ShippingProfile{
			ShopID:        shop.ID,
			EtsyProfileID: item.ShippingProfileID,
			Title:         item.Title,
			MinProcessing: item.MinProcessingDays,
			MaxProcessing: item.MaxProcessingDays,
			OriginCountry: item.OriginCountryIso,
		})
	}

	// 4. 批量 Upsert (核心逻辑)
	// 依赖于 idx_shop_profile 唯一索引
	err := s.ShopRepo.db.Clauses(clause.OnConflict{
		// 冲突列 (根据哪个字段判断重复)
		Columns: []clause.Column{{Name: "shop_id"}, {Name: "etsy_profile_id"}},

		// 冲突时更新哪些字段 (Do Updates)
		// 注意：不要更新 CreatedAt
		DoUpdates: clause.AssignmentColumns([]string{
			"title",
			"min_processing",
			"max_processing",
			"origin_country",
			"updated_at",
		}),
	}).Create(&profiles).Error // 直接传切片，GORM 会自动批量插入

	if err != nil {
		return fmt.Errorf("同步运费模板失败: %v", err)
	}

	return nil
}
*/
