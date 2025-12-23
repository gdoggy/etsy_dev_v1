package main

import (
	controller2 "etsy_dev_v1_202512/internal/controller"
	model2 "etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/router"
	service2 "etsy_dev_v1_202512/internal/service"
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
		&model2.SysUser{}, &model2.ShopMember{},
		// Account
		&model2.Proxy{}, &model2.Developer{},
		// Shop
		&model2.Shop{}, &model2.ShopAccount{},
		// Product
		&model2.Product{}, &model2.ProductImage{}, &model2.ProductVariant{},
	)

	aiConfig := service2.AIConfig{
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
	proxyService := service2.NewProxyService(proxyRepo, shopRepo)
	networkProvider := service2.NewNetworkProvider(shopRepo, proxyService)

	// 调度器
	dispatcher := net.NewDispatcher(networkProvider)

	// 消费者
	aiService := service2.NewAIService(aiConfig)
	storageService := service2.NewStorageService()
	authService := service2.NewAuthService(shopRepo, dispatcher)
	shopService := service2.NewShopService(shopRepo, dispatcher)

	productService := service2.NewProductService(shopRepo, aiService, storageService)

	// Controller 层
	proxyController := controller2.NewProxyController(proxyService)
	authController := controller2.NewAuthController(authService)
	shopController := controller2.NewShopController(shopService)
	productController := controller2.NewProductController(productService)

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
