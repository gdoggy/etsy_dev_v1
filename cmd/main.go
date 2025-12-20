package main

import (
	"etsy_dev_v1_202512/internal/api/controller"
	"etsy_dev_v1_202512/internal/core/model"
	"etsy_dev_v1_202512/internal/core/service"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/router"
	"etsy_dev_v1_202512/internal/task"
	"etsy_dev_v1_202512/pkg/database"
	"etsy_dev_v1_202512/pkg/net"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. utils 层
	db := database.InitDB(
		// Manager
		&model.SysUser{}, &model.ShopMember{},
		// Account
		&model.Proxy{}, &model.Developer{},
		// Shop
		&model.Shop{}, &model.ShopAccount{},
		// Product
		&model.Product{}, &model.ProductImage{}, &model.ProductVariant{},
	)

	aiConfig := service.AIConfig{
		ApiKey:     "",
		TextModel:  "",
		ImageModel: "",
		VideoModel: "",
	}

	// 2. 依赖注入 (层层组装)
	// Repo 层
	proxyRepo := repository.NewProxyRepo(db)
	shopRepo := repository.NewShopRepo(db)

	// Service 层
	// net
	proxyService := service.NewProxyService(proxyRepo, shopRepo)
	networkProvider := service.NewNetworkProvider(shopRepo, proxyService)

	// 调度器
	dispatcher := net.NewDispatcher(networkProvider)

	// 消费者
	aiService := service.NewAIService(aiConfig)
	storageService := service.NewStorageService()
	authService := service.NewAuthService(shopRepo, dispatcher)
	shopService := service.NewShopService(shopRepo, dispatcher)

	productService := service.NewProductService(shopRepo, aiService, storageService)

	// Controller 层
	proxyController := controller.NewProxyController(proxyService)
	authController := controller.NewAuthController(authService)
	shopController := controller.NewShopController(shopService)
	productController := controller.NewProductController(productService)

	// Task 层定时任务
	proxyMonitorTask := task.NewProxyMonitor(proxyRepo, proxyService)
	proxyMonitorTask.Start()
	tokenTask := task.NewTokenTask(shopRepo, authService)
	tokenTask.Start()

	// 3. 初始化路由
	r := gin.Default()

	// 4. 注册路由
	router.InitRoutes(r, proxyController, authController, shopController, productController)

	// 5. 启动服务
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
	log.Println("服务已启动")
}
