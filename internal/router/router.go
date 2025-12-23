package router

import (
	controller2 "etsy_dev_v1_202512/internal/controller"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "etsy_dev_v1_202512/docs"
)

// InitRoutes 注册所有路由
func InitRoutes(r *gin.Engine,
	proxyCtl *controller2.ProxyController,
	authCtrl *controller2.AuthController,
	shopCtl *controller2.ShopController,
	productCtrl *controller2.ProductController) {
	// 1. Swagger 文档路由
	// 访问 http://localhost:8080/swagger/index.html 即可查看
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 2. API 路由组
	api := r.Group("/api")
	{
		// proxy 代理维护
		proxy := api.Group("/proxies")
		{
			// GET /api/proxies
			proxy.GET("", proxyCtl.GetList)
			proxy.GET("/:id", proxyCtl.GetDetail)
			proxy.POST("", proxyCtl.Create)
			proxy.PUT("", proxyCtl.Update)
			// GET /api/proxies/callback
			proxy.GET("/callback", proxyCtl.Callback)
		}
		// auth 鉴权组
		auth := api.Group("/auth")
		{
			// GET /api/auth/login
			auth.GET("/login", authCtrl.Login)

			// GET /api/auth/callback
			auth.GET("/callback", authCtrl.Callback)

			// GET /api/auth/refresh
			// 前端不应该直接提供 refresh 功能，后端检查 token 过期后应更新 shop status，前端引导用户重新授权
			auth.POST("/refresh", authCtrl.RefreshToken)
		}
		// shop 店铺管理
		shop := api.Group("/shops")
		{
			// GET /api/shops/
			shop.GET("")
		}
		//product 组
		products := api.Group("/products")
		{
			products.GET("", productCtrl.GetProducts)
		}
	}
}
