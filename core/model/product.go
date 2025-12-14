package model

import (
	"gorm.io/gorm"
)

type Product struct {
	gorm.Model

	// --- 归属关系 ---
	ShopID uint `gorm:"not null;index"`    // 属于哪个店铺
	Shop   Shop `gorm:"foreignKey:ShopID"` // 预加载店铺信息

	// --- Etsy 原生数据 ---
	ListingID int64  `gorm:"uniqueIndex;not null"` // Etsy 的商品 ID (防重关键)
	Title     string `gorm:"size:255"`
	State     string `gorm:"size:20"` // active, inactive, sold_out

	// --- 价格与库存 ---
	// Etsy 返回的是 amount(分) 和 divisor(除数)。
	// 建议存 amount，显示时再除。
	PriceAmount  int
	PriceDivisor int
	Currency     string `gorm:"size:10"`

	Quantity int

	// --- 链接 ---
	Url string `gorm:"type:text"`

	// --- 本地业务扩展字段 (未来用) ---
	// IsSync      bool // 是否开启库存同步
	// CostPrice   float64 // 采购成本价
}
