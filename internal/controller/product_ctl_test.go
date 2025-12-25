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

type TestProductCtl struct {
	ID                int64     `gorm:"primaryKey" json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ShopID            int64     `json:"shop_id"`
	EtsyListingID     int64     `json:"etsy_listing_id"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	PriceAmount       int64     `json:"price_amount"`
	PriceDivisor      int64     `json:"price_divisor"`
	CurrencyCode      string    `json:"currency_code"`
	Quantity          int       `json:"quantity"`
	TaxonomyID        int64     `json:"taxonomy_id"`
	ShippingProfileID int64     `json:"shipping_profile_id"`
	State             string    `json:"state"`
	Views             int       `json:"views"`
	NumFavorers       int       `json:"num_favorers"`
}

func (TestProductCtl) TableName() string { return "products" }

type TestProductImageCtl struct {
	ID        int64  `gorm:"primaryKey" json:"id"`
	ProductID int64  `json:"product_id"`
	URL       string `json:"url"`
	Rank      int    `json:"rank"`
}

func (TestProductImageCtl) TableName() string { return "product_images" }

// ==================== 测试辅助 ====================

func setupProductCtlTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	db.AutoMigrate(&TestProductCtl{}, &TestProductImageCtl{})
	return db
}

func setupProductCtlRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	products := api.Group("/products")
	{
		products.GET("", func(c *gin.Context) {
			shopID := c.Query("shop_id")
			page := c.DefaultQuery("page", "1")
			pageSize := c.DefaultQuery("page_size", "20")

			var list []TestProductCtl
			query := db.Model(&TestProductCtl{})
			if shopID != "" {
				query = query.Where("shop_id = ?", shopID)
			}

			var total int64
			query.Count(&total)

			// 简单分页
			offset := (parseInt(page) - 1) * parseInt(pageSize)
			query.Offset(offset).Limit(parseInt(pageSize)).Find(&list)

			c.JSON(http.StatusOK, gin.H{
				"code":  0,
				"data":  list,
				"total": total,
				"page":  parseInt(page),
			})
		})

		products.GET("/stats", func(c *gin.Context) {
			shopID := c.Query("shop_id")

			var total, active, draft, inactive int64
			query := db.Model(&TestProductCtl{})
			if shopID != "" {
				query = query.Where("shop_id = ?", shopID)
			}
			query.Count(&total)
			db.Model(&TestProductCtl{}).Where("shop_id = ? AND state = ?", shopID, "active").Count(&active)
			db.Model(&TestProductCtl{}).Where("shop_id = ? AND state = ?", shopID, "draft").Count(&draft)
			db.Model(&TestProductCtl{}).Where("shop_id = ? AND state = ?", shopID, "inactive").Count(&inactive)

			type Stats struct {
				TotalViews    int64
				TotalFavorers int64
			}
			var stats Stats
			db.Model(&TestProductCtl{}).Where("shop_id = ?", shopID).
				Select("COALESCE(SUM(views), 0) as total_views, COALESCE(SUM(num_favorers), 0) as total_favorers").
				Scan(&stats)

			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": map[string]interface{}{
					"total":          total,
					"active":         active,
					"draft":          draft,
					"inactive":       inactive,
					"total_views":    stats.TotalViews,
					"total_favorers": stats.TotalFavorers,
				},
			})
		})

		products.GET("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var product TestProductCtl
			if err := db.First(&product, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
				return
			}

			var images []TestProductImageCtl
			db.Where("product_id = ?", id).Order("rank asc").Find(&images)

			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": map[string]interface{}{
					"product": product,
					"images":  images,
				},
			})
		})

		products.POST("", func(c *gin.Context) {
			var product TestProductCtl
			if err := c.ShouldBindJSON(&product); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			if product.State == "" {
				product.State = "draft"
			}
			db.Create(&product)
			c.JSON(http.StatusCreated, gin.H{"code": 0, "data": product})
		})

		products.PATCH("/:id", func(c *gin.Context) {
			id := c.Param("id")
			var body map[string]interface{}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}
			db.Model(&TestProductCtl{}).Where("id = ?", id).Updates(body)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "updated"})
		})

		products.DELETE("/:id", func(c *gin.Context) {
			id := c.Param("id")
			db.Where("product_id = ?", id).Delete(&TestProductImageCtl{})
			db.Delete(&TestProductCtl{}, id)
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
		})

		products.POST("/:id/activate", func(c *gin.Context) {
			id := c.Param("id")
			db.Model(&TestProductCtl{}).Where("id = ?", id).Update("state", "active")
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "activated"})
		})

		products.POST("/:id/deactivate", func(c *gin.Context) {
			id := c.Param("id")
			db.Model(&TestProductCtl{}).Where("id = ?", id).Update("state", "inactive")
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deactivated"})
		})

		products.POST("/ai/generate", func(c *gin.Context) {
			// 模拟 AI 生成
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "AI draft generation started",
				"task_id": 1,
			})
		})

		products.POST("/:id/approve", func(c *gin.Context) {
			id := c.Param("id")
			db.Model(&TestProductCtl{}).Where("id = ?", id).Update("state", "active")
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "approved"})
		})

		products.POST("/sync", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "sync started"})
		})

		products.POST("/:id/images", func(c *gin.Context) {
			id := c.Param("id")
			productID, _ := parseInt64(id)

			var maxRank int
			db.Model(&TestProductImageCtl{}).Where("product_id = ?", id).Select("COALESCE(MAX(rank), 0)").Scan(&maxRank)

			image := TestProductImageCtl{
				ProductID: productID,
				URL:       "https://storage.example.com/uploaded.jpg",
				Rank:      maxRank + 1,
			}
			db.Create(&image)
			c.JSON(http.StatusCreated, gin.H{"code": 0, "data": image})
		})
	}

	return r
}

