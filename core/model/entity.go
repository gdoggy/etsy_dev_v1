package model

import (
	"time"

	"gorm.io/gorm"
)

// Adapter 对应开发账号
type Adapter struct {
	ID         uint   `gorm:"primaryKey"`
	Name       string `gorm:"unique;not null;size:50"`
	ProxyURL   string `gorm:"not null;size:100"`
	EtsyAppKey string `gorm:"not null;size:100"` // Client ID
	Status     int    `gorm:"default:1"`         // 1:启用, 0:停用

	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Shop  对应 Etsy 店铺
type Shop struct {
	ID             uint   `gorm:"primaryKey"`
	AdapterID      uint   `gorm:"index;not null"`
	EtsyShopID     string `gorm:"unique;size:50"`
	EtsyUserID     string `gorm:"size:50"`
	ShopName       string `gorm:"size:100"`
	AccessToken    string `gorm:"type:text"`
	RefreshToken   string `gorm:"type:text"`
	TokenExpiresAt time.Time

	// 关联关系 (可选，方便查询时连表)
	Adapter   Adapter `gorm:"foreignKey:AdapterID"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
