package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/pkg/etsy"
	"etsy_dev_v1_202512/pkg/net"
	"fmt"
	"io"
	"net/http"
	"time"

	"gorm.io/gorm"
)

type ShippingProfileService struct {
	profileRepo     repository.ShippingProfileRepository
	destinationRepo repository.ShippingDestinationRepository
	upgradeRepo     repository.ShippingUpgradeRepository
	shopRepo        repository.ShopRepository
	developerRepo   repository.DeveloperRepository
	dispatcher      net.Dispatcher
}

func NewShippingProfileService(
	profileRepo repository.ShippingProfileRepository,
	destinationRepo repository.ShippingDestinationRepository,
	upgradeRepo repository.ShippingUpgradeRepository,
	shopRepo repository.ShopRepository,
	developerRepo repository.DeveloperRepository,
	dispatcher net.Dispatcher,
) *ShippingProfileService {
	return &ShippingProfileService{
		profileRepo:     profileRepo,
		destinationRepo: destinationRepo,
		upgradeRepo:     upgradeRepo,
		shopRepo:        shopRepo,
		developerRepo:   developerRepo,
		dispatcher:      dispatcher,
	}
}

// ==================== 查询方法 ====================

// GetProfileList 获取运费模板列表
func (s *ShippingProfileService) GetProfileList(ctx context.Context, shopID int64) (*dto.ShippingProfileListResp, error) {
	list, err := s.profileRepo.GetByShopID(ctx, shopID)
	if err != nil {
		return nil, err
	}

	respList := make([]dto.ShippingProfileResp, 0, len(list))
	for _, profile := range list {
		respList = append(respList, s.convertProfileToResp(&profile))
	}

	return &dto.ShippingProfileListResp{
		Total: int64(len(respList)),
		List:  respList,
	}, nil
}

// GetProfileDetail 获取运费模板详情（含目的地和升级选项）
func (s *ShippingProfileService) GetProfileDetail(ctx context.Context, id int64) (*dto.ShippingProfileDetailResp, error) {
	profile, err := s.profileRepo.GetByIDWithRelations(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("运费模板不存在")
		}
		return nil, err
	}

	resp := &dto.ShippingProfileDetailResp{
		ShippingProfileResp: s.convertProfileToResp(profile),
	}

	resp.Destinations = make([]dto.ShippingDestinationResp, 0, len(profile.Destinations))
	for _, dest := range profile.Destinations {
		resp.Destinations = append(resp.Destinations, s.convertDestinationToResp(&dest))
	}

	resp.Upgrades = make([]dto.ShippingUpgradeResp, 0, len(profile.Upgrades))
	for _, upgrade := range profile.Upgrades {
		resp.Upgrades = append(resp.Upgrades, s.convertUpgradeToResp(&upgrade))
	}

	return resp, nil
}

// ==================== Profile 同步方法 ====================

