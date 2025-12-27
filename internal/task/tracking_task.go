package task

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
)

// ==================== 接口定义 ====================

// ShipmentTracker 物流跟踪接口
type ShipmentTracker interface {
	RefreshTracking(ctx context.Context, shipmentID int64) error
	SyncToEtsy(ctx context.Context, shipmentID int64) error
}

// ==================== TrackingSyncTask 物流跟踪同步任务 ====================

// TrackingSyncTask 物流跟踪同步定时任务
// 包含两个子任务：
//   - 刷新物流跟踪信息（每 30 分钟）
//   - 同步发货信息到 Etsy（每 15 分钟）
type TrackingSyncTask struct {
	shipmentRepo repository.ShipmentRepository
	tracker      ShipmentTracker
	cron         *cron.Cron

	// 并发控制
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

// SetConcurrency 设置并发参数
func (t *TrackingSyncTask) SetConcurrency(limit int, sleep time.Duration) {
	t.concurrencyLimit = limit
	t.sleepTime = sleep
}

// Start 启动定时任务
func (t *TrackingSyncTask) Start() {
	// 物流跟踪刷新（每 30 分钟）
	_, err := t.cron.AddFunc("0 */30 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		t.refreshTrackingJob(ctx)
	})
	if err != nil {
		log.Printf("[TrackingSyncTask] 跟踪刷新任务启动失败: %v", err)
	}

	// Etsy 同步（每 15 分钟）
	_, err = t.cron.AddFunc("0 */15 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		t.syncToEtsyJob(ctx)
	})
	if err != nil {
		log.Printf("[TrackingSyncTask] Etsy 同步任务启动失败: %v", err)
	}

	t.cron.Start()
	log.Println("[TrackingSyncTask] 已启动 (跟踪刷新每30分钟/Etsy同步每15分钟)")
}

// Stop 停止任务
func (t *TrackingSyncTask) Stop() {
	ctx := t.cron.Stop()
	<-ctx.Done()
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
	var successCount, failCount int
	var mu sync.Mutex

	for i := range shipments {
		shipment := &shipments[i]
		select {
		case <-ctx.Done():
			log.Println("[TrackingSyncTask] 跟踪刷新任务超时停止")
			wg.Wait()
			return
		default:
		}

		sem <- struct{}{}
		wg.Add(1)
		time.Sleep(t.sleepTime)

		go func(s *model.Shipment) {
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
	var successCount, failCount int
	var mu sync.Mutex

	for i := range shipments {
		shipment := &shipments[i]
		select {
		case <-ctx.Done():
			log.Println("[TrackingSyncTask] Etsy 同步任务超时停止")
			wg.Wait()
			return
		default:
		}

		sem <- struct{}{}
		wg.Add(1)
		time.Sleep(t.sleepTime)

		go func(s *model.Shipment) {
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

// ==================== 手动触发 ====================

// RefreshNow 立即刷新物流跟踪
func (t *TrackingSyncTask) RefreshNow() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		t.refreshTrackingJob(ctx)
	}()
}

// SyncToEtsyNow 立即同步到 Etsy
func (t *TrackingSyncTask) SyncToEtsyNow() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		t.syncToEtsyJob(ctx)
	}()
}
