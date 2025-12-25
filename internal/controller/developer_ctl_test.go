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

type TestDeveloperCtl struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Name         string    `json:"name"`
	AppKey       string    `json:"app_key"`
	AppSecret    string    `json:"app_secret"`
	SharedSecret string    `json:"shared_secret"`
	RedirectURI  string    `json:"redirect_uri"`
	Status       int       `json:"status"`
	ShopCount    int       `json:"shop_count"`
	MaxShopCount int       `json:"max_shop_count"`
	Remark       string    `json:"remark"`
}

func (TestDeveloperCtl) TableName() string { return "developers" }

// ==================== 测试辅助 ====================

func setupDeveloperCtlTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestDeveloperCtl{})
	return db
}

func setupDeveloperCtlRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	developers := api.Group("/developers")
	{
		developers.GET("", func(c *gin.Context) {
			var list []TestDeveloperCtl
			db.Find(&list)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
		})
		developers.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var dev TestDeveloperCtl
			if err := db.First(&dev, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": dev})
		})
		developers.POST("", func(c *gin.Context) {
			var dev TestDeveloperCtl
			if err := c.ShouldBindJSON(&dev); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			db.Create(&dev)
			c.JSON(http.StatusCreated, gin.H{"code": 0, "data": dev})
		})
		developers.PUT("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var dev TestDeveloperCtl
			if err := db.First(&dev, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
				return
			}
			if err := c.ShouldBindJSON(&dev); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			db.Save(&dev)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": dev})
		})
		developers.PATCH("/:id/status", func(c *gin.Context) {
			id := c.Param("id")
			var body struct {
				Status int `json:"status"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			db.Model(&TestDeveloperCtl{}).Where("id = ?", id).Update("status", body.Status)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "updated"})
		})
		developers.DELETE("/:id", func(c *gin.Context) {
			id := c.Param("id")
			db.Delete(&TestDeveloperCtl{}, id)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
		})
		developers.POST("/:id/ping", func(c *gin.Context) {
			// 模拟测试连通性
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "pong", "latency": 100})
		})
	}

	return r
}

// ==================== 测试用例 ====================

func TestDeveloperController_GetList(t *testing.T) {
	db := setupDeveloperCtlTestDB(t)
	db.Create(&TestDeveloperCtl{ID: 1, Name: "Dev1", AppKey: "key1"})
	db.Create(&TestDeveloperCtl{ID: 2, Name: "Dev2", AppKey: "key2"})

	router := setupDeveloperCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/developers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int                `json:"code"`
		Data []TestDeveloperCtl `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Data) != 2 {
		t.Errorf("data length = %d, want 2", len(resp.Data))
	}
}

func TestDeveloperController_GetDetail(t *testing.T) {
	db := setupDeveloperCtlTestDB(t)
	db.Create(&TestDeveloperCtl{ID: 1, Name: "Dev1", AppKey: "key1"})

	router := setupDeveloperCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/developers/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int              `json:"code"`
		Data TestDeveloperCtl `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.Name != "Dev1" {
		t.Errorf("name = %s, want Dev1", resp.Data.Name)
	}
}

func TestDeveloperController_Create(t *testing.T) {
	db := setupDeveloperCtlTestDB(t)
	router := setupDeveloperCtlRouter(db)

	body := map[string]interface{}{
		"name":           "NewDev",
		"app_key":        "new_key",
		"app_secret":     "new_secret",
		"shared_secret":  "shared",
		"redirect_uri":   "https://example.com/callback",
		"status":         1,
		"max_shop_count": 10,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/developers", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestDeveloperController_Update(t *testing.T) {
	db := setupDeveloperCtlTestDB(t)
	db.Create(&TestDeveloperCtl{ID: 1, Name: "Original", AppKey: "key1"})

	router := setupDeveloperCtlRouter(db)

	body := map[string]interface{}{
		"id":         1,
		"name":       "Updated",
		"app_key":    "key1",
		"app_secret": "new_secret",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/developers/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestDeveloperController_UpdateStatus(t *testing.T) {
	db := setupDeveloperCtlTestDB(t)
	db.Create(&TestDeveloperCtl{ID: 1, Name: "Dev1", Status: 1})

	router := setupDeveloperCtlRouter(db)

	body := map[string]interface{}{"status": 0}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/developers/1/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var dev TestDeveloperCtl
	db.First(&dev, 1)
	if dev.Status != 0 {
		t.Errorf("status = %d, want 0", dev.Status)
	}
}

func TestDeveloperController_Delete(t *testing.T) {
	db := setupDeveloperCtlTestDB(t)
	db.Create(&TestDeveloperCtl{ID: 1, Name: "ToDelete"})

	router := setupDeveloperCtlRouter(db)

	req := httptest.NewRequest(http.MethodDelete, "/api/developers/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var count int64
	db.Model(&TestDeveloperCtl{}).Count(&count)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestDeveloperController_TestConnectivity(t *testing.T) {
	db := setupDeveloperCtlTestDB(t)
	db.Create(&TestDeveloperCtl{ID: 1, Name: "Dev1"})

	router := setupDeveloperCtlRouter(db)

	req := httptest.NewRequest(http.MethodPost, "/api/developers/1/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Latency int    `json:"latency"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Message != "pong" {
		t.Errorf("message = %s, want pong", resp.Message)
	}
}
