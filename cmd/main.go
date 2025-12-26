package main

import (
	"context"
	"errors"
	"etsy_dev_v1_202512/internal/controller"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/router"
	"etsy_dev_v1_202512/internal/service"
	"etsy_dev_v1_202512/internal/task"
	"etsy_dev_v1_202512/pkg/database"
	"etsy_dev_v1_202512/pkg/net"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	// 1. 初始化数据库
	db := initDatabase()

	// 2. 初始化依赖
	deps := initDependencies(db)

	// 3. 启动定时任务
	initTasks(deps)

	// 4. 初始化路由
	r := router.SetupRouter(deps.Controllers)

	// 5. 启动服务
	startServer(r)
}

// ==================== 依赖容器 ====================

// Dependencies 依赖容器
type Dependencies struct {
	DB          *gorm.DB
	Repos       *Repositories
	Dispatcher  net.Dispatcher
	Controllers *router.Controllers
	Services    *Services
}

// Repositories 仓库集合
type Repositories struct {
	Proxy           repository.ProxyRepository
	Developer       repository.DeveloperRepository
	Shop            repository.ShopRepository
	ShopSection     repository.ShopSectionRepository
	ShippingProfile repository.ShippingProfileRepository
	ShippingDest    repository.ShippingDestinationRepository
	ShippingUpgrade repository.ShippingUpgradeRepository
	ReturnPolicy    repository.ReturnPolicyRepository
	Product         repository.ProductRepository
	DraftUow        *repository.DraftUnitOfWork
	DraftTask       repository.DraftTaskRepository
	DraftProduct    repository.DraftProductRepository
	DraftImage      repository.DraftImageRepository
	AiCallLog       repository.AICallLogRepository
}

// Services 服务集合
type Services struct {
	Proxy        *service.ProxyService
	Developer    *service.DeveloperService
	Auth         *service.AuthService
	Shop         *service.ShopService
	Shipping     *service.ShippingProfileService
	ReturnPolicy *service.ReturnPolicyService
	Product      *service.ProductService
	Draft        *service.DraftService
	Storage      *service.StorageService
	AI           *service.AIService
	OneBound     *service.OneBoundService
}

// ==================== 初始化函数 ====================

// initDatabase 初始化数据库
func initDatabase() *gorm.DB {
	return database.InitDB(
		// Manager
		&model.SysUser{}, &model.ShopMember{},
		// Account
		&model.Proxy{}, &model.Developer{}, &model.DomainPool{},
		// Shop
		&model.Shop{}, &model.ShopAccount{}, &model.ShopSection{},
		// Shipping
		&model.ShippingProfile{}, &model.ShippingDestination{}, &model.ShippingUpgrade{},
		// Policy
		&model.ReturnPolicy{},
		// Product
		&model.Product{}, &model.ProductImage{}, &model.ProductVariant{},
		// Draft
		&model.DraftTask{}, &model.DraftProduct{}, &model.DraftImage{},
	)
}

// initDependencies 初始化所有依赖
func initDependencies(db *gorm.DB) *Dependencies {
	// -------- Repo 层 --------
	repos := initRepositories(db)

	// -------- 基础服务 --------
	proxyService := service.NewProxyService(repos.Proxy, repos.Shop)
	networkProvider := service.NewNetworkProvider(repos.Shop, proxyService)
	dispatcher := net.NewDispatcher(networkProvider)

	// -------- 存储 & AI 服务 --------
	storageSvc := initStorageService()
	aiSvc := service.NewAIService(&service.AIConfig{
		ApiKey: getEnv("GEMINI_API_KEY", ""),
	}, storageSvc, repos.AiCallLog)
	oneBoundSvc := service.NewOneBoundService(&service.OneBoundConfig{
		APIKey:    getEnv("ONEBOUND_API_KEY", ""),
		APISecret: getEnv("ONEBOUND_API_SECRET", ""),
	})

	// -------- 业务服务 --------
	services := &Services{
		Proxy:    proxyService,
		Storage:  storageSvc,
		AI:       aiSvc,
		OneBound: oneBoundSvc,
	}

	services.Developer = service.NewDeveloperService(repos.Developer, repos.Shop, dispatcher)
	services.Shipping = service.NewShippingProfileService(
		repos.ShippingProfile, repos.ShippingDest, repos.ShippingUpgrade,
		repos.Shop, repos.Developer, dispatcher,
	)
	services.ReturnPolicy = service.NewReturnPolicyService(
		repos.ReturnPolicy, repos.Shop, repos.Developer, dispatcher,
	)
	services.Shop = service.NewShopService(
		repos.Shop, repos.ShopSection,
		repos.ShippingProfile, repos.ShippingDest, repos.ShippingUpgrade,
		repos.ReturnPolicy, repos.Developer, dispatcher,
	)
	services.Auth = service.NewAuthService(services.Shop, dispatcher)
	services.Product = service.NewProductService(repos.Product, repos.Shop, aiSvc, storageSvc, dispatcher)
	services.Draft = service.NewDraftService(repos.DraftUow, repos.Shop, oneBoundSvc, aiSvc, storageSvc)

	// -------- Controller 层 --------
	controllers := initControllers(services)

	return &Dependencies{
		DB:          db,
		Repos:       repos,
		Dispatcher:  dispatcher,
		Controllers: controllers,
		Services:    services,
	}
}