// SyncProfilesFromEtsy 从 Etsy 同步运费模板（含目的地和升级选项）
func (s *ShippingProfileService) SyncProfilesFromEtsy(ctx context.Context, shopID int64) error {
	shop, developer, err := s.getShopWithDeveloper(ctx, shopID)
	if err != nil {
		return err
	}

	if shop.Status == model.ShopStatusInactive {
		return errors.New("店铺已停用，无法同步")
	}

	url := fmt.Sprintf("%s/shops/%d/shipping-profiles", EtsyAPIBaseURL, shop.EtsyShopID)
	req, err := net.BuildEtsyRequest(ctx, http.MethodGet, url, nil, developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}

	resp, err := s.dispatcher.Send(ctx, shopID, req)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	var etsyResp etsy.EtsyShippingProfilesResp
	if err := json.NewDecoder(resp.Body).Decode(&etsyResp); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	now := time.Now()

	for _, item := range etsyResp.Results {
		// 1. Upsert 运费模板
		profile := model.ShippingProfile{
			ShopID:            shopID,
			EtsyProfileID:     item.ShippingProfileID,
			Title:             item.Title,
			OriginCountryISO:  item.OriginCountryISO,
			OriginPostalCode:  item.OriginPostalCode,
			ProcessingDaysMin: item.MinProcessingDays,
			ProcessingDaysMax: item.MaxProcessingDays,
			EtsySyncedAt:      &now,
		}

		if err := s.profileRepo.BatchUpsert(ctx, shopID, []model.ShippingProfile{profile}); err != nil {
			return fmt.Errorf("保存运费模板失败: %v", err)
		}

		// 获取本地 Profile ID
		localProfile, err := s.profileRepo.GetByEtsyProfileID(ctx, shopID, item.ShippingProfileID)
		if err != nil {
			continue
		}
		profileID := int64(localProfile.ID)

		// 2. 同步目的地
		destinations := make([]model.ShippingDestination, 0, len(item.ShippingProfileDestinations))
		for _, dest := range item.ShippingProfileDestinations {
			destinations = append(destinations, model.ShippingDestination{
				ShippingProfileID:     profileID,
				EtsyDestinationID:     dest.ShippingProfileDestinationID,
				DestinationCountryISO: dest.DestinationCountryISO,
				DestinationRegion:     dest.DestinationRegion,
				PrimaryCost:           dest.PrimaryCost.ToInt64(),
				SecondaryCost:         dest.SecondaryCost.ToInt64(),
				CurrencyCode:          dest.PrimaryCost.CurrencyCode,
				ShippingCarrierID:     dest.ShippingCarrierID,
				MailClass:             dest.MailClass,
				DeliveryDaysMin:       dest.MinDeliveryDays,
				DeliveryDaysMax:       dest.MaxDeliveryDays,
			})
		}
		if len(destinations) > 0 {
			if err := s.destinationRepo.BatchUpsert(ctx, profileID, destinations); err != nil {
				return fmt.Errorf("保存运费目的地失败: %v", err)
			}
		}

		// 3. 同步升级选项
		upgrades := make([]model.ShippingUpgrade, 0, len(item.ShippingProfileUpgrades))
		for _, upgrade := range item.ShippingProfileUpgrades {
			upgrades = append(upgrades, model.ShippingUpgrade{
				ShippingProfileID: profileID,
				EtsyUpgradeID:     upgrade.UpgradeID,
				UpgradeName:       upgrade.UpgradeName,
				Type:              upgrade.Type,
				Price:             upgrade.Price.ToInt64(),
				SecondaryCost:     upgrade.SecondaryCost.ToInt64(),
				CurrencyCode:      upgrade.Price.CurrencyCode,
				ShippingCarrierID: upgrade.ShippingCarrierID,
				MailClass:         upgrade.MailClass,
				DeliveryDaysMin:   upgrade.MinDeliveryDays,
				DeliveryDaysMax:   upgrade.MaxDeliveryDays,
			})
		}
		if len(upgrades) > 0 {
			if err := s.upgradeRepo.BatchUpsert(ctx, profileID, upgrades); err != nil {
				return fmt.Errorf("保存加急配送选项失败: %v", err)
			}
		}
	}

	return nil
}

// CreateProfileToEtsy 创建运费模板到 Etsy
func (s *ShippingProfileService) CreateProfileToEtsy(ctx context.Context, req dto.ShippingProfileCreateReq) (*dto.ShippingProfileResp, error) {
	shop, developer, err := s.getShopWithDeveloper(ctx, req.ShopID)
	if err != nil {
		return nil, err
	}

	if shop.Status != model.ShopStatusActive {
		return nil, errors.New("仅正常状态的店铺可以创建运费模板")
	}

	etsyReq := etsy.EtsyShippingProfileCreateReq{
		Title:             req.Title,
		OriginCountryISO:  req.OriginCountryISO,
		OriginPostalCode:  req.OriginPostalCode,
		MinProcessingDays: req.ProcessingDaysMin,
		MaxProcessingDays: req.ProcessingDaysMax,
	}
	body, err := json.Marshal(etsyReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d/shipping-profiles", EtsyAPIBaseURL, shop.EtsyShopID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPost, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, req.ShopID, httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, s.parseEtsyError(resp)
	}

	var etsyProfile etsy.EtsyShippingProfileResp
	if err := json.NewDecoder(resp.Body).Decode(&etsyProfile); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	now := time.Now()
	profile := &model.ShippingProfile{
		ShopID:            req.ShopID,
		EtsyProfileID:     etsyProfile.ShippingProfileID,
		Title:             etsyProfile.Title,
		OriginCountryISO:  etsyProfile.OriginCountryISO,
		OriginPostalCode:  etsyProfile.OriginPostalCode,
		ProcessingDaysMin: etsyProfile.MinProcessingDays,
		ProcessingDaysMax: etsyProfile.MaxProcessingDays,
		EtsySyncedAt:      &now,
	}

	if err := s.profileRepo.Create(ctx, profile); err != nil {
		return nil, fmt.Errorf("创建本地运费模板失败: %v", err)
	}

	result := s.convertProfileToResp(profile)
	return &result, nil
}

