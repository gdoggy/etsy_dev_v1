package service

import (
	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/repository"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"gorm.io/gorm/clause"
)

type ProductService struct {
	ShopRepo *repository.ShopRepository
}

func NewProductService(shopRepo *repository.ShopRepository) *ProductService {
	return &ProductService{ShopRepo: shopRepo}
}

// 简单的 DTO，用于接收 Etsy 返回的商品数据
type EtsyListing struct {
	ListingID int64  `json:"listing_id"`
	Title     string `json:"title"`
	State     string `json:"state"`
	Price     struct {
		Amount   int    `json:"amount"`
		Divisor  int    `json:"divisor"`
		Currency string `json:"currency_code"`
	} `json:"price"`
	Quantity int    `json:"quantity"`
	Url      string `json:"url"`
}

type EtsyListingsResp struct {
	Count   int           `json:"count"`
	Results []EtsyListing `json:"results"`
}

// SyncListings 拉取指定店铺的在线商品
func (s *ProductService) SyncListings(dbShopID uint) ([]EtsyListing, error) {
	// 1. 从数据库获取店铺信息 (包含 Token 和 Proxy)
	var shop model.Shop
	if err := s.ShopRepo.DB.Preload("Proxy").Preload("Developer").First(&shop, dbShopID).Error; err != nil {
		return nil, fmt.Errorf("店铺不存在: %v", err)
	}

	// 简单校验 Token 是否过期 (实战中这里应该有自动刷新逻辑)
	// if time.Now().After(shop.TokenExpiresAt) { ... RefreshToken ... }

	// 2. 构造 API URL (使用我们要死守的 int64 ID)
	// 接口: Get all active listings for a shop
	apiUrl := fmt.Sprintf("https://api.etsy.com/v3/application/shops/%s/listings/active", shop.EtsyShopID)

	// 3. 构造代理客户端
	/*
		proxyURL := fmt.Sprintf("%s://%s:%s", shop.Proxy.Protocol, shop.Proxy.IP, shop.Proxy.Port)
		if shop.Proxy.Username != "" {
			proxyURL = fmt.Sprintf("%s://%s:%s@%s:%s",
				shop.Proxy.Protocol, shop.Proxy.Username, shop.Proxy.Password, shop.Proxy.IP, shop.Proxy.Port)
		}

		// 如果走代理有问题，要临时 SetDebug(true)
		client := resty.New().
			SetProxy(proxyURL).
			SetTimeout(10 * time.Second)

	*/
	client := resty.New().
		SetDebug(true). // 开启调试，方便看日志
		SetTimeout(10 * time.Second)

	// 4. 发起请求
	var res EtsyListingsResp
	resp, err := client.R().
		SetHeader("x-api-key", shop.Developer.AppKey).          // 用 Developer 的 Key
		SetHeader("Authorization", "Bearer "+shop.AccessToken). // 用 Shop 的 Token
		SetResult(&res).
		Get(apiUrl)

	if err != nil {
		return nil, fmt.Errorf("请求中断: %v", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("API 错误 [%d]: %s", resp.StatusCode(), resp.String())
	}

	return res.Results, nil
}

// SyncAndSaveListings 拉取并保存
func (s *ProductService) SyncAndSaveListings(dbShopID uint) error {
	// 1. 拉取数据 (复用刚才的逻辑，这里简写)
	// ... (代码同上，使用直连模式) ...
	// 假设拉取到的结果在 res.Results 里
	etsyListings, err := s.SyncListings(1)

	if len(etsyListings) == 0 {
		return nil
	}

	// 2. 数据转换 (DTO -> Model)
	var products []model.Product
	for _, item := range etsyListings {
		products = append(products, model.Product{
			ShopID:       dbShopID,
			ListingID:    item.ListingID,
			Title:        item.Title,
			State:        item.State,
			PriceAmount:  item.Price.Amount,
			PriceDivisor: item.Price.Divisor,
			Currency:     item.Price.Currency,
			Quantity:     item.Quantity,
			Url:          item.Url,
		})
	}

	// 3. 批量入库 (UPSERT)
	// 如果 ListingID 已经存在，则更新 价格、库存、状态、标题
	err = s.ShopRepo.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "listing_id"}}, // 冲突检测列
		DoUpdates: clause.AssignmentColumns([]string{"title", "state", "price_amount", "quantity", "updated_at"}),
	}).Create(&products).Error

	if err != nil {
		return fmt.Errorf("入库失败: %v", err)
	}

	fmt.Printf("✅ 成功同步 %d 个商品到数据库！\n", len(products))
	return nil
}
