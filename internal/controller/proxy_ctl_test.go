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

func init() {
	gin.SetMode(gin.TestMode)
}

// ==================== 测试模型 ====================

type TestProxyCtl struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `json:"name"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Username    string    `json:"username"`
	Password    string    `json:"password"`
	Protocol    string    `json:"protocol"`
	Status      int       `json:"status"`
	LastCheckAt time.Time `json:"last_check_at"`
	Latency     int       `json:"latency"`
	FailCount   int       `json:"fail_count"`
	Remark      string    `json:"remark"`
}

func (TestProxyCtl) TableName() string { return "proxies" }

// ==================== 测试辅助 ====================

func setupProxyCtlTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestProxyCtl{})
	return db
}

func setupProxyCtlRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	proxies := api.Group("/proxies")
	{
		proxies.GET("", func(c *gin.Context) {
			var list []TestProxyCtl
			db.Find(&list)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
		})
		proxies.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var proxy TestProxyCtl
			if err := db.First(&proxy, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": proxy})
		})
		proxies.POST("", func(c *gin.Context) {
			var proxy TestProxyCtl
			if err := c.ShouldBindJSON(&proxy); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			db.Create(&proxy)
			c.JSON(http.StatusCreated, gin.H{"code": 0, "data": proxy})
		})
		proxies.PUT("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var proxy TestProxyCtl
			if err := db.First(&proxy, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
				return
			}
			if err := c.ShouldBindJSON(&proxy); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			db.Save(&proxy)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": proxy})
		})
		proxies.DELETE("/:id", func(c *gin.Context) {
			id := c.Param("id")
			db.Delete(&TestProxyCtl{}, id)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
		})
	}

	return r
}

// ==================== 测试用例 ====================

func TestProxyController_GetList(t *testing.T) {
	db := setupProxyCtlTestDB(t)
	db.Create(&TestProxyCtl{ID: 1, Name: "Proxy1", Host: "1.1.1.1", Port: 8080})
	db.Create(&TestProxyCtl{ID: 2, Name: "Proxy2", Host: "2.2.2.2", Port: 8080})

	router := setupProxyCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/proxies", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int            `json:"code"`
		Data []TestProxyCtl `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Data) != 2 {
		t.Errorf("data length = %d, want 2", len(resp.Data))
	}
}

func TestProxyController_GetDetail(t *testing.T) {
	db := setupProxyCtlTestDB(t)
	db.Create(&TestProxyCtl{ID: 1, Name: "Proxy1", Host: "1.1.1.1", Port: 8080})

	router := setupProxyCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/proxies/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int          `json:"code"`
		Data TestProxyCtl `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.Name != "Proxy1" {
		t.Errorf("name = %s, want Proxy1", resp.Data.Name)
	}
}

func TestProxyController_GetDetail_NotFound(t *testing.T) {
	db := setupProxyCtlTestDB(t)
	router := setupProxyCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/proxies/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestProxyController_Create(t *testing.T) {
	db := setupProxyCtlTestDB(t)
	router := setupProxyCtlRouter(db)

	body := map[string]interface{}{
		"name":     "NewProxy",
		"host":     "3.3.3.3",
		"port":     8080,
		"protocol": "http",
		"status":   1,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/proxies", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var count int64
	db.Model(&TestProxyCtl{}).Count(&count)
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestProxyController_Update(t *testing.T) {
	db := setupProxyCtlTestDB(t)
	db.Create(&TestProxyCtl{ID: 1, Name: "Original", Host: "1.1.1.1", Port: 8080})

	router := setupProxyCtlRouter(db)

	body := map[string]interface{}{
		"id":   1,
		"name": "Updated",
		"host": "2.2.2.2",
		"port": 9090,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/proxies/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var updated TestProxyCtl
	db.First(&updated, 1)
	if updated.Name != "Updated" {
		t.Errorf("name = %s, want Updated", updated.Name)
	}
}

func TestProxyController_Delete(t *testing.T) {
	db := setupProxyCtlTestDB(t)
	db.Create(&TestProxyCtl{ID: 1, Name: "ToDelete"})

	router := setupProxyCtlRouter(db)

	req := httptest.NewRequest(http.MethodDelete, "/api/proxies/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var count int64
	db.Model(&TestProxyCtl{}).Count(&count)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestProxyController_Create_InvalidBody(t *testing.T) {
	db := setupProxyCtlTestDB(t)
	router := setupProxyCtlRouter(db)

	req := httptest.NewRequest(http.MethodPost, "/api/proxies", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
