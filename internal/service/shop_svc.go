package service

import (
	"context"
	"encoding/json"
	"errors"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/pkg/etsy"
	"etsy_dev_v1_202512/pkg/net"
	"fmt"
	"log"
)

type ShopService struct {
	ShopRepo   *repository.ShopRepo
	dispatcher net.Dispatcher
}

func NewShopService(shopRepo *repository.ShopRepo, dispatcher net.Dispatcher) *ShopService {
	return &ShopService{
		ShopRepo:   shopRepo,
		dispatcher: dispatcher,
	}
}

// GetShop 查店铺
func (s *ShopService) GetShop(ctx context.Context, shopId int) (model.Shop, error) {
	panic("implement me")
}

// CreateShop 新建店铺
func (s *ShopService) CreateShop(ctx context.Context, shop model.Shop) (model.Shop, error) {
	panic("implement me")
}

// SyncEtsyAccountInfo 同步 Etsy 账号信息到本地
func (s *ShopService) SyncEtsyAccountInfo(ctx context.Context, shopID int64) error {
	// 1. 获取本地配置
	shop, err := s.ShopRepo.GetShopByID(ctx, shopID)
	if err != nil {
		fmt.Printf("get shop by id err: %v\n", err)
		return err
	}
	if shop.AccessToken == "" {
		return errors.New("店铺未授权，无法同步信息")
	}

	// 2. 查 UserID
	userID, err := s.fetchEtsyUserID(ctx, shop)
	if err != nil {
		return fmt.Errorf("获取 UserID 失败: %v", err)
	}

	// 3. 查 Shop 详情
	// 这里返回的是 dto.EtsyShopDTO，而不是 service 内部结构体
	shopDTO, err := s.fetchEtsyShopDetails(ctx, shop, userID)
	if err != nil {
		return fmt.Errorf("获取店铺详情失败: %v", err)
	}

	// 4. 落库更新
	shop.UserID = userID
	shop.EtsyShopID = shopDTO.ShopID
	shop.ShopName = shopDTO.ShopName
	shop.CurrencyCode = shopDTO.CurrencyCode
	shop.IsVacation = shopDTO.IsVacation

	if err = s.ShopRepo.SaveOrUpdate(ctx, shop); err != nil {
		return fmt.Errorf("数据库同步失败: %v", err)
	}

	return nil
}

// 私有方法

func (s *ShopService) fetchEtsyUserID(ctx context.Context, shop *model.Shop) (int64, error) {
	etsyUrl := "https://api.etsy.com/v3/application/users/me"
	req, err := net.BuildEtsyRequest(ctx, "GET", etsyUrl, nil, shop.Developer.ApiKey, shop.AccessToken)
	if err != nil {
		log.Printf("build ETSY request err: %v\n", err)
		return 0, err
	}
	resp, err := s.dispatcher.Send(ctx, shop.ID, req)
	if err != nil {
		log.Printf("dispatcher send shop ID: %d, error: %v\n", shop.ID, err)
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("ETSY response error: %s", resp.Status)
		return 0, fmt.Errorf("ETSY api error: %d", resp.StatusCode)
	}

	// 使用 dto 包中的结构体
	var res etsy.UserResp
	if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
		log.Printf("unmarshal resp err: %v\n", err)
		return 0, err
	}
	return res.UserID, nil
}

func (s *ShopService) fetchEtsyShopDetails(ctx context.Context, shop *model.Shop, etsyUserID int64) (*etsy.ShopDTO, error) {
	etsyUrl := fmt.Sprintf("https://api.etsy.com/v3/application/users/%d/shops", etsyUserID)
	req, err := net.BuildEtsyRequest(ctx, "GET", etsyUrl, nil, shop.Developer.ApiKey, shop.AccessToken)
	if err != nil {
		log.Printf("build ETSY request err: %v\n", err)
		return nil, err
	}
	resp, err := s.dispatcher.Send(ctx, shop.ID, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("etsy api error: %d", resp.StatusCode)
	}

	// 使用 dto 包中的结构体
	var res etsy.ShopListResp
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if res.Count == 0 || len(res.Results) == 0 {
		return nil, errors.New("该 Etsy 账号下未创建任何店铺")
	}

	// 返回 DTO 指针
	return &res.Results[0], nil
}
