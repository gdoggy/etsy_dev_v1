package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ==================== 测试模型 ====================

type TestShippingProfileCtl struct {
	ID               int64     `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	ShopID           int64     `json:"shop_id"`
	EtsyProfileID    int64     `json:"etsy_profile_id"`
	Title            string    `json:"title"`
	OriginCountryISO string    `json:"origin_country_iso"`
	ProcessingMin    int       `json:"processing_min"`
	ProcessingMax    int       `json:"processing_max"`
}

func (TestShippingProfileCtl) TableName() string { return "shipping_profiles" }

type TestShippingDestCtl struct {
	ID                    int64  `gorm:"primaryKey" json:"id"`
	ShippingProfileID     int64  `json:"shipping_profile_id"`
	DestinationCountryISO string `json:"destination_country_iso"`
	PrimaryCost           int64  `json:"primary_cost"`
	SecondaryCost         int64  `json:"secondary_cost"`
}

func (TestShippingDestCtl) TableName() string { return "shipping_destinations" }

type TestShippingUpgradeCtl struct {
	ID                int64  `gorm:"primaryKey" json:"id"`
	ShippingProfileID int64  `json:"shipping_profile_id"`
	UpgradeName       string `json:"upgrade_name"`
	Price             int64  `json:"price"`
}

func (TestShippingUpgradeCtl) TableName() string { return "shipping_upgrades" }

// ==================== 测试辅助 ====================

func setupShippingCtlTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestShippingProfileCtl{}, &TestShippingDestCtl{}, &TestShippingUpgradeCtl{})
	return db
}

func setupShippingCtlRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")

	// 店铺下的运费模板
	shops := api.Group("/shops")
	{
		shops.GET("/:id/shipping-profiles", func(c *gin.Context) {
			id := c.Param("id")
			var profiles []TestShippingProfileCtl
			db.Where("shop_id = ?", id).Find(&profiles)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": profiles})
		})
		shops.POST("/:id/shipping-profiles", func(c *gin.Context) {
			id := c.Param("id")
			var profile TestShippingProfileCtl
			if err := c.ShouldBindJSON(&profile); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			profile.ShopID, _ = parseInt64(id)
			db.Create(&profile)
			c.JSON(http.StatusCreated, gin.H{"code": 0, "data": profile})
		})
		shops.POST("/:id/shipping-profiles/sync", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "synced"})
		})
	}

	// 运费模板操作
	shipping := api.Group("/shipping")
	{
		shipping.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var profile TestShippingProfileCtl
			if err := db.First(&profile, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
				return
			}

			var destinations []TestShippingDestCtl
			db.Where("shipping_profile_id = ?", id).Find(&destinations)

			var upgrades []TestShippingUpgradeCtl
			db.Where("shipping_profile_id = ?", id).Find(&upgrades)

			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": map[string]interface{}{
					"profile":      profile,
					"destinations": destinations,
					"upgrades":     upgrades,
				},
			})
		})
		shipping.PUT("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var body map[string]interface{}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			db.Model(&TestShippingProfileCtl{}).Where("id = ?", id).Updates(body)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "updated"})
		})
		shipping.DELETE("/:id", func(c *gin.Context) {
			id := c.Param("id")
			db.Where("shipping_profile_id = ?", id).Delete(&TestShippingDestCtl{})
			db.Where("shipping_profile_id = ?", id).Delete(&TestShippingUpgradeCtl{})
			db.Delete(&TestShippingProfileCtl{}, id)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
		})

		// 目的地
		shipping.POST("/:profileID/destinations", func(c *gin.Context) {
			profileID := c.Param("profileID")
			var dest TestShippingDestCtl
			if err := c.ShouldBindJSON(&dest); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			dest.ShippingProfileID, _ = parseInt64(profileID)
			db.Create(&dest)
			c.JSON(http.StatusCreated, gin.H{"code": 0, "data": dest})
		})

		// 升级选项
		shipping.POST("/:profileID/upgrades", func(c *gin.Context) {
			profileID := c.Param("profileID")
			var upgrade TestShippingUpgradeCtl
			if err := c.ShouldBindJSON(&upgrade); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			upgrade.ShippingProfileID, _ = parseInt64(profileID)
			db.Create(&upgrade)
			c.JSON(http.StatusCreated, gin.H{"code": 0, "data": upgrade})
		})
		shipping.PUT("/upgrades/:id", func(c *gin.Context) {
			id := c.Param("id")
			var body map[string]interface{}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			db.Model(&TestShippingUpgradeCtl{}).Where("id = ?", id).Updates(body)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "updated"})
		})
		shipping.DELETE("/upgrades/:id", func(c *gin.Context) {
			id := c.Param("id")
			db.Delete(&TestShippingUpgradeCtl{}, id)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
		})
	}

	return r
}

// ==================== 测试用例 ====================

func TestShippingController_GetProfileList(t *testing.T) {
	db := setupShippingCtlTestDB(t)
	db.Create(&TestShippingProfileCtl{ID: 1, ShopID: 1, Title: "Profile1"})
	db.Create(&TestShippingProfileCtl{ID: 2, ShopID: 1, Title: "Profile2"})

	router := setupShippingCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/shops/1/shipping-profiles", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int                      `json:"code"`
		Data []TestShippingProfileCtl `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Data) != 2 {
		t.Errorf("data length = %d, want 2", len(resp.Data))
	}
}

