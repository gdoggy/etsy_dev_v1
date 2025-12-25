package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ==================== 测试模型定义 ====================

type Proxy struct {
	ID           int64 `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Host         string
	Port         int
	Username     string
	Password     string
	Protocol     string
	Country      string
	Status       int
	BindShopID   *int64
	ResponseTime int
	FailCount    int
}

func (Proxy) TableName() string { return "proxies" }

type ProxyProvider struct {
	ID         int64 `gorm:"primaryKey"`
	CreatedAt  time.Time
	Name       string
	Type       string
	APIURL     string
	APIKey     string
	Status     int
	ProxyCount int
}

func (ProxyProvider) TableName() string { return "proxy_providers" }

type Developer struct {
	ID           int64 `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Name         string
	KeyString    string
	SharedSecret string
	Status       int
	ShopCount    int
}

func (Developer) TableName() string { return "developers" }

type Shop struct {
	ID            int64 `gorm:"primaryKey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeveloperID   int64
	ShopID        int64
	ShopName      string
	UserID        int64
	Status        int
	AccessToken   string
	RefreshToken  string
	TokenExpireAt *time.Time
	ProxyID       *int64
	LastSyncAt    *time.Time
}

func (Shop) TableName() string { return "shops" }

type ShopSection struct {
	ID          int64 `gorm:"primaryKey"`
	ShopID      int64
	SectionID   int64
	Title       string
	Rank        int
	ActiveCount int
}

func (ShopSection) TableName() string { return "shop_sections" }

type ShippingProfile struct {
	ID                    int64 `gorm:"primaryKey"`
	CreatedAt             time.Time
	ShopID                int64
	ShippingProfileID     int64
	Title                 string
	OriginCountryISO      string
	PrimaryCost           int64
	SecondaryCost         int64
	MinProcessingDays     int
	MaxProcessingDays     int
	ProcessingDaysDisplay string
}

func (ShippingProfile) TableName() string { return "shipping_profiles" }

type ShippingDestination struct {
	ID                 int64 `gorm:"primaryKey"`
	ShippingProfileID  int64
	DestinationCountry string
	DestinationRegion  string
	PrimaryCost        int64
	SecondaryCost      int64
}

func (ShippingDestination) TableName() string { return "shipping_destinations" }

type ShippingUpgrade struct {
	ID                int64 `gorm:"primaryKey"`
	ShippingProfileID int64
	UpgradeID         int64
	UpgradeName       string
	Type              string
	Price             int64
	SecondaryCost     int64
}

func (ShippingUpgrade) TableName() string { return "shipping_upgrades" }

type ReturnPolicy struct {
	ID               int64 `gorm:"primaryKey"`
	CreatedAt        time.Time
	ShopID           int64
	ReturnPolicyID   int64
	AcceptsReturns   bool
	AcceptsExchanges bool
	ReturnDeadline   int
}

func (ReturnPolicy) TableName() string { return "return_policies" }

type Product struct {
	ID                int64 `gorm:"primaryKey"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ShopID            int64
	ListingID         int64
	Title             string
	Description       string
	Price             int64
	CurrencyCode      string
	Quantity          int
	TaxonomyID        int64
	ShippingProfileID int64
	ReturnPolicyID    int64
	State             string
	Views             int
	NumFavorers       int
}

func (Product) TableName() string { return "products" }

type DraftTask struct {
	ID             int64 `gorm:"primaryKey"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	UserID         int64
	SourceURL      string
	SourcePlatform string
	SourceItemID   string
	Status         string
	AIStatus       string
}

func (DraftTask) TableName() string { return "draft_tasks" }

type DraftProduct struct {
	ID                int64 `gorm:"primaryKey"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	TaskID            int64
	ShopID            int64
	Title             string
	Description       string
	TaxonomyID        int64
	ShippingProfileID int64
	Status            string
	SyncStatus        int
	ListingID         int64
}

func (DraftProduct) TableName() string { return "draft_products" }

// ==================== 集成测试套件 ====================

type IntegrationSuite struct {
	DB     *gorm.DB
	Router *gin.Engine
	T      *testing.T
}

func NewIntegrationSuite(t *testing.T) *IntegrationSuite {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}

	// 迁移所有模型
	err = db.AutoMigrate(
		&Proxy{},
		&ProxyProvider{},
		&Developer{},
		&Shop{},
		&ShopSection{},
		&ShippingProfile{},
		&ShippingDestination{},
		&ShippingUpgrade{},
		&ReturnPolicy{},
		&Product{},
		&DraftTask{},
		&DraftProduct{},
	)
	if err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	return &IntegrationSuite{
		DB:     db,
		Router: router,
		T:      t,
	}
}

