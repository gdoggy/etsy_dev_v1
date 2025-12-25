package service

import (
	"context"
	"etsy_dev_v1_202512/internal/repository"
	"fmt"
	"log"
	"net/url"
	"time"
)

// NetworkProvider 专门实现  pkg/net.ProxyProvider 接口
type NetworkProvider struct {
	ShopRepo     *repository.ShopRepo
	ProxyService *ProxyService
}

func NewNetworkProvider(shopRepo *repository.ShopRepo, proxyService *ProxyService) *NetworkProvider {
	return &NetworkProvider{
		ShopRepo:     shopRepo,
		ProxyService: proxyService,
	}
}

// GetProxy 实现接口：根据 key (ShopID) 获取可用代理
func (n *NetworkProvider) GetProxy(ctx context.Context, shopID int64) (*url.URL, error) {
	// 无 ID 随机生成 proxy, 可用于连通性测试等场景
	if shopID == 0 {
		randomProxy, err := n.ProxyService.PickRandomProxy(ctx)
		if err != nil {
			return nil, fmt.Errorf("no random proxy available: %v", err)
		}
		proxyURL, err := randomProxy.ProxyToURL()
		return proxyURL, err
	}
	// 1. 查库获取 Shop 信息
	shop, err := n.ShopRepo.GetByID(ctx, shopID)
	if err != nil {
		return nil, err
	}

	// 2. 惰性绑定逻辑 (Lazy Binding)
	// 如果当前没有代理，或者代理字段为空，调用 ProxyService 分配
	if shop.ProxyID == 0 || shop.Proxy == nil {
		// 调用 ProxyService 的 PickBestProxy
		newProxy, err := n.ProxyService.PickBestProxy(ctx, shop.Region)
		if err != nil {
			return nil, fmt.Errorf("no proxy available: %v", err)
		}

		// 绑定并更新
		err = n.ShopRepo.UpdateProxyBinding(ctx, shop.ID, newProxy.ID)
		if err != nil {
			return nil, err
		}
		shop.Proxy = newProxy
	}

	// 3. 返回标准 URL 对象给 Dispatcher
	return shop.Proxy.ProxyToURL()
}

// ReportError 实现接口：处理故障
// 任务：解绑店铺，触发代理巡检
func (n *NetworkProvider) ReportError(ctx context.Context, shopID int64) {
	/// 1. 获取当前绑定关系
	shop, err := n.ShopRepo.GetByID(ctx, shopID)
	if err != nil {
		log.Printf("failed to get shop: %v", err)
		return
	}
	if shop.ProxyID == 0 || shop.Proxy == nil {
		log.Printf("shop ID : %d no proxy available", shop.ID)
		return
	}

	badProxy := shop.Proxy
	log.Printf("[Network] 收到故障上报 ShopID=%d, Proxy=%s, 正在解绑...", shopID, badProxy.IP)
	// 2. 解绑店铺 (业务动作)
	if err = n.ShopRepo.UnbindProxy(ctx, shopID); err != nil {
		log.Printf("[Network] 解绑失败: %v", err)
	}

	// 3. 异步触发巡检 (复用 ProxyService 能力)
	go func() {
		checkCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err = n.ProxyService.VerifyAndHeal(checkCtx, badProxy); err != nil {
			log.Printf("verify proxy failed: %v", err)
		}
	}()
}
