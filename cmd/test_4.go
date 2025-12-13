package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"etsy_dev_v1_202512/core/controller"
	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/repository"
	"etsy_dev_v1_202512/core/service"
	"etsy_dev_v1_202512/pkg/database"
)

func main() {
	log.Println(">>> 开始第四步：Controller 层接口测试...")

	// 1. 初始化 DB (使用正确的密码 1234)
	dsn := "host=localhost user=etsy_admin password=1234 dbname=etsy_farm port=5432 sslmode=disable"
	db := database.InitDB(dsn)

	// 2. 组装依赖
	adapterRepo := repository.NewAdapterRepo(db)
	shopRepo := repository.NewShopRepo(db)
	authService := service.NewAuthService(adapterRepo, shopRepo)
	authController := controller.NewAuthController(authService)

	// 3. 准备测试数据 (确保有一个可用 Adapter)
	// 先清理旧数据
	db.Exec("DELETE FROM adapters WHERE name = ?", "Controller_Test_Adapter")
	testAdapter := model.Adapter{
		Name:       "Controller_Test_Adapter",
		ProxyURL:   "http://127.0.0.1:7897", // 您的正确端口
		EtsyAppKey: "Mock_App_Key_For_Test",
		Status:     1,
	}
	db.Create(&testAdapter)

	// 4. 启动 Gin 路由
	r := gin.Default()
	r.GET("/auth/login", authController.LoginHandler)

	log.Println("🚀 测试服务器已启动，监听 :8081")
	log.Println("👉 请在浏览器或 Postman 访问: http://localhost:8081/auth/login")
	log.Println("👉 预期结果: 应该看到一段 JSON，包含 'auth_url' 字段")

	// 监听 8081 防止和之前的冲突
	r.Run(":8081")
}
