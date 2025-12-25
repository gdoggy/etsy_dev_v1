package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// ==================== 配置 ====================

type OneBoundConfig struct {
	APIKey    string
	APISecret string
	BaseURL   string // 默认 https://api-gw.onebound.cn
	Timeout   time.Duration
}

// ==================== 统一数据结构 ====================

// ScrapedProduct 统一抓取结果（跨平台通用）
type ScrapedProduct struct {
	Platform    string          `json:"platform"`
	ItemID      string          `json:"item_id"`
	Title       string          `json:"title"`
	Price       float64         `json:"price"`
	Currency    string          `json:"currency"`
	Images      []string        `json:"images"`
	Description string          `json:"description"`
	DescImages  []string        `json:"desc_images"`
	Attributes  string          `json:"attributes"`
	Video       string          `json:"video,omitempty"`
	SKUs        []ScrapedSKU    `json:"skus,omitempty"`
	Props       []ScrapedProp   `json:"props,omitempty"`
	Location    string          `json:"location"`
	MinOrderQty int             `json:"min_order_qty"`
	RawData     json.RawMessage `json:"raw_data"`
}

type ScrapedSKU struct {
	SkuID      string  `json:"sku_id"`
	Price      float64 `json:"price"`
	Quantity   int     `json:"quantity"`
	Properties string  `json:"properties"`
	PropName   string  `json:"prop_name"`
}

type ScrapedProp struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ==================== 服务实现 ====================

type OneBoundService struct {
	Config     *OneBoundConfig
	HttpClient *http.Client
}

