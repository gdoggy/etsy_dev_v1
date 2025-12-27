package task

import (
	"context"
	"log"
	"time"

	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/service"
)

// ==================== TaskManager 业务同步任务管理器 ====================

// TaskManager 统一管理业务同步任务
// 管理范围：Shop、Product、Order、Tracking 同步
// 不包含：Token 刷新、代理监控、分区维护（基础设施层独立管理）
type TaskManager struct {
	shopTask     *ShopSyncTask
	productTask  *ProductSyncTask
	orderTask    *OrderSyncTask
	trackingTask *TrackingSyncTask
}

// TaskManagerDeps 任务管理器依赖
type TaskManagerDeps struct {
	// Repositories
	ShopRepo     repository.ShopRepository
	ShipmentRepo repository.ShipmentRepository

	// Services
	ShopService     *service.ShopService
	ProfileService  *service.ShippingProfileService
	PolicyService   *service.ReturnPolicyService
	ProductService  *service.ProductService
	OrderService    *service.OrderService
	ShipmentService ShipmentTracker
}

// TaskManagerConfig 任务管理器配置
type TaskManagerConfig struct {
	// Shop 同步
	ShopEnabled     bool
	ShopConcurrency int
	ShopSyncProfile bool
	ShopSyncPolicy  bool
	ShopSyncSection bool

	// Product 同步
	ProductEnabled     bool
	ProductConcurrency int
	ProductBatchSize   int

	// Order 同步
	OrderEnabled     bool
	OrderConcurrency int

	// Tracking 同步
	TrackingEnabled     bool
	TrackingConcurrency int
}

// DefaultConfig 默认配置
func DefaultConfig() *TaskManagerConfig {
	return &TaskManagerConfig{
		ShopEnabled:     true,
		ShopConcurrency: 5,
		ShopSyncProfile: true,
		ShopSyncPolicy:  true,
		ShopSyncSection: true,

		ProductEnabled:     true,
		ProductConcurrency: 3,
		ProductBatchSize:   100,

		OrderEnabled:     true,
		OrderConcurrency: 10,

		TrackingEnabled:     true,
		TrackingConcurrency: 20,
	}
}

// NewTaskManager 创建任务管理器
func NewTaskManager(deps *TaskManagerDeps, cfg *TaskManagerConfig) *TaskManager {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	tm := &TaskManager{}

	// Shop 同步任务
	if cfg.ShopEnabled && deps.ShopService != nil {
		tm.shopTask = NewShopSyncTask(
			deps.ShopRepo,
			deps.ShopService,
			deps.ProfileService,
			deps.PolicyService,
		)
		tm.shopTask.SetConcurrency(cfg.ShopConcurrency, 200*time.Millisecond)
		tm.shopTask.SetSyncOptions(cfg.ShopSyncProfile, cfg.ShopSyncPolicy, cfg.ShopSyncSection)
	}

	// Product 同步任务
	if cfg.ProductEnabled && deps.ProductService != nil {
		tm.productTask = NewProductSyncTask(deps.ShopRepo, deps.ProductService)
		tm.productTask.SetConcurrency(cfg.ProductConcurrency, cfg.ProductBatchSize, 300*time.Millisecond)
	}

	// Order 同步任务
	if cfg.OrderEnabled && deps.OrderService != nil {
		tm.orderTask = NewOrderSyncTask(deps.ShopRepo, deps.OrderService)
		tm.orderTask.SetConcurrency(cfg.OrderConcurrency, 200*time.Millisecond)
	}

	// Tracking 同步任务
	if cfg.TrackingEnabled && deps.ShipmentService != nil {
		tm.trackingTask = NewTrackingSyncTask(deps.ShipmentRepo, deps.ShipmentService)
		tm.trackingTask.SetConcurrency(cfg.TrackingConcurrency, 100*time.Millisecond)
	}

	return tm
}

// ==================== 生命周期管理 ====================

// Start 启动所有任务
func (tm *TaskManager) Start() {
	log.Println("[TaskManager] 正在启动业务同步任务...")

	if tm.shopTask != nil {
		tm.shopTask.Start()
	}
	if tm.productTask != nil {
		tm.productTask.Start()
	}
	if tm.orderTask != nil {
		tm.orderTask.Start()
	}
	if tm.trackingTask != nil {
		tm.trackingTask.Start()
	}

	log.Println("[TaskManager] 业务同步任务已全部启动")
}

// Stop 停止所有任务
func (tm *TaskManager) Stop() {
	log.Println("[TaskManager] 正在停止业务同步任务...")

	if tm.shopTask != nil {
		tm.shopTask.Stop()
	}
	if tm.productTask != nil {
		tm.productTask.Stop()
	}
	if tm.orderTask != nil {
		tm.orderTask.Stop()
	}
	if tm.trackingTask != nil {
		tm.trackingTask.Stop()
	}

	log.Println("[TaskManager] 业务同步任务已全部停止")
}

// ==================== 手动触发接口 ====================

// TriggerShopSync 触发店铺同步
func (tm *TaskManager) TriggerShopSync(ctx context.Context, shopID int64) error {
	if tm.shopTask == nil {
		return ErrTaskDisabled
	}
	return tm.shopTask.SyncShopNow(ctx, shopID)
}

// TriggerAllShopsSync 触发所有店铺同步
func (tm *TaskManager) TriggerAllShopsSync() {
	if tm.shopTask != nil {
		tm.shopTask.SyncAllNow()
	}
}

// TriggerProductSync 触发商品同步
func (tm *TaskManager) TriggerProductSync(ctx context.Context, shopID int64, fullSync bool) error {
	if tm.productTask == nil {
		return ErrTaskDisabled
	}
	return tm.productTask.SyncShopNow(ctx, shopID, fullSync)
}

// TriggerAllProductsSync 触发所有商品同步
func (tm *TaskManager) TriggerAllProductsSync(fullSync bool) {
	if tm.productTask != nil {
		tm.productTask.SyncAllNow(fullSync)
	}
}

// TriggerOrderSync 触发订单同步
func (tm *TaskManager) TriggerOrderSync(ctx context.Context, shopID int64, forceSync bool) (*dto.SyncOrdersResponse, error) {
	if tm.orderTask == nil {
		return nil, ErrTaskDisabled
	}
	return tm.orderTask.SyncShopNow(ctx, shopID, forceSync)
}

// TriggerAllOrdersSync 触发所有订单同步
func (tm *TaskManager) TriggerAllOrdersSync() {
	if tm.orderTask != nil {
		tm.orderTask.SyncAllNow()
	}
}

// TriggerTrackingRefresh 触发物流跟踪刷新
func (tm *TaskManager) TriggerTrackingRefresh() {
	if tm.trackingTask != nil {
		tm.trackingTask.RefreshNow()
	}
}

// ==================== 状态查询 ====================

// Status 获取任务状态
func (tm *TaskManager) Status() map[string]bool {
	return map[string]bool{
		"shop":     tm.shopTask != nil,
		"product":  tm.productTask != nil,
		"order":    tm.orderTask != nil,
		"tracking": tm.trackingTask != nil,
	}
}

// ==================== 错误定义 ====================

type TaskError string

func (e TaskError) Error() string { return string(e) }

const (
	ErrTaskDisabled TaskError = "task is disabled"
)
