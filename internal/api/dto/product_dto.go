package dto

// ==================== 请求 DTO ====================

// CreateProductReq AI生成草稿 / 手动创建请求
type CreateProductReq struct {
	ShopID int64 `json:"shop_id" binding:"required"`

	// 基础信息
	Title       string   `json:"title" binding:"required,max=140"`
	Description string   `json:"description" binding:"required"`
	Tags        []string `json:"tags" binding:"max=13"`
	Materials   []string `json:"materials" binding:"max=13"`
	Styles      []string `json:"styles" binding:"max=2"`

	// 价格与库存
	Price    float64 `json:"price" binding:"required,gt=0"` // 前端传小数, 后端转分
	Currency string  `json:"currency"`                      // 默认 USD
	Quantity int     `json:"quantity" binding:"required,gte=1"`

	// 分类与运费
	TaxonomyID        int64 `json:"taxonomy_id" binding:"required"`
	ShippingProfileID int64 `json:"shipping_profile_id" binding:"required"`
	ReturnPolicyID    int64 `json:"return_policy_id"`
	ShopSectionID     int64 `json:"shop_section_id"`

	// Etsy 必填
	WhoMade  string `json:"who_made"`  // i_did, someone_else, collective
	WhenMade string `json:"when_made"` // made_to_order, 2020_2024, ...
	IsSupply bool   `json:"is_supply"`

	// 物理属性
	ItemWeight         float64 `json:"item_weight"`
	ItemWeightUnit     string  `json:"item_weight_unit"` // oz, lb, g, kg
	ItemLength         float64 `json:"item_length"`
	ItemWidth          float64 `json:"item_width"`
	ItemHeight         float64 `json:"item_height"`
	ItemDimensionsUnit string  `json:"item_dimensions_unit"` // in, ft, mm, cm, m

	// 定制选项
	IsPersonalizable            bool   `json:"is_personalizable"`
	PersonalizationIsRequired   bool   `json:"personalization_is_required"`
	PersonalizationCharCountMax int    `json:"personalization_char_count_max"`
	PersonalizationInstructions string `json:"personalization_instructions"`

	// 其他
	ListingType     string `json:"listing_type"` // physical, download
	ShouldAutoRenew bool   `json:"should_auto_renew"`
	IsTaxable       bool   `json:"is_taxable"`

	// 图片 (已上传到 Etsy 的 image_id)
	ImageIDs []int64 `json:"image_ids"`

	// AI 生成来源
	SourceMaterial string `json:"source_material"`
}

// UpdateProductReq 更新商品请求
type UpdateProductReq struct {
	ID int64 `json:"id" binding:"required"`

	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Materials   []string `json:"materials,omitempty"`
	Styles      []string `json:"styles,omitempty"`

	Price    *float64 `json:"price,omitempty"`
	Quantity *int     `json:"quantity,omitempty"`

	TaxonomyID        *int64 `json:"taxonomy_id,omitempty"`
	ShippingProfileID *int64 `json:"shipping_profile_id,omitempty"`
	ReturnPolicyID    *int64 `json:"return_policy_id,omitempty"`
	ShopSectionID     *int64 `json:"shop_section_id,omitempty"`

	WhoMade  *string `json:"who_made,omitempty"`
	WhenMade *string `json:"when_made,omitempty"`
	IsSupply *bool   `json:"is_supply,omitempty"`

	ItemWeight         *float64 `json:"item_weight,omitempty"`
	ItemWeightUnit     *string  `json:"item_weight_unit,omitempty"`
	ItemLength         *float64 `json:"item_length,omitempty"`
	ItemWidth          *float64 `json:"item_width,omitempty"`
	ItemHeight         *float64 `json:"item_height,omitempty"`
	ItemDimensionsUnit *string  `json:"item_dimensions_unit,omitempty"`

	IsPersonalizable            *bool   `json:"is_personalizable,omitempty"`
	PersonalizationIsRequired   *bool   `json:"personalization_is_required,omitempty"`
	PersonalizationCharCountMax *int    `json:"personalization_char_count_max,omitempty"`
	PersonalizationInstructions *string `json:"personalization_instructions,omitempty"`

	ShouldAutoRenew *bool `json:"should_auto_renew,omitempty"`
	IsTaxable       *bool `json:"is_taxable,omitempty"`
}

