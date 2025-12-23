package model

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
	APIKey       string `gorm:"size:100;index;"`
	SharedSecret string `gorm:"size:100"`
	// 创建开发者账号后，系统生成 CallbackURL
	CallbackURL string `gorm:"size:255"`
	// 3. 关联关系
	// 一个开发者 Key 可以授权给多个店铺使用
	Shops []Shop `gorm:"foreignKey:DeveloperID"`
}

func (Developer) TableName() string {
	return "developers"
}
