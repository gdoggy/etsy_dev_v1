package model

import "gorm.io/gorm"

type Developer struct {
	gorm.Model
	// 基础信息
	Name       string `gorm:"unique;size:50;not null"`
	LoingEmail string `gorm:"size:100"`
	LoingPwd   string `gorm:"size:255"`
	Status     int    `gorm:"default:1;not null"` // 1.正常 2.异常

	// API 申请信息
	AppKey string `gorm:"size:100;not null"`
	Secret string `gorm:"size:100;not null"`

	// 代理信息，必须属于某个 Proxy
	ProxyID uint `gorm:"index;not null"`

	// 关联关系
	Shops []Shop `gorm:"foreignkey:DeveloperID"`
}
