package task

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/service"
)

// ==================== OrderSyncTask 订单同步任务 ====================

// OrderSyncTask 订单同步定时任务
type OrderSyncTask struct {
	shopRepo     repository.ShopRepository
	orderService *service.OrderService
	cron         *cron.Cron

	// 并发控制
	concurrencyLimit int
	sleepTime        time.Duration
}

// NewOrderSyncTask 创建订单同步任务
func NewOrderSyncTask(
	shopRepo repository.ShopRepository,
	orderService *service.OrderService,
) *OrderSyncTask {
	return &OrderSyncTask{
		shopRepo:         shopRepo,
		orderService:     orderService,
		cron:             cron.New(cron.WithSeconds()),
		concurrencyLimit: 10,
		sleepTime:        200 * time.Millisecond,
	}
}

// SetConcurrency 设置并发参数
func (t *OrderSyncTask) SetConcurrency(limit int, sleep time.Duration) {
	t.concurrencyLimit = limit
	t.sleepTime = sleep
}

// Start 启动定时任务
func (t *OrderSyncTask) Start() {
	// 首次执行
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		log.Println("[OrderSyncTask] 执行首次订单同步...")
		t.syncAllShops(ctx)
	}()

	// 每 10 分钟执行
	_, err := t.cron.AddFunc("0 */10 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		t.syncAllShops(ctx)
	})
	if err != nil {
		log.Printf("[OrderSyncTask] 定时任务启动失败: %v", err)
		return
	}

	t.cron.Start()
	log.Println("[OrderSyncTask] 已启动 (每10分钟)")
}

// Stop 停止任务
func (t *OrderSyncTask) Stop() {
	ctx := t.cron.Stop()
	<-ctx.Done()
	log.Println("[OrderSyncTask] 已停止")
}

// syncAllShops 同步所有店铺的订单
func (t *OrderSyncTask) syncAllShops(ctx context.Context) {
	log.Println("[OrderSyncTask] 开始同步订单...")

	shops, _, err := t.shopRepo.List(ctx, repository.ShopFilter{
		Status:   1,
		PageSize: 1000,
	})
	if err != nil {
		log.Printf("[OrderSyncTask] 获取店铺列表失败: %v", err)
		return
	}

	if len(shops) == 0 {
		log.Println("[OrderSyncTask] 无活跃店铺需要同步")
		return
	}

	sem := make(chan struct{}, t.concurrencyLimit)
	var wg sync.WaitGroup

	var (
		totalNew     int
		totalUpdated int
		totalErrors  int
		mu           sync.Mutex
	)

	log.Printf("[OrderSyncTask] 开始处理 %d 个店铺", len(shops))

	for i := range shops {
		shop := shops[i]
		select {
		case <-ctx.Done():
			log.Println("[OrderSyncTask] 任务超时停止")
			wg.Wait()
			return
		default:
		}

		sem <- struct{}{}
		wg.Add(1)
		time.Sleep(t.sleepTime)

		go func(shopID int64, shopName string) {
			defer wg.Done()
			defer func() { <-sem }()

			resp, err := t.orderService.SyncOrders(ctx, &dto.SyncOrdersRequest{
				ShopID:    shopID,
				ForceSync: false,
			})

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				log.Printf("[OrderSyncTask] 店铺 %s(%d) 同步失败: %v", shopName, shopID, err)
				totalErrors++
				return
			}

			totalNew += resp.NewOrders
			totalUpdated += resp.UpdatedOrders

			if resp.NewOrders > 0 || resp.UpdatedOrders > 0 {
				log.Printf("[OrderSyncTask] 店铺 %s: 新增 %d, 更新 %d",
					shopName, resp.NewOrders, resp.UpdatedOrders)
			}

			for _, e := range resp.Errors {
				log.Printf("[OrderSyncTask] 店铺 %s 警告: %s", shopName, e)
			}
		}(shop.ID, shop.ShopName)
	}

	wg.Wait()
	log.Printf("[OrderSyncTask] 同步完成: 店铺 %d, 新增 %d, 更新 %d, 错误 %d",
		len(shops), totalNew, totalUpdated, totalErrors)
}

// ==================== 手动触发 ====================

// SyncShopNow 立即同步单个店铺订单
func (t *OrderSyncTask) SyncShopNow(ctx context.Context, shopID int64, forceSync bool) (*dto.SyncOrdersResponse, error) {
	return t.orderService.SyncOrders(ctx, &dto.SyncOrdersRequest{
		ShopID:    shopID,
		ForceSync: forceSync,
	})
}

// SyncAllNow 立即同步所有店铺订单
func (t *OrderSyncTask) SyncAllNow() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		t.syncAllShops(ctx)
	}()
}