// UpdateProfileToEtsy 更新运费模板到 Etsy
func (s *ShippingProfileService) UpdateProfileToEtsy(ctx context.Context, id int64, req dto.ShippingProfileUpdateReq) error {
	profile, err := s.profileRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("运费模板不存在")
		}
		return err
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, profile.ShopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以修改运费模板")
	}

	etsyReq := etsy.EtsyShippingProfileUpdateReq{
		Title:             req.Title,
		OriginCountryISO:  req.OriginCountryISO,
		OriginPostalCode:  req.OriginPostalCode,
		MinProcessingDays: req.ProcessingDaysMin,
		MaxProcessingDays: req.ProcessingDaysMax,
	}
	body, err := json.Marshal(etsyReq)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d/shipping-profiles/%d", EtsyAPIBaseURL, shop.EtsyShopID, profile.EtsyProfileID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPut, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, profile.ShopID, httpReq)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	now := time.Now()
	fields := map[string]interface{}{
		"etsy_synced_at": now,
	}
	if req.Title != "" {
		fields["title"] = req.Title
	}
	if req.OriginCountryISO != "" {
		fields["origin_country_iso"] = req.OriginCountryISO
	}
	if req.OriginPostalCode != "" {
		fields["origin_postal_code"] = req.OriginPostalCode
	}
	if req.ProcessingDaysMin > 0 {
		fields["processing_days_min"] = req.ProcessingDaysMin
	}
	if req.ProcessingDaysMax > 0 {
		fields["processing_days_max"] = req.ProcessingDaysMax
	}

	return s.profileRepo.UpdateFields(ctx, id, fields)
}

// DeleteProfileFromEtsy 从 Etsy 删除运费模板
func (s *ShippingProfileService) DeleteProfileFromEtsy(ctx context.Context, id int64) error {
	profile, err := s.profileRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("运费模板不存在")
		}
		return err
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, profile.ShopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以删除运费模板")
	}

	url := fmt.Sprintf("%s/shops/%d/shipping-profiles/%d", EtsyAPIBaseURL, shop.EtsyShopID, profile.EtsyProfileID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodDelete, url, nil, developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}

	resp, err := s.dispatcher.Send(ctx, profile.ShopID, httpReq)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	// 删除关联数据
	profileID := int64(profile.ID)
	if err := s.destinationRepo.DeleteByProfileID(ctx, profileID); err != nil {
		return err
	}
	if err := s.upgradeRepo.DeleteByProfileID(ctx, profileID); err != nil {
		return err
	}

	return s.profileRepo.Delete(ctx, id)
}

// ==================== Destination 操作 ====================

// CreateDestinationToEtsy 创建运费目的地到 Etsy
func (s *ShippingProfileService) CreateDestinationToEtsy(ctx context.Context, req dto.ShippingDestinationCreateReq) (*dto.ShippingDestinationResp, error) {
	profile, err := s.profileRepo.GetByID(ctx, req.ShippingProfileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("运费模板不存在")
		}
		return nil, err
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, profile.ShopID)
	if err != nil {
		return nil, err
	}

	if shop.Status != model.ShopStatusActive {
		return nil, errors.New("仅正常状态的店铺可以创建运费目的地")
	}

	etsyReq := etsy.EtsyShippingDestinationCreateReq{
		DestinationCountryISO: req.DestinationCountryISO,
		DestinationRegion:     req.DestinationRegion,
		PrimaryCost:           req.PrimaryCost,
		SecondaryCost:         req.SecondaryCost,
		ShippingCarrierID:     req.ShippingCarrierID,
		MailClass:             req.MailClass,
		MinDeliveryDays:       req.DeliveryDaysMin,
		MaxDeliveryDays:       req.DeliveryDaysMax,
	}
	body, err := json.Marshal(etsyReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d/shipping-profiles/%d/destinations", EtsyAPIBaseURL, shop.EtsyShopID, profile.EtsyProfileID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPost, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, profile.ShopID, httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, s.parseEtsyError(resp)
	}

	var etsyDest etsy.EtsyShippingDestinationResp
	if err := json.NewDecoder(resp.Body).Decode(&etsyDest); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	dest := &model.ShippingDestination{
		ShippingProfileID:     req.ShippingProfileID,
		EtsyDestinationID:     etsyDest.ShippingProfileDestinationID,
		DestinationCountryISO: etsyDest.DestinationCountryISO,
		DestinationRegion:     etsyDest.DestinationRegion,
		PrimaryCost:           etsyDest.PrimaryCost.ToInt64(),
		SecondaryCost:         etsyDest.SecondaryCost.ToInt64(),
		CurrencyCode:          etsyDest.PrimaryCost.CurrencyCode,
		ShippingCarrierID:     etsyDest.ShippingCarrierID,
		MailClass:             etsyDest.MailClass,
		DeliveryDaysMin:       etsyDest.MinDeliveryDays,
		DeliveryDaysMax:       etsyDest.MaxDeliveryDays,
	}

	if err := s.destinationRepo.Create(ctx, dest); err != nil {
		return nil, fmt.Errorf("创建本地运费目的地失败: %v", err)
	}

	result := s.convertDestinationToResp(dest)
	return &result, nil
}

