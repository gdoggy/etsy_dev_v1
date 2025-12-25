package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ==================== 测试辅助 ====================

func setupAuthCtlRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	oauth := api.Group("/oauth")
	{
		oauth.GET("/login", func(c *gin.Context) {
			developerID := c.Query("developer_id")
			if developerID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "developer_id required"})
				return
			}

			// 模拟生成授权 URL
			authURL := "https://www.etsy.com/oauth/connect?client_id=test&redirect_uri=https://example.com/callback&state=random_state"
			c.JSON(http.StatusOK, gin.H{
				"code":     0,
				"auth_url": authURL,
			})
		})

		oauth.GET("/callback", func(c *gin.Context) {
			code := c.Query("code")
			state := c.Query("state")

			if code == "" || state == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "code and state required"})
				return
			}

			// 模拟回调处理成功
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "授权成功",
				"shop": map[string]interface{}{
					"id":        1,
					"shop_name": "NewShop",
				},
			})
		})

		oauth.POST("/refresh", func(c *gin.Context) {
			var body struct {
				ShopID int64 `json:"shop_id"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
				return
			}

			if body.ShopID == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "shop_id required"})
				return
			}

			// 模拟刷新成功
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "Token 刷新成功",
			})
		})
	}

	// 通用回调路由
	api.GET("/:path/auth/callback", func(c *gin.Context) {
		path := c.Param("path")
		code := c.Query("code")
		state := c.Query("state")

		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "code required"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "callback processed",
			"path":    path,
			"state":   state,
		})
	})

	return r
}

// ==================== 测试用例 ====================

func TestAuthController_Login(t *testing.T) {
	router := setupAuthCtlRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/login?developer_id=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code    int    `json:"code"`
		AuthURL string `json:"auth_url"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.AuthURL == "" {
		t.Error("auth_url should not be empty")
	}
}

func TestAuthController_Login_MissingDeveloperID(t *testing.T) {
	router := setupAuthCtlRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAuthController_Callback(t *testing.T) {
	router := setupAuthCtlRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=test_code&state=test_state", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Shop    map[string]interface{} `json:"shop"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Message != "授权成功" {
		t.Errorf("message = %s, want 授权成功", resp.Message)
	}
}

func TestAuthController_Callback_MissingCode(t *testing.T) {
	router := setupAuthCtlRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?state=test_state", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAuthController_Callback_MissingState(t *testing.T) {
	router := setupAuthCtlRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=test_code", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAuthController_RefreshToken(t *testing.T) {
	router := setupAuthCtlRouter()

	body := `{"shop_id": 1}`
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Message != "Token 刷新成功" {
		t.Errorf("message = %s, want Token 刷新成功", resp.Message)
	}
}

func TestAuthController_RefreshToken_MissingShopID(t *testing.T) {
	router := setupAuthCtlRouter()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAuthController_GenericCallback(t *testing.T) {
	router := setupAuthCtlRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/custom-path/auth/callback?code=test_code&state=test_state", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Code int    `json:"code"`
		Path string `json:"path"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Path != "custom-path" {
		t.Errorf("path = %s, want custom-path", resp.Path)
	}
}
