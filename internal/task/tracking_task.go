package task

import (
	"context"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// ==================== 外部依赖接口 ====================

// ShipmentTracker 物流跟踪接口
type ShipmentTracker interface {
	RefreshTracking(ctx context.Context, shipmentID int64) error
	SyncToEtsy(ctx context.Context, shipmentID int64) error
}

// ==================== TrackingSyncTask 物流跟踪同步任务 ====================

// TrackingSyncTask 定时同步物流跟踪信息
type TrackingSyncTask struct {
	shipmentRepo repository.ShipmentRepository
	tracker      ShipmentTracker
	cron         *cron.Cron

	concurrencyLimit int
	sleepTime        time.Duration
}

// NewTrackingSyncTask 创建物流跟踪同步任务
func NewTrackingSyncTask(
	shipmentRepo repository.ShipmentRepository,
	tracker ShipmentTracker,
) *TrackingSyncTask {
	return &TrackingSyncTask{
		shipmentRepo:     shipmentRepo,
		tracker:          tracker,
		cron:             cron.New(cron.WithSeconds()),
		concurrencyLimit: 20,
		sleepTime:        100 * time.Millisecond,
	}
}

// Start 启动定时任务
func (t *TrackingSyncTask) Start() {
	// 物流跟踪刷新（每30分钟）
	_, err := t.cron.AddFunc("0 0/30 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		t.refreshTrackingJob(ctx)
	})
	if err != nil {
		log.Fatalf("[TrackingSyncTask] 无法启动跟踪刷新任务: %v", err)
	}

	// Etsy 同步（每15分钟）
	_, err = t.cron.AddFunc("0 0/15 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		t.syncToEtsyJob(ctx)
	})
	if err != nil {
		log.Fatalf("[TrackingSyncTask] 无法启动 Etsy 同步任务: %v", err)
	}

	t.cron.Start()
	log.Println("[TrackingSyncTask] 物流跟踪同步任务已启动")
}

// Stop 停止定时任务
func (t *TrackingSyncTask) Stop() {
	t.cron.Stop()
	log.Println("[TrackingSyncTask] 已停止")
}

