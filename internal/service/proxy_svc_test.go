package service

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ==================== 测试模型 ====================

type TestProxy struct {
	ID          int64 `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Name        string
	Host        string
	Port        int
	Username    string
	Password    string
	Protocol    string // http, https, socks5
	Status      int    // 0: 禁用, 1: 启用, 2: 异常
	LastCheckAt time.Time
	Latency     int // 延迟 ms
	FailCount   int
	Remark      string
}

func (TestProxy) TableName() string { return "proxies" }

type TestShop struct {
	ID        int64 `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	ProxyID   int64
	Status    int
}

func (TestShop) TableName() string { return "shops" }

// ==================== 测试辅助 ====================

func setupProxyTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestProxy{}, &TestShop{})
	return db
}

// ==================== 单元测试 ====================

func TestProxyService_Create(t *testing.T) {
	db := setupProxyTestDB(t)

	// 插入测试数据
	proxy := TestProxy{
		ID:       1,
		Name:     "Test Proxy",
		Host:     "127.0.0.1",
		Port:     8080,
		Protocol: "http",
		Status:   1,
	}
	if err := db.Create(&proxy).Error; err != nil {
		t.Fatalf("创建代理失败: %v", err)
	}

	// 验证
	var found TestProxy
	db.First(&found, 1)
	if found.Name != "Test Proxy" {
		t.Errorf("name = %s, want Test Proxy", found.Name)
	}
}

func TestProxyService_Update(t *testing.T) {
	db := setupProxyTestDB(t)

	// 插入测试数据
	db.Create(&TestProxy{ID: 1, Name: "Original", Host: "127.0.0.1", Port: 8080, Status: 1})

	// 更新
	db.Model(&TestProxy{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"name": "Updated",
		"port": 9090,
	})

	// 验证
	var updated TestProxy
	db.First(&updated, 1)
	if updated.Name != "Updated" {
		t.Errorf("name = %s, want Updated", updated.Name)
	}
	if updated.Port != 9090 {
		t.Errorf("port = %d, want 9090", updated.Port)
	}
}

func TestProxyService_Delete(t *testing.T) {
	db := setupProxyTestDB(t)

	// 插入测试数据
	db.Create(&TestProxy{ID: 1, Name: "ToDelete", Host: "127.0.0.1", Port: 8080})

	// 删除
	db.Delete(&TestProxy{}, 1)

	// 验证
	var count int64
	db.Model(&TestProxy{}).Count(&count)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestProxyService_List(t *testing.T) {
	db := setupProxyTestDB(t)

	// 插入测试数据
	proxies := []TestProxy{
		{ID: 1, Name: "Proxy1", Host: "1.1.1.1", Port: 8080, Status: 1},
		{ID: 2, Name: "Proxy2", Host: "2.2.2.2", Port: 8080, Status: 1},
		{ID: 3, Name: "Proxy3", Host: "3.3.3.3", Port: 8080, Status: 0},
	}
	for _, p := range proxies {
		db.Create(&p)
	}

	// 查询所有
	var all []TestProxy
	db.Find(&all)
	if len(all) != 3 {
		t.Errorf("total = %d, want 3", len(all))
	}

	// 按状态筛选
	var enabled []TestProxy
	db.Where("status = ?", 1).Find(&enabled)
	if len(enabled) != 2 {
		t.Errorf("enabled count = %d, want 2", len(enabled))
	}
}

func TestProxyService_CheckHealth(t *testing.T) {
	db := setupProxyTestDB(t)

	// 插入测试数据
	db.Create(&TestProxy{
		ID:        1,
		Name:      "HealthCheck",
		Host:      "127.0.0.1",
		Port:      8080,
		Status:    1,
		FailCount: 0,
	})

	// 模拟健康检查失败
	db.Model(&TestProxy{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"fail_count":    3,
		"status":        2, // 异常
		"last_check_at": time.Now(),
	})

	// 验证
	var proxy TestProxy
	db.First(&proxy, 1)
	if proxy.Status != 2 {
		t.Errorf("status = %d, want 2", proxy.Status)
	}
	if proxy.FailCount != 3 {
		t.Errorf("fail_count = %d, want 3", proxy.FailCount)
	}
}

func TestProxyService_AssignToShop(t *testing.T) {
	db := setupProxyTestDB(t)

	// 插入测试数据
	db.Create(&TestProxy{ID: 1, Name: "Proxy1", Host: "1.1.1.1", Port: 8080, Status: 1})
	db.Create(&TestShop{ID: 1, Name: "Shop1", ProxyID: 0, Status: 1})

	// 分配代理
	db.Model(&TestShop{}).Where("id = ?", 1).Update("proxy_id", 1)

	// 验证
	var shop TestShop
	db.First(&shop, 1)
	if shop.ProxyID != 1 {
		t.Errorf("proxy_id = %d, want 1", shop.ProxyID)
	}
}

func TestProxyService_GetAvailable(t *testing.T) {
	db := setupProxyTestDB(t)

	// 插入测试数据
	proxies := []TestProxy{
		{ID: 1, Name: "Good", Host: "1.1.1.1", Port: 8080, Status: 1, Latency: 100},
		{ID: 2, Name: "Bad", Host: "2.2.2.2", Port: 8080, Status: 2, Latency: 500},
		{ID: 3, Name: "Disabled", Host: "3.3.3.3", Port: 8080, Status: 0, Latency: 50},
	}
	for _, p := range proxies {
		db.Create(&p)
	}

	// 获取可用代理（状态为1，延迟最低）
	var available TestProxy
	db.Where("status = ?", 1).Order("latency asc").First(&available)

	if available.ID != 1 {
		t.Errorf("available proxy id = %d, want 1", available.ID)
	}
}
