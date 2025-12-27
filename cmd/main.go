package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"etsy_dev_v1_202512/internal/controller"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/router"
	"etsy_dev_v1_202512/internal/service"
	"etsy_dev_v1_202512/internal/task"
	"etsy_dev_v1_202512/pkg/database"
	"etsy_dev_v1_202512/pkg/net"
)

func main() {
	// 1. 初始化数据库
	db := database.InitDatabase()

	// 2. 初始化依赖
	deps := initDependencies(db)

	// 4. 启动业务同步任务
	startInfraTasks(deps)
	deps.TaskManager.Start()

	// 4. 初始化路由
	r := router.SetupRouter(deps.Controllers)

	// 5. 启动服务
	startServer(r, deps.TaskManager)
}

// ==================== 依赖容器 ====================

// Dependencies 依赖容器
type Dependencies struct {
	DB          *gorm.DB
	Repos       *Repositories
	Dispatcher  net.Dispatcher
	Controllers *router.Controllers
	Services    *Services
	TaskManager *task.TaskManager
}

// Repositories 仓库集合
type Repositories struct {
	User            repository.UserRepository
	ShopMember      repository.ShopMemberRepository
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
	Order           repository.OrderRepository
	OrderItem       repository.OrderItemRepository
	Shipment        repository.ShipmentRepository
	TrackingEvent   repository.TrackingEventRepository
	AiCallLog       repository.AICallLogRepository
}

// Services 服务集合
type Services struct {
	User         *service.UserService
	Proxy        *service.ProxyService
	Developer    *service.DeveloperService
	Auth         *service.AuthService
	Shop         *service.ShopService
	Shipping     *service.ShippingProfileService
	ReturnPolicy *service.ReturnPolicyService
	Product      *service.ProductService
	Draft        *service.DraftService
	Order        *service.OrderService
	Shipment     *service.ShipmentService
	Karrio       *service.KarrioClient
	Storage      *service.StorageService
	AI           *service.AIService
	OneBound     *service.OneBoundService
}

// ==================== 初始化函数 ====================

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

	// -------- Karrio 客户端 --------
	karrioClient := initKarrioClient()

	// -------- 业务服务 --------
	services := &Services{
		Proxy:    proxyService,
		Storage:  storageSvc,
		AI:       aiSvc,
		OneBound: oneBoundSvc,
		Karrio:   karrioClient,
	}

	services.User = service.NewUserService(repos.User)
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
		repos.ReturnPolicy, repos.Developer, dispatcher, repos.Proxy,
	)
	services.Auth = service.NewAuthService(services.Shop, dispatcher)
	services.Product = service.NewProductService(repos.Product, repos.Shop, aiSvc, storageSvc, dispatcher)
	services.Draft = service.NewDraftService(repos.DraftUow, repos.Shop, oneBoundSvc, aiSvc, storageSvc)
	services.Order = service.NewOrderService(
		repos.Order, repos.OrderItem, repos.Shipment, repos.Shop, dispatcher,
	)
	services.Shipment = service.NewShipmentService(
		repos.Shipment, repos.TrackingEvent, repos.Order, repos.Shop,
		karrioClient, nil, // EtsyShipmentSyncer 可后续实现
	)

	// -------- TaskManager（业务同步任务）--------
	taskManager := initTaskManager(repos, services)
	// -------- Controller 层 --------
	controllers := initControllers(services, taskManager)

	return &Dependencies{
		DB:          db,
		Repos:       repos,
		Dispatcher:  dispatcher,
		Controllers: controllers,
		Services:    services,
		TaskManager: taskManager,
	}
}

// initRepositories 初始化所有仓库
func initRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		User:            repository.NewUserRepository(db),
		ShopMember:      repository.NewShopMemberRepository(db),
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
		Order:           repository.NewOrderRepository(db),
		OrderItem:       repository.NewOrderItemRepository(db),
		Shipment:        repository.NewShipmentRepository(db),
		TrackingEvent:   repository.NewTrackingEventRepository(db),
		AiCallLog:       repository.NewAICallLogRepository(db),
	}
}

// initStorageService 初始化存储服务
func initStorageService() *service.StorageService {
	storageSvc, err := service.NewStorageService(&service.StorageConfig{
		Provider:  getEnv("STORAGE_PROVIDER", "s3"),
		Endpoint:  getEnv("STORAGE_ENDPOINT", "https://s3.amazonaws.com"),
		Region:    getEnv("STORAGE_REGION", ""),
		AccessKey: getEnv("STORAGE_ACCESS_KEY", ""),
		SecretKey: getEnv("STORAGE_SECRET_KEY", ""),
		Bucket:    getEnv("STORAGE_BUCKET", ""),
		CDNDomain: getEnv("STORAGE_CDN_DOMAIN", ""),
		BasePath:  getEnv("STORAGE_BASE_PATH", "etsy-erp"),
	})
	if err != nil {
		log.Printf("警告: 存储服务初始化失败: %v", err)
		return nil
	}
	return storageSvc
}

