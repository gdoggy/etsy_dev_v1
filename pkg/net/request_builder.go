package net

import (
	"context"
	"io"
	"net/http"
)

// BuildEtsyRequest 通用 Etsy 请求构建器
// 适用方：ShopService, ProductService, OrderService 等所有业务服务
// 职责：统一封装鉴权头 (x-api-key, Authorization) 和标准头 (Accept, Content-Type)
// 注意：如果 Content-Type 不是 JSON (如 form-data)，调用方获取 req 后可手动覆盖 Header
func BuildEtsyRequest(ctx context.Context, method, url string, body io.Reader, apiKey, accessToken string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	return req, nil
}

// BuildEtsyGetRequest 构建 Etsy GET 请求
func BuildEtsyGetRequest(ctx context.Context, url string, apiKey, accessToken string) (*http.Request, error) {
	return BuildEtsyRequest(ctx, http.MethodGet, url, nil, apiKey, accessToken)
}

// BuildEtsyPostRequest 构建 Etsy POST 请求
func BuildEtsyPostRequest(ctx context.Context, url string, body io.Reader, apiKey, accessToken string) (*http.Request, error) {
	return BuildEtsyRequest(ctx, http.MethodPost, url, body, apiKey, accessToken)
}

// BuildEtsyPutRequest 构建 Etsy PUT 请求
func BuildEtsyPutRequest(ctx context.Context, url string, body io.Reader, apiKey, accessToken string) (*http.Request, error) {
	return BuildEtsyRequest(ctx, http.MethodPut, url, body, apiKey, accessToken)
}

// BuildEtsyDeleteRequest 构建 Etsy DELETE 请求
func BuildEtsyDeleteRequest(ctx context.Context, url string, apiKey, accessToken string) (*http.Request, error) {
	return BuildEtsyRequest(ctx, http.MethodDelete, url, nil, apiKey, accessToken)
}
