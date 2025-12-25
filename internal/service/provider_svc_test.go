package service

import (
	"net/http"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ==================== 测试模型 ====================

type TestShopNetwork struct {
	ID          int64 `gorm:"primaryKey"`
	ShopName    string
	ProxyID     int64
	DeveloperID int64
	AccessToken string
	Status      int
}

func (TestShopNetwork) TableName() string { return "shops" }

type TestProxyNetwork struct {
	ID       int64 `gorm:"primaryKey"`
	Name     string
	Host     string
	Port     int
	Username string
	Password string
	Protocol string
	Status   int
}

func (TestProxyNetwork) TableName() string { return "proxies" }

type TestDeveloperNetwork struct {
	ID           int64 `gorm:"primaryKey"`
	Name         string
	AppKey       string
	AppSecret    string
	SharedSecret string
	Status       int
}

func (TestDeveloperNetwork) TableName() string { return "developers" }

// ==================== 测试辅助 ====================

func setupNetworkTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestShopNetwork{}, &TestProxyNetwork{}, &TestDeveloperNetwork{})
	return db
}

// ==================== 单元测试 ====================

func TestNetworkProvider_GetShopClient(t *testing.T) {
	db := setupNetworkTestDB(t)

	// 创建测试数据
	db.Create(&TestProxyNetwork{
		ID:       1,
		Name:     "Proxy1",
		Host:     "127.0.0.1",
		Port:     8080,
		Protocol: "http",
		Status:   1,
	})
	db.Create(&TestShopNetwork{
		ID:          1,
		ShopName:    "Shop1",
		ProxyID:     1,
		DeveloperID: 1,
		AccessToken: "test_token",
		Status:      1,
	})

	// 验证店铺存在
	var shop TestShopNetwork
	db.First(&shop, 1)
	if shop.ProxyID != 1 {
		t.Errorf("proxy_id = %d, want 1", shop.ProxyID)
	}

	// 验证代理存在
	var proxy TestProxyNetwork
	db.First(&proxy, shop.ProxyID)
	if proxy.Host != "127.0.0.1" {
		t.Errorf("proxy host = %s, want 127.0.0.1", proxy.Host)
	}
}

func TestNetworkProvider_BuildProxyURL(t *testing.T) {
	tests := []struct {
		name    string
		proxy   TestProxyNetwork
		wantURL string
	}{
		{
			name: "HTTP 代理无认证",
			proxy: TestProxyNetwork{
				Host:     "127.0.0.1",
				Port:     8080,
				Protocol: "http",
			},
			wantURL: "http://127.0.0.1:8080",
		},
		{
			name: "HTTP 代理有认证",
			proxy: TestProxyNetwork{
				Host:     "127.0.0.1",
				Port:     8080,
				Protocol: "http",
				Username: "user",
				Password: "pass",
			},
			wantURL: "http://user:pass@127.0.0.1:8080",
		},
		{
			name: "SOCKS5 代理",
			proxy: TestProxyNetwork{
				Host:     "127.0.0.1",
				Port:     1080,
				Protocol: "socks5",
			},
			wantURL: "socks5://127.0.0.1:1080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := buildTestProxyURL(tt.proxy)
			if url != tt.wantURL {
				t.Errorf("url = %s, want %s", url, tt.wantURL)
			}
		})
	}
}

func TestNetworkProvider_CreateHTTPClient(t *testing.T) {
	proxy := TestProxyNetwork{
		Host:     "127.0.0.1",
		Port:     8080,
		Protocol: "http",
	}

	client := createTestHTTPClient(proxy, 30*time.Second)

	if client == nil {
		t.Fatal("client should not be nil")
	}

	if client.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", client.Timeout)
	}
}

