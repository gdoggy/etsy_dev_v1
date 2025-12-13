package main

import (
	"log"

	"github.com/gin-gonic/gin"

	// 注意：这里的路径必须跟您的 go.mod 中的 module name 一致
	"etsy_dev_v1_202512/core/controller"
	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/repository"
	"etsy_dev_v1_202512/core/router"
	"etsy_dev_v1_202512/core/service"
	"etsy_dev_v1_202512/pkg/database"
)

func main() {
	// 1. 初始化数据库
	// 注意：这里填入您 docker-compose 中配置的真实账号密码
	dsn := "host=localhost user=etsy_admin password=1234 dbname=etsy_farm port=5432 sslmode=disable"
	db := database.InitDB(dsn)

	// 2. 自动迁移 (创建表结构)
	err := db.AutoMigrate(&model.Adapter{}, &model.Shop{})
	if err != nil {
		log.Fatalf("❌ 数据库迁移失败: %v", err)
	}

	// 3. 依赖注入 (层层组装)
	// Repo 层
	adapterRepo := repository.NewAdapterRepo(db)
	shopRepo := repository.NewShopRepo(db)

	// Service 层
	authService := service.NewAuthService(adapterRepo, shopRepo)

	// Controller 层
	authController := controller.NewAuthController(authService)

	// 4. 初始化路由
	r := gin.Default()

	// 注册路由
	router.InitRoutes(r, authController)

	// 5. 启动服务
	log.Println("🚀 服务启动中，监听端口 :8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("❌ 服务启动失败: %v", err)
	}
}