// UpdateDestinationToEtsy 更新运费目的地到 Etsy
func (s *ShippingProfileService) UpdateDestinationToEtsy(ctx context.Context, id int64, req dto.ShippingDestinationUpdateReq) error {
	dest, err := s.destinationRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("运费目的地不存在")
		}
		return err
	}

	profile, err := s.profileRepo.GetByID(ctx, dest.ShippingProfileID)
	if err != nil {
		return fmt.Errorf("获取运费模板失败: %v", err)
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, profile.ShopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以修改运费目的地")
	}

	etsyReq := etsy.EtsyShippingDestinationUpdateReq{
		DestinationCountryISO: req.DestinationCountryISO,
		DestinationRegion:     req.DestinationRegion,
		PrimaryCost:           req.PrimaryCost,
		SecondaryCost:         req.SecondaryCost,
		ShippingCarrierID:     req.ShippingCarrierID,
		MailClass:             req.MailClass,
		MinDeliveryDays:       req.DeliveryDaysMin,
		MaxDeliveryDays:       req.DeliveryDaysMax,
	}
	body, err := json.Marshal(etsyReq)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d/shipping-profiles/%d/destinations/%d",
		EtsyAPIBaseURL, shop.EtsyShopID, profile.EtsyProfileID, dest.EtsyDestinationID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPut, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, profile.ShopID, httpReq)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	fields := map[string]interface{}{}
	if req.DestinationCountryISO != "" {
		fields["destination_country_iso"] = req.DestinationCountryISO
	}
	if req.DestinationRegion != "" {
		fields["destination_region"] = req.DestinationRegion
	}
	fields["primary_cost"] = req.PrimaryCost
	fields["secondary_cost"] = req.SecondaryCost
	if req.CurrencyCode != "" {
		fields["currency_code"] = req.CurrencyCode
	}
	fields["shipping_carrier_id"] = req.ShippingCarrierID
	fields["mail_class"] = req.MailClass
	fields["delivery_days_min"] = req.DeliveryDaysMin
	fields["delivery_days_max"] = req.DeliveryDaysMax

	return s.destinationRepo.UpdateFields(ctx, id, fields)
}

// DeleteDestinationFromEtsy 从 Etsy 删除运费目的地
func (s *ShippingProfileService) DeleteDestinationFromEtsy(ctx context.Context, id int64) error {
	dest, err := s.destinationRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("运费目的地不存在")
		}
		return err
	}

	profile, err := s.profileRepo.GetByID(ctx, dest.ShippingProfileID)
	if err != nil {
		return fmt.Errorf("获取运费模板失败: %v", err)
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, profile.ShopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以删除运费目的地")
	}

	url := fmt.Sprintf("%s/shops/%d/shipping-profiles/%d/destinations/%d",
		EtsyAPIBaseURL, shop.EtsyShopID, profile.EtsyProfileID, dest.EtsyDestinationID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodDelete, url, nil, developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}

	resp, err := s.dispatcher.Send(ctx, profile.ShopID, httpReq)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	return s.destinationRepo.Delete(ctx, id)
}

// ==================== Upgrade 操作 ====================

