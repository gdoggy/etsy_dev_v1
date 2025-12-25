package main

import (
	"etsy_dev_v1_202512/internal/controller"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/router"
	"etsy_dev_v1_202512/internal/service"
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
		&model.Proxy{}, &model.Developer{}, &model.DomainPool{},
		// Shop
		&model.Shop{}, &model.ShopAccount{}, &model.ShopSection{},
		// shipping
		&model.ShippingProfile{}, &model.ShippingDestination{}, &model.ShippingUpgrade{},
		// policy
		&model.ReturnPolicy{},
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
	developerRepo := repository.NewDeveloperRepo(db)
	shippingProfileRepo := repository.NewShippingProfileRepo(db)
	shippingUpgradeRepo := repository.NewShippingUpgradeRepo(db)
	shippingDestRepo := repository.NewShippingDestinationRepo(db)
	shopSectionRepo := repository.NewShopSectionRepo(db)
	returnPolicyRepo := repository.NewReturnPolicyRepo(db)
	shopRepo := repository.NewShopRepo(db)
	productRepo := repository.NewProductRepo(db)

	// Service 层
	// net
	proxyService := service.NewProxyService(proxyRepo, shopRepo)
	networkProvider := service.NewNetworkProvider(shopRepo, proxyService)

	// 调度器
	dispatcher := net.NewDispatcher(networkProvider)

	// 消费者
	aiService := service.NewAIService(aiConfig)
	storageService := service.NewStorageService()
	developerService := service.NewDeveloperService(developerRepo, shopRepo, dispatcher) // 修改此行
	shippingService := service.NewShippingProfileService(
		shippingProfileRepo,
		shippingDestRepo,
		shippingUpgradeRepo,
		shopRepo,
		developerRepo,
		dispatcher,
	)
	returnPolicyService := service.NewReturnPolicyService(
		returnPolicyRepo,
		shopRepo,
		developerRepo,
		dispatcher,
	)

	shopService := service.NewShopService(
		shopRepo,
		shopSectionRepo,
		shippingProfileRepo,
		shippingDestRepo,
		shippingUpgradeRepo,
		returnPolicyRepo,
		developerRepo,
		dispatcher,
	)
	authService := service.NewAuthService(shopService, dispatcher)

	productService := service.NewProductService(productRepo, shopRepo, aiService, storageService, dispatcher)

	// Controller 层
	proxyController := controller.NewProxyController(proxyService)
	developController := controller.NewDeveloperController(developerService)
	authController := controller.NewAuthController(authService)
	shippingController := controller.NewShippingProfileController(shippingService)
	returnPolicyController := controller.NewReturnPolicyController(returnPolicyService)
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
	router.InitRoutes(
		r,
		proxyController,
		developController,
		authController,
		shopController,
		shippingController,
		returnPolicyController,
		productController,
	)

	// 5. 启动服务
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
	log.Println("服务已启动")
}