// initRepositories 初始化所有仓库
func initRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Proxy:           repository.NewProxyRepository(db),
		Developer:       repository.NewDeveloperRepository(db),
		Shop:            repository.NewShopRepository(db),
		ShopSection:     repository.NewShopSectionRepository(db),
		ShippingProfile: repository.NewShippingProfileRepository(db),
		ShippingDest:    repository.NewShippingDestinationRepository(db),
		ShippingUpgrade: repository.NewShippingUpgradeRepository(db),
		ReturnPolicy:    repository.NewReturnPolicyRepository(db),
		Product:         repository.NewProductRepository(db),
		DraftUow:        repository.NewDraftUnitOfWork(db),
		DraftTask:       repository.NewDraftTaskRepository(db),
		DraftProduct:    repository.NewDraftProductRepository(db),
		DraftImage:      repository.NewDraftImageRepository(db),
		AiCallLog:       repository.NewAICallLogRepository(db),
	}
}

// initStorageService 初始化存储服务
func initStorageService() *service.StorageService {
	storageSvc, err := service.NewStorageService(&service.StorageConfig{
		Provider:  getEnv("STORAGE_PROVIDER", "s3"),
		Bucket:    getEnv("AWS_BUCKET", ""),
		Region:    getEnv("AWS_REGION", ""),
		AccessKey: getEnv("AWS_ACCESS_KEY_ID", ""),
		SecretKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		CDNDomain: getEnv("AWS_CDN_DOMAIN", ""),
		BasePath:  getEnv("STORAGE_BASE_PATH", "etsy-erp"),
	})
	if err != nil {
		log.Printf("警告: 存储服务初始化失败: %v", err)
		return nil
	}
	return storageSvc
}

// initControllers 初始化所有控制器
func initControllers(svc *Services) *router.Controllers {
	return &router.Controllers{
		Proxy:        controller.NewProxyController(svc.Proxy),
		Developer:    controller.NewDeveloperController(svc.Developer),
		Auth:         controller.NewAuthController(svc.Auth),
		Shop:         controller.NewShopController(svc.Shop),
		Shipping:     controller.NewShippingProfileController(svc.Shipping),
		ReturnPolicy: controller.NewReturnPolicyController(svc.ReturnPolicy),
		Product:      controller.NewProductController(svc.Product),
		Draft:        controller.NewDraftController(svc.Draft),
	}
}

// ==================== 定时任务 ====================

// initTasks 初始化定时任务
func initTasks(deps *Dependencies) {
	// 代理监控
	proxyMonitor := task.NewProxyMonitor(
		deps.Services.Proxy.ProxyRepo,
		deps.Services.Proxy,
	)
	proxyMonitor.Start()

	// Token 刷新
	tokenTask := task.NewTokenTask(
		deps.Repos.Shop,
		deps.Services.Auth,
	)
	tokenTask.Start()

	// 草稿清理
	if deps.Services.Storage != nil {
		cleanupTask := task.NewDraftCleanupTask(
			deps.Repos.DraftTask,
			deps.Repos.DraftProduct,
			deps.Repos.DraftImage,
			deps.Services.Storage,
		)
		cleanupTask.Start()
	}

	log.Println("定时任务已启动")
}

// ==================== 服务启动 ====================

// startServer 启动服务
func startServer(r *gin.Engine) {
	port := getEnv("SERVER_PORT", "8080")

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// 异步启动服务
	go func() {
		log.Printf("服务启动在 :%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务...")

	// 优雅关闭，最多等待 10 秒
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("服务强制关闭: %v", err)
	}

	log.Println("服务已退出")
}

// ==================== 工具函数 ====================

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