// CreateUpgradeToEtsy 创建加急配送选项到 Etsy
func (s *ShippingProfileService) CreateUpgradeToEtsy(ctx context.Context, req dto.ShippingUpgradeCreateReq) (*dto.ShippingUpgradeResp, error) {
	profile, err := s.profileRepo.GetByID(ctx, req.ShippingProfileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("运费模板不存在")
		}
		return nil, err
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, profile.ShopID)
	if err != nil {
		return nil, err
	}

	if shop.Status != model.ShopStatusActive {
		return nil, errors.New("仅正常状态的店铺可以创建加急配送选项")
	}

	etsyReq := etsy.EtsyShippingUpgradeCreateReq{
		UpgradeName:       req.UpgradeName,
		Type:              req.Type,
		Price:             req.Price,
		SecondaryCost:     req.SecondaryCost,
		ShippingCarrierID: req.ShippingCarrierID,
		MailClass:         req.MailClass,
		MinDeliveryDays:   req.DeliveryDaysMin,
		MaxDeliveryDays:   req.DeliveryDaysMax,
	}
	body, err := json.Marshal(etsyReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d/shipping-profiles/%d/upgrades", EtsyAPIBaseURL, shop.EtsyShopID, profile.EtsyProfileID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPost, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, profile.ShopID, httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, s.parseEtsyError(resp)
	}

	var etsyUpgrade etsy.EtsyShippingUpgradeResp
	if err := json.NewDecoder(resp.Body).Decode(&etsyUpgrade); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	upgrade := &model.ShippingUpgrade{
		ShippingProfileID: req.ShippingProfileID,
		EtsyUpgradeID:     etsyUpgrade.UpgradeID,
		UpgradeName:       etsyUpgrade.UpgradeName,
		Type:              etsyUpgrade.Type,
		Price:             etsyUpgrade.Price.ToInt64(),
		SecondaryCost:     etsyUpgrade.SecondaryCost.ToInt64(),
		CurrencyCode:      etsyUpgrade.Price.CurrencyCode,
		ShippingCarrierID: etsyUpgrade.ShippingCarrierID,
		MailClass:         etsyUpgrade.MailClass,
		DeliveryDaysMin:   etsyUpgrade.MinDeliveryDays,
		DeliveryDaysMax:   etsyUpgrade.MaxDeliveryDays,
	}

	if err := s.upgradeRepo.Create(ctx, upgrade); err != nil {
		return nil, fmt.Errorf("创建本地加急配送选项失败: %v", err)
	}

	result := s.convertUpgradeToResp(upgrade)
	return &result, nil
}

// UpdateUpgradeToEtsy 更新加急配送选项到 Etsy
func (s *ShippingProfileService) UpdateUpgradeToEtsy(ctx context.Context, id int64, req dto.ShippingUpgradeUpdateReq) error {
	upgrade, err := s.upgradeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("加急配送选项不存在")
		}
		return err
	}

	profile, err := s.profileRepo.GetByID(ctx, upgrade.ShippingProfileID)
	if err != nil {
		return fmt.Errorf("获取运费模板失败: %v", err)
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, profile.ShopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以修改加急配送选项")
	}

	etsyReq := etsy.EtsyShippingUpgradeUpdateReq{
		UpgradeName:       req.UpgradeName,
		Type:              req.Type,
		Price:             req.Price,
		SecondaryCost:     req.SecondaryCost,
		ShippingCarrierID: req.ShippingCarrierID,
		MailClass:         req.MailClass,
		MinDeliveryDays:   req.DeliveryDaysMin,
		MaxDeliveryDays:   req.DeliveryDaysMax,
	}
	body, err := json.Marshal(etsyReq)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d/shipping-profiles/%d/upgrades/%d",
		EtsyAPIBaseURL, shop.EtsyShopID, profile.EtsyProfileID, upgrade.EtsyUpgradeID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPut, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, profile.ShopID, httpReq)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	fields := map[string]interface{}{}
	if req.UpgradeName != "" {
		fields["upgrade_name"] = req.UpgradeName
	}
	fields["type"] = req.Type
	fields["price"] = req.Price
	fields["secondary_cost"] = req.SecondaryCost
	if req.CurrencyCode != "" {
		fields["currency_code"] = req.CurrencyCode
	}
	fields["shipping_carrier_id"] = req.ShippingCarrierID
	fields["mail_class"] = req.MailClass
	fields["delivery_days_min"] = req.DeliveryDaysMin
	fields["delivery_days_max"] = req.DeliveryDaysMax

	return s.upgradeRepo.UpdateFields(ctx, id, fields)
}

// DeleteUpgradeFromEtsy 从 Etsy 删除加急配送选项
func (s *ShippingProfileService) DeleteUpgradeFromEtsy(ctx context.Context, id int64) error {
	upgrade, err := s.upgradeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("加急配送选项不存在")
		}
		return err
	}

	profile, err := s.profileRepo.GetByID(ctx, upgrade.ShippingProfileID)
	if err != nil {
		return fmt.Errorf("获取运费模板失败: %v", err)
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, profile.ShopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以删除加急配送选项")
	}

	url := fmt.Sprintf("%s/shops/%d/shipping-profiles/%d/upgrades/%d",
		EtsyAPIBaseURL, shop.EtsyShopID, profile.EtsyProfileID, upgrade.EtsyUpgradeID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodDelete, url, nil, developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}

	resp, err := s.dispatcher.Send(ctx, profile.ShopID, httpReq)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	return s.upgradeRepo.Delete(ctx, id)
}

