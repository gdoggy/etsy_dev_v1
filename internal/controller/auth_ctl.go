package controller

import (
	"etsy_dev_v1_202512/internal/service"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authService *service.AuthService
}

func NewAuthController(s *service.AuthService) *AuthController {
	return &AuthController{authService: s}
}

// Login
// @Summary 获取 Etsy 授权链接
// @Description 为店铺生成授权链接，并生成 OAuth 授权跳转链接；必传参数 region
// @Tags Auth (授权模块)
// @Accept json
// @Produce json
// @Param shop_id query int "店铺 shop ID (Database Primary Key)，初次授权时为空"
// @Param region query string true "国家，必填字段"
// @Success 200 {string} string "点击按钮手动复制链接 url"
// @Failure 400 {string} string "错误信息"
// @Router /oauth/login [get]
func (ctrl *AuthController) Login(c *gin.Context) {
	// 1. 获取 region
	region := c.Query("region")
	if region == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "region 为空"})
		return
	}
	// 2. 获取 shop_id
	var shopID int64 = 0
	shopIDStr := c.Query("shop_id")
	if shopIDStr != "" {
		var err error
		shopID, err = strconv.ParseInt(shopIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "shop_id 必须是数字"})
			return
		}
	}

	// 2. 调用 Service
	url, err := ctrl.authService.GenerateLoginURL(c.Request.Context(), shopID, region)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "生成失败",
			"detail": err.Error(),
		})
		return
	}

	// 返回 JSON 给前端，由于网络限制，前端只能生成链接不能重定向，实际使用时可以复制链接到指纹浏览器手动跳转
	c.JSON(http.StatusOK, gin.H{
		"message":  "获取成功",
		"auth_url": url,
	})
}

// Callback
// @Summary Etsy 授权回调
// @Description 接收 Etsy 返回的 code 和 state，换取 Token 并入库
// @Tags Auth (授权模块)
// @Accept json
// @Produce json
// @Param code query string true "授权码"
// @Param state query string true "安全校验码"
// @Success 200 {object} map[string]interface{} "授权成功信息"
// @Failure 400 {object} map[string]string "拒绝授权/参数错误"
// @Router /api/oauth/callback [get]
func (ctrl *AuthController) Callback(c *gin.Context) {
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
	shop, err := ctrl.authService.HandleCallback(c.Request.Context(), code, state)
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

// RefreshToken 手动强制刷新 Token
// @Summary 刷新店铺 Token
// @Description 手动控制刷新店铺 TOKEN
// @Tags Auth (授权模块)
// @Accept json
// @Produce json
// @Param shop_id query int true "预置的店铺 ID (Database Primary Key)"
// @Success 200 {object} map[string]interface{} "成功消息+下一次过期时间"
// @Failure 400 {string} string "错误信息"
// @Router /oauth/refresh [get]
func (ctrl *AuthController) RefreshToken(c *gin.Context) {
	shopIDStr := c.Query("shop_id")
	if shopIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 shop_id 参数"})
		return
	}

	// 转为 int64
	id, err := strconv.ParseInt(shopIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shop_id 必须是数字"})
		return
	}

	shop, err := ctrl.authService.ShopService.GetByID(c.Request.Context(), id)
	if err != nil {
		log.Printf("shop id : %d, refresh token err:%v", id, err)
		c.JSON(404, gin.H{"error": "查询店铺出错"})
		return
	}

	// 调用 Service 强制刷新
	err = ctrl.authService.RefreshAccessToken(c.Request.Context(), shop)
	if err != nil {
		c.JSON(500, gin.H{"error": "刷新失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message":    "Token 刷新成功",
		"new_expiry": shop.TokenExpiresAt.Format("2006-01-02 15:04:05"),
	})
}