func TestShippingController_CreateProfile(t *testing.T) {
	db := setupShippingCtlTestDB(t)
	router := setupShippingCtlRouter(db)

	body := map[string]interface{}{
		"title":              "New Profile",
		"origin_country_iso": "US",
		"processing_min":     1,
		"processing_max":     3,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/shops/1/shipping-profiles", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestShippingController_GetProfileDetail(t *testing.T) {
	db := setupShippingCtlTestDB(t)
	db.Create(&TestShippingProfileCtl{ID: 1, ShopID: 1, Title: "Profile1"})
	db.Create(&TestShippingDestCtl{ID: 1, ShippingProfileID: 1, DestinationCountryISO: "US"})
	db.Create(&TestShippingUpgradeCtl{ID: 1, ShippingProfileID: 1, UpgradeName: "Express"})

	router := setupShippingCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/shipping/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestShippingController_UpdateProfile(t *testing.T) {
	db := setupShippingCtlTestDB(t)
	db.Create(&TestShippingProfileCtl{ID: 1, ShopID: 1, Title: "Original"})

	router := setupShippingCtlRouter(db)

	body := map[string]interface{}{"title": "Updated", "processing_min": 2}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/shipping/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestShippingController_DeleteProfile(t *testing.T) {
	db := setupShippingCtlTestDB(t)
	db.Create(&TestShippingProfileCtl{ID: 1, ShopID: 1, Title: "ToDelete"})
	db.Create(&TestShippingDestCtl{ID: 1, ShippingProfileID: 1})
	db.Create(&TestShippingUpgradeCtl{ID: 1, ShippingProfileID: 1})

	router := setupShippingCtlRouter(db)

	req := httptest.NewRequest(http.MethodDelete, "/api/shipping/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var profileCount, destCount, upgradeCount int64
	db.Model(&TestShippingProfileCtl{}).Count(&profileCount)
	db.Model(&TestShippingDestCtl{}).Count(&destCount)
	db.Model(&TestShippingUpgradeCtl{}).Count(&upgradeCount)

	if profileCount != 0 || destCount != 0 || upgradeCount != 0 {
		t.Error("删除后应该没有数据")
	}
}

func TestShippingController_CreateDestination(t *testing.T) {
	db := setupShippingCtlTestDB(t)
	db.Create(&TestShippingProfileCtl{ID: 1, ShopID: 1, Title: "Profile1"})

	router := setupShippingCtlRouter(db)

	body := map[string]interface{}{
		"destination_country_iso": "US",
		"primary_cost":            599,
		"secondary_cost":          199,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/shipping/1/destinations", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestShippingController_CreateUpgrade(t *testing.T) {
	db := setupShippingCtlTestDB(t)
	db.Create(&TestShippingProfileCtl{ID: 1, ShopID: 1, Title: "Profile1"})

	router := setupShippingCtlRouter(db)

	body := map[string]interface{}{
		"upgrade_name": "Express Shipping",
		"price":        1299,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/shipping/1/upgrades", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestShippingController_UpdateUpgrade(t *testing.T) {
	db := setupShippingCtlTestDB(t)
	db.Create(&TestShippingUpgradeCtl{ID: 1, ShippingProfileID: 1, UpgradeName: "Original", Price: 999})

	router := setupShippingCtlRouter(db)

	body := map[string]interface{}{"upgrade_name": "Updated", "price": 1499}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/shipping/upgrades/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestShippingController_DeleteUpgrade(t *testing.T) {
	db := setupShippingCtlTestDB(t)
	db.Create(&TestShippingUpgradeCtl{ID: 1, ShippingProfileID: 1, UpgradeName: "ToDelete"})

	router := setupShippingCtlRouter(db)

	req := httptest.NewRequest(http.MethodDelete, "/api/shipping/upgrades/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var count int64
	db.Model(&TestShippingUpgradeCtl{}).Count(&count)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}
