package model

// Developer 状态常量
const (
	DeveloperStatusPending = 0 // 未配置（用户未回填 CallbackURL 到 Etsy）
	DeveloperStatusActive  = 1 // 正常启用
	DeveloperStatusBanned  = 2 // 异常/封禁
)

type DomainPool struct {
	BaseModel
	AuditMixin
	Host       string      `gorm:"size:255;unique"`
	IsActive   bool        `gorm:"default:true"`
	Developers []Developer `gorm:"foreignkey:DomainPoolID"`
}
type Developer struct {
	BaseModel
	AuditMixin
	// 1. 账号基础信息 (登录 Etsy 开发者后台用)
	Name       string `gorm:"size:50"` // 备注名称，如 "开发者账号A"
	LoginEmail string `gorm:"uniqueIndex;size:100;not null"`
	LoginPwd   string `gorm:"size:255;not null"`

	// 状态管理: 0.未配置 1.正常启用 2.异常(被封)
	Status int `gorm:"default:0;index"`

	// 2. API 凭证 (核心资产)
	ApiKey       string `gorm:"size:100;index;"`
	SharedSecret string `gorm:"size:100"`
	// 防关联
	DomainPoolID int64       `gorm:"index"`
	DomainPoll   *DomainPool `gorm:"foreignkey:DomainPoolID"`
	SubDomain    string      `gorm:"size:50"`
	CallbackPath string      `gorm:"size:50"`
	CallbackURL  string      `gorm:"size:255"`
	// 3. 关联关系
	// 一个开发者 Key 可以授权给多个店铺使用
	Shops []Shop `gorm:"foreignKey:DeveloperID"`
}

func (*Developer) TableName() string {
	return "developers"
}
