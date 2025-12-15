package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"etsy_dev_v1_202512/core/controller"
	_ "etsy_dev_v1_202512/docs"
)

// InitRoutes 注册所有路由
func InitRoutes(r *gin.Engine, authCtrl *controller.AuthController, productCtrl *controller.ProductController) {
	// 1. Swagger 文档路由
	// 访问 http://localhost:8080/swagger/index.html 即可查看
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 2. API 路由组
	api := r.Group("/api")
	{
		// auth 鉴权组
		auth := api.Group("/auth")
		{
			// GET /api/auth/login
			auth.GET("/login", authCtrl.LoginHandler)

			// GET /api/auth/callback
			auth.GET("/callback", authCtrl.CallbackHandler)

			// GET /api/auth/refresh
			// 前端不应该直接提供 refresh 功能，后端检查 token 过期后应更新 shop status，前端引导用户重新授权
			auth.POST("/refresh", authCtrl.RefreshTokenHandler)
		}

		//product 组
		products := api.Group("/products")
		{
			products.GET("/", productCtrl.GetProductsHandler)
		}
	}
}
