package task

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/service"
)

// ==================== ProductSyncTask 商品同步任务 ====================

// ProductSyncTask 商品同步定时任务
// 同步策略：
//   - 增量同步：每 30 分钟，基于 EtsyLastModifiedTS 筛选
//   - 全量同步：每日凌晨 3 点
type ProductSyncTask struct {
	shopRepo       repository.ShopRepository
	productService *service.ProductService
	cron           *cron.Cron

	// 并发控制
	concurrencyLimit int
	batchSize        int
	sleepTime        time.Duration
}

// NewProductSyncTask 创建商品同步任务
func NewProductSyncTask(
	shopRepo repository.ShopRepository,
	productService *service.ProductService,
) *ProductSyncTask {
	return &ProductSyncTask{
		shopRepo:         shopRepo,
		productService:   productService,
		cron:             cron.New(cron.WithSeconds()),
		concurrencyLimit: 3,
		batchSize:        100,
		sleepTime:        300 * time.Millisecond,
	}
}

// SetConcurrency 设置并发参数
func (t *ProductSyncTask) SetConcurrency(limit, batchSize int, sleep time.Duration) {
	t.concurrencyLimit = limit
	t.batchSize = batchSize
	t.sleepTime = sleep
}

// Start 启动定时任务
func (t *ProductSyncTask) Start() {
	// 首次执行（延迟 60 秒，等待店铺同步完成）
	go func() {
		time.Sleep(60 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		log.Println("[ProductSyncTask] 执行首次商品同步...")
		t.syncAllShops(ctx, false)
	}()

	// 增量同步：每 30 分钟
	_, _ = t.cron.AddFunc("0 */30 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()
		t.syncAllShops(ctx, false)
	})

	// 全量同步：每日凌晨 3 点
	_, _ = t.cron.AddFunc("0 0 3 * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
		defer cancel()
		log.Println("[ProductSyncTask] 开始每日全量商品同步...")
		t.syncAllShops(ctx, true)
	})

	t.cron.Start()
	log.Println("[ProductSyncTask] 已启动 (增量每30分钟/全量每日3点)")
}

// Stop 停止任务
func (t *ProductSyncTask) Stop() {
	ctx := t.cron.Stop()
	<-ctx.Done()
	log.Println("[ProductSyncTask] 已停止")
}

// syncAllShops 同步所有店铺的商品
func (t *ProductSyncTask) syncAllShops(ctx context.Context, fullSync bool) {
	syncType := "增量"
	if fullSync {
		syncType = "全量"
	}
	log.Printf("[ProductSyncTask] 开始%s商品同步...", syncType)

	shops, _, err := t.shopRepo.List(ctx, repository.ShopFilter{
		Status:   1,
		PageSize: 1000,
	})
	if err != nil {
		log.Printf("[ProductSyncTask] 获取店铺列表失败: %v", err)
		return
	}

	if len(shops) == 0 {
		log.Println("[ProductSyncTask] 无活跃店铺需要同步")
		return
	}

	sem := make(chan struct{}, t.concurrencyLimit)
	var wg sync.WaitGroup

	var (
		successCount int
		failCount    int
		totalNew     int
		totalUpdated int
		mu           sync.Mutex
	)

	log.Printf("[ProductSyncTask] 开始处理 %d 个店铺", len(shops))

	for i := range shops {
		shop := shops[i]
		select {
		case <-ctx.Done():
			log.Println("[ProductSyncTask] 任务超时停止")
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

			newCount, updatedCount, err := t.syncShopProducts(ctx, shopID)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				log.Printf("[ProductSyncTask] 店铺 %s(%d) 同步失败: %v", shopName, shopID, err)
				failCount++
			} else {
				successCount++
				totalNew += newCount
				totalUpdated += updatedCount
				if newCount > 0 || updatedCount > 0 {
					log.Printf("[ProductSyncTask] 店铺 %s: 新增 %d, 更新 %d", shopName, newCount, updatedCount)
				}
			}
		}(shop.ID, shop.ShopName)
	}

	wg.Wait()
	log.Printf("[ProductSyncTask] %s同步完成: 店铺成功 %d, 失败 %d, 新增商品 %d, 更新商品 %d",
		syncType, successCount, failCount, totalNew, totalUpdated)
}

// syncShopProducts 同步单个店铺商品
func (t *ProductSyncTask) syncShopProducts(ctx context.Context, shopID int64) (newCount, updatedCount int, err error) {
	// 调用 Service 层同步
	err = t.productService.SyncListingsFromEtsy(ctx, shopID)
	// TODO: Service 层返回同步统计信息
	return 0, 0, err
}

// ==================== 手动触发 ====================

// SyncShopNow 立即同步单个店铺商品
func (t *ProductSyncTask) SyncShopNow(ctx context.Context, shopID int64, fullSync bool) error {
	_, _, err := t.syncShopProducts(ctx, shopID)
	return err
}

// SyncAllNow 立即同步所有店铺商品
func (t *ProductSyncTask) SyncAllNow(fullSync bool) {
	go func() {
		timeout := 1 * time.Hour
		if fullSync {
			timeout = 4 * time.Hour
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		t.syncAllShops(ctx, fullSync)
	}()
}
