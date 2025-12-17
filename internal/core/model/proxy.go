package model

import (
	"fmt"
)

type Proxy struct {
	BaseModel

	// 1. 基础配置
	IP       string `gorm:"size:100;not null;index"` // IP 必须索引，防重复录入
	Port     string `gorm:"size:10;not null"`        // String 类型兼容性更好
	Username string `gorm:"size:100"`
	Password string `gorm:"size:255"`
	Protocol string `gorm:"size:10;default:'http'"` // http, https, socks5

	// 2. 状态管理
	// status: 1.正常 2.过期 3.阻塞 4.其他
	Status int `gorm:"default:1;index"`

	// 3. 资源分配策略
	Region string `gorm:"size:20;default:'US';index"`

	// 容量类型：1:独享(Private) 2:共享(Shared)
	Capacity int `gorm:"default:2"`

	// 软删除外的业务开关
	IsActive bool `gorm:"default:true"`

	// 4. 关联关系
	Developers []Developer `gorm:"foreignKey:ProxyID"`
	// 一个代理可以给多个店铺使用（取决于 Capacity）
	Shops []Shop `gorm:"foreignKey:ProxyID"`
}

// ProxyURL 生成代理字符串，例如: "http://127.0.0.1:7890"
func (p *Proxy) ProxyURL() string {
	if p.IP == "" || p.Port == "" {
		return ""
	}

	protocol := p.Protocol
	if protocol == "" {
		protocol = "http"
	}

	// 格式化输出
	if p.Username != "" {
		return fmt.Sprintf("%s://%s:%s@%s:%s", protocol, p.Username, p.Password, p.IP, p.Port)
	}
	return fmt.Sprintf("%s://%s:%s", protocol, p.IP, p.Port)
}

func (*Proxy) TableName() string {
	return "proxies"
}