func NewOneBoundService(cfg *OneBoundConfig) *OneBoundService {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api-gw.onebound.cn"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &OneBoundService{
		Config: cfg,
		HttpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// ==================== 公共方法 ====================

// ParseURL 解析URL，提取平台和商品ID
func (s *OneBoundService) ParseURL(sourceURL string) (platform string, itemID string, err error) {
	u, err := url.Parse(sourceURL)
	if err != nil {
		return "", "", fmt.Errorf("无效的URL: %v", err)
	}

	host := u.Host

	// 1688
	if regexp.MustCompile(`(detail\.1688\.com|m\.1688\.com)`).MatchString(host) {
		// https://detail.1688.com/offer/610947572360.html
		re := regexp.MustCompile(`/offer/(\d+)`)
		if matches := re.FindStringSubmatch(u.Path); len(matches) > 1 {
			return "1688", matches[1], nil
		}
		// URL参数形式
		if id := u.Query().Get("offerId"); id != "" {
			return "1688", id, nil
		}
	}

	// 淘宝
	if regexp.MustCompile(`(item\.taobao\.com|detail\.tmall\.com)`).MatchString(host) {
		if id := u.Query().Get("id"); id != "" {
			return "taobao", id, nil
		}
	}

	// 速卖通
	if regexp.MustCompile(`aliexpress\.com`).MatchString(host) {
		re := regexp.MustCompile(`/item/(\d+)\.html`)
		if matches := re.FindStringSubmatch(u.Path); len(matches) > 1 {
			return "aliexpress", matches[1], nil
		}
	}

	// Amazon
	if regexp.MustCompile(`amazon\.(com|co\.uk|de|fr|co\.jp)`).MatchString(host) {
		re := regexp.MustCompile(`/dp/([A-Z0-9]+)`)
		if matches := re.FindStringSubmatch(u.Path); len(matches) > 1 {
			return "amazon", matches[1], nil
		}
	}

	// eBay
	if regexp.MustCompile(`ebay\.com`).MatchString(host) {
		re := regexp.MustCompile(`/itm/(\d+)`)
		if matches := re.FindStringSubmatch(u.Path); len(matches) > 1 {
			return "ebay", matches[1], nil
		}
	}

	return "", "", fmt.Errorf("不支持的平台或无法解析商品ID: %s", host)
}

// FetchProduct 抓取商品数据
func (s *OneBoundService) FetchProduct(ctx context.Context, platform, itemID string) (*ScrapedProduct, error) {
	switch platform {
	case "1688":
		return s.fetch1688(ctx, itemID)
	case "taobao":
		return s.fetchTaobao(ctx, itemID)
	case "aliexpress":
		return s.fetchAliExpress(ctx, itemID)
	case "amazon":
		return s.fetchAmazon(ctx, itemID)
	case "ebay":
		return s.fetchEbay(ctx, itemID)
	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}

// GetSupportedPlatforms 获取支持的平台列表
func (s *OneBoundService) GetSupportedPlatforms() []string {
	return []string{"1688", "taobao", "aliexpress", "amazon", "ebay"}
}

// ==================== 平台实现 ====================

// fetch1688 抓取1688商品
func (s *OneBoundService) fetch1688(ctx context.Context, itemID string) (*ScrapedProduct, error) {
	apiURL := fmt.Sprintf("%s/1688/item_get/?key=%s&secret=%s&num_iid=%s",
		s.Config.BaseURL, s.Config.APIKey, s.Config.APISecret, itemID)

	resp, err := s.doRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}

	return s.parse1688Response(resp, itemID)
}

// parse1688Response 解析1688响应
func (s *OneBoundService) parse1688Response(data []byte, itemID string) (*ScrapedProduct, error) {
	var resp struct {
		Item struct {
			NumIid   int64  `json:"num_iid"`
			Title    string `json:"title"`
			Price    string `json:"price"`
			PicURL   string `json:"pic_url"`
			Desc     string `json:"desc"`
			Location string `json:"location"`
			MinNum   int    `json:"min_num"`
			Video    string `json:"video"`
			ItemImgs []struct {
				URL string `json:"url"`
			} `json:"item_imgs"`
			DescImg []string `json:"desc_img"`
			Props   []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"props"`
			Skus struct {
				Sku []struct {
					Price          interface{} `json:"price"`
					Quantity       int         `json:"quantity"`
					SkuID          interface{} `json:"sku_id"`
					Properties     string      `json:"properties"`
					PropertiesName string      `json:"properties_name"`
				} `json:"sku"`
			} `json:"skus"`
		} `json:"item"`
		Error     string `json:"error"`
		ErrorCode string `json:"error_code"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("API错误: %s", resp.Error)
	}

	// 提取图片
	images := make([]string, 0)
	for _, img := range resp.Item.ItemImgs {
		if img.URL != "" {
			images = append(images, img.URL)
		}
	}

	// 提取SKU
	skus := make([]ScrapedSKU, 0)
	for _, sku := range resp.Item.Skus.Sku {
		price := parseFloat(sku.Price)
		skuID := fmt.Sprintf("%v", sku.SkuID)
		skus = append(skus, ScrapedSKU{
			SkuID:      skuID,
			Price:      price,
			Quantity:   sku.Quantity,
			Properties: sku.Properties,
			PropName:   sku.PropertiesName,
		})
	}

	// 提取属性
	props := make([]ScrapedProp, 0)
	for _, p := range resp.Item.Props {
		props = append(props, ScrapedProp{
			Name:  p.Name,
			Value: p.Value,
		})
	}

	return &ScrapedProduct{
		Platform:    "1688",
		ItemID:      itemID,
		Title:       resp.Item.Title,
		Price:       parseFloat(resp.Item.Price),
		Currency:    "CNY",
		Images:      images,
		Description: resp.Item.Desc,
		DescImages:  resp.Item.DescImg,
		Video:       resp.Item.Video,
		SKUs:        skus,
		Props:       props,
		Location:    resp.Item.Location,
		MinOrderQty: resp.Item.MinNum,
		RawData:     data,
	}, nil
}

// fetchTaobao 抓取淘宝商品 (预留)
func (s *OneBoundService) fetchTaobao(ctx context.Context, itemID string) (*ScrapedProduct, error) {
	apiURL := fmt.Sprintf("%s/taobao/item_get/?key=%s&secret=%s&num_iid=%s",
		s.Config.BaseURL, s.Config.APIKey, s.Config.APISecret, itemID)

	resp, err := s.doRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}

	// 淘宝响应结构与1688类似，复用解析逻辑
	product, err := s.parse1688Response(resp, itemID)
	if err != nil {
		return nil, err
	}
	product.Platform = "taobao"
	return product, nil
}

// fetchAliExpress 抓取速卖通商品 (预留)
func (s *OneBoundService) fetchAliExpress(ctx context.Context, itemID string) (*ScrapedProduct, error) {
	return nil, fmt.Errorf("速卖通平台暂未实现")
}

// fetchAmazon 抓取Amazon商品 (预留)
func (s *OneBoundService) fetchAmazon(ctx context.Context, itemID string) (*ScrapedProduct, error) {
	return nil, fmt.Errorf("Amazon 平台暂未实现")
}

// fetchEbay 抓取eBay商品 (预留)
func (s *OneBoundService) fetchEbay(ctx context.Context, itemID string) (*ScrapedProduct, error) {
	return nil, fmt.Errorf("eBay 平台暂未实现")
}

// ==================== 内部方法 ====================

func (s *OneBoundService) doRequest(ctx context.Context, apiURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := s.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP错误 [%d]: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// parseFloat 将 interface{} 解析为 float64
func parseFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}
