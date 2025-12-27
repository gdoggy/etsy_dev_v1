package task

import (
	"context"
	"etsy_dev_v1_202512/internal/model"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/service"
)

// ==================== ShopSyncTask 店铺同步任务 ====================

// ShopSyncTask 店铺信息同步定时任务
// 同步内容：店铺基本信息、Section、运费模板、退货政策
type ShopSyncTask struct {
	shopRepo       repository.ShopRepository
	shopService    *service.ShopService
	profileService *service.ShippingProfileService
	policyService  *service.ReturnPolicyService
	cron           *cron.Cron

	// 并发控制
	concurrencyLimit int
	sleepTime        time.Duration

	// 同步选项
	syncProfile bool
	syncPolicy  bool
	syncSection bool
}

// NewShopSyncTask 创建店铺同步任务
func NewShopSyncTask(
	shopRepo repository.ShopRepository,
	shopService *service.ShopService,
	profileService *service.ShippingProfileService,
	policyService *service.ReturnPolicyService,
) *ShopSyncTask {
	return &ShopSyncTask{
		shopRepo:         shopRepo,
		shopService:      shopService,
		profileService:   profileService,
		policyService:    policyService,
		cron:             cron.New(cron.WithSeconds()),
		concurrencyLimit: 5,
		sleepTime:        200 * time.Millisecond,
		syncProfile:      true,
		syncPolicy:       true,
		syncSection:      true,
	}
}

// SetConcurrency 设置并发参数
func (t *ShopSyncTask) SetConcurrency(limit int, sleep time.Duration) {
	t.concurrencyLimit = limit
	t.sleepTime = sleep
}

// SetSyncOptions 设置同步选项
func (t *ShopSyncTask) SetSyncOptions(profile, policy, section bool) {
	t.syncProfile = profile
	t.syncPolicy = policy
	t.syncSection = section
}

// Start 启动定时任务
func (t *ShopSyncTask) Start() {
	// 首次执行（延迟 30 秒）
	go func() {
		time.Sleep(30 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		log.Println("[ShopSyncTask] 执行首次店铺同步...")
		t.syncAllShops(ctx)
	}()

	// 每 6 小时执行
	_, err := t.cron.AddFunc("0 0 */6 * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		t.syncAllShops(ctx)
	})
	if err != nil {
		log.Printf("[ShopSyncTask] 定时任务启动失败: %v", err)
		return
	}

	t.cron.Start()
	log.Println("[ShopSyncTask] 已启动 (每6小时)")
}

// Stop 停止任务
func (t *ShopSyncTask) Stop() {
	ctx := t.cron.Stop()
	<-ctx.Done()
	log.Println("[ShopSyncTask] 已停止")
}

// syncAllShops 同步所有店铺
func (t *ShopSyncTask) syncAllShops(ctx context.Context) {
	log.Println("[ShopSyncTask] 开始同步店铺信息...")

	shops, _, err := t.shopRepo.List(ctx, repository.ShopFilter{
		Status:   model.ShopStatusActive, // Active
		PageSize: 1000,
	})
	if err != nil {
		log.Printf("[ShopSyncTask] 获取店铺列表失败: %v", err)
		return
	}

	if len(shops) == 0 {
		log.Println("[ShopSyncTask] 无活跃店铺需要同步")
		return
	}

	sem := make(chan struct{}, t.concurrencyLimit)
	var wg sync.WaitGroup
	var successCount, failCount int
	var mu sync.Mutex

	log.Printf("[ShopSyncTask] 开始处理 %d 个店铺", len(shops))

	for i := range shops {
		shop := shops[i]
		select {
		case <-ctx.Done():
			log.Println("[ShopSyncTask] 任务超时停止")
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

			if err := t.syncSingleShop(ctx, shopID); err != nil {
				log.Printf("[ShopSyncTask] 店铺 %s(%d) 同步失败: %v", shopName, shopID, err)
				mu.Lock()
				failCount++
				mu.Unlock()
			} else {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(shop.ID, shop.ShopName)
	}

	wg.Wait()
	log.Printf("[ShopSyncTask] 同步完成: 成功 %d, 失败 %d", successCount, failCount)
}

// syncSingleShop 同步单个店铺
func (t *ShopSyncTask) syncSingleShop(ctx context.Context, shopID int64) error {
	// 1. 同步基本信息
	if _, err := t.shopService.ManualSyncShop(ctx, shopID); err != nil {
		return err
	}

	// 2. 同步 Section
	if t.syncSection {
		if err := t.shopService.SyncSectionsFromEtsy(ctx, shopID); err != nil {
			log.Printf("[ShopSyncTask] 店铺 %d Section 同步警告: %v", shopID, err)
		}
	}

	// 3. 同步运费模板
	if t.syncProfile && t.profileService != nil {
		if err := t.profileService.SyncProfilesFromEtsy(ctx, shopID); err != nil {
			log.Printf("[ShopSyncTask] 店铺 %d 运费模板同步警告: %v", shopID, err)
		}
	}

	// 4. 同步退货政策
	if t.syncPolicy && t.policyService != nil {
		if err := t.policyService.SyncPoliciesFromEtsy(ctx, shopID); err != nil {
			log.Printf("[ShopSyncTask] 店铺 %d 退货政策同步警告: %v", shopID, err)
		}
	}

	return nil
}

// ==================== 手动触发 ====================

// SyncShopNow 立即同步单个店铺
func (t *ShopSyncTask) SyncShopNow(ctx context.Context, shopID int64) error {
	return t.syncSingleShop(ctx, shopID)
}

// SyncAllNow 立即同步所有店铺
func (t *ShopSyncTask) SyncAllNow() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		t.syncAllShops(ctx)
	}()
}
