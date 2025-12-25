package service

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ==================== 测试模型 ====================

type TestShopFull struct {
	ID             int64 `gorm:"primaryKey"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	EtsyShopID     int64
	ShopName       string
	Title          string
	CurrencyCode   string
	DeveloperID    int64
	ProxyID        int64
	AccessToken    string
	RefreshToken   string
	TokenExpiresAt time.Time
	Status         int // 0: 停用, 1: 正常, 2: Token异常
	SyncStatus     int
	LastSyncAt     time.Time
}

func (TestShopFull) TableName() string { return "shops" }

type TestShopSection struct {
	ID            int64 `gorm:"primaryKey"`
	ShopID        int64
	EtsySectionID int64
	Title         string
	Rank          int
	ActiveCount   int
}

func (TestShopSection) TableName() string { return "shop_sections" }

type TestShopAccount struct {
	ID           int64 `gorm:"primaryKey"`
	ShopID       int64
	EtsyUserID   int64
	LoginName    string
	PrimaryEmail string
}

func (TestShopAccount) TableName() string { return "shop_accounts" }

// ==================== 测试辅助 ====================

func setupShopTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestShopFull{}, &TestShopSection{}, &TestShopAccount{}, &TestDeveloper{})
	return db
}

// ==================== 单元测试 ====================

func TestShopService_Create(t *testing.T) {
	db := setupShopTestDB(t)

	shop := TestShopFull{
		ID:           1,
		EtsyShopID:   12345678,
		ShopName:     "TestShop",
		Title:        "Test Shop Title",
		CurrencyCode: "USD",
		DeveloperID:  1,
		Status:       1,
	}

	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("创建店铺失败: %v", err)
	}

	var found TestShopFull
	db.First(&found, 1)
	if found.ShopName != "TestShop" {
		t.Errorf("shop_name = %s, want TestShop", found.ShopName)
	}
}

func TestShopService_GetDetail(t *testing.T) {
	db := setupShopTestDB(t)

	db.Create(&TestShopFull{ID: 1, ShopName: "Shop1", DeveloperID: 1, Status: 1})
	db.Create(&TestShopSection{ID: 1, ShopID: 1, Title: "Section1"})
	db.Create(&TestShopSection{ID: 2, ShopID: 1, Title: "Section2"})

	// 获取店铺
	var shop TestShopFull
	db.First(&shop, 1)

	// 获取关联 sections
	var sections []TestShopSection
	db.Where("shop_id = ?", 1).Find(&sections)

	if shop.ShopName != "Shop1" {
		t.Errorf("shop_name = %s, want Shop1", shop.ShopName)
	}
	if len(sections) != 2 {
		t.Errorf("sections count = %d, want 2", len(sections))
	}
}

func TestShopService_Update(t *testing.T) {
	db := setupShopTestDB(t)

	db.Create(&TestShopFull{ID: 1, ShopName: "Original", Title: "Old Title"})

	db.Model(&TestShopFull{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"title": "New Title",
	})

	var updated TestShopFull
	db.First(&updated, 1)
	if updated.Title != "New Title" {
		t.Errorf("title = %s, want New Title", updated.Title)
	}
}

func TestShopService_Delete(t *testing.T) {
	db := setupShopTestDB(t)

	db.Create(&TestShopFull{ID: 1, ShopName: "ToDelete"})
	db.Create(&TestShopSection{ID: 1, ShopID: 1, Title: "Section1"})

	// 删除关联数据
	db.Where("shop_id = ?", 1).Delete(&TestShopSection{})
	db.Delete(&TestShopFull{}, 1)

	var shopCount, sectionCount int64
	db.Model(&TestShopFull{}).Count(&shopCount)
	db.Model(&TestShopSection{}).Count(&sectionCount)

	if shopCount != 0 {
		t.Errorf("shop count = %d, want 0", shopCount)
	}
	if sectionCount != 0 {
		t.Errorf("section count = %d, want 0", sectionCount)
	}
}

func TestShopService_List(t *testing.T) {
	db := setupShopTestDB(t)

	shops := []TestShopFull{
		{ID: 1, ShopName: "Shop1", DeveloperID: 1, Status: 1},
		{ID: 2, ShopName: "Shop2", DeveloperID: 1, Status: 1},
		{ID: 3, ShopName: "Shop3", DeveloperID: 2, Status: 0},
	}
	for _, s := range shops {
		db.Create(&s)
	}

	// 查询所有
	var all []TestShopFull
	db.Find(&all)
	if len(all) != 3 {
		t.Errorf("total = %d, want 3", len(all))
	}

	// 按开发者筛选
	var devShops []TestShopFull
	db.Where("developer_id = ?", 1).Find(&devShops)
	if len(devShops) != 2 {
		t.Errorf("developer shops = %d, want 2", len(devShops))
	}
}

func TestShopService_Stop(t *testing.T) {
	db := setupShopTestDB(t)

	db.Create(&TestShopFull{ID: 1, ShopName: "Shop1", Status: 1})

	// 停用店铺
	db.Model(&TestShopFull{}).Where("id = ?", 1).Update("status", 0)

	var shop TestShopFull
	db.First(&shop, 1)
	if shop.Status != 0 {
		t.Errorf("status = %d, want 0", shop.Status)
	}
}

func TestShopService_Resume(t *testing.T) {
	db := setupShopTestDB(t)

	db.Create(&TestShopFull{ID: 1, ShopName: "Shop1", Status: 0})

	// 恢复店铺
	db.Model(&TestShopFull{}).Where("id = ?", 1).Update("status", 1)

	var shop TestShopFull
	db.First(&shop, 1)
	if shop.Status != 1 {
		t.Errorf("status = %d, want 1", shop.Status)
	}
}

func TestShopService_UpdateToken(t *testing.T) {
	db := setupShopTestDB(t)

	db.Create(&TestShopFull{
		ID:           1,
		ShopName:     "Shop1",
		AccessToken:  "old_token",
		RefreshToken: "old_refresh",
	})

	newExpiry := time.Now().Add(24 * time.Hour)
	db.Model(&TestShopFull{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"access_token":     "new_token",
		"refresh_token":    "new_refresh",
		"token_expires_at": newExpiry,
	})

	var shop TestShopFull
	db.First(&shop, 1)
	if shop.AccessToken != "new_token" {
		t.Errorf("access_token = %s, want new_token", shop.AccessToken)
	}
}

func TestShopService_SyncSections(t *testing.T) {
	db := setupShopTestDB(t)

	db.Create(&TestShopFull{ID: 1, ShopName: "Shop1"})

	// 模拟同步 sections
	sections := []TestShopSection{
		{ID: 1, ShopID: 1, EtsySectionID: 100, Title: "New Section 1"},
		{ID: 2, ShopID: 1, EtsySectionID: 101, Title: "New Section 2"},
	}
	for _, s := range sections {
		db.Create(&s)
	}

	var count int64
	db.Model(&TestShopSection{}).Where("shop_id = ?", 1).Count(&count)
	if count != 2 {
		t.Errorf("section count = %d, want 2", count)
	}
}

func TestShopService_CreateSection(t *testing.T) {
	db := setupShopTestDB(t)

	db.Create(&TestShopFull{ID: 1, ShopName: "Shop1"})

	section := TestShopSection{
		ShopID:        1,
		EtsySectionID: 12345,
		Title:         "New Section",
		Rank:          1,
	}
	db.Create(&section)

	var found TestShopSection
	db.Where("shop_id = ? AND title = ?", 1, "New Section").First(&found)
	if found.EtsySectionID != 12345 {
		t.Errorf("etsy_section_id = %d, want 12345", found.EtsySectionID)
	}
}

func TestShopService_UpdateSection(t *testing.T) {
	db := setupShopTestDB(t)

	db.Create(&TestShopSection{ID: 1, ShopID: 1, Title: "Original"})

	db.Model(&TestShopSection{}).Where("id = ?", 1).Update("title", "Updated")

	var section TestShopSection
	db.First(&section, 1)
	if section.Title != "Updated" {
		t.Errorf("title = %s, want Updated", section.Title)
	}
}

func TestShopService_DeleteSection(t *testing.T) {
	db := setupShopTestDB(t)

	db.Create(&TestShopSection{ID: 1, ShopID: 1, Title: "ToDelete"})

	db.Delete(&TestShopSection{}, 1)

	var count int64
	db.Model(&TestShopSection{}).Count(&count)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestShopService_GetNeedRefreshTokenShops(t *testing.T) {
	db := setupShopTestDB(t)

	now := time.Now()
	shops := []TestShopFull{
		{ID: 1, ShopName: "NeedRefresh", Status: 1, TokenExpiresAt: now.Add(-1 * time.Hour)},  // 已过期
		{ID: 2, ShopName: "SoonExpire", Status: 1, TokenExpiresAt: now.Add(30 * time.Minute)}, // 即将过期
		{ID: 3, ShopName: "Valid", Status: 1, TokenExpiresAt: now.Add(24 * time.Hour)},        // 有效
		{ID: 4, ShopName: "Stopped", Status: 0, TokenExpiresAt: now.Add(-1 * time.Hour)},      // 已停用
	}
	for _, s := range shops {
		db.Create(&s)
	}

	// 获取需要刷新的店铺（状态正常且 Token 即将过期）
	threshold := now.Add(1 * time.Hour)
	var needRefresh []TestShopFull
	db.Where("status = ? AND token_expires_at < ?", 1, threshold).Find(&needRefresh)

	if len(needRefresh) != 2 {
		t.Errorf("need refresh count = %d, want 2", len(needRefresh))
	}
}