// ==================== 辅助方法 ====================

func (s *ShippingProfileService) getShopWithDeveloper(ctx context.Context, shopID int64) (*model.Shop, *model.Developer, error) {
	shop, err := s.shopRepo.GetByID(ctx, shopID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("店铺不存在")
		}
		return nil, nil, err
	}

	if shop.DeveloperID == 0 {
		return nil, nil, errors.New("店铺未绑定开发者账号")
	}

	developer, err := s.developerRepo.GetByID(ctx, shop.DeveloperID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取开发者账号失败: %v", err)
	}

	return shop, developer, nil
}

func (s *ShippingProfileService) parseEtsyError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Etsy API 错误 (状态码: %d)", resp.StatusCode)
	}

	var etsyErr etsy.EtsyErrorResp
	if err := json.Unmarshal(body, &etsyErr); err != nil {
		return fmt.Errorf("Etsy API 错误 (状态码: %d): %s", resp.StatusCode, string(body))
	}

	if etsyErr.ErrorDescription != "" {
		return fmt.Errorf("Etsy API 错误: %s", etsyErr.ErrorDescription)
	}
	if etsyErr.Error != "" {
		return fmt.Errorf("Etsy API 错误: %s", etsyErr.Error)
	}

	return fmt.Errorf("Etsy API 错误 (状态码: %d)", resp.StatusCode)
}

// ==================== DTO 转换方法 ====================

func (s *ShippingProfileService) convertProfileToResp(profile *model.ShippingProfile) dto.ShippingProfileResp {
	return dto.ShippingProfileResp{
		ID:                int64(profile.ID),
		ShopID:            profile.ShopID,
		EtsyProfileID:     profile.EtsyProfileID,
		Title:             profile.Title,
		OriginCountryISO:  profile.OriginCountryISO,
		OriginPostalCode:  profile.OriginPostalCode,
		ProcessingDaysMin: profile.ProcessingDaysMin,
		ProcessingDaysMax: profile.ProcessingDaysMax,
		EtsySyncedAt:      profile.EtsySyncedAt,
		CreatedAt:         profile.CreatedAt,
		UpdatedAt:         profile.UpdatedAt,
	}
}

func (s *ShippingProfileService) convertDestinationToResp(dest *model.ShippingDestination) dto.ShippingDestinationResp {
	return dto.ShippingDestinationResp{
		ID:                    int64(dest.ID),
		ShippingProfileID:     dest.ShippingProfileID,
		EtsyDestinationID:     dest.EtsyDestinationID,
		DestinationCountryISO: dest.DestinationCountryISO,
		DestinationRegion:     dest.DestinationRegion,
		PrimaryCost:           dest.PrimaryCost,
		SecondaryCost:         dest.SecondaryCost,
		CurrencyCode:          dest.CurrencyCode,
		ShippingCarrierID:     dest.ShippingCarrierID,
		MailClass:             dest.MailClass,
		DeliveryDaysMin:       dest.DeliveryDaysMin,
		DeliveryDaysMax:       dest.DeliveryDaysMax,
		CreatedAt:             dest.CreatedAt,
		UpdatedAt:             dest.UpdatedAt,
	}
}

func (s *ShippingProfileService) convertUpgradeToResp(upgrade *model.ShippingUpgrade) dto.ShippingUpgradeResp {
	typeText := "国内"
	if upgrade.Type == model.ShippingUpgradeTypeInternational {
		typeText = "国际"
	}

	return dto.ShippingUpgradeResp{
		ID:                int64(upgrade.ID),
		ShippingProfileID: upgrade.ShippingProfileID,
		EtsyUpgradeID:     upgrade.EtsyUpgradeID,
		UpgradeName:       upgrade.UpgradeName,
		Type:              upgrade.Type,
		TypeText:          typeText,
		Price:             upgrade.Price,
		SecondaryCost:     upgrade.SecondaryCost,
		CurrencyCode:      upgrade.CurrencyCode,
		ShippingCarrierID: upgrade.ShippingCarrierID,
		MailClass:         upgrade.MailClass,
		DeliveryDaysMin:   upgrade.DeliveryDaysMin,
		DeliveryDaysMax:   upgrade.DeliveryDaysMax,
		CreatedAt:         upgrade.CreatedAt,
		UpdatedAt:         upgrade.UpdatedAt,
	}
}