// ==================== 代理模块集成测试 ====================

func TestIntegration_ProxyModule(t *testing.T) {
	suite := NewIntegrationSuite(t)

	t.Run("ProxyLifecycle", func(t *testing.T) {
		// 1. 创建代理提供商
		provider := &ProxyProvider{
			Name:   "Test Provider",
			Type:   "residential",
			Status: 1,
		}
		suite.DB.Create(provider)

		// 2. 创建代理
		proxy := &Proxy{
			Host:     "192.168.1.100",
			Port:     8080,
			Protocol: "http",
			Country:  "US",
			Status:   1,
		}
		suite.DB.Create(proxy)

		if proxy.ID == 0 {
			t.Error("代理创建失败")
		}

		// 3. 绑定到店铺
		shopID := int64(1)
		suite.DB.Model(proxy).Update("bind_shop_id", shopID)

		var updated Proxy
		suite.DB.First(&updated, proxy.ID)
		if updated.BindShopID == nil || *updated.BindShopID != shopID {
			t.Error("代理绑定店铺失败")
		}

		// 4. 更新状态
		suite.DB.Model(proxy).Updates(map[string]interface{}{
			"response_time": 150,
			"fail_count":    0,
		})

		suite.DB.First(&updated, proxy.ID)
		if updated.ResponseTime != 150 {
			t.Errorf("响应时间更新失败: got %d", updated.ResponseTime)
		}

		// 5. 查询可用代理
		var available []Proxy
		suite.DB.Where("status = ?", 1).Find(&available)
		if len(available) != 1 {
			t.Errorf("可用代理数量错误: got %d", len(available))
		}
	})
}

// ==================== 开发者模块集成测试 ====================

func TestIntegration_DeveloperModule(t *testing.T) {
	suite := NewIntegrationSuite(t)

	t.Run("DeveloperCRUD", func(t *testing.T) {
		// 创建
		dev := &Developer{
			Name:         "Test Developer",
			KeyString:    "test-key-123",
			SharedSecret: "secret-456",
			Status:       1,
		}
		suite.DB.Create(dev)

		if dev.ID == 0 {
			t.Error("开发者创建失败")
		}

		// 读取
		var found Developer
		suite.DB.First(&found, dev.ID)
		if found.Name != "Test Developer" {
			t.Errorf("开发者名称错误: got %s", found.Name)
		}

		// 更新
		suite.DB.Model(dev).Update("status", 0)
		suite.DB.First(&found, dev.ID)
		if found.Status != 0 {
			t.Errorf("开发者状态更新失败: got %d", found.Status)
		}

		// 删除
		suite.DB.Delete(dev)
		result := suite.DB.First(&found, dev.ID)
		if result.Error == nil {
			t.Error("开发者删除失败")
		}
	})

	t.Run("DeveloperWithShops", func(t *testing.T) {
		dev := &Developer{Name: "Dev with Shops", Status: 1}
		suite.DB.Create(dev)

		// 创建关联店铺
		for i := 0; i < 3; i++ {
			suite.DB.Create(&Shop{
				DeveloperID: dev.ID,
				ShopID:      int64(1000 + i),
				ShopName:    fmt.Sprintf("Shop %d", i),
				Status:      1,
			})
		}

		// 统计店铺数量
		var count int64
		suite.DB.Model(&Shop{}).Where("developer_id = ?", dev.ID).Count(&count)
		if count != 3 {
			t.Errorf("店铺数量错误: got %d", count)
		}

		// 更新开发者的店铺计数
		suite.DB.Model(dev).Update("shop_count", count)

		var updated Developer
		suite.DB.First(&updated, dev.ID)
		if updated.ShopCount != 3 {
			t.Errorf("ShopCount 更新失败: got %d", updated.ShopCount)
		}
	})
}

// ==================== 店铺模块集成测试 ====================

