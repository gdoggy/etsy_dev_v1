package utils

import (
	"etsy_dev_v1_202512/core/model"
	"time"

	"github.com/go-resty/resty/v2"
)

// NewProxiedClient 创建一个配置好代理、超时和调试模式的 Resty 客户端
// 它是全系统统一的网络请求入口
func NewProxiedClient(proxy *model.Proxy) *resty.Client {
	client := resty.New().
		SetDebug(true).                            // 全局调试开关 (上线可改为 false 或由配置控制)
		SetTimeout(20*time.Second).                // 全局默认超时 (拉取商品可能比较慢，给 20s)
		SetHeader("User-Agent", "Etsy-Go-App/1.0") // 模拟标准 UA

	// 核心：动态配置代理
	// 只要传入了 Proxy 对象，且生成的 URL 不为空，就挂载代理
	if proxy != nil {
		proxyURL := proxy.ProxyURL() // 调用 model 中定义的辅助方法
		if proxyURL != "" {
			client.SetProxy(proxyURL)
		}
	}

	return client
}
