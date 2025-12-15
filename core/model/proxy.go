package model

import (
	"fmt"

	"gorm.io/gorm"
)

type Proxy struct {
	gorm.Model

	// 基础配置
	IP       string `gorm:"not null;size:50;index"`
	Port     string `gorm:"not null;size:10"`
	Username string `gorm:"size:100"`
	Password string `gorm:"size:255"`
	Protocol string `gorm:"size:100;default:'http'"` // http/socks

	// 状态 -> 周期检测 ip 是否过期/阻塞
	// status: 1.正常 2.过期 3.阻塞 4.其他
	Status int `gorm:"default:1;not null"`

	// 资源管理
	Region   string `gorm:"size:20;default:'US'"`
	Capacity int    `gorm:"default:2"` // 容量：1独享；2复用
	IsActive bool   `gorm:"default:true"`

	// 关联关系
	Developers []Developer `gorm:"foreignkey:ProxyID"`
	Shops      []Shop      `gorm:"foreignkey:ProxyID"`
}

// ProxyURL 生成代理字符串，例如: "http://127.0.0.1:7890"
// 如果有账号密码，自动生成 "http://user:pass@ip:port"
func (p *Proxy) ProxyURL() string {
	if p.IP == "" || p.Port == "" {
		return ""
	}

	protocol := p.Protocol
	if protocol == "" {
		protocol = "http"
	}

	if p.Username != "" {
		return fmt.Sprintf("%s://%s:%s@%s:%s", protocol, p.Username, p.Password, p.IP, p.Port)
	}
	return fmt.Sprintf("%s://%s:%s", protocol, p.IP, p.Port)
}
