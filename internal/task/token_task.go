package task

import (
	"context"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/service"
	"log"
	"sync"
	"time"

	"etsy_dev_v1_202512/internal/repository"

	"github.com/robfig/cron/v3"
)

type TokenTask struct {
	ShopRepo    *repository.ShopRepo
	AuthService *service.AuthService
	Cron        *cron.Cron

	// 控制并发探测的数量，防止把本地带宽打满
	concurrencyLimit int
	sleepTime        time.Duration
}

func NewTokenTask(shopRepo *repository.ShopRepo, authService *service.AuthService) *TokenTask {
	return &TokenTask{
		ShopRepo:         shopRepo,
		AuthService:      authService,
		Cron:             cron.New(cron.WithSeconds()), // 支持秒级控制
		concurrencyLimit: 50,                           // 稍微调低并发，给其他业务让路
		sleepTime:        50 * time.Millisecond,        // 每个协程启动间隔，平滑波峰
	}
}

// Start 启动定时任务
func (t *TokenTask) Start() {
	// 首次执行
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		log.Println("[Task] 服务启动，正在执行首次 Token 检查...")
		t.refreshJob(ctx)
	}()

	// 定时策略
	_, err := t.Cron.AddFunc("0 0/40 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		t.refreshJob(ctx)
	})

	if err != nil {
		log.Fatalf("无法启动 Token 定时任务: %v", err)
	}

	t.Cron.Start()
	log.Println("Token 保活任务已启动 (每40分钟检查一次)")
}

// 自动刷新逻辑
func (t *TokenTask) refreshJob(ctx context.Context) {
	shops, err := t.ShopRepo.FindExpiringShops(ctx)
	if err != nil {
		log.Printf("[Cron] 店铺过期状态查询失败: %v", err)
		return
	}

	// 1. 定义信号量通道，容量即为并发上限
	sem := make(chan struct{}, t.concurrencyLimit)
	var wg sync.WaitGroup

	log.Printf("[Cron] 开始处理 %d 个店铺的 Token 刷新，并发上限: %d", len(shops), t.concurrencyLimit)

	for _, shop := range shops {
		// 检查上下文是否已取消（超时处理）
		select {
		case <-ctx.Done():
			log.Println("[Cron] 任务超时停止")
			return
		default:
		}

		// 2. 获取信号量（如果已满则阻塞在此，起到限流作用）
		sem <- struct{}{}
		wg.Add(1)

		// 3. 平滑波峰
		time.Sleep(t.sleepTime)

		// 4. 解决循环变量捕获问题 (Go 常见坑)
		currentShop := shop

		go func(s model.Shop) {
			defer wg.Done()
			defer func() { <-sem }() // 任务结束释放信号量

			// 执行核心业务
			err = t.AuthService.RefreshAccessToken(ctx, &s)
			if err != nil {
				// 日志仅记录，不中断其他协程
				log.Printf("[Cron] 店铺 [%s] 刷新失败: %v", s.ShopName, err)
			} else {
				// 成功日志（可选，调试用）
				// log.Printf("[Cron] 店铺 [%s] 刷新成功", s.ShopName)
			}
		}(currentShop)
	}

	// 5. 等待所有 Goroutine 完成
	wg.Wait()
	log.Println("[Cron] 本轮 Token 刷新任务完成")
}