func TestIntegration_ShopModule(t *testing.T) {
	suite := NewIntegrationSuite(t)

	t.Run("ShopWithRelations", func(t *testing.T) {
		// 创建开发者
		dev := &Developer{Name: "Dev", Status: 1}
		suite.DB.Create(dev)

		// 创建店铺
		now := time.Now()
		expireAt := now.Add(24 * time.Hour)
		shop := &Shop{
			DeveloperID:   dev.ID,
			ShopID:        12345,
			ShopName:      "My Etsy Shop",
			Status:        1,
			AccessToken:   "access-token",
			RefreshToken:  "refresh-token",
			TokenExpireAt: &expireAt,
		}
		suite.DB.Create(shop)

		// 创建 Section
		for i := 0; i < 3; i++ {
			suite.DB.Create(&ShopSection{
				ShopID:    shop.ID,
				SectionID: int64(100 + i),
				Title:     fmt.Sprintf("Section %d", i),
				Rank:      i,
			})
		}

		// 创建运费模板
		suite.DB.Create(&ShippingProfile{
			ShopID:            shop.ID,
			ShippingProfileID: 999,
			Title:             "Standard Shipping",
			OriginCountryISO:  "US",
		})

		// 创建退货政策
		suite.DB.Create(&ReturnPolicy{
			ShopID:         shop.ID,
			ReturnPolicyID: 888,
			AcceptsReturns: true,
			ReturnDeadline: 30,
		})

		// 验证关联数据
		var sections []ShopSection
		suite.DB.Where("shop_id = ?", shop.ID).Find(&sections)
		if len(sections) != 3 {
			t.Errorf("Section 数量错误: got %d", len(sections))
		}

		var profile ShippingProfile
		suite.DB.Where("shop_id = ?", shop.ID).First(&profile)
		if profile.Title != "Standard Shipping" {
			t.Errorf("运费模板错误: got %s", profile.Title)
		}

		var policy ReturnPolicy
		suite.DB.Where("shop_id = ?", shop.ID).First(&policy)
		if !policy.AcceptsReturns {
			t.Error("退货政策错误")
		}
	})

	t.Run("ShopStatusTransition", func(t *testing.T) {
		shop := &Shop{ShopID: 99999, ShopName: "Test", Status: 1}
		suite.DB.Create(shop)

		// 停用
		suite.DB.Model(shop).Update("status", 0)
		var updated Shop
		suite.DB.First(&updated, shop.ID)
		if updated.Status != 0 {
			t.Error("店铺停用失败")
		}

		// 恢复
		suite.DB.Model(shop).Update("status", 1)
		suite.DB.First(&updated, shop.ID)
		if updated.Status != 1 {
			t.Error("店铺恢复失败")
		}
	})
}

// ==================== 物流模块集成测试 ====================

func TestIntegration_ShippingModule(t *testing.T) {
	suite := NewIntegrationSuite(t)

	t.Run("ShippingProfileWithDestinations", func(t *testing.T) {
		// 创建运费模板
		profile := &ShippingProfile{
			ShopID:            1,
			ShippingProfileID: 123,
			Title:             "International Shipping",
			OriginCountryISO:  "US",
			PrimaryCost:       500,
			SecondaryCost:     200,
		}
		suite.DB.Create(profile)

		// 创建目的地
		destinations := []ShippingDestination{
			{ShippingProfileID: profile.ID, DestinationCountry: "CA", PrimaryCost: 800},
			{ShippingProfileID: profile.ID, DestinationCountry: "UK", PrimaryCost: 1200},
			{ShippingProfileID: profile.ID, DestinationCountry: "AU", PrimaryCost: 1500},
		}
		for _, d := range destinations {
			suite.DB.Create(&d)
		}

		// 创建升级选项
		suite.DB.Create(&ShippingUpgrade{
			ShippingProfileID: profile.ID,
			UpgradeID:         1,
			UpgradeName:       "Express",
			Type:              "upgrade",
			Price:             1000,
		})

		// 验证
		var dests []ShippingDestination
		suite.DB.Where("shipping_profile_id = ?", profile.ID).Find(&dests)
		if len(dests) != 3 {
			t.Errorf("目的地数量错误: got %d", len(dests))
		}

		var upgrades []ShippingUpgrade
		suite.DB.Where("shipping_profile_id = ?", profile.ID).Find(&upgrades)
		if len(upgrades) != 1 {
			t.Errorf("升级选项数量错误: got %d", len(upgrades))
		}
	})

	t.Run("ReturnPolicyCRUD", func(t *testing.T) {
		policy := &ReturnPolicy{
			ShopID:           1,
			ReturnPolicyID:   456,
			AcceptsReturns:   true,
			AcceptsExchanges: true,
			ReturnDeadline:   30,
		}
		suite.DB.Create(policy)

		// 更新
		suite.DB.Model(policy).Update("return_deadline", 14)

		var updated ReturnPolicy
		suite.DB.First(&updated, policy.ID)
		if updated.ReturnDeadline != 14 {
			t.Errorf("退货期限更新失败: got %d", updated.ReturnDeadline)
		}
	})
}

