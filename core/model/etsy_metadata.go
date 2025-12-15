package model

import "time"

// ShippingProfile 运费模板表
type ShippingProfile struct {
	ID uint `gorm:"primarykey"`
	// 设置联合唯一索引 idx_shop_profile
	ShopID        uint  `gorm:"index;uniqueIndex:idx_shop_profile"`
	EtsyProfileID int64 `gorm:"index;uniqueIndex:idx_shop_profile"`

	Title         string `gorm:"size:255"`
	MinProcessing int    // 最小处理天数
	MaxProcessing int    // 最大处理天数
	OriginCountry string `gorm:"size:10"` // 发货国家 ISO 代码

	UpdatedAt time.Time
}

// TaxonomyNode 分类树表 (简化版，只存常用的或顶层的)
type TaxonomyNode struct {
	ID       uint   `gorm:"primarykey"`
	EtsyID   int64  `gorm:"uniqueIndex"`
	Name     string `gorm:"size:255"`
	ParentID int64  `gorm:"index"` // 父节点 ID
	Level    int    // 层级
	FullPath string // 如 "Home & Living > Bedding"
}