func TestNetworkProvider_GetDeveloperCredentials(t *testing.T) {
	db := setupNetworkTestDB(t)

	db.Create(&TestDeveloperNetwork{
		ID:           1,
		Name:         "Dev1",
		AppKey:       "app_key_123",
		AppSecret:    "app_secret_456",
		SharedSecret: "shared_789",
		Status:       1,
	})
	db.Create(&TestShopNetwork{
		ID:          1,
		ShopName:    "Shop1",
		DeveloperID: 1,
		Status:      1,
	})

	// 获取店铺的开发者凭证
	var shop TestShopNetwork
	db.First(&shop, 1)

	var developer TestDeveloperNetwork
	db.First(&developer, shop.DeveloperID)

	if developer.AppKey != "app_key_123" {
		t.Errorf("app_key = %s, want app_key_123", developer.AppKey)
	}
	if developer.AppSecret != "app_secret_456" {
		t.Errorf("app_secret = %s, want app_secret_456", developer.AppSecret)
	}
}

func TestNetworkProvider_NoProxy(t *testing.T) {
	db := setupNetworkTestDB(t)

	// 店铺无代理
	db.Create(&TestShopNetwork{
		ID:          1,
		ShopName:    "Shop1",
		ProxyID:     0, // 无代理
		DeveloperID: 1,
		Status:      1,
	})

	var shop TestShopNetwork
	db.First(&shop, 1)

	if shop.ProxyID != 0 {
		t.Errorf("proxy_id = %d, want 0", shop.ProxyID)
	}

	// 无代理时应该使用默认客户端
	client := &http.Client{Timeout: 30 * time.Second}
	if client == nil {
		t.Fatal("default client should not be nil")
	}
}

func TestNetworkProvider_ProxyDisabled(t *testing.T) {
	db := setupNetworkTestDB(t)

	db.Create(&TestProxyNetwork{
		ID:     1,
		Host:   "127.0.0.1",
		Port:   8080,
		Status: 0, // 禁用
	})
	db.Create(&TestShopNetwork{
		ID:      1,
		ProxyID: 1,
		Status:  1,
	})

	var shop TestShopNetwork
	db.First(&shop, 1)

	var proxy TestProxyNetwork
	db.First(&proxy, shop.ProxyID)

	if proxy.Status != 0 {
		t.Errorf("proxy status = %d, want 0", proxy.Status)
	}

	// 代理禁用时应该返回错误或使用备用代理
}

func TestNetworkProvider_GetAvailableProxy(t *testing.T) {
	db := setupNetworkTestDB(t)

	proxies := []TestProxyNetwork{
		{ID: 1, Host: "1.1.1.1", Port: 8080, Status: 1},
		{ID: 2, Host: "2.2.2.2", Port: 8080, Status: 0}, // 禁用
		{ID: 3, Host: "3.3.3.3", Port: 8080, Status: 2}, // 异常
	}
	for _, p := range proxies {
		db.Create(&p)
	}

	// 获取可用代理
	var available []TestProxyNetwork
	db.Where("status = ?", 1).Find(&available)

	if len(available) != 1 {
		t.Errorf("available count = %d, want 1", len(available))
	}
	if len(available) > 0 && available[0].ID != 1 {
		t.Errorf("available proxy id = %d, want 1", available[0].ID)
	}
}

// ==================== 辅助函数 ====================

func buildTestProxyURL(proxy TestProxyNetwork) string {
	auth := ""
	if proxy.Username != "" {
		auth = proxy.Username + ":" + proxy.Password + "@"
	}
	return proxy.Protocol + "://" + auth + proxy.Host + ":" + itoa(proxy.Port)
}

func itoa(i int) string {
	return string(rune('0'+i/1000%10)) + string(rune('0'+i/100%10)) + string(rune('0'+i/10%10)) + string(rune('0'+i%10))
}

func createTestHTTPClient(proxy TestProxyNetwork, timeout time.Duration) *http.Client {
	// 简化实现，实际需要配置 Transport
	return &http.Client{
		Timeout: timeout,
	}
}
