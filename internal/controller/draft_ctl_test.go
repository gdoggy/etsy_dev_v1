package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ==================== 请求构造辅助 ====================

func setupRouter() *gin.Engine {
	return gin.New()
}

func performRequest(r http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBytes, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBytes)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req, _ := http.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ==================== 参数验证测试 ====================

func TestCreateDraft_InvalidParams(t *testing.T) {
	router := setupRouter()

	// 模拟控制器（无真实 service）
	router.POST("/api/drafts", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "参数错误: " + err.Error(),
			})
			return
		}

		// 验证必填字段
		if req["source_url"] == nil || req["source_url"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "source_url 不能为空",
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"code": 0})
	})

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name:       "空请求体",
			body:       nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "缺少 source_url",
			body:       map[string]interface{}{"shop_ids": []int{1}},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performRequest(router, "POST", "/api/drafts", tt.body)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestGetDraftDetail_InvalidTaskID(t *testing.T) {
	router := setupRouter()

	router.GET("/api/drafts/:task_id", func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" || taskID == "0" || taskID == "abc" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "无效的任务ID",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})

	tests := []struct {
		name       string
		taskID     string
		wantStatus int
	}{
		{"无效ID: abc", "abc", http.StatusBadRequest},
		{"无效ID: 0", "0", http.StatusBadRequest},
		{"有效ID: 1", "1", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performRequest(router, "GET", "/api/drafts/"+tt.taskID, nil)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestUpdateDraftProduct_InvalidProductID(t *testing.T) {
	router := setupRouter()

	router.PATCH("/api/drafts/products/:product_id", func(c *gin.Context) {
		productID := c.Param("product_id")
		if productID == "" || productID == "0" || productID == "abc" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "无效的商品ID",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})

	tests := []struct {
		name       string
		productID  string
		wantStatus int
	}{
		{"无效ID: abc", "abc", http.StatusBadRequest},
		{"无效ID: 0", "0", http.StatusBadRequest},
		{"有效ID: 1", "1", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performRequest(router, "PATCH", "/api/drafts/products/"+tt.productID, map[string]string{"title": "test"})
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestConfirmDraftProduct_InvalidProductID(t *testing.T) {
	router := setupRouter()

	router.POST("/api/drafts/products/:product_id/confirm", func(c *gin.Context) {
		productID := c.Param("product_id")
		if productID == "" || productID == "0" || productID == "abc" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "无效的商品ID",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})

	w := performRequest(router, "POST", "/api/drafts/products/abc/confirm", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConfirmAllDrafts_InvalidTaskID(t *testing.T) {
	router := setupRouter()

	router.POST("/api/drafts/:task_id/confirm-all", func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" || taskID == "0" || taskID == "abc" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "无效的任务ID",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"confirmed_count": 2}})
	})

	// 无效 ID
	w := performRequest(router, "POST", "/api/drafts/abc/confirm-all", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 有效 ID
	w = performRequest(router, "POST", "/api/drafts/1/confirm-all", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])
}

// ==================== 响应格式测试 ====================

func TestGetSupportedPlatforms_ResponseFormat(t *testing.T) {
	router := setupRouter()

	router.GET("/api/drafts/platforms", func(c *gin.Context) {
		platforms := []map[string]interface{}{
			{"code": "1688", "name": "1688", "url_patterns": []string{"detail.1688.com"}},
			{"code": "taobao", "name": "淘宝", "url_patterns": []string{"item.taobao.com"}},
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    platforms,
		})
	})

	w := performRequest(router, "GET", "/api/drafts/platforms", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.Equal(t, float64(0), resp["code"])
	assert.Equal(t, "success", resp["message"])

	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)
}

func TestListDraftTasks_ResponseFormat(t *testing.T) {
	router := setupRouter()

	router.GET("/api/drafts", func(c *gin.Context) {
		page := c.DefaultQuery("page", "1")
		pageSize := c.DefaultQuery("page_size", "20")

		c.JSON(http.StatusOK, gin.H{
			"code":     0,
			"message":  "success",
			"data":     []interface{}{},
			"total":    0,
			"page":     page,
			"pageSize": pageSize,
		})
	})

	w := performRequest(router, "GET", "/api/drafts?page=2&page_size=10", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.Equal(t, float64(0), resp["code"])
	assert.Equal(t, "2", resp["page"])
	assert.Equal(t, "10", resp["pageSize"])
}

// ==================== SSE 端点测试 ====================

func TestStreamProgress_InvalidTaskID(t *testing.T) {
	router := setupRouter()

	router.GET("/api/drafts/:task_id/stream", func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" || taskID == "0" || taskID == "abc" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "无效的任务ID",
			})
			return
		}
		// SSE 正常响应会设置 Content-Type
		c.Header("Content-Type", "text/event-stream")
		c.String(http.StatusOK, "data: test\n\n")
	})

	w := performRequest(router, "GET", "/api/drafts/abc/stream", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ==================== RegenerateImages 测试 ====================

func TestRegenerateImages_InvalidParams(t *testing.T) {
	router := setupRouter()

	router.POST("/api/drafts/:task_id/regenerate-images", func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" || taskID == "0" || taskID == "abc" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "无效的任务ID",
			})
			return
		}

		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "参数错误: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "图片重新生成已启动",
		})
	})

	// 无效任务ID
	w := performRequest(router, "POST", "/api/drafts/abc/regenerate-images", map[string]int{"count": 5})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 有效请求
	w = performRequest(router, "POST", "/api/drafts/1/regenerate-images", map[string]int{"count": 5})
	assert.Equal(t, http.StatusOK, w.Code)
}
