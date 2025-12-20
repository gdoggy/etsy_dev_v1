package net

import (
	"context"
	"fmt"
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
		return nil, fmt.Errorf("create request failed: %v", err)
	}

	// 1. 强制注入 Etsy V3 鉴权头
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	// 2. 注入标准头
	req.Header.Set("Accept", "application/json")

	return req, nil
}