// PublishProductReq 上架请求
type PublishProductReq struct {
	ID int64 `json:"id" binding:"required"`
}

// SyncProductsReq 同步商品请求
type SyncProductsReq struct {
	ShopID int64 `json:"shop_id" binding:"required"`
}

// AIGenerateReq AI生成草稿请求
type AIGenerateReq struct {
	ShopID         int64  `json:"shop_id" binding:"required"`
	SourceMaterial string `json:"source_material" binding:"required"` // 原始素材/描述
	TargetCategory int64  `json:"target_category"`                    // 目标分类
	StyleHint      string `json:"style_hint"`                         // 风格提示
}

// ProductListReq 商品列表请求
type ProductListReq struct {
	ShopID   int64  `form:"shop_id" binding:"required"`
	State    string `form:"state"`     // draft, active, inactive...
	Keyword  string `form:"keyword"`   // 标题搜索
	Page     int    `form:"page"`      // 默认1
	PageSize int    `form:"page_size"` // 默认20
}

// ==================== 响应 DTO ====================

// ProductResp 商品详情响应
type ProductResp struct {
	ID        int64 `json:"id"`
	ListingID int64 `json:"listing_id"`
	ShopID    int64 `json:"shop_id"`

	// 基础信息
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Materials   []string `json:"materials"`
	Styles      []string `json:"styles"`
	Url         string   `json:"url"`

	// 价格
	Price        float64 `json:"price"` // 转换后的小数价格
	CurrencyCode string  `json:"currency_code"`
	Quantity     int     `json:"quantity"`

	// 状态
	State      string `json:"state"`
	SyncStatus int    `json:"sync_status"`
	SyncError  string `json:"sync_error,omitempty"`
	EditStatus int    `json:"edit_status"`

	// 分类
	TaxonomyID        int64 `json:"taxonomy_id"`
	ShippingProfileID int64 `json:"shipping_profile_id"`
	ReturnPolicyID    int64 `json:"return_policy_id"`
	ShopSectionID     int64 `json:"shop_section_id"`

	// Etsy 必填
	WhoMade  string `json:"who_made"`
	WhenMade string `json:"when_made"`
	IsSupply bool   `json:"is_supply"`

	// 物理属性
	ItemWeight         float64 `json:"item_weight"`
	ItemWeightUnit     string  `json:"item_weight_unit"`
	ItemLength         float64 `json:"item_length"`
	ItemWidth          float64 `json:"item_width"`
	ItemHeight         float64 `json:"item_height"`
	ItemDimensionsUnit string  `json:"item_dimensions_unit"`

	// 统计
	Views       int `json:"views"`
	NumFavorers int `json:"num_favorers"`

	// 关联
	Images   []ProductImageResp   `json:"images"`
	Variants []ProductVariantResp `json:"variants,omitempty"`

	// 时间
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ProductListResp 商品列表响应
type ProductListResp struct {
	Code     int           `json:"code"`
	Message  string        `json:"message"`
	Data     []ProductResp `json:"data"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// ProductImageResp 图片响应
type ProductImageResp struct {
	ID          int64  `json:"id"`
	EtsyImageID int64  `json:"etsy_image_id"`
	Url         string `json:"url"`
	LocalPath   string `json:"local_path,omitempty"`
	Rank        int    `json:"rank"`
	AltText     string `json:"alt_text"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// ProductVariantResp 变体响应
type ProductVariantResp struct {
	ID             int64                  `json:"id"`
	EtsyProductID  int64                  `json:"etsy_product_id"`
	EtsyOfferingID int64                  `json:"etsy_offering_id"`
	PropertyValues map[string]interface{} `json:"property_values"`
	Price          float64                `json:"price"`
	Quantity       int                    `json:"quantity"`
	LocalSKU       string                 `json:"local_sku"`
	EtsySKU        string                 `json:"etsy_sku"`
	IsEnabled      bool                   `json:"is_enabled"`
}

// ProductStatsResp 商品统计响应
type ProductStatsResp struct {
	ShopID  int64            `json:"shop_id"`
	Total   int64            `json:"total"`
	ByState map[string]int64 `json:"by_state"`
	BySync  map[string]int64 `json:"by_sync"`
}
