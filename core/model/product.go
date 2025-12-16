package model

import (
	"gorm.io/gorm"
)

type Product struct {
	gorm.Model

	// --- 归属关系 ---
	ShopID uint `gorm:"not null;index" json:"shop_id"`
	// 这里不需要 json tag，因为通常 API 返回时不嵌套整个 Shop 对象，防止循环引用
	Shop Shop `gorm:"foreignKey:ShopID" json:"-"`

	// --- 核心标识 ---
	ListingID int64 `gorm:"uniqueIndex;not null" json:"listing_id"` // Etsy 的唯一商品ID
	UserID    int64 `gorm:"index;default:0" json:"user_id"`         // 对应 Etsy 的 user_id

	// --- 基本信息 ---
	Title       string `gorm:"size:255;not null" json:"title"`
	Description string `gorm:"type:text" json:"description"`
	Url         string `gorm:"type:text" json:"url"`
	State       string `gorm:"size:50;index;default:'active'" json:"state"` // active, draft, sold_out

	// --- 必填枚举 (用于草稿回显) ---
	// 如果不存这三个，前端编辑草稿时就不知道用户之前选了什么
	WhoMade  string `gorm:"size:50" json:"who_made"`                // i_did, someone_else
	WhenMade string `gorm:"size:50" json:"when_made"`               // made_to_order, 2020_2025
	Type     string `gorm:"size:20;default:'physical'" json:"type"` // physical, download

	// --- 价格体系 ---
	// 建议设置 default:0，防止计算空指针
	PriceAmount  int    `gorm:"default:0" json:"price_amount"`
	PriceDivisor int    `gorm:"default:100" json:"price_divisor"`
	CurrencyCode string `gorm:"size:10;default:'USD'" json:"currency_code"`

	// --- 库存与统计 ---
	Quantity    int `gorm:"default:0" json:"quantity"`
	Views       int `gorm:"default:0" json:"views"`
	NumFavorers int `gorm:"default:0" json:"num_favorers"`

	// --- 复杂结构 (JSON) ---
	// Tags: 自动序列化为 JSON 字符串存储
	Tags []string `gorm:"serializer:json" json:"tags"`
	// ImageIDs: 图片ID列表，方便知道这个草稿关联了哪些图
	ImageIDs []int64 `gorm:"serializer:json" json:"image_ids"`

	// --- 关联 ID ---
	TaxonomyID        int64 `gorm:"index;default:0" json:"taxonomy_id"`
	ShippingProfileID int64 `gorm:"index;default:0" json:"shipping_profile_id"`

	// --- 时间戳 (Etsy 原始数据) ---
	CreationTsz     int64 `gorm:"default:0" json:"creation_tsz"`
	LastModifiedTsz int64 `gorm:"default:0" json:"last_modified_tsz"`
	EndingTsz       int64 `gorm:"default:0" json:"ending_tsz"`
}

// ProductVariant 商品变体
// 对应 Etsy 的 Inventory 概念，处理如 "Red/L", "Blue/M" 的组合
type ProductVariant struct {
	gorm.Model

	ProductID uint    `gorm:"index;not null" json:"product_id"`
	Product   Product `gorm:"foreignKey:ProductID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`

	// --- Etsy 标识 ---
	EtsyVariantID int64 `gorm:"index;default:0" json:"etsy_variant_id"`

	// --- 核心属性 ---
	// 组合属性，推荐存储 JSON 字符串，例如: [{"property_id":200, "value_id":1, "name":"Color", "value":"Red"}]
	// 这样设计比单纯存 "Red" 更符合 Etsy API V3 的规范
	PropertyValues string `gorm:"type:text;serializer:json" json:"property_values"`

	// --- 销售数据 ---
	Price    float64 `gorm:"not null" json:"price"`
	Quantity int     `gorm:"not null" json:"quantity"`
	Sku      string  `gorm:"size:100;index" json:"sku"` // SKU 加上索引方便查询

	// --- 状态 ---
	IsEnabled bool `gorm:"default:true" json:"is_enabled"`
}
