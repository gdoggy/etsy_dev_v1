package model

import (
	"time"

	"gorm.io/gorm"
)

type Shop struct {
	gorm.Model
	// 基础信息
	// shopID 店铺身份
	EtsyShopID string `gorm:"unique;size:50"`
	// userID 店主/用户 ID
	EtsyUserID string `gorm:"size:50"`
	ShopName   string `gorm:"size:100"`

	// 代理关系
	ProxyID uint  `gorm:"not null;index"`
	Proxy   Proxy `gorm:"foreignkey:ProxyID"`
	// 开发者账号关系
	DeveloperID *uint     `gorm:"index"`
	Developer   Developer `gorm:"foreignkey:DeveloperID"`

	// API Token
	AccessToken    string `gorm:"type:text"`
	RefreshToken   string `gorm:"type:text"`
	TokenExpiresAt time.Time

	// 关联关系

	// 1. 账号数据
	// 含义：每个 shop 账号拥有 1 个 shop account
	ShopAccount *ShopAccount `gorm:"foreignKey:ShopID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	// 2. 订单数据
	// 含义：orders 每个 shop 对应多个 order 订单
	// Orders []Order `gorm:"foreignKey:ShopID"`

	// 3. 财务数据
	// 含义：每个 shop 对应多个 finance 资金条目
	// Finances []Finance `gorm:"foreignKey:ShopID"`
}

type ShopAccount struct {
	gorm.Model
	ShopID        uint   `gorm:"uniqueIndex;not null"`
	LoginEmail    string `gorm:"size:100"`
	LoginPwd      string `gorm:"size:255"`
	RecoveryEmail string `gorm:"size:100"` // 辅助邮箱
	TwoFASecret   string `gorm:"size:100"` // 2FA 密钥 (用于自动生成验证码)

	// --- 指纹环境 ---
	UserAgent string `gorm:"type:text"`
	Cookies   string `gorm:"type:text"`

	// 备注
	Note string `gorm:"type:text"`
}
