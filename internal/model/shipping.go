package model

import "time"

// TaxonomyNode 分类树表 (简化版，只存常用的或顶层的)
type TaxonomyNode struct {
	BaseModel
	EtsyID   int64  `gorm:"uniqueIndex"`
	Name     string `gorm:"size:255"`
	ParentID int64  `gorm:"index"` // 父节点 ID
	Level    int    // 层级
	FullPath string // 如 "Home & Living > Bedding"
}

// ShippingProfile 运费模板模型
type ShippingProfile struct {
	BaseModel

	// 关联店铺
	ShopID int64 `gorm:"index;not null;comment:关联店铺ID"`
	Shop   *Shop `gorm:"foreignKey:ShopID"`

	// Etsy 运费模板信息
	EtsyProfileID    int64  `gorm:"index;comment:Etsy运费模板ID"`
	Title            string `gorm:"size:255;not null;comment:模板名称"`
	OriginCountryISO string `gorm:"size:10;comment:发货国家ISO"`
	OriginPostalCode string `gorm:"size:20;comment:发货邮编"`

	// 处理时间
	ProcessingDaysMin int `gorm:"default:1;comment:最短处理天数"`
	ProcessingDaysMax int `gorm:"default:3;comment:最长处理天数"`

	// 同步时间
	EtsySyncedAt *time.Time `gorm:"comment:最后Etsy同步时间"`

	// 关联数据（一对多）
	Destinations []ShippingDestination `gorm:"foreignKey:ShippingProfileID"`
	Upgrades     []ShippingUpgrade     `gorm:"foreignKey:ShippingProfileID"`
}

// ShippingDestination 运费目的地模型
type ShippingDestination struct {
	BaseModel

	// 关联运费模板
	ShippingProfileID int64            `gorm:"index;not null;comment:关联运费模板ID"`
	ShippingProfile   *ShippingProfile `gorm:"foreignKey:ShippingProfileID"`

	// Etsy 目的地信息
	EtsyDestinationID     int64  `gorm:"index;comment:Etsy目的地ID"`
	DestinationCountryISO string `gorm:"size:10;comment:目的地国家ISO"`
	DestinationRegion     string `gorm:"size:50;comment:目的地区域"`

	// 运费（单位：分）
	PrimaryCost   int64  `gorm:"default:0;comment:首件运费(分)"`
	SecondaryCost int64  `gorm:"default:0;comment:续件运费(分)"`
	CurrencyCode  string `gorm:"size:10;default:USD;comment:货币代码"`

	// 承运商信息
	ShippingCarrierID int64  `gorm:"default:0;comment:承运商ID"`
	MailClass         string `gorm:"size:50;comment:邮寄类型"`

	// 配送时间
	DeliveryDaysMin int `gorm:"default:0;comment:最短配送天数"`
	DeliveryDaysMax int `gorm:"default:0;comment:最长配送天数"`
}

// ShippingUpgrade 类型常量
const (
	ShippingUpgradeTypeDomestic      = 0 // 国内
	ShippingUpgradeTypeInternational = 1 // 国际
)

// ShippingUpgrade 加急配送选项模型
type ShippingUpgrade struct {
	BaseModel

	// 关联运费模板
	ShippingProfileID int64            `gorm:"index;not null;comment:关联运费模板ID"`
	ShippingProfile   *ShippingProfile `gorm:"foreignKey:ShippingProfileID"`

	// Etsy 升级选项信息
	EtsyUpgradeID int64  `gorm:"index;comment:Etsy升级选项ID"`
	UpgradeName   string `gorm:"size:100;not null;comment:升级名称"`
	Type          int    `gorm:"default:0;comment:类型 0-国内 1-国际"`

	// 费用（单位：分）
	Price         int64  `gorm:"default:0;comment:价格(分)"`
	SecondaryCost int64  `gorm:"default:0;comment:续件费用(分)"`
	CurrencyCode  string `gorm:"size:10;default:USD;comment:货币代码"`

	// 承运商信息
	ShippingCarrierID int64  `gorm:"default:0;comment:承运商ID"`
	MailClass         string `gorm:"size:50;comment:邮寄类型"`

	// 配送时间
	DeliveryDaysMin int `gorm:"default:0;comment:最短配送天数"`
	DeliveryDaysMax int `gorm:"default:0;comment:最长配送天数"`
}

// ReturnPolicy 退货政策模型
type ReturnPolicy struct {
	BaseModel

	// 关联店铺
	ShopID int64 `gorm:"index;not null;comment:关联店铺ID"`
	Shop   *Shop `gorm:"foreignKey:ShopID"`

	// Etsy 退货政策信息
	EtsyPolicyID     int64 `gorm:"index;comment:Etsy退货政策ID"`
	AcceptsReturns   bool  `gorm:"default:false;comment:是否接受退货"`
	AcceptsExchanges bool  `gorm:"default:false;comment:是否接受换货"`
	ReturnDeadline   int   `gorm:"default:0;comment:退货期限(天)"`

	// 同步时间
	EtsySyncedAt *time.Time `gorm:"comment:最后Etsy同步时间"`
}

func (ShippingProfile) TableName() string {
	return "shipping_profiles"
}
func (ShippingDestination) TableName() string {
	return "shipping_destinations"
}
func (ShippingUpgrade) TableName() string {
	return "shipping_upgrades"
}
func (ReturnPolicy) TableName() string {
	return "return_policies"
}
