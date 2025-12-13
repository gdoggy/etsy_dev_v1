package controller

import (
	"etsy_dev_v1_202512/core/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	AuthService *service.AuthService
}

func NewAuthController(s *service.AuthService) *AuthController {
	return &AuthController{AuthService: s}
}

// LoginHandler
// @Summary 获取 Etsy 授权链接
// @Description 系统会自动分配一个负载最低的开发者账号，并生成 OAuth 授权跳转链接
// @Tags Auth (授权模块)
// @Accept JSON
// @Produce JSON
// @Success 302 {string} string "Redirect to Etsy"
// @Failure 503 {object} map[string]string "资源不足错误"
// @Router /auth/login [get]
func (ctrl *AuthController) LoginHandler(c *gin.Context) {
	// 调用业务逻辑
	url, err := ctrl.AuthService.GenerateLoginURL()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":  "无法生成授权链接",
			"detail": err.Error(),
		})
		return
	}

	// 直接重定向跳转到 Etsy，或者返回 URL 给前端让前端跳
	// 这里演示返回 JSON 给前端（更符合前后端分离）
	c.JSON(http.StatusOK, gin.H{
		"message":  "获取成功",
		"auth_url": url,
	})
}

// CallbackHandler
// @Summary Etsy 授权回调
// @Description 接收 Etsy 返回的 code 和 state，换取 Token 并入库
// @Tags Auth (授权模块)
// @Accept JSON
// @Produce JSON
// @Param code query string true "授权码"
// @Param state query string true "安全校验码"
// @Success 200 {object} map[string]interface{} "授权成功信息"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /api/auth/callback [get]
func (ctrl *AuthController) CallbackHandler(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	errParam := c.Query("error")

	if errParam != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户拒绝了授权", "etsy_msg": errParam})
		return
	}

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要参数 code 或 state"})
		return
	}

	// 调用业务层换 Token
	shop, err := ctrl.AuthService.HandleCallback(code, state)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "授权失败",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "店铺绑定成功",
		"shop_name": shop.ShopName,
		"shop_id":   shop.EtsyShopID,
		"expire_at": shop.TokenExpiresAt,
	})
}
