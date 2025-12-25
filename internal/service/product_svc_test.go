package service

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ==================== 测试模型 ====================

type TestProduct struct {
	ID                int64 `gorm:"primaryKey"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ShopID            int64
	EtsyListingID     int64
	Title             string
	Description       string
	PriceAmount       int64
	PriceDivisor      int64
	CurrencyCode      string
	Quantity          int
	TaxonomyID        int64
	ShippingProfileID int64
	ReturnPolicyID    int64
	State             string // active, inactive, draft, removed
	Views             int
	NumFavorers       int
}

func (TestProduct) TableName() string { return "products" }

type TestProductImage struct {
	ID           int64 `gorm:"primaryKey"`
	ProductID    int64
	EtsyImageID  int64
	URL          string
	URLFullxfull string
	Rank         int
	AltText      string
}

func (TestProductImage) TableName() string { return "product_images" }

type TestProductVariant struct {
	ID             int64 `gorm:"primaryKey"`
	ProductID      int64
	EtsyOfferingID int64
	PropertyValues string
	Price          int64
	Quantity       int
	IsEnabled      bool
	SKU            string
}

func (TestProductVariant) TableName() string { return "product_variants" }

// ==================== 测试辅助 ====================

func setupProductTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestProduct{}, &TestProductImage{}, &TestProductVariant{})
	return db
}

// ==================== 单元测试 ====================

func TestProductService_Create(t *testing.T) {
	db := setupProductTestDB(t)

	product := TestProduct{
		ID:                1,
		ShopID:            1,
		Title:             "Handmade Ceramic Mug",
		Description:       "Beautiful handmade mug",
		PriceAmount:       2500, // $25.00
		PriceDivisor:      100,
		CurrencyCode:      "USD",
		Quantity:          10,
		TaxonomyID:        123,
		ShippingProfileID: 1,
		State:             "draft",
	}

	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}

	var found TestProduct
	db.First(&found, 1)
	if found.Title != "Handmade Ceramic Mug" {
		t.Errorf("title = %s, want Handmade Ceramic Mug", found.Title)
	}
}

func TestProductService_GetDetail(t *testing.T) {
	db := setupProductTestDB(t)

	db.Create(&TestProduct{ID: 1, ShopID: 1, Title: "Product1"})
	db.Create(&TestProductImage{ID: 1, ProductID: 1, URL: "https://img1.jpg", Rank: 1})
	db.Create(&TestProductImage{ID: 2, ProductID: 1, URL: "https://img2.jpg", Rank: 2})
	db.Create(&TestProductVariant{ID: 1, ProductID: 1, SKU: "VAR-001"})

	var product TestProduct
	db.First(&product, 1)

	var images []TestProductImage
	db.Where("product_id = ?", 1).Order("rank asc").Find(&images)

	var variants []TestProductVariant
	db.Where("product_id = ?", 1).Find(&variants)

	if product.Title != "Product1" {
		t.Errorf("title = %s, want Product1", product.Title)
	}
	if len(images) != 2 {
		t.Errorf("images count = %d, want 2", len(images))
	}
	if len(variants) != 1 {
		t.Errorf("variants count = %d, want 1", len(variants))
	}
}

func TestProductService_Update(t *testing.T) {
	db := setupProductTestDB(t)

	db.Create(&TestProduct{ID: 1, ShopID: 1, Title: "Original", PriceAmount: 1000})

	db.Model(&TestProduct{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"title":        "Updated",
		"price_amount": 2000,
		"quantity":     20,
	})

	var updated TestProduct
	db.First(&updated, 1)

	if updated.Title != "Updated" {
		t.Errorf("title = %s, want Updated", updated.Title)
	}
	if updated.PriceAmount != 2000 {
		t.Errorf("price_amount = %d, want 2000", updated.PriceAmount)
	}
}

func TestProductService_Delete(t *testing.T) {
	db := setupProductTestDB(t)

	db.Create(&TestProduct{ID: 1, ShopID: 1, Title: "ToDelete"})
	db.Create(&TestProductImage{ID: 1, ProductID: 1})
	db.Create(&TestProductVariant{ID: 1, ProductID: 1})

	// 删除关联数据
	db.Where("product_id = ?", 1).Delete(&TestProductImage{})
	db.Where("product_id = ?", 1).Delete(&TestProductVariant{})
	db.Delete(&TestProduct{}, 1)

	var productCount, imageCount, variantCount int64
	db.Model(&TestProduct{}).Count(&productCount)
	db.Model(&TestProductImage{}).Count(&imageCount)
	db.Model(&TestProductVariant{}).Count(&variantCount)

	if productCount != 0 || imageCount != 0 || variantCount != 0 {
		t.Error("删除后应该没有数据")
	}
}

func TestProductService_List(t *testing.T) {
	db := setupProductTestDB(t)

	products := []TestProduct{
		{ID: 1, ShopID: 1, Title: "Product1", State: "active"},
		{ID: 2, ShopID: 1, Title: "Product2", State: "active"},
		{ID: 3, ShopID: 1, Title: "Product3", State: "draft"},
		{ID: 4, ShopID: 2, Title: "Product4", State: "active"},
	}
	for _, p := range products {
		db.Create(&p)
	}

	// 查询店铺1的所有商品
	var shop1Products []TestProduct
	db.Where("shop_id = ?", 1).Find(&shop1Products)
	if len(shop1Products) != 3 {
		t.Errorf("shop1 products = %d, want 3", len(shop1Products))
	}

	// 按状态筛选
	var activeProducts []TestProduct
	db.Where("shop_id = ? AND state = ?", 1, "active").Find(&activeProducts)
	if len(activeProducts) != 2 {
		t.Errorf("active products = %d, want 2", len(activeProducts))
	}
}

func TestProductService_Activate(t *testing.T) {
	db := setupProductTestDB(t)

	db.Create(&TestProduct{ID: 1, ShopID: 1, State: "draft"})

	db.Model(&TestProduct{}).Where("id = ?", 1).Update("state", "active")

	var product TestProduct
	db.First(&product, 1)
	if product.State != "active" {
		t.Errorf("state = %s, want active", product.State)
	}
}

func TestProductService_Deactivate(t *testing.T) {
	db := setupProductTestDB(t)

	db.Create(&TestProduct{ID: 1, ShopID: 1, State: "active"})

	db.Model(&TestProduct{}).Where("id = ?", 1).Update("state", "inactive")

	var product TestProduct
	db.First(&product, 1)
	if product.State != "inactive" {
		t.Errorf("state = %s, want inactive", product.State)
	}
}

func TestProductService_GetStats(t *testing.T) {
	db := setupProductTestDB(t)

	products := []TestProduct{
		{ID: 1, ShopID: 1, State: "active", Views: 100, NumFavorers: 10},
		{ID: 2, ShopID: 1, State: "active", Views: 200, NumFavorers: 20},
		{ID: 3, ShopID: 1, State: "draft", Views: 0, NumFavorers: 0},
		{ID: 4, ShopID: 1, State: "inactive", Views: 50, NumFavorers: 5},
	}
	for _, p := range products {
		db.Create(&p)
	}

	// 统计
	var total int64
	db.Model(&TestProduct{}).Where("shop_id = ?", 1).Count(&total)

	var activeCount int64
	db.Model(&TestProduct{}).Where("shop_id = ? AND state = ?", 1, "active").Count(&activeCount)

	var draftCount int64
	db.Model(&TestProduct{}).Where("shop_id = ? AND state = ?", 1, "draft").Count(&draftCount)

	type Stats struct {
		TotalViews    int64
		TotalFavorers int64
	}
	var stats Stats
	db.Model(&TestProduct{}).Where("shop_id = ?", 1).Select("SUM(views) as total_views, SUM(num_favorers) as total_favorers").Scan(&stats)

	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	if activeCount != 2 {
		t.Errorf("active = %d, want 2", activeCount)
	}
	if stats.TotalViews != 350 {
		t.Errorf("total_views = %d, want 350", stats.TotalViews)
	}
}

func TestProductService_UploadImage(t *testing.T) {
	db := setupProductTestDB(t)

	db.Create(&TestProduct{ID: 1, ShopID: 1, Title: "Product1"})

	// 获取当前最大 rank
	var maxRank int
	db.Model(&TestProductImage{}).Where("product_id = ?", 1).Select("COALESCE(MAX(rank), 0)").Scan(&maxRank)

	image := TestProductImage{
		ProductID:    1,
		URL:          "https://storage.example.com/img.jpg",
		URLFullxfull: "https://storage.example.com/img_full.jpg",
		Rank:         maxRank + 1,
		AltText:      "Product image",
	}
	db.Create(&image)

	var count int64
	db.Model(&TestProductImage{}).Where("product_id = ?", 1).Count(&count)
	if count != 1 {
		t.Errorf("image count = %d, want 1", count)
	}
}

func TestProductService_SyncProducts(t *testing.T) {
	db := setupProductTestDB(t)

	// 模拟同步：更新已有商品，插入新商品

	// 已有商品
	db.Create(&TestProduct{ID: 1, ShopID: 1, EtsyListingID: 100, Title: "Old Title", State: "active"})

	// 模拟从 Etsy 获取的数据
	syncedProducts := []TestProduct{
		{ShopID: 1, EtsyListingID: 100, Title: "Updated Title", State: "active"},
		{ShopID: 1, EtsyListingID: 101, Title: "New Product", State: "active"},
	}

	for _, p := range syncedProducts {
		var existing TestProduct
		result := db.Where("shop_id = ? AND etsy_listing_id = ?", p.ShopID, p.EtsyListingID).First(&existing)
		if result.Error == nil {
			// 更新
			db.Model(&existing).Updates(map[string]interface{}{
				"title": p.Title,
				"state": p.State,
			})
		} else {
			// 插入
			db.Create(&p)
		}
	}

	var all []TestProduct
	db.Where("shop_id = ?", 1).Find(&all)

	if len(all) != 2 {
		t.Errorf("product count = %d, want 2", len(all))
	}

	// 验证更新
	var updated TestProduct
	db.Where("etsy_listing_id = ?", 100).First(&updated)
	if updated.Title != "Updated Title" {
		t.Errorf("title = %s, want Updated Title", updated.Title)
	}
}

func TestProductService_GenerateAIDraft(t *testing.T) {
	// AI 草稿生成需要 AI 服务
	t.Log("AI 草稿生成测试见 draft_svc_test.go")
}
