package net

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ProxyProvider 定义“提供代理”的行为标准
type ProxyProvider interface {
	// GetProxy 根据业务唯一键 (shopID) 获取一个可用的代理地址
	GetProxy(ctx context.Context, shopID int64) (*url.URL, error)

	// ReportError 上报该业务键对应的代理已失效
	// 业务层实现需在此方法中执行：解绑旧代理、标记故障、触发巡检等
	ReportError(ctx context.Context, shopID int64)
}

// Dispatcher 网络调度器 (通用组件)
type Dispatcher interface {
	// Send 发送 HTTP 请求
	// shopID: 业务实体的唯一标识
	// req: 标准的 http.Request 对象
	Send(ctx context.Context, shopID int64, req *http.Request) (*http.Response, error)
}

// httpDispatcher 是 Dispatcher 接口的具体实现
// 注意：它是私有的，外部只能通过 NewDispatcher 获取接口
type httpDispatcher struct {
	provider       ProxyProvider
	transportCache sync.Map
	maxRetries     int
}

var _ Dispatcher = (*httpDispatcher)(nil)

func NewDispatcher(provider ProxyProvider) Dispatcher {
	return &httpDispatcher{
		provider:   provider,
		maxRetries: 2,
	}
}

// Send 发送 HTTP 请求 (自动处理重试与代理切换)
// shopID: 标识谁在发请求 (如 "shop_1024")
func (d *httpDispatcher) Send(ctx context.Context, shopID int64, req *http.Request) (*http.Response, error) {
	var lastErr error

	for i := 0; i <= d.maxRetries; i++ {
		// 1. 通过接口回调，获取代理 (惰性绑定逻辑在业务层实现)
		proxyURL, err := d.provider.GetProxy(ctx, shopID)
		if err != nil {
			return nil, fmt.Errorf("proxy provider error: %v", err)
		}

		// 2. 获取/复用 Transport
		client := d.getClient(proxyURL)

		// 3. 发送请求
		resp, err := client.Do(req)

		// 成功
		if err == nil {
			return resp, nil
		}

		// 失败
		lastErr = err

		// 还有重试机会时，报错并触发切换
		if i < d.maxRetries {
			// 回调业务层：这个 Key 对应的代理坏了，请处理
			d.provider.ReportError(ctx, shopID)
			// 清理本地 Transport 缓存
			if proxyURL != nil {
				d.transportCache.Delete(proxyURL.String())
			}
		}
	}

	return nil, fmt.Errorf("request failed after retries: %v", lastErr)
}

// getClient 内部复用逻辑
func (d *httpDispatcher) getClient(proxyURL *url.URL) *http.Client {
	// 缓存 Key: "http://user:pass@ip:port"
	cacheKey := proxyURL.String()

	if val, ok := d.transportCache.Load(cacheKey); ok {
		return &http.Client{
			Transport: val.(*http.Transport),
			Timeout:   30 * time.Second,
		}
	}

	// 缓存未命中，创建新 Transport
	tr := &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 可选
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
	}

	// 3. 存入缓存 (LoadOrStore 防止并发重复创建)
	actual, _ := d.transportCache.LoadOrStore(cacheKey, tr)

	return &http.Client{
		Transport: actual.(*http.Transport),
		Timeout:   30 * time.Second,
	}
}

/*
type ClientFactory struct {
	proxyService *service.ProxyService
	shopRepo     *repository.ShopRepo

	// 缓存 Transport 实现 TCP 复用
	transportCache sync.Map

	// API 批量获取 proxy
	loinUser      string
	loginPassword string
}

func (f *ClientFactory) NewClientFactory(proxyService *service.ProxyService, repo *repository.ShopRepo) *ClientFactory {
	return &ClientFactory{
		proxyService:  proxyService,
		shopRepo:      repo,
		loinUser:      "EtsyApiV1",
		loginPassword: "EtsyApiPassword",
	}
}

// GetClient 获取一个配置好的 HTTP 客户端
// 如果 Shop 当前无代理，会自动分配一个
func (f *ClientFactory) GetClient(ctx context.Context, shop *model.Shop) (*http.Client, error) {
	// 1. 惰性绑定检查：如果店铺没绑定代理，现场分配一个
	if shop.ProxyID == 0 || shop.Proxy == nil {
		log.Printf("[Factory] Shop %s has no proxy, assigning new one...", shop.ShopName)

		newProxy, err := f.proxyService.PickBestProxy(ctx, shop.Region)
		if err != nil {
			return nil, fmt.Errorf("failed to assign proxy: %v", err)
		}

		// 更新 DB 绑定关系
		if err := f.shopRepo.UpdateProxyBinding(ctx, shop.ID, newProxy.ID); err != nil {
			return nil, fmt.Errorf("failed to bind proxy to shop: %v", err)
		}

		// 更新内存对象，供后续使用
		shop.Proxy = newProxy
		shop.ProxyID = newProxy.ID
	}

	// 2. 获取复用的 Transport (TCP Keep-Alive 关键)
	tr := f.getTransport(shop.Proxy)

	// 3. 返回 Client
	return &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second, // 统一业务超时
	}, nil
}

// ReportProxyError 业务层上报代理故障
// 动作：解绑店铺 + 触发代理体检
func (f *ClientFactory) ReportProxyError(ctx context.Context, shop *model.Shop) {
	if shop.Proxy == nil {
		return
	}

	failedProxy := shop.Proxy
	log.Printf("[Factory] Reported error for Proxy %s (Shop %d). Handling...", failedProxy.IP, shop.ID)

	// 1. 立即解绑当前店铺 (防止下次重试还拿到这个坏代理)
	// 更新 DB 为 0
	if err := f.shopRepo.UnbindProxy(ctx, shop.ID); err != nil {
		log.Printf("[Factory] Failed to unbind shop: %v", err)
	}
	// 更新内存对象，确保业务层重试时能触发 Lazy Binding
	shop.ProxyID = 0
	shop.Proxy = nil

	// 2. 异步触发：对那个坏代理进行全身检查
	// 如果它真的死透了，VerifyAndHeal 会负责把还在它上面的其他店铺也移走
	go func() {
		healCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		err := f.proxyService.VerifyAndHeal(healCtx, failedProxy)
		if err != nil {
			log.Printf("[Factory] Failed to verify and heal: %v", err)
		}
	}()

	// 3. 清理本地 Transport 缓存
	// 生成 key 并从 sync.Map 中 delete，强制下次重建连接
	cacheKey := f.genCacheKey(failedProxy)
	f.transportCache.Delete(cacheKey)
}

// getTransport 内部方法：从缓存获取或创建新的 Transport
func (f *ClientFactory) getTransport(p *model.Proxy) *http.Transport {
	key := f.genCacheKey(p)

	// 尝试命中缓存
	if val, ok := f.transportCache.Load(key); ok {
		return val.(*http.Transport)
	}

	// 缓存未命中，创建新的
	proxyUrlStr := fmt.Sprintf("%s://%s:%s", p.Protocol, p.IP, p.Port)
	uri, _ := url.Parse(proxyUrlStr)

	if p.Username != "" {
		uri.User = url.UserPassword(p.Username, p.Password)
	}

	tr := &http.Transport{
		Proxy: http.ProxyURL(uri),
		// 优化连接参数
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// 存入缓存 (LoadOrStore 防止并发重复创建)
	actual, _ := f.transportCache.LoadOrStore(key, tr)
	return actual.(*http.Transport)
}

func (f *ClientFactory) genCacheKey(p *model.Proxy) string {
	return fmt.Sprintf("%s:%s:%s:%s", p.IP, p.Port, p.Username, p.Password)
}
*/
