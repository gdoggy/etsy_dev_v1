package service

import (
	"context"
	"testing"
	"time"
)

func TestOneBoundService_ParseURL(t *testing.T) {
	svc := NewOneBoundService(&OneBoundConfig{})

	tests := []struct {
		name         string
		url          string
		wantPlatform string
		wantItemID   string
		wantErr      bool
	}{
		{
			name:         "1688 详情页",
			url:          "https://detail.1688.com/offer/610947572360.html",
			wantPlatform: "1688",
			wantItemID:   "610947572360",
			wantErr:      false,
		},
		{
			name:         "1688 移动端",
			url:          "https://m.1688.com/offer/610947572360.html",
			wantPlatform: "1688",
			wantItemID:   "610947572360",
			wantErr:      false,
		},
		{
			name:         "淘宝详情页",
			url:          "https://item.taobao.com/item.htm?id=123456789",
			wantPlatform: "taobao",
			wantItemID:   "123456789",
			wantErr:      false,
		},
		{
			name:         "天猫详情页",
			url:          "https://detail.tmall.com/item.htm?id=987654321",
			wantPlatform: "taobao",
			wantItemID:   "987654321",
			wantErr:      false,
		},
		{
			name:         "速卖通详情页",
			url:          "https://www.aliexpress.com/item/1005001234567890.html",
			wantPlatform: "aliexpress",
			wantItemID:   "1005001234567890",
			wantErr:      false,
		},
		{
			name:         "Amazon 详情页",
			url:          "https://www.amazon.com/dp/B08N5WRWNW",
			wantPlatform: "amazon",
			wantItemID:   "B08N5WRWNW",
			wantErr:      false,
		},
		{
			name:         "eBay 详情页",
			url:          "https://www.ebay.com/itm/123456789012",
			wantPlatform: "ebay",
			wantItemID:   "123456789012",
			wantErr:      false,
		},
		{
			name:    "不支持的平台",
			url:     "https://www.example.com/product/123",
			wantErr: true,
		},
		{
			name:    "无效 URL",
			url:     "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform, itemID, err := svc.ParseURL(tt.url)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if platform != tt.wantPlatform {
					t.Errorf("ParseURL() platform = %v, want %v", platform, tt.wantPlatform)
				}
				if itemID != tt.wantItemID {
					t.Errorf("ParseURL() itemID = %v, want %v", itemID, tt.wantItemID)
				}
			}
		})
	}
}

func TestOneBoundService_FetchProduct(t *testing.T) {
	// 跳过需要真实 API Key 的测试
	apiKey := "" // 设置环境变量 ONEBOUND_API_KEY
	if apiKey == "" {
		t.Skip("跳过: 需要设置 ONEBOUND_API_KEY")
	}

	svc := NewOneBoundService(&OneBoundConfig{
		APIKey:  apiKey,
		Timeout: 30 * time.Second,
	})

	ctx := context.Background()

	tests := []struct {
		name     string
		platform string
		itemID   string
		wantErr  bool
	}{
		{
			name:     "1688 商品",
			platform: "1688",
			itemID:   "610947572360",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product, err := svc.FetchProduct(ctx, tt.platform, tt.itemID)

			if (err != nil) != tt.wantErr {
				t.Errorf("FetchProduct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && product != nil {
				if product.Title == "" {
					t.Error("FetchProduct() 返回的商品标题为空")
				}
				if len(product.Images) == 0 {
					t.Error("FetchProduct() 返回的商品图片为空")
				}
				t.Logf("商品标题: %s", product.Title)
				t.Logf("商品价格: %.2f %s", product.Price, product.Currency)
				t.Logf("图片数量: %d", len(product.Images))
			}
		})
	}
}

func TestNewOneBoundService_DefaultConfig(t *testing.T) {
	svc := NewOneBoundService(&OneBoundConfig{})

	if svc.Config.BaseURL != "https://api-gw.onebound.cn" {
		t.Errorf("默认 BaseURL 不正确: got %s", svc.Config.BaseURL)
	}

	if svc.Config.Timeout != 30*time.Second {
		t.Errorf("默认 Timeout 不正确: got %v", svc.Config.Timeout)
	}

	if svc.HttpClient == nil {
		t.Error("HttpClient 未初始化")
	}
}
