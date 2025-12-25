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

type TestShopCtl struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	EtsyShopID   int64     `json:"etsy_shop_id"`
	ShopName     string    `json:"shop_name"`
	Title        string    `json:"title"`
	CurrencyCode string    `json:"currency_code"`
	DeveloperID  int64     `json:"developer_id"`
	ProxyID      int64     `json:"proxy_id"`
	Status       int       `json:"status"`
}

func (TestShopCtl) TableName() string { return "shops" }

type TestSectionCtl struct {
	ID            int64  `gorm:"primaryKey" json:"id"`
	ShopID        int64  `json:"shop_id"`
	EtsySectionID int64  `json:"etsy_section_id"`
	Title         string `json:"title"`
	Rank          int    `json:"rank"`
}

func (TestSectionCtl) TableName() string { return "shop_sections" }

// ==================== 测试辅助 ====================

func setupShopCtlTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestShopCtl{}, &TestSectionCtl{})
	return db
}

func setupShopCtlRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	shops := api.Group("/shops")
	{
		shops.GET("", func(c *gin.Context) {
			var list []TestShopCtl
			db.Find(&list)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
		})
		shops.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var shop TestShopCtl
			if err := db.First(&shop, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": shop})
		})
		shops.PUT("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var shop TestShopCtl
			if err := db.First(&shop, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
				return
			}
			var body map[string]interface{}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			db.Model(&shop).Updates(body)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": shop})
		})
		shops.DELETE("/:id", func(c *gin.Context) {
			id := c.Param("id")
			db.Where("shop_id = ?", id).Delete(&TestSectionCtl{})
			db.Delete(&TestShopCtl{}, id)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
		})
		shops.POST("/:id/stop", func(c *gin.Context) {
			id := c.Param("id")
			db.Model(&TestShopCtl{}).Where("id = ?", id).Update("status", 0)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "stopped"})
		})
		shops.POST("/:id/resume", func(c *gin.Context) {
			id := c.Param("id")
			db.Model(&TestShopCtl{}).Where("id = ?", id).Update("status", 1)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "resumed"})
		})
		shops.POST("/:id/sync", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "synced"})
		})
		// Sections
		shops.GET("/:id/sections", func(c *gin.Context) {
			id := c.Param("id")
			var sections []TestSectionCtl
			db.Where("shop_id = ?", id).Find(&sections)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": sections})
		})
		shops.POST("/:id/sections", func(c *gin.Context) {
			id := c.Param("id")
			var section TestSectionCtl
			if err := c.ShouldBindJSON(&section); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			section.ShopID, _ = parseInt64(id)
			db.Create(&section)
			c.JSON(http.StatusCreated, gin.H{"code": 0, "data": section})
		})
		shops.POST("/:id/sections/sync", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "sections synced"})
		})
	}

	sections := api.Group("/sections")
	{
		sections.PUT("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var body map[string]interface{}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			db.Model(&TestSectionCtl{}).Where("id = ?", id).Updates(body)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "updated"})
		})
		sections.DELETE("/:id", func(c *gin.Context) {
			id := c.Param("id")
			db.Delete(&TestSectionCtl{}, id)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
		})
	}

	return r
}

func parseInt64(s string) (int64, error) {
	var result int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int64(c-'0')
		}
	}
	return result, nil
}

// ==================== 测试用例 ====================

func TestShopController_GetList(t *testing.T) {
	db := setupShopCtlTestDB(t)
	db.Create(&TestShopCtl{ID: 1, ShopName: "Shop1"})
	db.Create(&TestShopCtl{ID: 2, ShopName: "Shop2"})

	router := setupShopCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/shops", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int           `json:"code"`
		Data []TestShopCtl `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Data) != 2 {
		t.Errorf("data length = %d, want 2", len(resp.Data))
	}
}

func TestShopController_GetDetail(t *testing.T) {
	db := setupShopCtlTestDB(t)
	db.Create(&TestShopCtl{ID: 1, ShopName: "Shop1", Title: "Test Shop"})

	router := setupShopCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/shops/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int         `json:"code"`
		Data TestShopCtl `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.ShopName != "Shop1" {
		t.Errorf("shop_name = %s, want Shop1", resp.Data.ShopName)
	}
}

