package model

import "gorm.io/gorm"

type Proxy struct {
	gorm.Model

	// 基础配置
	IP       string `gorm:"not null;size:50;index"`
	Port     string `gorm:"not null;size:10"`
	Username string `gorm:"size:100"`
	Password string `gorm:"size:255"`
	Protocol string `gorm:"size:100;default:'http'"` // http/socks

	// 资源管理
	Region   string `gorm:"size:20;default:'US'"`
	Capacity int    `gorm:"default:2"` // 容量：1独享；2复用
	IsActive bool   `gorm:"default:true"`

	// 关联关系
	Developers []Developer `gorm:"foreignkey:ProxyID"`
	Shops      []Shop      `gorm:"foreignkey:ProxyID"`
}
