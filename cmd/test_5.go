package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"etsy_dev_v1_202512/core/controller"
	"etsy_dev_v1_202512/core/repository"
	"etsy_dev_v1_202512/core/router" // 引入 Router
	"etsy_dev_v1_202512/core/service"
	"etsy_dev_v1_202512/pkg/database"
)

// @title Etsy 店群管理系统 API
// @version 1.0
// @description 这是一个用于管理多个 Etsy 店铺的自动化系统 API
// @host localhost:8082
// @BasePath /api
func main() {
	log.Println(">>> 开始第五步：Router & Swagger 测试...")

	// 1. 初始化 (配置还是用您的 7897 和 1234)
	dsn := "host=localhost user=etsy_admin password=1234 dbname=etsy_farm port=5432 sslmode=disable"
	db := database.InitDB(dsn)

	adapterRepo := repository.NewAdapterRepo(db)
	shopRepo := repository.NewShopRepo(db)
	authService := service.NewAuthService(adapterRepo, shopRepo)
	authController := controller.NewAuthController(authService)

	// 2. 初始化 Gin 和 Router
	r := gin.Default()

	// 调用我们刚写的路由注册函数
	router.InitRoutes(r, authController)

	log.Println("🚀 文档服务器已启动！")
	log.Println("👉 请务必在浏览器访问: http://localhost:8082/swagger/index.html")
	log.Println("👉 预期结果: 看到深绿色的 Swagger UI 页面，并且能点开 Auth 接口详情")

	// 监听 8082 (避免端口冲突)
	r.Run(":8082")
}