// refreshTrackingJob 刷新物流跟踪信息
func (t *TrackingSyncTask) refreshTrackingJob(ctx context.Context) {
	shipments, err := t.shipmentRepo.GetPendingTrackingShipments(ctx, 100)
	if err != nil {
		log.Printf("[TrackingSyncTask] 获取待跟踪发货记录失败: %v", err)
		return
	}

	if len(shipments) == 0 {
		return
	}

	log.Printf("[TrackingSyncTask] 开始刷新 %d 条物流跟踪信息", len(shipments))

	sem := make(chan struct{}, t.concurrencyLimit)
	var wg sync.WaitGroup
	var successCount, failCount int32
	var mu sync.Mutex

	for _, shipment := range shipments {
		select {
		case <-ctx.Done():
			log.Println("[TrackingSyncTask] 跟踪刷新任务超时停止")
			return
		default:
		}

		sem <- struct{}{}
		wg.Add(1)
		time.Sleep(t.sleepTime)

		go func(s model.Shipment) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := t.tracker.RefreshTracking(ctx, s.ID); err != nil {
				log.Printf("[TrackingSyncTask] 发货 %d 跟踪刷新失败: %v", s.ID, err)
				mu.Lock()
				failCount++
				mu.Unlock()
			} else {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(shipment)
	}

	wg.Wait()
	log.Printf("[TrackingSyncTask] 跟踪刷新完成: 成功 %d, 失败 %d", successCount, failCount)
}

// syncToEtsyJob 同步发货信息到 Etsy
func (t *TrackingSyncTask) syncToEtsyJob(ctx context.Context) {
	shipments, err := t.shipmentRepo.GetPendingEtsySyncShipments(ctx, 50)
	if err != nil {
		log.Printf("[TrackingSyncTask] 获取待同步发货记录失败: %v", err)
		return
	}

	if len(shipments) == 0 {
		return
	}

	log.Printf("[TrackingSyncTask] 开始同步 %d 条发货记录到 Etsy", len(shipments))

	sem := make(chan struct{}, t.concurrencyLimit)
	var wg sync.WaitGroup
	var successCount, failCount int32
	var mu sync.Mutex

	for _, shipment := range shipments {
		select {
		case <-ctx.Done():
			log.Println("[TrackingSyncTask] Etsy 同步任务超时停止")
			return
		default:
		}

		sem <- struct{}{}
		wg.Add(1)
		time.Sleep(t.sleepTime)

		go func(s model.Shipment) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := t.tracker.SyncToEtsy(ctx, s.ID); err != nil {
				log.Printf("[TrackingSyncTask] 发货 %d 同步 Etsy 失败: %v", s.ID, err)
				mu.Lock()
				failCount++
				mu.Unlock()
			} else {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(shipment)
	}

	wg.Wait()
	log.Printf("[TrackingSyncTask] Etsy 同步完成: 成功 %d, 失败 %d", successCount, failCount)
}

// ==================== EtsySyncTask Etsy 发货同步任务 ====================

// EtsySyncTask 专门处理 Etsy 发货同步
type EtsySyncTask struct {
	shipmentRepo repository.ShipmentRepository
	orderRepo    repository.OrderRepository
	syncer       EtsyShipmentSyncer
	cron         *cron.Cron

	concurrencyLimit int
}

// EtsyShipmentSyncer Etsy 发货同步器接口
type EtsyShipmentSyncer interface {
	CreateReceipt(ctx context.Context, shopID int64, receiptID int64, trackingCode, carrierName string) error
}

// NewEtsySyncTask 创建 Etsy 同步任务
func NewEtsySyncTask(
	shipmentRepo repository.ShipmentRepository,
	orderRepo repository.OrderRepository,
	syncer EtsyShipmentSyncer,
) *EtsySyncTask {
	return &EtsySyncTask{
		shipmentRepo:     shipmentRepo,
		orderRepo:        orderRepo,
		syncer:           syncer,
		cron:             cron.New(cron.WithSeconds()),
		concurrencyLimit: 10,
	}
}

// Start 启动任务
func (t *EtsySyncTask) Start() {
	// 每10分钟检查一次
	_, err := t.cron.AddFunc("0 0/10 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		t.syncJob(ctx)
	})
	if err != nil {
		log.Fatalf("[EtsySyncTask] 无法启动任务: %v", err)
	}

	t.cron.Start()
	log.Println("[EtsySyncTask] Etsy 发货同步任务已启动 (每10分钟)")
}

// Stop 停止任务
func (t *EtsySyncTask) Stop() {
	t.cron.Stop()
	log.Println("[EtsySyncTask] 已停止")
}

// syncJob 同步任务
func (t *EtsySyncTask) syncJob(ctx context.Context) {
	shipments, err := t.shipmentRepo.GetPendingEtsySyncShipments(ctx, 50)
	if err != nil {
		log.Printf("[EtsySyncTask] 获取待同步记录失败: %v", err)
		return
	}

	if len(shipments) == 0 {
		return
	}

	log.Printf("[EtsySyncTask] 开始同步 %d 条发货记录", len(shipments))

	sem := make(chan struct{}, t.concurrencyLimit)
	var wg sync.WaitGroup

	for _, shipment := range shipments {
		select {
		case <-ctx.Done():
			return
		default:
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(s model.Shipment) {
			defer wg.Done()
			defer func() { <-sem }()

			// 获取订单信息
			order, err := t.orderRepo.GetByID(ctx, s.OrderID)
			if err != nil {
				log.Printf("[EtsySyncTask] 获取订单失败: %v", err)
				t.shipmentRepo.MarkEtsySyncFailed(ctx, s.ID, "订单不存在")
				return
			}

			// 调用 Etsy API
			err = t.syncer.CreateReceipt(ctx, order.ShopID, order.EtsyReceiptID, s.TrackingNumber, s.CarrierName)
			if err != nil {
				log.Printf("[EtsySyncTask] 同步失败: %v", err)
				t.shipmentRepo.MarkEtsySyncFailed(ctx, s.ID, err.Error())
				return
			}

			// 标记成功
			t.shipmentRepo.MarkEtsySynced(ctx, s.ID)
			log.Printf("[EtsySyncTask] 发货 %d 同步成功", s.ID)
		}(shipment)
	}

	wg.Wait()
	log.Println("[EtsySyncTask] 本轮同步完成")
}
