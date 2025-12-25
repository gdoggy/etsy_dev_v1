package model

import (
	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// ProductState 商品状态枚举
type ProductState string

const (
	ProductStateDraft    ProductState = "draft"
	ProductStateActive   ProductState = "active"
	ProductStateInactive ProductState = "inactive"
	ProductStateSoldOut  ProductState = "sold_out"
	ProductStateExpired  ProductState = "expired"
	ProductStateRemoved  ProductState = "removed" // ERP 本地删除标记
)

// ProductSyncStatus 同步状态
type ProductSyncStatus int

const (
	ProductSyncStatusSynced  ProductSyncStatus = 0 // 已同步
	ProductSyncStatusPending ProductSyncStatus = 1 // 待推送
	ProductSyncStatusFailed  ProductSyncStatus = 2 // 同步失败
	ProductSyncStatusLocal   ProductSyncStatus = 3 // 仅本地(AI草稿未上传)
)

// ProductEditStatus 编辑状态(AI流程)
type ProductEditStatus int

const (
	EditStatusNone      ProductEditStatus = 0 // 无AI介入
	EditStatusAIDraft   ProductEditStatus = 1 // AI生成草稿
	EditStatusReviewing ProductEditStatus = 2 // 待审核
	EditStatusApproved  ProductEditStatus = 3 // 已审核通过
	EditStatusRejected  ProductEditStatus = 4 // 已拒绝
)

type Product struct {
	BaseModel

	// --- ERP 内部管理字段 ---
	ShopID     int64  `gorm:"index:idx_shop_state;not null;comment:店铺ID"`
	Shop       *Shop  `gorm:"foreignKey:ShopID"`
	LocalSKU   string `gorm:"size:100;index;comment:ERP内部SKU"`
	SyncStatus int    `gorm:"default:0;index;comment:同步状态 0已同步 1待推送 2失败 3仅本地"`
	SyncError  string `gorm:"size:500;comment:最近同步错误信息"`

	// --- Etsy 核心身份字段 ---
	ListingID int64 `gorm:"uniqueIndex;comment:Etsy listing_id"` // 未上传时为0
	UserID    int64 `gorm:"index;comment:Etsy user_id"`

	// --- Etsy 必填字段 (createDraftListing) ---
	Title             string `gorm:"size:140;index;comment:标题(max 140)"`
	Description       string `gorm:"type:text;comment:描述"`
	Quantity          int    `gorm:"default:1;comment:库存数量"`
	TaxonomyID        int64  `gorm:"not null;comment:分类ID"`
	ShippingProfileID int64  `gorm:"not null;comment:运费模板ID"`
	ReturnPolicyID    int64  `gorm:"comment:退货政策ID"`
	WhoMade           string `gorm:"size:50;default:i_did;comment:制作者 i_did/someone_else/collective"`
	WhenMade          string `gorm:"size:50;default:made_to_order;comment:制作时间"`
	IsSupply          bool   `gorm:"default:false;comment:是否为原材料/工具"`

	// --- 价格 (Etsy 使用 Money 对象) ---
	PriceAmount  int64  `gorm:"default:0;comment:价格(分)"`
	PriceDivisor int64  `gorm:"default:100;comment:价格除数"`
	CurrencyCode string `gorm:"size:5;default:USD;comment:货币代码"`

	// --- 商品属性 ---
	State                       ProductState `gorm:"size:20;index:idx_shop_state;default:draft;comment:状态"`
	Url                         string       `gorm:"size:255;comment:Etsy商品链接"`
	ListingType                 string       `gorm:"size:50;default:physical;comment:类型 physical/download"`
	IsPersonalizable            bool         `gorm:"default:false;comment:是否可定制"`
	PersonalizationIsRequired   bool         `gorm:"default:false;comment:定制是否必填"`
	PersonalizationCharCountMax int          `gorm:"default:0;comment:定制字符上限"`
	PersonalizationInstructions string       `gorm:"size:255;comment:定制说明"`
	ShouldAutoRenew             bool         `gorm:"default:true;comment:是否自动续期"`
	IsCustomizable              bool         `gorm:"default:false;comment:是否可定制(旧字段)"`
	IsTaxable                   bool         `gorm:"default:false;comment:是否征税"`
	HasVariations               bool         `gorm:"default:false;comment:是否有变体"`
	Language                    string       `gorm:"size:10;default:en;comment:语言"`

	// --- 数组/标签类数据 ---
	Tags      pq.StringArray `gorm:"type:text[];comment:标签(max 13个)"`
	Materials pq.StringArray `gorm:"type:text[];comment:材质(max 13个)"`
	Styles    pq.StringArray `gorm:"type:text[];comment:风格(max 2个)"`

	// --- 物理属性 (实物商品必填) ---
	ItemWeight         float64 `gorm:"default:0;comment:重量"`
	ItemWeightUnit     string  `gorm:"size:10;default:oz;comment:重量单位 oz/lb/g/kg"`
	ItemLength         float64 `gorm:"default:0;comment:长度"`
	ItemWidth          float64 `gorm:"default:0;comment:宽度"`
	ItemHeight         float64 `gorm:"default:0;comment:高度"`
	ItemDimensionsUnit string  `gorm:"size:10;default:in;comment:尺寸单位 in/ft/mm/cm/m"`

	// --- 分类与分区 ---
	ShopSectionID int64 `gorm:"index;default:0;comment:店铺分区ID"`

	// --- Etsy 时间戳 ---
	EtsyCreationTS     int64 `gorm:"index;comment:Etsy创建时间戳"`
	EtsyEndingTS       int64 `gorm:"index;comment:Etsy下架时间戳"`
	EtsyLastModifiedTS int64 `gorm:"default:0;comment:Etsy最后修改时间戳"`
	EtsyStateTS        int64 `gorm:"default:0;comment:Etsy状态变更时间戳"`

	// --- 统计数据 (只读,从Etsy同步) ---
	Views       int `gorm:"default:0;comment:浏览数"`
	NumFavorers int `gorm:"default:0;comment:收藏数"`

	// --- 变体控制数组 ---
	PriceOnProperty    pq.Int64Array `gorm:"type:bigint[];comment:影响价格的属性ID"`
	QuantityOnProperty pq.Int64Array `gorm:"type:bigint[];comment:影响库存的属性ID"`
	SkuOnProperty      pq.Int64Array `gorm:"type:bigint[];comment:影响SKU的属性ID"`

	// --- 关联关系 ---
	Variants []ProductVariant `gorm:"foreignKey:ProductID"`
	Images   []ProductImage   `gorm:"foreignKey:ProductID"`

	// --- AI 处理上下文 ---
	SourceMaterial string            `gorm:"type:text;comment:AI生成来源素材"`
	AiContext      datatypes.JSON    `gorm:"type:jsonb;comment:AI上下文"`
	LockedFields   pq.StringArray    `gorm:"type:text[];comment:锁定不可AI修改的字段"`
	EditStatus     ProductEditStatus `gorm:"default:0;index;comment:编辑状态"`
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
	EtsyProductID  int64 `gorm:"index;comment:Etsy product_id"`
	EtsyOfferingID int64 `gorm:"index;comment:Etsy offering_id"`

	// --- 规格组合 ---
	PropertyValues datatypes.JSON `gorm:"type:jsonb;comment:属性值组合"`
	EtsyRawProps   datatypes.JSON `gorm:"type:jsonb;comment:Etsy原始属性数据"`

	// --- 销售数据 ---
	PriceAmount  int64  `gorm:"default:0;comment:价格(分)"`
	PriceDivisor int64  `gorm:"default:100"`
	CurrencyCode string `gorm:"size:5;default:USD"`
	Quantity     int    `gorm:"default:0"`
	IsEnabled    bool   `gorm:"default:true"`

	// --- SKU ---
	LocalSKU string `gorm:"size:100;index;comment:ERP内部SKU"`
	EtsySKU  string `gorm:"size:100;index;comment:Etsy SKU"`
}

func (ProductVariant) TableName() string {
	return "product_variants"
}

type ProductImage struct {
	BaseModel

	// --- 关联关系 ---
	ProductID int64    `gorm:"index;not null"`
	Product   *Product `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	// --- Etsy 同步信息 ---
	EtsyImageID int64 `gorm:"index;comment:Etsy image_id"`
	Rank        int   `gorm:"default:99;comment:排序(1为主图)"`

	// --- 资源地址 ---
	LocalPath string `gorm:"size:255;comment:本地存储路径"`
	EtsyUrl   string `gorm:"size:512;comment:Etsy CDN地址"`

	// --- 图片元数据 ---
	AltText string `gorm:"size:250;comment:替代文本"`
	HexCode string `gorm:"size:10;comment:主色调"`
	Height  int    `gorm:"default:0"`
	Width   int    `gorm:"default:0"`

	// --- 业务标记 ---
	IsAiGenerated bool   `gorm:"default:false;comment:是否AI生成"`
	SyncStatus    int    `gorm:"default:0;comment:同步状态"`
	SyncError     string `gorm:"size:255;comment:同步错误"`
}

func (*ProductImage) TableName() string {
	return "product_images"
}
