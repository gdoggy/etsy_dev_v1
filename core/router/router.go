package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"etsy_dev_v1_202512/core/controller"
	_ "etsy_dev_v1_202512/docs"
)

// InitRoutes 注册所有路由
func InitRoutes(r *gin.Engine, authCtrl *controller.AuthController) {
	// 1. Swagger 文档路由
	// 访问 http://localhost:8080/swagger/index.html 即可查看
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 2. API 路由组
	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			// GET /api/auth/login
			auth.GET("/login", authCtrl.LoginHandler)

			// GET /api/auth/callback
			auth.GET("/callback", authCtrl.CallbackHandler)
		}
	}
}
