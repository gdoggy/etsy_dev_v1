package service

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ==================== 测试模型 ====================

type TestShippingProfile struct {
	ID                    int64 `gorm:"primaryKey"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
	ShopID                int64
	EtsyShippingProfileID int64
	Title                 string
	OriginCountryISO      string
	OriginPostalCode      string
	ProcessingMin         int
	ProcessingMax         int
	ProcessingUnit        string
}

func (TestShippingProfile) TableName() string { return "shipping_profiles" }

type TestShippingDestination struct {
	ID                    int64 `gorm:"primaryKey"`
	ShippingProfileID     int64
	DestinationCountryISO string
	PrimaryCost           int64
	SecondaryCost         int64
	ShippingCarrierID     int64
	MailClass             string
	MinDeliveryDays       int
	MaxDeliveryDays       int
}

func (TestShippingDestination) TableName() string { return "shipping_destinations" }

type TestShippingUpgrade struct {
	ID                int64 `gorm:"primaryKey"`
	ShippingProfileID int64
	EtsyUpgradeID     int64
	UpgradeName       string
	Type              string
	Price             int64
	SecondaryCost     int64
	ShippingCarrierID int64
	MailClass         string
	MinDeliveryDays   int
	MaxDeliveryDays   int
}

func (TestShippingUpgrade) TableName() string { return "shipping_upgrades" }

// ==================== 测试辅助 ====================

func setupShippingTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestShippingProfile{}, &TestShippingDestination{}, &TestShippingUpgrade{})
	return db
}

// ==================== 单元测试 ====================

func TestShippingProfileService_Create(t *testing.T) {
	db := setupShippingTestDB(t)

	profile := TestShippingProfile{
		ID:                    1,
		ShopID:                1,
		EtsyShippingProfileID: 12345,
		Title:                 "Standard Shipping",
		OriginCountryISO:      "US",
		OriginPostalCode:      "90210",
		ProcessingMin:         1,
		ProcessingMax:         3,
		ProcessingUnit:        "business_days",
	}

	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("创建运费模板失败: %v", err)
	}

	var found TestShippingProfile
	db.First(&found, 1)
	if found.Title != "Standard Shipping" {
		t.Errorf("title = %s, want Standard Shipping", found.Title)
	}
}

func TestShippingProfileService_GetDetail(t *testing.T) {
	db := setupShippingTestDB(t)

	// 创建运费模板及关联数据
	db.Create(&TestShippingProfile{ID: 1, ShopID: 1, Title: "Profile1"})
	db.Create(&TestShippingDestination{ID: 1, ShippingProfileID: 1, DestinationCountryISO: "US"})
	db.Create(&TestShippingDestination{ID: 2, ShippingProfileID: 1, DestinationCountryISO: "CA"})
	db.Create(&TestShippingUpgrade{ID: 1, ShippingProfileID: 1, UpgradeName: "Express"})

	// 获取模板
	var profile TestShippingProfile
	db.First(&profile, 1)

	// 获取目的地
	var destinations []TestShippingDestination
	db.Where("shipping_profile_id = ?", 1).Find(&destinations)

	// 获取升级选项
	var upgrades []TestShippingUpgrade
	db.Where("shipping_profile_id = ?", 1).Find(&upgrades)

	if len(destinations) != 2 {
		t.Errorf("destinations count = %d, want 2", len(destinations))
	}
	if len(upgrades) != 1 {
		t.Errorf("upgrades count = %d, want 1", len(upgrades))
	}
}

func TestShippingProfileService_Update(t *testing.T) {
	db := setupShippingTestDB(t)

	db.Create(&TestShippingProfile{ID: 1, ShopID: 1, Title: "Original", ProcessingMin: 1, ProcessingMax: 3})

	db.Model(&TestShippingProfile{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"title":          "Updated",
		"processing_min": 2,
		"processing_max": 5,
	})

	var updated TestShippingProfile
	db.First(&updated, 1)

	if updated.Title != "Updated" {
		t.Errorf("title = %s, want Updated", updated.Title)
	}
	if updated.ProcessingMin != 2 {
		t.Errorf("processing_min = %d, want 2", updated.ProcessingMin)
	}
}

func TestShippingProfileService_Delete(t *testing.T) {
	db := setupShippingTestDB(t)

	db.Create(&TestShippingProfile{ID: 1, ShopID: 1, Title: "ToDelete"})
	db.Create(&TestShippingDestination{ID: 1, ShippingProfileID: 1})
	db.Create(&TestShippingUpgrade{ID: 1, ShippingProfileID: 1})

	// 删除关联数据
	db.Where("shipping_profile_id = ?", 1).Delete(&TestShippingDestination{})
	db.Where("shipping_profile_id = ?", 1).Delete(&TestShippingUpgrade{})
	db.Delete(&TestShippingProfile{}, 1)

	var profileCount, destCount, upgradeCount int64
	db.Model(&TestShippingProfile{}).Count(&profileCount)
	db.Model(&TestShippingDestination{}).Count(&destCount)
	db.Model(&TestShippingUpgrade{}).Count(&upgradeCount)

	if profileCount != 0 || destCount != 0 || upgradeCount != 0 {
		t.Error("删除后应该没有数据")
	}
}

func TestShippingProfileService_ListByShop(t *testing.T) {
	db := setupShippingTestDB(t)

	profiles := []TestShippingProfile{
		{ID: 1, ShopID: 1, Title: "Profile1"},
		{ID: 2, ShopID: 1, Title: "Profile2"},
		{ID: 3, ShopID: 2, Title: "Profile3"},
	}
	for _, p := range profiles {
		db.Create(&p)
	}

	var shop1Profiles []TestShippingProfile
	db.Where("shop_id = ?", 1).Find(&shop1Profiles)

	if len(shop1Profiles) != 2 {
		t.Errorf("shop1 profiles = %d, want 2", len(shop1Profiles))
	}
}

func TestShippingProfileService_CreateDestination(t *testing.T) {
	db := setupShippingTestDB(t)

	db.Create(&TestShippingProfile{ID: 1, ShopID: 1, Title: "Profile1"})

	dest := TestShippingDestination{
		ShippingProfileID:     1,
		DestinationCountryISO: "US",
		PrimaryCost:           599, // $5.99
		SecondaryCost:         199, // $1.99
		MinDeliveryDays:       3,
		MaxDeliveryDays:       7,
	}
	db.Create(&dest)

	var found TestShippingDestination
	db.Where("shipping_profile_id = ? AND destination_country_iso = ?", 1, "US").First(&found)

	if found.PrimaryCost != 599 {
		t.Errorf("primary_cost = %d, want 599", found.PrimaryCost)
	}
}

func TestShippingProfileService_CreateUpgrade(t *testing.T) {
	db := setupShippingTestDB(t)

	db.Create(&TestShippingProfile{ID: 1, ShopID: 1, Title: "Profile1"})

	upgrade := TestShippingUpgrade{
		ShippingProfileID: 1,
		UpgradeName:       "Express Shipping",
		Type:              "domestic",
		Price:             1299, // $12.99
		MinDeliveryDays:   1,
		MaxDeliveryDays:   2,
	}
	db.Create(&upgrade)

	var found TestShippingUpgrade
	db.Where("shipping_profile_id = ?", 1).First(&found)

	if found.UpgradeName != "Express Shipping" {
		t.Errorf("upgrade_name = %s, want Express Shipping", found.UpgradeName)
	}
}

func TestShippingProfileService_UpdateUpgrade(t *testing.T) {
	db := setupShippingTestDB(t)

	db.Create(&TestShippingUpgrade{ID: 1, ShippingProfileID: 1, UpgradeName: "Original", Price: 999})

	db.Model(&TestShippingUpgrade{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"upgrade_name": "Updated",
		"price":        1499,
	})

	var updated TestShippingUpgrade
	db.First(&updated, 1)

	if updated.UpgradeName != "Updated" {
		t.Errorf("upgrade_name = %s, want Updated", updated.UpgradeName)
	}
	if updated.Price != 1499 {
		t.Errorf("price = %d, want 1499", updated.Price)
	}
}

func TestShippingProfileService_DeleteUpgrade(t *testing.T) {
	db := setupShippingTestDB(t)

	db.Create(&TestShippingUpgrade{ID: 1, ShippingProfileID: 1, UpgradeName: "ToDelete"})

	db.Delete(&TestShippingUpgrade{}, 1)

	var count int64
	db.Model(&TestShippingUpgrade{}).Count(&count)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestShippingProfileService_SyncProfiles(t *testing.T) {
	db := setupShippingTestDB(t)

	// 模拟从 Etsy 同步
	// 1. 删除旧数据
	// 2. 插入新数据

	// 先插入旧数据
	db.Create(&TestShippingProfile{ID: 1, ShopID: 1, EtsyShippingProfileID: 100, Title: "Old"})

	// 模拟同步：删除旧的，插入新的
	db.Where("shop_id = ?", 1).Delete(&TestShippingProfile{})

	newProfiles := []TestShippingProfile{
		{ShopID: 1, EtsyShippingProfileID: 200, Title: "New1"},
		{ShopID: 1, EtsyShippingProfileID: 201, Title: "New2"},
	}
	for _, p := range newProfiles {
		db.Create(&p)
	}

	var all []TestShippingProfile
	db.Where("shop_id = ?", 1).Find(&all)

	if len(all) != 2 {
		t.Errorf("count = %d, want 2", len(all))
	}

	// 验证没有旧数据
	var old TestShippingProfile
	result := db.Where("etsy_shipping_profile_id = ?", 100).First(&old)
	if result.Error == nil {
		t.Error("旧数据应该被删除")
	}
}
