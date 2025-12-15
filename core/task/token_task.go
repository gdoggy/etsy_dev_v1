package task

import (
	"log"
	"time"

	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/repository"
	"etsy_dev_v1_202512/core/service"

	"github.com/robfig/cron/v3"
)

type TokenTask struct {
	ShopRepo    *repository.ShopRepository
	AuthService *service.AuthService
	Cron        *cron.Cron
}

func NewTokenTask(shopRepo *repository.ShopRepository, authService *service.AuthService) *TokenTask {
	return &TokenTask{
		ShopRepo:    shopRepo,
		AuthService: authService,
		Cron:        cron.New(cron.WithSeconds()), // 支持秒级控制
	}
}

// Start 启动定时任务
func (t *TokenTask) Start() {
	// 首次执行自动更新
	go func() {
		log.Println("[Task] 服务启动，正在执行首次 Token 检查...")
		t.refreshJob()
	}()

	// 策略：每 30 分钟执行一次检查
	// Cron 表达式: "0 0/30 * * * *" (秒 分 时 日 月 周)
	_, err := t.Cron.AddFunc("0 0/30 * * * *", func() {
		t.refreshJob()
	})
	if err != nil {
		log.Fatalf("无法启动 Token 定时任务: %v", err)
	}

	t.Cron.Start()
	log.Println("Token 保活任务已启动 (每30分钟检查一次)")
}

// 自动刷新逻辑
func (t *TokenTask) refreshJob() {
	// 查询条件：
	// 1. 快过期 (ExpiresAt < Now + 1h)
	// 2. 状态不是 'auth_invalid' (如果已经坏了，就不浪费资源去刷了，等人工处理)
	threshold := time.Now().Add(1 * time.Hour)

	var shops []model.Shop
	t.ShopRepo.DB.Preload("Developer").Preload("Proxy").
		Where("token_expires_at < ? AND token_status != ?", threshold, model.TokenStatusInvalid).
		Find(&shops)

	// ... 遍历 shops ...
	for _, shop := range shops {
		err := t.AuthService.RefreshAccessToken(&shop)
		if err != nil {
			// 这里不需要太惊慌，因为 AuthService 内部已经区分了网络错误和鉴权错误
			// 鉴权错误已经更新了 DB 状态
			log.Printf("[Cron] 店铺 [%s] 维护: %v", shop.ShopName, err)
		}
	}
}
