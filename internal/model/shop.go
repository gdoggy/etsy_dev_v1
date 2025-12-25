package model

import (
	"time"
)

// Shop 店铺状态常量
const (
	ShopStatusPending  = 0 // 待授权
	ShopStatusActive   = 1 // 正常
	ShopStatusInactive = 2 // 已停用
)

// Token 状态常量
const (
	TokenStatusValid   = "valid"        // 有效
	TokenStatusExpired = "expired"      // 已过期
	TokenStatusInvalid = "auth_invalid" // 需重新授权
)

type Shop struct {
	BaseModel // 包含 ID(int64), CreatedAt, UpdatedAt
	AuditMixin
	// 1. 核心身份
	// 改名为 EtsyShopID 以区分主键 ID，且与 Product 表外键保持一致
	EtsyShopID   int64  `gorm:"uniqueIndex"` // 对应 Etsy 平台的 shop_id
	EtsyUserID   int64  `gorm:"index"`       // 对应 Etsy 平台的 user_id
	ShopName     string `gorm:"size:100"`
	Title        string `gorm:"size:255;comment:店铺标题"`
	Announcement string `gorm:"type:text;comment:店铺公告"`
	Region       string `gorm:"size:20;not null;default:'IDN'"` // 重要，默认印尼！必填字段，区分账户地区以分配 proxy & developer

	// 2. 运营关键指标
	ListingActiveCount   int     `gorm:"default:0"`                   // 在售数
	TransactionSoldCount int     `gorm:"default:0"`                   // 总销量
	ReviewCount          int     `gorm:"default:0"`                   // 评价数
	ReviewAverage        float64 `gorm:"type:decimal(3,1);default:0"` // 评分
	CurrencyCode         string  `gorm:"size:20"`

	// 3. 消息设置
	SaleMessage        string `gorm:"type:text;comment:购买成功消息"`
	DigitalSaleMessage string `gorm:"type:text;comment:数字商品购买消息"`

	// 4. 店铺状态
	IsVacation      bool   `gorm:"default:false"`
	VacationMessage string `gorm:"type:text"`
	Url             string `gorm:"size:255"`
	IconUrl         string `gorm:"size:255"`

	// 5. 同步状态
	Status       int        `gorm:"default:0;comment:状态 0-待授权 1-正常 2-已停用"`
	EtsySyncedAt *time.Time `gorm:"comment:最后同步时间"`

	// 6. 基础设施绑定 (外键)
	// --- 代理关系 ---
	ProxyID int64  `gorm:"index"`
	Proxy   *Proxy `gorm:"foreignKey:ProxyID"`

	// --- 开发者账号关系 ---
	DeveloperID int64      `gorm:"index"`
	Developer   *Developer `gorm:"foreignKey:developerID"`

	// 7. API Token
	// 周期检测 token 是否过期
	TokenStatus    string    `gorm:"index;size:20;default:'auth_invalid'"`
	AccessToken    string    `gorm:"size:255"`
	RefreshToken   string    `gorm:"size:255"`
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

	// 3.
	Sections         []ShopSection     `gorm:"foreignKey:ShopID" json:"sections,omitempty"`
	ShippingProfiles []ShippingProfile `gorm:"foreignKey:ShopID" json:"shipping_profiles,omitempty"`
	ReturnPolicies   []ReturnPolicy    `gorm:"foreignKey:ShopID" json:"return_policies,omitempty"`

	// 4. 权限关联
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

// ShopSection 店铺分区模型
type ShopSection struct {
	BaseModel

	// 关联店铺
	ShopID int64 `gorm:"index;not null;comment:关联店铺ID"`
	Shop   *Shop `gorm:"foreignKey:ShopID"`

	// Etsy 分区信息
	EtsySectionID      int64  `gorm:"index;comment:Etsy分区ID"`
	Title              string `gorm:"size:255;not null;comment:分区名称"`
	Rank               int    `gorm:"default:0;comment:排序权重"`
	ActiveListingCount int    `gorm:"default:0;comment:该分区商品数"`

	// 同步时间
	EtsySyncedAt *time.Time `gorm:"comment:最后Etsy同步时间"`
}

// ShopMember 定义用户和店铺的关联关系及权限
// GORM 自定义连接表 (Join Table)
type ShopMember struct {
	BaseModel
	AuditMixin
	// 联合唯一索引
	// 确保一个用户在一个店铺里只有一条记录
	SysUserID int64 `gorm:"index;uniqueIndex:idx_user_shop;not null"`
	ShopID    int64 `gorm:"index;uniqueIndex:idx_user_shop;not null"`

	// 权限控制
	// 角色: owner, manager, editor, viewer
	Role string `gorm:"size:20;default:'viewer'"`

	// 关联对象 (Belongs To)
	SysUser *SysUser `gorm:"foreignKey:SysUserID"`
	Shop    *Shop    `gorm:"foreignKey:ShopID"`
}

func (Shop) TableName() string {
	return "shops"
}

func (ShopAccount) TableName() string {
	return "shop_accounts"
}

func (ShopSection) TableName() string {
	return "shop_sections"
}

func (ShopMember) TableName() string {
	return "shop_members"
}