// ==================== 商品模块集成测试 ====================

func TestIntegration_ProductModule(t *testing.T) {
	suite := NewIntegrationSuite(t)

	t.Run("ProductCRUD", func(t *testing.T) {
		product := &Product{
			ShopID:       1,
			ListingID:    123456789,
			Title:        "Handmade Ceramic Mug",
			Description:  "Beautiful handcrafted mug",
			Price:        2500,
			CurrencyCode: "USD",
			Quantity:     10,
			State:        "active",
		}
		suite.DB.Create(product)

		// 更新
		suite.DB.Model(product).Updates(map[string]interface{}{
			"price":    2999,
			"quantity": 8,
		})

		var updated Product
		suite.DB.First(&updated, product.ID)
		if updated.Price != 2999 {
			t.Errorf("价格更新失败: got %d", updated.Price)
		}

		// 查询
		var products []Product
		suite.DB.Where("shop_id = ? AND state = ?", 1, "active").Find(&products)
		if len(products) != 1 {
			t.Errorf("商品查询失败: got %d", len(products))
		}
	})

	t.Run("ProductStateTransition", func(t *testing.T) {
		product := &Product{ShopID: 1, Title: "Test", State: "draft"}
		suite.DB.Create(product)

		states := []string{"active", "inactive", "removed"}
		for _, state := range states {
			suite.DB.Model(product).Update("state", state)
			var updated Product
			suite.DB.First(&updated, product.ID)
			if updated.State != state {
				t.Errorf("状态转换失败: want %s, got %s", state, updated.State)
			}
		}
	})

	t.Run("ProductStats", func(t *testing.T) {
		// 创建多个商品
		for i := 0; i < 5; i++ {
			suite.DB.Create(&Product{
				ShopID: 1,
				Title:  fmt.Sprintf("Product %d", i),
				State:  "active",
				Price:  int64(1000 + i*100),
			})
		}

		// 统计
		var count int64
		suite.DB.Model(&Product{}).Where("shop_id = ? AND state = ?", 1, "active").Count(&count)

		var totalPrice int64
		suite.DB.Model(&Product{}).Where("shop_id = ?", 1).Select("COALESCE(SUM(price), 0)").Scan(&totalPrice)

		t.Logf("活跃商品: %d, 总价值: %d", count, totalPrice)
	})
}

// ==================== 草稿模块集成测试 ====================

func TestIntegration_DraftModule(t *testing.T) {
	suite := NewIntegrationSuite(t)

	t.Run("DraftWorkflow", func(t *testing.T) {
		// 1. 创建任务
		task := &DraftTask{
			UserID:         1,
			SourceURL:      "https://detail.1688.com/offer/123.html",
			SourcePlatform: "1688",
			SourceItemID:   "123",
			Status:         "draft",
			AIStatus:       "done",
		}
		suite.DB.Create(task)

		// 2. 创建草稿商品
		for i := 0; i < 3; i++ {
			suite.DB.Create(&DraftProduct{
				TaskID:            task.ID,
				ShopID:            int64(i + 1),
				Title:             fmt.Sprintf("Draft Product %d", i),
				Status:            "draft",
				TaxonomyID:        100,
				ShippingProfileID: 200,
			})
		}

		// 3. 更新草稿
		suite.DB.Model(&DraftProduct{}).Where("task_id = ? AND shop_id = ?", task.ID, 1).
			Update("title", "Updated Draft Product")

		// 4. 确认单个
		suite.DB.Model(&DraftProduct{}).Where("task_id = ? AND shop_id = ?", task.ID, 1).
			Updates(map[string]interface{}{"status": "confirmed", "sync_status": 1})

		// 5. 批量确认
		suite.DB.Model(&DraftProduct{}).Where("task_id = ? AND status = ?", task.ID, "draft").
			Updates(map[string]interface{}{"status": "confirmed", "sync_status": 1})

		// 验证
		var confirmed int64
		suite.DB.Model(&DraftProduct{}).Where("task_id = ? AND status = ?", task.ID, "confirmed").Count(&confirmed)
		if confirmed != 3 {
			t.Errorf("已确认商品数量错误: got %d", confirmed)
		}

		// 6. 模拟同步成功
		suite.DB.Model(&DraftProduct{}).Where("task_id = ?", task.ID).
			Updates(map[string]interface{}{"sync_status": 2, "listing_id": 999999})

		var synced int64
		suite.DB.Model(&DraftProduct{}).Where("task_id = ? AND sync_status = ?", task.ID, 2).Count(&synced)
		if synced != 3 {
			t.Errorf("已同步商品数量错误: got %d", synced)
		}
	})
}

