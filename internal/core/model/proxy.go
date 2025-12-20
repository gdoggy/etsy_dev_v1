package model

import (
	"fmt"
	"net/url"
	"time"
)

type Proxy struct {
	BaseModel
	AuditMixin
	// 1. 基础配置
	IP       string `gorm:"size:100;not null;index"` // IP 必须索引，防重复录入
	Port     string `gorm:"size:10;not null"`        // String 类型兼容性更好
	Username string `gorm:"size:100"`
	Password string `gorm:"size:255"`
	Protocol string `gorm:"size:10;default:'http'"` // http, https, socks5

	// 2. 状态管理
	// status: 1.正常 2.暂时故障 3.彻底凉凉 Task 不再巡检，人工介入
	Status        int `gorm:"default:1;index"`
	FailureCount  int `gorm:"default:0"`
	LastCheckTime time.Time

	// 3. 资源分配策略
	Region string `gorm:"size:20;default:'US';index"`

	// 容量类型：1:独享(Private) 2:共享(Shared)
	Capacity int `gorm:"default:2"`

	// 软删除外的业务开关
	IsActive bool `gorm:"default:true"`
	// 4. 关联关系
	// 一个代理可以给多个店铺使用（取决于 Capacity）
	Shops []Shop `gorm:"foreignKey:ProxyID"`
}

func (*Proxy) TableName() string {
	return "proxies"
}

func (p *Proxy) ProxyToURL() (*url.URL, error) {
	scheme := p.Protocol
	if scheme == "" {
		scheme = "http"
	}

	proxyURL := &url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s:%s", p.IP, p.Port),
	}

	// 如果有账号密码，设置 UserInfo
	if p.Username != "" {
		proxyURL.User = url.UserPassword(p.Username, p.Password)
	}

	return proxyURL, nil
}