// initKarrioClient 初始化 Karrio 客户端
func initKarrioClient() *service.KarrioClient {
	baseURL := getEnv("KARRIO_BASE_URL", "")
	apiKey := getEnv("KARRIO_API_KEY", "")

	if baseURL == "" {
		log.Println("警告: KARRIO_BASE_URL 未配置，Karrio 客户端未初始化")
		return nil
	}

	return service.NewKarrioClient(&service.KarrioConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Timeout: 30 * time.Second,
	})
}

// initControllers 初始化所有控制器
func initControllers(svc *Services, taskManager *task.TaskManager) *router.Controllers {
	return &router.Controllers{
		User:         controller.NewUserController(svc.User),
		Proxy:        controller.NewProxyController(svc.Proxy),
		Developer:    controller.NewDeveloperController(svc.Developer),
		Auth:         controller.NewAuthController(svc.Auth),
		Shop:         controller.NewShopController(svc.Shop),
		Shipping:     controller.NewShippingProfileController(svc.Shipping),
		ReturnPolicy: controller.NewReturnPolicyController(svc.ReturnPolicy),
		Product:      controller.NewProductController(svc.Product),
		Draft:        controller.NewDraftController(svc.Draft),
		Order:        controller.NewOrderController(svc.Order),
		Shipment:     controller.NewShipmentController(svc.Shipment),
		Karrio:       controller.NewKarrioController(svc.Karrio),
		Sync:         controller.NewSyncController(taskManager),
	}
}

// ==================== 定时任务 ====================
// initTaskManager 创建业务同步任务管理器
func initTaskManager(repos *Repositories, services *Services) *task.TaskManager {
	return task.NewTaskManager(
		&task.TaskManagerDeps{
			// Repositories
			ShopRepo:     repos.Shop,
			ShipmentRepo: repos.Shipment,

			// Services
			ShopService:     services.Shop,
			ProfileService:  services.Shipping,
			PolicyService:   services.ReturnPolicy,
			ProductService:  services.Product,
			OrderService:    services.Order,
			ShipmentService: services.Shipment,
		},
		&task.TaskManagerConfig{
			// Shop 同步
			ShopEnabled:     true,
			ShopConcurrency: 5,
			ShopSyncProfile: true,
			ShopSyncPolicy:  true,
			ShopSyncSection: true,

			// Product 同步
			ProductEnabled:     true,
			ProductConcurrency: 3,
			ProductBatchSize:   100,

			// Order 同步
			OrderEnabled:     true,
			OrderConcurrency: 10,

			// Tracking 同步
			TrackingEnabled:     services.Shipment != nil,
			TrackingConcurrency: 20,
		},
	)
}

// startInfraTasks 启动基础设施层任务
func startInfraTasks(deps *Dependencies) {
	// 1. 分区维护任务
	if init := database.Global(); init != nil {
		partitionTask := database.NewPartitionTask(
			init.GetManager(),
			database.WithFutureMonths(6),
			database.WithInterval(100*24*time.Hour),
		)
		partitionTask.Start()
	}

	// 2. 代理监控任务
	proxyMonitor := task.NewProxyMonitor(
		deps.Services.Proxy.ProxyRepo,
		deps.Services.Proxy,
	)
	proxyMonitor.Start()

	// 3. Token 刷新任务（其他任务依赖有效 Token）
	tokenTask := task.NewTokenTask(
		deps.Repos.Shop,
		deps.Services.Auth,
	)
	tokenTask.Start()

	// 4. 草稿清理任务
	if deps.Services.Storage != nil {
		cleanupTask := task.NewDraftCleanupTask(
			deps.Repos.DraftTask,
			deps.Repos.DraftProduct,
			deps.Repos.DraftImage,
			deps.Services.Storage,
		)
		cleanupTask.Start()
	}

	log.Println("[Tasks] 基础设施层任务已启动")
}

// ==================== 服务启动 ====================

// startServer 启动服务
func startServer(r *gin.Engine, taskManager *task.TaskManager) {
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

	// 停止业务同步任务
	if taskManager != nil {
		taskManager.Stop()
	}

	// 优雅关闭 HTTP 服务，最多等待 30 秒
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