func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	if result == 0 {
		return 1
	}
	return result
}

// ==================== 测试用例 ====================

func TestProductController_GetList(t *testing.T) {
	db := setupProductCtlTestDB(t)
	db.Create(&TestProductCtl{ID: 1, ShopID: 1, Title: "Product1", State: "active"})
	db.Create(&TestProductCtl{ID: 2, ShopID: 1, Title: "Product2", State: "draft"})
	db.Create(&TestProductCtl{ID: 3, ShopID: 2, Title: "Product3", State: "active"})

	router := setupProductCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/products?shop_id=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code  int              `json:"code"`
		Data  []TestProductCtl `json:"data"`
		Total int64            `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Data) != 2 {
		t.Errorf("data length = %d, want 2", len(resp.Data))
	}
}

func TestProductController_GetStats(t *testing.T) {
	db := setupProductCtlTestDB(t)
	db.Create(&TestProductCtl{ID: 1, ShopID: 1, State: "active", Views: 100, NumFavorers: 10})
	db.Create(&TestProductCtl{ID: 2, ShopID: 1, State: "active", Views: 200, NumFavorers: 20})
	db.Create(&TestProductCtl{ID: 3, ShopID: 1, State: "draft", Views: 0, NumFavorers: 0})

	router := setupProductCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/products/stats?shop_id=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data["total"].(float64) != 3 {
		t.Errorf("total = %v, want 3", resp.Data["total"])
	}
	if resp.Data["active"].(float64) != 2 {
		t.Errorf("active = %v, want 2", resp.Data["active"])
	}
}

func TestProductController_GetDetail(t *testing.T) {
	db := setupProductCtlTestDB(t)
	db.Create(&TestProductCtl{ID: 1, ShopID: 1, Title: "Product1"})
	db.Create(&TestProductImageCtl{ID: 1, ProductID: 1, URL: "https://img1.jpg", Rank: 1})
	db.Create(&TestProductImageCtl{ID: 2, ProductID: 1, URL: "https://img2.jpg", Rank: 2})

	router := setupProductCtlRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/api/products/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProductController_Create(t *testing.T) {
	db := setupProductCtlTestDB(t)
	router := setupProductCtlRouter(db)

	body := map[string]interface{}{
		"shop_id":      1,
		"title":        "New Product",
		"description":  "Description",
		"price_amount": 2500,
		"quantity":     10,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/products", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestProductController_Update(t *testing.T) {
	db := setupProductCtlTestDB(t)
	db.Create(&TestProductCtl{ID: 1, ShopID: 1, Title: "Original", PriceAmount: 1000})

	router := setupProductCtlRouter(db)

	body := map[string]interface{}{"title": "Updated", "price_amount": 2000}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/products/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestProductController_Delete(t *testing.T) {
	db := setupProductCtlTestDB(t)
	db.Create(&TestProductCtl{ID: 1, ShopID: 1, Title: "ToDelete"})
	db.Create(&TestProductImageCtl{ID: 1, ProductID: 1})

	router := setupProductCtlRouter(db)

	req := httptest.NewRequest(http.MethodDelete, "/api/products/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var productCount, imageCount int64
	db.Model(&TestProductCtl{}).Count(&productCount)
	db.Model(&TestProductImageCtl{}).Count(&imageCount)

	if productCount != 0 || imageCount != 0 {
		t.Error("删除后应该没有数据")
	}
}

func TestProductController_Activate(t *testing.T) {
	db := setupProductCtlTestDB(t)
	db.Create(&TestProductCtl{ID: 1, ShopID: 1, State: "draft"})

	router := setupProductCtlRouter(db)

	req := httptest.NewRequest(http.MethodPost, "/api/products/1/activate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var product TestProductCtl
	db.First(&product, 1)
	if product.State != "active" {
		t.Errorf("state = %s, want active", product.State)
	}
}

func TestProductController_Deactivate(t *testing.T) {
	db := setupProductCtlTestDB(t)
	db.Create(&TestProductCtl{ID: 1, ShopID: 1, State: "active"})

	router := setupProductCtlRouter(db)

	req := httptest.NewRequest(http.MethodPost, "/api/products/1/deactivate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var product TestProductCtl
	db.First(&product, 1)
	if product.State != "inactive" {
		t.Errorf("state = %s, want inactive", product.State)
	}
}

func TestProductController_UploadImage(t *testing.T) {
	db := setupProductCtlTestDB(t)
	db.Create(&TestProductCtl{ID: 1, ShopID: 1, Title: "Product1"})

	router := setupProductCtlRouter(db)

	req := httptest.NewRequest(http.MethodPost, "/api/products/1/images", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var count int64
	db.Model(&TestProductImageCtl{}).Where("product_id = ?", 1).Count(&count)
	if count != 1 {
		t.Errorf("image count = %d, want 1", count)
	}
}

func TestProductController_GenerateAIDraft(t *testing.T) {
	db := setupProductCtlTestDB(t)
	router := setupProductCtlRouter(db)

	body := map[string]interface{}{
		"source_url": "https://detail.1688.com/offer/123.html",
		"shop_ids":   []int64{1, 2},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/products/ai/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