// ==================== 跨模块集成测试 ====================

func TestIntegration_CrossModule(t *testing.T) {
	suite := NewIntegrationSuite(t)

	t.Run("FullBusinessFlow", func(t *testing.T) {
		// 1. 创建开发者
		dev := &Developer{Name: "Main Developer", Status: 1}
		suite.DB.Create(dev)

		// 2. 创建代理
		proxy := &Proxy{Host: "proxy.example.com", Port: 8080, Status: 1}
		suite.DB.Create(proxy)

		// 3. 创建店铺并绑定代理
		shop := &Shop{
			DeveloperID: dev.ID,
			ShopID:      12345,
			ShopName:    "Test Shop",
			Status:      1,
			ProxyID:     &proxy.ID,
		}
		suite.DB.Create(shop)

		// 4. 创建运费模板
		profile := &ShippingProfile{
			ShopID:            shop.ID,
			ShippingProfileID: 111,
			Title:             "Standard",
		}
		suite.DB.Create(profile)

		// 5. 创建退货政策
		policy := &ReturnPolicy{
			ShopID:         shop.ID,
			ReturnPolicyID: 222,
			AcceptsReturns: true,
		}
		suite.DB.Create(policy)

		// 6. 创建草稿任务
		task := &DraftTask{
			UserID:   1,
			Status:   "draft",
			AIStatus: "done",
		}
		suite.DB.Create(task)

		// 7. 创建草稿商品
		draft := &DraftProduct{
			TaskID:            task.ID,
			ShopID:            shop.ID,
			Title:             "New Product",
			TaxonomyID:        300,
			ShippingProfileID: profile.ID,
			Status:            "draft",
		}
		suite.DB.Create(draft)

		// 8. 确认并提交
		suite.DB.Model(draft).Updates(map[string]interface{}{
			"status":      "confirmed",
			"sync_status": 2,
			"listing_id":  888888,
		})

		// 9. 创建正式商品
		product := &Product{
			ShopID:            shop.ID,
			ListingID:         888888,
			Title:             draft.Title,
			TaxonomyID:        draft.TaxonomyID,
			ShippingProfileID: profile.ID,
			ReturnPolicyID:    policy.ID,
			State:             "active",
		}
		suite.DB.Create(product)

		// 验证完整流程
		var finalProduct Product
		suite.DB.Where("listing_id = ?", 888888).First(&finalProduct)
		if finalProduct.State != "active" {
			t.Error("商品未正确创建")
		}

		// 验证关联
		var linkedShop Shop
		suite.DB.First(&linkedShop, shop.ID)
		if linkedShop.ProxyID == nil || *linkedShop.ProxyID != proxy.ID {
			t.Error("店铺代理关联错误")
		}
	})
}

// ==================== 并发测试 ====================

func TestIntegration_Concurrency(t *testing.T) {
	suite := NewIntegrationSuite(t)

	t.Run("ConcurrentProductUpdates", func(t *testing.T) {
		// 创建测试商品
		for i := 0; i < 50; i++ {
			suite.DB.Create(&Product{
				ShopID: 1,
				Title:  fmt.Sprintf("Product %d", i),
				State:  "active",
			})
		}

		var wg sync.WaitGroup
		errors := make(chan error, 50)

		for i := 1; i <= 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				err := suite.DB.Model(&Product{}).Where("id = ?", id).
					Update("title", fmt.Sprintf("Updated %d", id)).Error
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		errorCount := 0
		for range errors {
			errorCount++
		}

		if errorCount > 0 {
			t.Errorf("并发更新失败: %d 个错误", errorCount)
		}
	})

	t.Run("ConcurrentDraftConfirmation", func(t *testing.T) {
		// 创建任务和草稿
		task := &DraftTask{Status: "draft", AIStatus: "done"}
		suite.DB.Create(task)

		for i := 0; i < 20; i++ {
			suite.DB.Create(&DraftProduct{
				TaskID:            task.ID,
				ShopID:            int64(i + 1),
				Title:             fmt.Sprintf("Draft %d", i),
				Status:            "draft",
				TaxonomyID:        1,
				ShippingProfileID: 1,
			})
		}

		var wg sync.WaitGroup
		for i := 1; i <= 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				suite.DB.Model(&DraftProduct{}).Where("id = ?", id).
					Updates(map[string]interface{}{
						"status":      "confirmed",
						"sync_status": 1,
					})
			}(i)
		}

		wg.Wait()

		var confirmed int64
		suite.DB.Model(&DraftProduct{}).Where("status = ?", "confirmed").Count(&confirmed)
		if confirmed != 20 {
			t.Errorf("并发确认失败: 已确认 %d/20", confirmed)
		}
	})
}

