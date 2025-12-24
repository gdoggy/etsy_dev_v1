package model

import (
	"github.com/lib/pq"
	"gorm.io/datatypes"
)

type Product struct {
	// --- ERP 内部管理字段 ---
	BaseModel
	AuditMixin
	ShopID     int64  `gorm:"index:idx_shop_state;not null"` // 店铺 ID (多店铺隔离核心)
	Shop       *Shop  `gorm:"foreignKey:ShopID"`
	LocalSKU   string `gorm:"size:100;index"`  // ERP 内部管理的 SKU
	SyncStatus int    `gorm:"default:0;index"` // 0:已同步, 1:待更新, 2:失败

	// --- Etsy 核心身份字段 ---
	ListingID int64 `gorm:"uniqueIndex;not null"` // Etsy 侧唯一 ID
	UserID    int64 `gorm:"index;not null"`       // 补充 index

	// --- 商品基本信息 ---
	Title       string `gorm:"size:255;index:idx_title_search,type:GIN,expression:title gin_trgm_ops"`
	Description string `gorm:"type:text;"`
	Url         string `gorm:"size:255;"`
	State       string `gorm:"size:20;index:idx_shop_state"` // active, inactive

	// --- 价格与数量 ---
	PriceAmount  int64  `gorm:"default:0"`
	PriceDivisor int64  `gorm:"default:0"`
	CurrencyCode string `gorm:"size:5;index;"`
	Quantity     int    `gorm:"default:0"`

	// --- 数组/标签类数据 (Postgres Array) ---
	Tags      pq.StringArray `gorm:"type:text[]"`
	Materials pq.StringArray `gorm:"type:text[]"`
	Styles    pq.StringArray `gorm:"type:text[]"`

	// --- 时间戳 ---
	EtsyCreationTS     int64 `gorm:"index"`
	EtsyEndingTS       int64 `gorm:"index"`
	EtsyLastModifiedTS int64 `gorm:"default:0"`
	EtsyStateTS        int64 `gorm:"default:0"`

	// --- 分类与属性 ---
	TaxonomyID    int64  `gorm:"default:0"`
	ShopSectionID int64  `gorm:"index;default:0"`
	ListingType   string `gorm:"size:50;"`

	// --- 设置与开关 ---
	ShippingProfileID int64 `gorm:"default:0"`
	ReturnPolicyID    int64 `gorm:"default:0"`
	// Boolean 必须给默认值 false
	IsPersonalizable bool   `gorm:"default:false"`
	ShouldAutoRenew  bool   `gorm:"default:false"`
	HasVariations    bool   `gorm:"default:false"`
	Language         string `gorm:"size:20;"`

	// --- 变体控制数组 ---
	PriceOnProperty    pq.Int64Array `gorm:"type:bigint[]"`
	QuantityOnProperty pq.Int64Array `gorm:"type:bigint[]"`
	SkuOnProperty      pq.Int64Array `gorm:"type:bigint[]"`

	// --- 关联关系 ---
	Variants []ProductVariant `gorm:"foreignKey:ProductID"`
	Images   []ProductImage   `gorm:"foreignKey:ProductID"`

	// --- AI 处理上下文 ---
	SourceMaterial string         `gorm:"type:text"`
	AiContext      datatypes.JSON `gorm:"type:jsonb"`
	LockedFields   pq.StringArray `gorm:"type:text[]"`
	EditStatus     int            `gorm:"default:0;index"`
}

func (Product) TableName() string {
	return "products"
}

type ProductVariant struct {
	BaseModel
	// --- 关联 ---
	ProductID int64    `gorm:"index;not null"`
	Product   *Product `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ShopID    int64    `gorm:"index"`
	// --- Etsy 身份标识 ---
	EtsyVariantID int64 `gorm:"index"`
	// --- 规格组合 ---
	Properties   datatypes.JSON `gorm:"type:jsonb"` // {"Color":"Red"}
	EtsyRawProps datatypes.JSON `gorm:"type:jsonb"` // 原始数据
	// --- 销售数据 ---
	EtsyOfferingID int64  `gorm:"default:0"`
	PriceAmount    int64  `gorm:"default:0"`
	PriceDivisor   int64  `gorm:"default:0"`
	CurrencyCode   string `gorm:"size:5;"`
	Quantity       int    `gorm:"default:0"`
	IsEnabled      bool   `gorm:"default:true"`
	// --- SKU ---
	LocalSKU string `gorm:"size:100;index"`
	EtsySKU  string `gorm:"size:100;index"`
}

func (ProductVariant) TableName() string {
	return "product_variants"
}

type ProductImage struct {
	BaseModel

	// --- 关联关系 ---
	ProductID int64    `gorm:"index;not null"`
	Product   *Product `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	// --- 资源地址 ---
	LocalPath string `gorm:"size:255;"`
	Url       string `gorm:"size:512"`

	// --- Etsy 同步信息 ---
	EtsyImageID int64 `gorm:"index"`
	Rank        int   `gorm:"default:99"`

	// --- 图片元数据 ---
	HexCode string `gorm:"size:10"`
	Height  int    `gorm:"default:0"`
	Width   int    `gorm:"default:0"`

	// --- 业务标记 ---
	IsAiGenerated bool `gorm:"default:false"`
}

func (*ProductImage) TableName() string {
	return "product_images"
}
