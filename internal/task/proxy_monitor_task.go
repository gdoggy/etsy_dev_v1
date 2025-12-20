package task

import (
	"context"
	"etsy_dev_v1_202512/internal/core/model"
	"etsy_dev_v1_202512/internal/core/service"
	"etsy_dev_v1_202512/internal/repository"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// ProxyMonitor 代理巡检任务
type ProxyMonitor struct {
	proxyRepo    *repository.ProxyRepo
	proxyService *service.ProxyService
	Cron         *cron.Cron

	// 控制并发探测的数量，防止把本地带宽打满
	concurrencyLimit int
	sleepTime        time.Duration
}

func NewProxyMonitor(proxyRepo *repository.ProxyRepo, proxyService *service.ProxyService) *ProxyMonitor {
	return &ProxyMonitor{
		proxyRepo:        proxyRepo,
		proxyService:     proxyService,
		Cron:             cron.New(cron.WithSeconds()), // 支持秒级控制
		concurrencyLimit: 100,                          // 稍微调低并发，给其他业务让路
		sleepTime:        50 * time.Millisecond,        // 每个协程启动间隔，平滑波峰
	}
}

// Start 启动代理巡检任务
func (m *ProxyMonitor) Start() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		log.Println("[ProxyMonitor] 服务启动，正在执行首次巡检...")
		m.Execute(ctx)
	}()

	// 策略：每 15 分钟巡检一次
	// Cron: "0 0/15 * * * *"
	_, err := m.Cron.AddFunc("0 0/15 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		m.Execute(ctx)
	})

	if err != nil {
		log.Fatalf("无法启动 ProxyMonitor: %v", err)
	}

	m.Cron.Start()
	log.Println("ProxyMonitor 巡检任务已启动 (每15分钟检查一次)")

}

// Execute 执行一次完整的巡检 (由 Cron 定时触发)
func (m *ProxyMonitor) Execute(ctx context.Context) {
	log.Println("[ProxyMonitor] Start checking proxies...")

	// 1. 查正常和暂时异常的代理，只有 status = 3 的才被抛弃
	proxies, err := m.proxyRepo.FindCheckList(ctx)
	if err != nil {
		log.Printf("[ProxyMonitor] Failed to fetch proxy list: %v\n", err)
		return
	}

	if len(proxies) == 0 {
		log.Printf("[ProxyMonitor] No proxies found\n")
		return
	}

	// 2. 并发探测 (使用信号量控制并发)
	var wg sync.WaitGroup
	sem := make(chan struct{}, m.concurrencyLimit)

	for _, p := range proxies {
		select {
		case <-ctx.Done():
			log.Println("[ProxyMonitor] Task context timeout, stopping...")
			return
		default:
		}
		wg.Add(1)
		sem <- struct{}{} // 获取令牌

		time.Sleep(m.sleepTime)

		go func(proxy model.Proxy) {
			defer wg.Done()
			defer func() { <-sem }() // 释放令牌

			if err := m.proxyService.VerifyAndHeal(ctx, &proxy); err != nil {
				// 这里的 err 通常是数据库层面的严重错误，需记录
				log.Printf("[Task] Logic error for proxy %s: %v", proxy.IP, err)
			}
		}(p)

	}

	wg.Wait()
	log.Println("[ProxyMonitor] Check finished.")
}