// ============  退货逻辑 =============

type ReturnPolicyService struct {
	policyRepo    repository.ReturnPolicyRepository
	shopRepo      repository.ShopRepository
	developerRepo repository.DeveloperRepository
	dispatcher    net.Dispatcher
}

func NewReturnPolicyService(
	policyRepo repository.ReturnPolicyRepository,
	shopRepo repository.ShopRepository,
	developerRepo repository.DeveloperRepository,
	dispatcher net.Dispatcher,
) *ReturnPolicyService {
	return &ReturnPolicyService{
		policyRepo:    policyRepo,
		shopRepo:      shopRepo,
		developerRepo: developerRepo,
		dispatcher:    dispatcher,
	}
}

// ==================== 查询方法 ====================

// GetPolicyList 获取退货政策列表
func (s *ReturnPolicyService) GetPolicyList(ctx context.Context, shopID int64) (*dto.ReturnPolicyListResp, error) {
	list, err := s.policyRepo.GetByShopID(ctx, shopID)
	if err != nil {
		return nil, err
	}

	respList := make([]dto.ReturnPolicyResp, 0, len(list))
	for _, policy := range list {
		respList = append(respList, s.convertToResp(&policy))
	}

	return &dto.ReturnPolicyListResp{
		Total: int64(len(respList)),
		List:  respList,
	}, nil
}

// GetPolicyDetail 获取退货政策详情
func (s *ReturnPolicyService) GetPolicyDetail(ctx context.Context, id int64) (*dto.ReturnPolicyResp, error) {
	policy, err := s.policyRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("退货政策不存在")
		}
		return nil, err
	}

	resp := s.convertToResp(policy)
	return &resp, nil
}

// ==================== Etsy 同步方法 ====================

// SyncPoliciesFromEtsy 从 Etsy 同步退货政策
func (s *ReturnPolicyService) SyncPoliciesFromEtsy(ctx context.Context, shopID int64) error {
	shop, developer, err := s.getShopWithDeveloper(ctx, shopID)
	if err != nil {
		return err
	}

	if shop.Status == model.ShopStatusInactive {
		return errors.New("店铺已停用，无法同步")
	}

	url := fmt.Sprintf("%s/shops/%d/policies/return", EtsyAPIBaseURL, shop.EtsyShopID)
	req, err := net.BuildEtsyRequest(ctx, http.MethodGet, url, nil, developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}

	resp, err := s.dispatcher.Send(ctx, shopID, req)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	var etsyResp etsy.EtsyReturnPoliciesResp
	if err := json.NewDecoder(resp.Body).Decode(&etsyResp); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	now := time.Now()
	policies := make([]model.ReturnPolicy, 0, len(etsyResp.Results))
	for _, item := range etsyResp.Results {
		policies = append(policies, model.ReturnPolicy{
			ShopID:           shopID,
			EtsyPolicyID:     item.ReturnPolicyID,
			AcceptsReturns:   item.AcceptsReturns,
			AcceptsExchanges: item.AcceptsExchanges,
			ReturnDeadline:   item.ReturnDeadline,
			EtsySyncedAt:     &now,
		})
	}

	return s.policyRepo.BatchUpsert(ctx, shopID, policies)
}

// CreatePolicyToEtsy 创建退货政策到 Etsy
func (s *ReturnPolicyService) CreatePolicyToEtsy(ctx context.Context, req dto.ReturnPolicyCreateReq) (*dto.ReturnPolicyResp, error) {
	shop, developer, err := s.getShopWithDeveloper(ctx, req.ShopID)
	if err != nil {
		return nil, err
	}

	if shop.Status != model.ShopStatusActive {
		return nil, errors.New("仅正常状态的店铺可以创建退货政策")
	}

	etsyReq := etsy.EtsyReturnPolicyCreateReq{
		AcceptsReturns:   req.AcceptsReturns,
		AcceptsExchanges: req.AcceptsExchanges,
		ReturnDeadline:   req.ReturnDeadline,
	}
	body, err := json.Marshal(etsyReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d/policies/return", EtsyAPIBaseURL, shop.EtsyShopID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPost, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, req.ShopID, httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, s.parseEtsyError(resp)
	}

	var etsyPolicy etsy.EtsyReturnPolicyResp
	if err := json.NewDecoder(resp.Body).Decode(&etsyPolicy); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	now := time.Now()
	policy := &model.ReturnPolicy{
		ShopID:           req.ShopID,
		EtsyPolicyID:     etsyPolicy.ReturnPolicyID,
		AcceptsReturns:   etsyPolicy.AcceptsReturns,
		AcceptsExchanges: etsyPolicy.AcceptsExchanges,
		ReturnDeadline:   etsyPolicy.ReturnDeadline,
		EtsySyncedAt:     &now,
	}

	if err := s.policyRepo.Create(ctx, policy); err != nil {
		return nil, fmt.Errorf("创建本地退货政策失败: %v", err)
	}

	result := s.convertToResp(policy)
	return &result, nil
}

