package main

import (
	"etsy_dev_v1_202512/internal/api/handler"
	"etsy_dev_v1_202512/internal/core/model"
	"etsy_dev_v1_202512/internal/core/service"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/router"
	"etsy_dev_v1_202512/internal/task"
	"etsy_dev_v1_202512/pkg/database"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 初始化数据库
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
	shopRepo := repository.NewShopRepo(db)

	// Service 层
	aiService := service.NewAIService(aiConfig)
	storageService := service.NewStorageService()
	authService := service.NewAuthService(shopRepo)
	productService := service.NewProductService(shopRepo, aiService, storageService)

	// Controller 层
	authController := handler.NewAuthController(authService)
	productController := handler.NewProductController(productService)

	// Task 层
	tokenTask := task.NewTokenTask(shopRepo, authService)
	tokenTask.Start()

	// 3. 初始化路由
	r := gin.Default()

	// 4. 注册路由
	router.InitRoutes(r, authController, productController)

	// 5. 启动服务
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
