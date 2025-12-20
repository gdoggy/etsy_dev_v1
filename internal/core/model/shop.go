package model

import (
	"time"
)

// Token 状态常量
const (
	TokenStatusActive  = "active"       // 正常
	TokenStatusExpired = "expired"      // 已过期 (待刷新)
	TokenStatusInvalid = "auth_invalid" // 彻底失效 (需要人工重新授权)
)

type Shop struct {
	BaseModel // 包含 ID(int64), CreatedAt, UpdatedAt
	AuditMixin
	// 1. 核心身份
	// 改名为 EtsyShopID 以区分主键 ID，且与 Product 表外键保持一致
	EtsyShopID int64  `gorm:"uniqueIndex;not null"` // 对应 Etsy 平台的 shop_id
	UserID     int64  `gorm:"index;not null"`       // 对应 Etsy 平台的 user_id
	ShopName   string `gorm:"type:varchar(100)"`
	LoginName  string `gorm:"type:varchar(100)"` // 登录名

	// 2. 运营关键指标
	ListingActiveCount   int     `gorm:"default:0"`                   // 在售数
	TransactionSoldCount int     `gorm:"default:0"`                   // 总销量
	ReviewCount          int     `gorm:"default:0"`                   // 评价数
	ReviewAverage        float64 `gorm:"type:decimal(3,1);default:0"` // 评分
	CurrencyCode         string  `gorm:"type:varchar(10)"`

	// 3. 店铺状态
	IsVacation      bool   `gorm:"default:false"`
	VacationMessage string `gorm:"type:text"`
	Url             string `gorm:"type:varchar(255)"`
	IconUrl         string `gorm:"type:varchar(255)"`

	// 4. 基础设施绑定 (外键)
	// --- 代理关系 ---
	ProxyID int64  `gorm:"index"`
	Proxy   *Proxy `gorm:"foreignKey:ProxyID"`
	Region  string `gorm:"type:text"`

	// --- 开发者账号关系 ---
	DeveloperID int64      `gorm:"index"`
	Developer   *Developer `gorm:"foreignKey:developerID"`

	// 5. API Token
	// 周期检测 token 是否过期
	TokenStatus    string    `gorm:"index;size:20;default:'active'"`
	AccessToken    string    `gorm:"type:text"`
	RefreshToken   string    `gorm:"type:text"`
	TokenExpiresAt time.Time // Token 具体的过期时间点

	// 6. 关联关系

	// 1. 账号敏感数据 (Has One)
	// 含义：每个 shop 账号拥有 1 个 shop account (登录密码/环境/2FA)
	// 使用 ShopID 作为外键关联
	ShopAccount *ShopAccount `gorm:"foreignKey:ShopID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	// 2. 商品数据 (Has Many)
	// 注意：这里 references 指向的是本表的 EtsyShopID，而不是主键 ID
	// 因为 Product 表存的是 EtsyShopID
	Products []Product `gorm:"foreignKey:ShopID;references:EtsyShopID"`

	// 3. 权限关联
	// 获取该店铺的所有成员及其角色 (Has Many)
	Memberships []ShopMember `gorm:"foreignKey:ShopID"`
	// 获取该店铺的所有成员列表 (Many to Many, 忽略角色)
	Members []SysUser `gorm:"many2many:shop_members;"`
}

// ShopAccount 存储店铺的登录凭证和指纹环境 (敏感表)
type ShopAccount struct {
	BaseModel
	// 加上 uniqueIndex 确保 1:1 关系 (一个 Shop 只能有一条 Account 记录)
	ShopID int64 `gorm:"uniqueIndex;not null"`

	LoginEmail    string `gorm:"size:100"`
	LoginPwd      string `gorm:"size:255"` // 加密
	RecoveryEmail string `gorm:"size:100"` // 辅助邮箱
	TwoFASecret   string `gorm:"size:100"` // 2FA 密钥 (OTP)

	// --- 指纹环境 ---
	UserAgent string `gorm:"type:text"`
	Cookies   string `gorm:"type:text"` // 加密

	// 备注
	Note string `gorm:"type:text"`
}

func (Shop) TableName() string {
	return "shops"
}

func (ShopAccount) TableName() string {
	return "shop_accounts"
}