// UpdatePolicyToEtsy 更新退货政策到 Etsy
func (s *ReturnPolicyService) UpdatePolicyToEtsy(ctx context.Context, id int64, req dto.ReturnPolicyUpdateReq) error {
	policy, err := s.policyRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("退货政策不存在")
		}
		return err
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, policy.ShopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以修改退货政策")
	}

	etsyReq := etsy.EtsyReturnPolicyUpdateReq{
		AcceptsReturns:   req.AcceptsReturns,
		AcceptsExchanges: req.AcceptsExchanges,
		ReturnDeadline:   req.ReturnDeadline,
	}
	body, err := json.Marshal(etsyReq)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d/policies/return/%d", EtsyAPIBaseURL, shop.EtsyShopID, policy.EtsyPolicyID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPut, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, policy.ShopID, httpReq)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	now := time.Now()
	fields := map[string]interface{}{
		"accepts_returns":   req.AcceptsReturns,
		"accepts_exchanges": req.AcceptsExchanges,
		"return_deadline":   req.ReturnDeadline,
		"etsy_synced_at":    now,
	}

	return s.policyRepo.UpdateFields(ctx, id, fields)
}

// DeletePolicyFromEtsy 从 Etsy 删除退货政策
func (s *ReturnPolicyService) DeletePolicyFromEtsy(ctx context.Context, id int64) error {
	policy, err := s.policyRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("退货政策不存在")
		}
		return err
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, policy.ShopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以删除退货政策")
	}

	url := fmt.Sprintf("%s/shops/%d/policies/return/%d", EtsyAPIBaseURL, shop.EtsyShopID, policy.EtsyPolicyID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodDelete, url, nil, developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}

	resp, err := s.dispatcher.Send(ctx, policy.ShopID, httpReq)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	return s.policyRepo.Delete(ctx, id)
}

// ==================== 辅助方法 ====================

func (s *ReturnPolicyService) getShopWithDeveloper(ctx context.Context, shopID int64) (*model.Shop, *model.Developer, error) {
	shop, err := s.shopRepo.GetByID(ctx, shopID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("店铺不存在")
		}
		return nil, nil, err
	}

	if shop.DeveloperID == 0 {
		return nil, nil, errors.New("店铺未绑定开发者账号")
	}

	developer, err := s.developerRepo.GetByID(ctx, shop.DeveloperID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取开发者账号失败: %v", err)
	}

	return shop, developer, nil
}

func (s *ReturnPolicyService) parseEtsyError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Etsy API 错误 (状态码: %d)", resp.StatusCode)
	}

	var etsyErr etsy.EtsyErrorResp
	if err := json.Unmarshal(body, &etsyErr); err != nil {
		return fmt.Errorf("Etsy API 错误 (状态码: %d): %s", resp.StatusCode, string(body))
	}

	if etsyErr.ErrorDescription != "" {
		return fmt.Errorf("Etsy API 错误: %s", etsyErr.ErrorDescription)
	}
	if etsyErr.Error != "" {
		return fmt.Errorf("Etsy API 错误: %s", etsyErr.Error)
	}

	return fmt.Errorf("Etsy API 错误 (状态码: %d)", resp.StatusCode)
}

func (s *ReturnPolicyService) convertToResp(policy *model.ReturnPolicy) dto.ReturnPolicyResp {
	return dto.ReturnPolicyResp{
		ID:               int64(policy.ID),
		ShopID:           policy.ShopID,
		EtsyPolicyID:     policy.EtsyPolicyID,
		AcceptsReturns:   policy.AcceptsReturns,
		AcceptsExchanges: policy.AcceptsExchanges,
		ReturnDeadline:   policy.ReturnDeadline,
		EtsySyncedAt:     policy.EtsySyncedAt,
		CreatedAt:        policy.CreatedAt,
		UpdatedAt:        policy.UpdatedAt,
	}
}