func TestShopController_Update(t *testing.T) {
	db := setupShopCtlTestDB(t)
	db.Create(&TestShopCtl{ID: 1, ShopName: "Shop1", Title: "Original"})

	router := setupShopCtlRouter(db)

	body := map[string]interface{}{"title": "Updated Title"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/shops/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestShopController_Delete(t *testing.T) {
	db := setupShopCtlTestDB(t)
	db.Create(&TestShopCtl{ID: 1, ShopName: "ToDelete"})
	db.Create(&TestSectionCtl{ID: 1, ShopID: 1, Title: "Section1"})

	router := setupShopCtlRouter(db)

	req := httptest.NewRequest(http.MethodDelete, "/api/shops/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var shopCount, sectionCount int64
	db.Model(&TestShopCtl{}).Count(&shopCount)
	db.Model(&TestSectionCtl{}).Count(&sectionCount)

	if shopCount != 0 || sectionCount != 0 {
		t.Error("删除后应该没有数据")
	}
}

func TestShopController_Stop(t *testing.T) {
	db := setupShopCtlTestDB(t)
	db.Create(&TestShopCtl{ID: 1, ShopName: "Shop1", Status: 1})

	router := setupShopCtlRouter(db)

	req := httptest.NewRequest(http.MethodPost, "/api/shops/1/stop", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var shop TestShopCtl
	db.First(&shop, 1)
	if shop.Status != 0 {
		t.Errorf("status = %d, want 0", shop.Status)
	}
}

func TestShopController_Resume(t *testing.T) {
	db := setupShopCtlTestDB(t)
	db.Create(&TestShopCtl{ID: 1, ShopName: "Shop1", Status: 0})

	router := setupShopCtlRouter(db)

	req := httptest.NewRequest(http.MethodPost, "/api/shops/1/resume", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var shop TestShopCtl
	db.First(&shop, 1)
	if shop.Status != 1 {
		t.Errorf("status = %d, want 1", shop.Status)
	}
}

func TestShopController_CreateSection(t *testing.T) {
	db := setupShopCtlTestDB(t)
	db.Create(&TestShopCtl{ID: 1, ShopName: "Shop1"})

	router := setupShopCtlRouter(db)

	body := map[string]interface{}{
		"title": "New Section",
		"rank":  1,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/shops/1/sections", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var count int64
	db.Model(&TestSectionCtl{}).Where("shop_id = ?", 1).Count(&count)
	if count != 1 {
		t.Errorf("section count = %d, want 1", count)
	}
}

func TestShopController_GetSections(t *testing.T) {
	db := setupShopCtlTestDB(t)
	db.Create(&TestShopCtl{ID: 1, ShopName: "Shop1"})
	db.Create(&TestSectionCtl{ID: 1, ShopID: 1, Title: "Section1"})
	db.Create(&TestSectionCtl{ID: 2, ShopID: 1, Title: "Section2"})

	router := setupShopCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/shops/1/sections", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int              `json:"code"`
		Data []TestSectionCtl `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Data) != 2 {
		t.Errorf("sections count = %d, want 2", len(resp.Data))
	}
}

func TestShopController_UpdateSection(t *testing.T) {
	db := setupShopCtlTestDB(t)
	db.Create(&TestSectionCtl{ID: 1, ShopID: 1, Title: "Original"})

	router := setupShopCtlRouter(db)

	body := map[string]interface{}{"title": "Updated"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/sections/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestShopController_DeleteSection(t *testing.T) {
	db := setupShopCtlTestDB(t)
	db.Create(&TestSectionCtl{ID: 1, ShopID: 1, Title: "ToDelete"})

	router := setupShopCtlRouter(db)

	req := httptest.NewRequest(http.MethodDelete, "/api/sections/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var count int64
	db.Model(&TestSectionCtl{}).Count(&count)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}
