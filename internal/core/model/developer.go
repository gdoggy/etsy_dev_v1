package model

type Developer struct {
	BaseModel
	AuditMixin
	// 1. 账号基础信息 (登录 Etsy 开发者后台用)
	Name       string `gorm:"size:50"` // 备注名称，如 "开发者账号A"
	LoginEmail string `gorm:"uniqueIndex;size:100;not null"`
	LoginPwd   string `gorm:"size:255;not null"`

	// status: 1.正常 2.异常(被封号)
	Status int `gorm:"default:1;index"`

	// 2. API 凭证 (核心资产)
	APIKey       string `gorm:"size:100;index;"`
	SharedSecret string `gorm:"size:100"`

	// 3. 关联关系
	// 一个开发者 Key 可以授权给多个店铺使用
	Shops []Shop `gorm:"foreignKey:DeveloperID"`
}

func (Developer) TableName() string {
	return "developers"
}
