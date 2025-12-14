package main

import (
	"etsy_dev_v1_202512/core/controller"
	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/repository"
	"etsy_dev_v1_202512/core/router"
	"etsy_dev_v1_202512/core/service"
	"etsy_dev_v1_202512/pkg/database"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 初始化数据库
	// docker-compose 中配置的真实账号密码
	dsn := "host=localhost user=etsy_admin password=1234 dbname=etsy_farm port=5432 sslmode=disable"
	db := database.InitDB(dsn,
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

	// Controller 层
	authController := controller.NewAuthController(authService)

	// 3. 初始化路由
	r := gin.Default()

	// 4. 注册路由
	router.InitRoutes(r, authController)

	// --- 临时测试区域 ---
	productService := service.NewProductService(shopRepo)
	go func() {
		// 这里的 1 是您数据库里那个刚授权成功的店铺 ID
		err := productService.SyncAndSaveListings(1)
		if err != nil {
			fmt.Println(err)
		}
	}()
	// ------------------

	// 5. 启动服务
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("❌ 服务启动失败: %v", err)
	}
}