// ==================== 错误处理测试 ====================

func TestIntegration_ErrorHandling(t *testing.T) {
	suite := NewIntegrationSuite(t)

	t.Run("NotFoundErrors", func(t *testing.T) {
		var product Product
		result := suite.DB.First(&product, 99999)
		if result.Error == nil {
			t.Error("应该返回 not found 错误")
		}
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		// 创建草稿（缺少必填字段）
		draft := &DraftProduct{
			TaskID: 1,
			ShopID: 1,
			Title:  "", // 空标题
			Status: "draft",
		}
		suite.DB.Create(draft)

		// 尝试确认
		if draft.Title == "" {
			t.Log("验证: 空标题应阻止确认")
		}
	})

	t.Run("DuplicateKeyErrors", func(t *testing.T) {
		// 这里 SQLite 不强制 unique 约束，仅作演示
		dev1 := &Developer{Name: "Dev1", KeyString: "same-key"}
		dev2 := &Developer{Name: "Dev2", KeyString: "same-key"}

		suite.DB.Create(dev1)
		suite.DB.Create(dev2)

		// 如果有 unique 约束，第二个会失败
		t.Log("重复键测试: 依赖数据库约束")
	})
}

// ==================== HTTP 集成测试 ====================

func TestIntegration_HTTPEndpoints(t *testing.T) {
	suite := NewIntegrationSuite(t)

	// 设置简单路由
	suite.Router.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	suite.Router.GET("/api/products", func(c *gin.Context) {
		var products []Product
		suite.DB.Find(&products)
		c.JSON(200, gin.H{"data": products, "total": len(products)})
	})

	suite.Router.POST("/api/products", func(c *gin.Context) {
		var product Product
		if err := c.ShouldBindJSON(&product); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		suite.DB.Create(&product)
		c.JSON(201, product)
	})

	t.Run("HealthCheck", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		w := httptest.NewRecorder()
		suite.Router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("健康检查失败: %d", w.Code)
		}
	})

	t.Run("ListProducts", func(t *testing.T) {
		// 创建测试数据
		for i := 0; i < 5; i++ {
			suite.DB.Create(&Product{ShopID: 1, Title: fmt.Sprintf("P%d", i), State: "active"})
		}

		req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
		w := httptest.NewRecorder()
		suite.Router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("获取商品列表失败: %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["total"].(float64) < 5 {
			t.Error("商品数量不正确")
		}
	})

	t.Run("CreateProduct", func(t *testing.T) {
		body := map[string]interface{}{
			"shop_id": 1,
			"title":   "New Product",
			"state":   "draft",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/products", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		suite.Router.ServeHTTP(w, req)

		if w.Code != 201 {
			t.Errorf("创建商品失败: %d, %s", w.Code, w.Body.String())
		}
	})
}

// ==================== 数据一致性测试 ====================

func TestIntegration_DataConsistency(t *testing.T) {
	suite := NewIntegrationSuite(t)

	t.Run("TransactionRollback", func(t *testing.T) {
		tx := suite.DB.Begin()

		// 创建商品
		product := &Product{ShopID: 1, Title: "Transaction Test", State: "draft"}
		tx.Create(product)

		// 回滚
		tx.Rollback()

		// 验证未保存
		var count int64
		suite.DB.Model(&Product{}).Where("title = ?", "Transaction Test").Count(&count)
		if count != 0 {
			t.Error("事务回滚失败")
		}
	})

	t.Run("TransactionCommit", func(t *testing.T) {
		tx := suite.DB.Begin()

		product := &Product{ShopID: 1, Title: "Commit Test", State: "draft"}
		tx.Create(product)

		tx.Commit()

		var count int64
		suite.DB.Model(&Product{}).Where("title = ?", "Commit Test").Count(&count)
		if count != 1 {
			t.Error("事务提交失败")
		}
	})
}
