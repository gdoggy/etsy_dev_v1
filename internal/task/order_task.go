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
	orderService *service.OrderService
	shopRepo     repository.ShopRepository
	cron         *cron.Cron

	// 并发控制
	concurrencyLimit int
	sleepTime        time.Duration
}

// NewOrderSyncTask 创建订单同步任务
func NewOrderSyncTask(orderService *service.OrderService, shopRepo repository.ShopRepository) *OrderSyncTask {
	return &OrderSyncTask{
		orderService:     orderService,
		shopRepo:         shopRepo,
		cron:             cron.New(cron.WithSeconds()),
		concurrencyLimit: 10,                     // 订单同步并发上限
		sleepTime:        200 * time.Millisecond, // 协程启动间隔
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
		log.Println("[OrderSyncTask] 服务启动，正在执行首次订单同步...")
		t.syncAllShops(ctx)
	}()

	// 定时策略：每10分钟执行
	_, err := t.cron.AddFunc("0 */10 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		t.syncAllShops(ctx)
	})

	if err != nil {
		log.Fatalf("[OrderSyncTask] 无法启动定时任务: %v", err)
	}

	t.cron.Start()
	log.Println("[OrderSyncTask] 订单同步任务已启动 (每10分钟)")
}

// Stop 停止任务
func (t *OrderSyncTask) Stop() {
	ctx := t.cron.Stop()
	<-ctx.Done()
	log.Println("[OrderSyncTask] 已停止")
}

// syncAllShops 同步所有店铺的订单（并发控制）
func (t *OrderSyncTask) syncAllShops(ctx context.Context) {
	log.Println("[OrderSyncTask] 开始同步订单...")

	// 获取所有活跃店铺
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

	// 信号量控制并发
	sem := make(chan struct{}, t.concurrencyLimit)
	var wg sync.WaitGroup

	// 统计结果
	var (
		totalNew     int
		totalUpdated int
		totalErrors  int
		mu           sync.Mutex
	)

	log.Printf("[OrderSyncTask] 开始处理 %d 个店铺，并发上限: %d", len(shops), t.concurrencyLimit)

	for _, shop := range shops {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			log.Println("[OrderSyncTask] 任务超时停止")
			wg.Wait()
			return
		default:
		}

		// 获取信号量
		sem <- struct{}{}
		wg.Add(1)

		// 平滑波峰
		time.Sleep(t.sleepTime)

		// 避免循环变量捕获
		currentShop := shop

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

			// 记录同步警告
			for _, e := range resp.Errors {
				log.Printf("[OrderSyncTask] 店铺 %s 警告: %s", shopName, e)
			}
		}(currentShop.ID, currentShop.ShopName)
	}

	wg.Wait()
	log.Printf("[OrderSyncTask] 同步完成，店铺: %d, 新增: %d, 更新: %d, 错误: %d",
		len(shops), totalNew, totalUpdated, totalErrors)
}

// ==================== 手动触发接口 ====================

// SyncShopOrders 手动触发单个店铺订单同步
func (t *OrderSyncTask) SyncShopOrders(ctx context.Context, shopID int64, forceSync bool) (*dto.SyncOrdersResponse, error) {
	return t.orderService.SyncOrders(ctx, &dto.SyncOrdersRequest{
		ShopID:    shopID,
		ForceSync: forceSync,
	})
}

// SyncAllShopsNow 立即同步所有店铺（手动触发）
func (t *OrderSyncTask) SyncAllShopsNow() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		t.syncAllShops(ctx)
	}()
}
