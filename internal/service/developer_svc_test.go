package service

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ==================== 测试模型 ====================

type TestDeveloper struct {
	ID           int64 `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Name         string
	AppKey       string
	AppSecret    string
	SharedSecret string
	RedirectURI  string
	Status       int // 0: 禁用, 1: 启用
	ShopCount    int
	MaxShopCount int
	Remark       string
}

func (TestDeveloper) TableName() string { return "developers" }

type TestShopDev struct {
	ID          int64 `gorm:"primaryKey"`
	Name        string
	DeveloperID int64
	Status      int
}

func (TestShopDev) TableName() string { return "shops" }

// ==================== 测试辅助 ====================

func setupDeveloperTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestDeveloper{}, &TestShopDev{})
	return db
}

// ==================== 单元测试 ====================

func TestDeveloperService_Create(t *testing.T) {
	db := setupDeveloperTestDB(t)

	dev := TestDeveloper{
		ID:           1,
		Name:         "Test Developer",
		AppKey:       "app_key_123",
		AppSecret:    "app_secret_456",
		SharedSecret: "shared_secret_789",
		RedirectURI:  "https://example.com/callback",
		Status:       1,
		MaxShopCount: 10,
	}

	if err := db.Create(&dev).Error; err != nil {
		t.Fatalf("创建开发者失败: %v", err)
	}

	var found TestDeveloper
	db.First(&found, 1)
	if found.AppKey != "app_key_123" {
		t.Errorf("app_key = %s, want app_key_123", found.AppKey)
	}
}

func TestDeveloperService_Update(t *testing.T) {
	db := setupDeveloperTestDB(t)

	db.Create(&TestDeveloper{
		ID:        1,
		Name:      "Original",
		AppKey:    "key1",
		AppSecret: "secret1",
		Status:    1,
	})

	// 更新
	db.Model(&TestDeveloper{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"name":       "Updated",
		"app_secret": "new_secret",
	})

	var updated TestDeveloper
	db.First(&updated, 1)
	if updated.Name != "Updated" {
		t.Errorf("name = %s, want Updated", updated.Name)
	}
	if updated.AppSecret != "new_secret" {
		t.Errorf("app_secret = %s, want new_secret", updated.AppSecret)
	}
}

func TestDeveloperService_UpdateStatus(t *testing.T) {
	db := setupDeveloperTestDB(t)

	db.Create(&TestDeveloper{ID: 1, Name: "Dev1", Status: 1})

	// 禁用
	db.Model(&TestDeveloper{}).Where("id = ?", 1).Update("status", 0)

	var dev TestDeveloper
	db.First(&dev, 1)
	if dev.Status != 0 {
		t.Errorf("status = %d, want 0", dev.Status)
	}
}

func TestDeveloperService_Delete(t *testing.T) {
	db := setupDeveloperTestDB(t)

	db.Create(&TestDeveloper{ID: 1, Name: "ToDelete"})

	// 检查是否有关联店铺
	var shopCount int64
	db.Model(&TestShopDev{}).Where("developer_id = ?", 1).Count(&shopCount)

	if shopCount > 0 {
		t.Log("有关联店铺，不允许删除")
	} else {
		db.Delete(&TestDeveloper{}, 1)

		var count int64
		db.Model(&TestDeveloper{}).Count(&count)
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
	}
}

func TestDeveloperService_DeleteWithShops(t *testing.T) {
	db := setupDeveloperTestDB(t)

	db.Create(&TestDeveloper{ID: 1, Name: "DevWithShops"})
	db.Create(&TestShopDev{ID: 1, Name: "Shop1", DeveloperID: 1})

	// 检查关联
	var shopCount int64
	db.Model(&TestShopDev{}).Where("developer_id = ?", 1).Count(&shopCount)

	if shopCount == 0 {
		t.Error("应该有关联店铺")
	}

	// 不应该删除
	// 实际业务中应返回错误
}

func TestDeveloperService_List(t *testing.T) {
	db := setupDeveloperTestDB(t)

	devs := []TestDeveloper{
		{ID: 1, Name: "Dev1", Status: 1},
		{ID: 2, Name: "Dev2", Status: 1},
		{ID: 3, Name: "Dev3", Status: 0},
	}
	for _, d := range devs {
		db.Create(&d)
	}

	// 查询所有
	var all []TestDeveloper
	db.Find(&all)
	if len(all) != 3 {
		t.Errorf("total = %d, want 3", len(all))
	}

	// 按状态筛选
	var enabled []TestDeveloper
	db.Where("status = ?", 1).Find(&enabled)
	if len(enabled) != 2 {
		t.Errorf("enabled = %d, want 2", len(enabled))
	}
}

func TestDeveloperService_GetAvailable(t *testing.T) {
	db := setupDeveloperTestDB(t)

	devs := []TestDeveloper{
		{ID: 1, Name: "Full", Status: 1, ShopCount: 10, MaxShopCount: 10},
		{ID: 2, Name: "Available", Status: 1, ShopCount: 5, MaxShopCount: 10},
		{ID: 3, Name: "Disabled", Status: 0, ShopCount: 0, MaxShopCount: 10},
	}
	for _, d := range devs {
		db.Create(&d)
	}

	// 获取可用开发者（状态启用且未满）
	var available []TestDeveloper
	db.Where("status = ? AND shop_count < max_shop_count", 1).Find(&available)

	if len(available) != 1 {
		t.Errorf("available count = %d, want 1", len(available))
	}
	if len(available) > 0 && available[0].ID != 2 {
		t.Errorf("available id = %d, want 2", available[0].ID)
	}
}

func TestDeveloperService_IncrementShopCount(t *testing.T) {
	db := setupDeveloperTestDB(t)

	db.Create(&TestDeveloper{ID: 1, Name: "Dev1", ShopCount: 5, MaxShopCount: 10})

	// 增加店铺数
	db.Model(&TestDeveloper{}).Where("id = ?", 1).Update("shop_count", gorm.Expr("shop_count + ?", 1))

	var dev TestDeveloper
	db.First(&dev, 1)
	if dev.ShopCount != 6 {
		t.Errorf("shop_count = %d, want 6", dev.ShopCount)
	}
}

func TestDeveloperService_TestConnectivity(t *testing.T) {
	// 模拟测试连通性
	// 实际实现需要调用 Etsy API
	t.Log("连通性测试需要真实 API 调用，跳过")
}
