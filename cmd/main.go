package main

import (
	"etsy_dev_v1_202512/core/controller"
	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/repository"
	"etsy_dev_v1_202512/core/router"
	"etsy_dev_v1_202512/core/service"
	"etsy_dev_v1_202512/core/task"
	"etsy_dev_v1_202512/pkg/database"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 初始化数据库
	db := database.InitDB(
		&model.Proxy{},
		&model.Developer{},
		&model.Shop{},
		&model.ShopAccount{},
		&model.Product{},
	)

	// 2. 依赖注入 (层层组装)
	// Repo 层
	shopRepo := repository.NewShopRepo(db)

	// Service 层
	authService := service.NewAuthService(shopRepo)
	productService := service.NewProductService(shopRepo)

	// Controller 层
	authController := controller.NewAuthController(authService)
	productController := controller.NewProductController(productService)

	// Task 层
	tokenTask := task.NewTokenTask(shopRepo, authService)
	tokenTask.Start()

	// 3. 初始化路由
	r := gin.Default()

	// 4. 注册路由
	router.InitRoutes(r, authController, productController)

	// 5. 启动服务
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("❌ 服务启动失败: %v", err)
	}
}
