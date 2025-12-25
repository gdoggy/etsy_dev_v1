package net

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"time"
)

// ProxyProvider 定义“提供代理”的行为标准
type ProxyProvider interface {
	// GetProxy 根据业务唯一键 (shopID) 获取一个可用的代理地址
	// 若 shop ID = 0，则随机返回一个可用代理
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
	SendMultipart(ctx context.Context, shopID int64, req *MultipartRequest) (*http.Response, error)
	Ping(ctx context.Context, req *http.Request) (*http.Response, error)
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

// FileData 文件数据
type FileData struct {
	Data     []byte
	Filename string
}

// MultipartRequest multipart 请求参数
type MultipartRequest struct {
	URL     string
	Headers map[string]string
	Files   map[string]FileData // fieldName -> fileData
	Fields  map[string]string   // 普通表单字段
}

// SendMultipart 发送 multipart/form-data 请求
func (d *httpDispatcher) SendMultipart(ctx context.Context, shopID int64, req *MultipartRequest) (*http.Response, error) {
	// 1. 构建 multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件
	for fieldName, fileData := range req.Files {
		part, err := writer.CreateFormFile(fieldName, filepath.Base(fileData.Filename))
		if err != nil {
			return nil, fmt.Errorf("创建文件字段失败: %v", err)
		}
		if _, err := io.Copy(part, bytes.NewReader(fileData.Data)); err != nil {
			return nil, fmt.Errorf("写入文件数据失败: %v", err)
		}
	}

	// 添加普通字段
	for fieldName, value := range req.Fields {
		if err := writer.WriteField(fieldName, value); err != nil {
			return nil, fmt.Errorf("写入字段失败: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 writer 失败: %v", err)
	}

	// 2. 构建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, body)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %v", err)
	}

	// 设置 Content-Type (必须使用 writer.FormDataContentType() 获取正确的 boundary)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	// 设置其他 headers
	for k, v := range req.Headers {
		if k != "Content-Type" { // 跳过 Content-Type，已设置
			httpReq.Header.Set(k, v)
		}
	}

	// 3. 通过 Dispatcher 发送 (自动处理代理)
	return d.Send(ctx, shopID, httpReq)
}

// Ping 随机选择端口 Ping 测试
func (d *httpDispatcher) Ping(ctx context.Context, req *http.Request) (*http.Response, error) {
	proxyURL, err := d.provider.GetProxy(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("proxy provider error: %v", err)
	}
	client := d.getClient(proxyURL)
	resp, err := client.Do(req)
	if err == nil {
		return resp, nil
	}
	return nil, fmt.Errorf("request failed after retries: %v", err)
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
