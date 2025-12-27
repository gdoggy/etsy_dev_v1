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

const (
	EtsyAPIBaseURL     = "https://api.etsy.com/v3/application"
	ManualSyncCooldown = 1 * time.Hour // 手动同步冷却时间
)

type ShopService struct {
	shopRepo        repository.ShopRepository
	sectionRepo     repository.ShopSectionRepository
	profileRepo     repository.ShippingProfileRepository
	destinationRepo repository.ShippingDestinationRepository
	upgradeRepo     repository.ShippingUpgradeRepository
	policyRepo      repository.ReturnPolicyRepository
	developerRepo   repository.DeveloperRepository
	dispatcher      net.Dispatcher
	proxyRepo       repository.ProxyRepository
}

func NewShopService(
	shopRepo repository.ShopRepository,
	sectionRepo repository.ShopSectionRepository,
	profileRepo repository.ShippingProfileRepository,
	destinationRepo repository.ShippingDestinationRepository,
	upgradeRepo repository.ShippingUpgradeRepository,
	policyRepo repository.ReturnPolicyRepository,
	developerRepo repository.DeveloperRepository,
	dispatcher net.Dispatcher,
	proxyRepo repository.ProxyRepository,
) *ShopService {
	return &ShopService{
		shopRepo:        shopRepo,
		sectionRepo:     sectionRepo,
		profileRepo:     profileRepo,
		destinationRepo: destinationRepo,
		upgradeRepo:     upgradeRepo,
		policyRepo:      policyRepo,
		developerRepo:   developerRepo,
		dispatcher:      dispatcher,
		proxyRepo:       proxyRepo,
	}
}

// ==================== 查询方法 ====================

// GetByID 根据ID获取店铺
func (s *ShopService) GetByID(ctx context.Context, id int64) (*model.Shop, error) {
	return s.shopRepo.GetByID(ctx, id)
}

// GetShopList 获取店铺列表
func (s *ShopService) GetShopList(ctx context.Context, req dto.ShopListReq) (*dto.ShopListResp, error) {
	filter := repository.ShopFilter{
		ShopName:    req.ShopName,
		Status:      req.Status,
		ProxyID:     req.ProxyID,
		DeveloperID: req.DeveloperID,
		Page:        req.Page,
		PageSize:    req.PageSize,
	}

	list, total, err := s.shopRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	respList := make([]dto.ShopResp, 0, len(list))
	for _, shop := range list {
		respList = append(respList, s.convertToResp(&shop))
	}

	return &dto.ShopListResp{
		Total: total,
		List:  respList,
	}, nil
}

// GetShopDetail 获取店铺详情
func (s *ShopService) GetShopDetail(ctx context.Context, id int64) (*dto.ShopDetailResp, error) {
	shop, err := s.shopRepo.GetByIDWithRelations(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("店铺不存在")
		}
		return nil, err
	}

	resp := &dto.ShopDetailResp{
		ShopResp: s.convertToResp(shop),
	}

	resp.Sections = make([]dto.ShopSectionResp, 0, len(shop.Sections))
	for _, section := range shop.Sections {
		resp.Sections = append(resp.Sections, s.convertSectionToResp(&section))
	}

	resp.ShippingProfiles = make([]dto.ShippingProfileResp, 0, len(shop.ShippingProfiles))
	for _, profile := range shop.ShippingProfiles {
		resp.ShippingProfiles = append(resp.ShippingProfiles, s.convertProfileToResp(&profile))
	}

	resp.ReturnPolicies = make([]dto.ReturnPolicyResp, 0, len(shop.ReturnPolicies))
	for _, policy := range shop.ReturnPolicies {
		resp.ReturnPolicies = append(resp.ReturnPolicies, s.convertPolicyToResp(&policy))
	}

	return resp, nil
}

// ==================== 授权相关（供 AuthService 调用）====================

// CreateShop 创建空店铺记录
// 调用方：AuthService（初次授权时）
func (s *ShopService) CreateShop(ctx context.Context, shop *model.Shop) (*model.Shop, error) {
	existing, err := s.shopRepo.GetByID(ctx, shop.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("店铺已存在")
	}

	shop.Status = model.ShopStatusPending
	shop.TokenStatus = model.ShopTokenStatusInvalid
	// 绑定 developer
	dev, err := s.developerRepo.FindBestDev(ctx)
	if err != nil {
		return nil, err
	}
	shop.DeveloperID = dev.ID
	// 绑定 proxy
	proxy, err := s.proxyRepo.FindSpareProxy(ctx, shop.Region)
	if err != nil {
		return nil, err
	}
	shop.ProxyID = proxy.ID
	if err := s.shopRepo.Create(ctx, shop); err != nil {
		return nil, err
	}
	return shop, nil
}

// GetByEtsyShopID 根据 EtsyShopID 查询店铺
// 调用方：AuthService（判断是否已存在）
func (s *ShopService) GetByEtsyShopID(ctx context.Context, etsyShopID int64) (*model.Shop, error) {
	return s.shopRepo.GetByEtsyShopID(ctx, etsyShopID)
}

// UpdateTokenInfo 更新 Token 信息
// 调用方：AuthService（初次授权/重新授权/Token 刷新）
func (s *ShopService) UpdateTokenInfo(ctx context.Context, shopID int64, accessToken, refreshToken string, tokenExpiry time.Time) error {
	fields := map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_expiry":  tokenExpiry,
		"token_status":  model.ShopTokenStatusValid,
		"status":        model.ShopStatusActive,
	}
	return s.shopRepo.UpdateFields(ctx, shopID, fields)
}

// UpdateTokenStatus 更新 Token 状态
// 调用方：Task（巡检发现 Token 失效时）
func (s *ShopService) UpdateTokenStatus(ctx context.Context, shopID int64, tokenStatus string) error {
	fields := map[string]interface{}{
		"token_status": tokenStatus,
	}

	if tokenStatus == model.ShopTokenStatusInvalid || tokenStatus == model.ShopTokenStatusExpired {
		fields["status"] = model.ShopStatusPending
	}

	return s.shopRepo.UpdateFields(ctx, shopID, fields)
}

// UpdateStatusBySystem 系统自动更新状态
// 调用方：Task / AuthService
// 仅允许设置为 0（待授权）或 1（正常）
func (s *ShopService) UpdateStatusBySystem(ctx context.Context, shopID int64, status int) error {
	if status != model.ShopStatusPending && status != model.ShopStatusActive {
		return errors.New("系统仅允许设置状态为待授权或正常")
	}
	return s.shopRepo.UpdateFields(ctx, shopID, map[string]interface{}{"status": status})
}

// ==================== 用户操作 ====================

// StopShop 用户停用店铺
func (s *ShopService) StopShop(ctx context.Context, shopID int64) error {
	shop, err := s.shopRepo.GetByID(ctx, shopID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("店铺不存在")
		}
		return err
	}

	if shop.Status == model.ShopStatusInactive {
		return errors.New("店铺已处于停用状态")
	}

	return s.shopRepo.UpdateFields(ctx, shopID, map[string]interface{}{"status": model.ShopStatusInactive})
}

// ResumeShop 用户恢复店铺（触发重新授权）
func (s *ShopService) ResumeShop(ctx context.Context, shopID int64) error {
	shop, err := s.shopRepo.GetByID(ctx, shopID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("店铺不存在")
		}
		return err
	}

	if shop.Status != model.ShopStatusInactive {
		return errors.New("仅停用状态的店铺可以恢复")
	}

	return s.shopRepo.UpdateFields(ctx, shopID, map[string]interface{}{
		"status":       model.ShopStatusPending,
		"token_status": model.ShopTokenStatusInvalid,
	})
}

// DeleteShop 删除店铺（仅 ERP 解绑）
func (s *ShopService) DeleteShop(ctx context.Context, shopID int64) error {
	shop, err := s.shopRepo.GetByID(ctx, shopID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("店铺不存在")
		}
		return err
	}

	if shop.Status != model.ShopStatusInactive {
		return errors.New("请先停用店铺再删除")
	}

	// 删除关联数据
	if err := s.sectionRepo.DeleteByShopID(ctx, shopID); err != nil {
		return err
	}

	profiles, err := s.profileRepo.GetByShopID(ctx, shopID)
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		profileID := int64(profile.ID)
		if err := s.destinationRepo.DeleteByProfileID(ctx, profileID); err != nil {
			return err
		}
		if err := s.upgradeRepo.DeleteByProfileID(ctx, profileID); err != nil {
			return err
		}
	}

	if err := s.profileRepo.DeleteByShopID(ctx, shopID); err != nil {
		return err
	}

	if err := s.policyRepo.DeleteByShopID(ctx, shopID); err != nil {
		return err
	}

	return s.shopRepo.Delete(ctx, shopID)
}

// ==================== Etsy 数据同步 ====================

// CanManualSync 检查是否可手动同步
func (s *ShopService) CanManualSync(ctx context.Context, shopID int64) (bool, *time.Time, error) {
	shop, err := s.shopRepo.GetByID(ctx, shopID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, errors.New("店铺不存在")
		}
		return false, nil, err
	}

	if shop.EtsySyncedAt == nil {
		return true, nil, nil
	}

	nextSyncTime := shop.EtsySyncedAt.Add(ManualSyncCooldown)
	if time.Now().Before(nextSyncTime) {
		return false, &nextSyncTime, nil
	}

	return true, nil, nil
}

// ManualSyncShop 手动同步店铺数据（含频率限制）
func (s *ShopService) ManualSyncShop(ctx context.Context, shopID int64) (*dto.ShopSyncResp, error) {
	canSync, nextSyncTime, err := s.CanManualSync(ctx, shopID)
	if err != nil {
		return nil, err
	}

	if !canSync {
		return &dto.ShopSyncResp{
			Success:      false,
			Message:      "同步过于频繁，请稍后再试",
			NextSyncTime: nextSyncTime,
		}, nil
	}

	if err := s.SyncShopFromEtsy(ctx, shopID); err != nil {
		return &dto.ShopSyncResp{
			Success: false,
			Message: fmt.Sprintf("同步失败: %v", err),
		}, nil
	}

	now := time.Now()
	nextTime := now.Add(ManualSyncCooldown)

	return &dto.ShopSyncResp{
		Success:      true,
		Message:      "同步成功",
		SyncedAt:     &now,
		NextSyncTime: &nextTime,
	}, nil
}

// SyncShopFromEtsy 从 Etsy 同步店铺信息
// 调用方：AuthService（授权后）/ Task（定时）/ 手动触发
func (s *ShopService) SyncShopFromEtsy(ctx context.Context, shopID int64) error {
	shop, developer, err := s.getShopWithDeveloper(ctx, shopID)
	if err != nil {
		return err
	}

	if shop.Status == model.ShopStatusInactive {
		return errors.New("店铺已停用，无法同步")
	}

	url := fmt.Sprintf("%s/shops/%d", EtsyAPIBaseURL, shop.EtsyShopID)
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

	var etsyShop etsy.EtsyShopResp
	if err := json.NewDecoder(resp.Body).Decode(&etsyShop); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	now := time.Now()
	fields := map[string]interface{}{
		"etsy_user_id":           etsyShop.UserID,
		"shop_name":              etsyShop.ShopName,
		"title":                  etsyShop.Title,
		"announcement":           etsyShop.Announcement,
		"sale_message":           etsyShop.SaleMessage,
		"digital_sale_message":   etsyShop.DigitalSaleMessage,
		"currency_code":          etsyShop.CurrencyCode,
		"listing_active_count":   etsyShop.ListingActiveCount,
		"transaction_sold_count": etsyShop.TransactionSoldCount,
		"review_count":           etsyShop.ReviewCount,
		"review_average":         etsyShop.ReviewAverage,
		"etsy_synced_at":         now,
	}

	return s.shopRepo.UpdateFields(ctx, shopID, fields)
}

// UpdateShopToEtsy 推送店铺修改到 Etsy
// 流程：推送 Etsy → 成功后更新本地
func (s *ShopService) UpdateShopToEtsy(ctx context.Context, shopID int64, req dto.ShopUpdateToEtsyReq) error {
	shop, developer, err := s.getShopWithDeveloper(ctx, shopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以修改")
	}

	etsyReq := etsy.EtsyShopUpdateReq{
		Title:              req.Title,
		Announcement:       req.Announcement,
		SaleMessage:        req.SaleMessage,
		DigitalSaleMessage: req.DigitalSaleMessage,
	}

	body, err := json.Marshal(etsyReq)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d", EtsyAPIBaseURL, shop.EtsyShopID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPut, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, shopID, httpReq)
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
	if req.Announcement != "" {
		fields["announcement"] = req.Announcement
	}
	if req.SaleMessage != "" {
		fields["sale_message"] = req.SaleMessage
	}
	if req.DigitalSaleMessage != "" {
		fields["digital_sale_message"] = req.DigitalSaleMessage
	}

	return s.shopRepo.UpdateFields(ctx, shopID, fields)
}

// ==================== Shop Section 同步 ====================

// SyncSectionsFromEtsy 从 Etsy 同步店铺分区
func (s *ShopService) SyncSectionsFromEtsy(ctx context.Context, shopID int64) error {
	shop, developer, err := s.getShopWithDeveloper(ctx, shopID)
	if err != nil {
		return err
	}

	if shop.Status == model.ShopStatusInactive {
		return errors.New("店铺已停用，无法同步")
	}

	url := fmt.Sprintf("%s/shops/%d/sections", EtsyAPIBaseURL, shop.EtsyShopID)
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

	var etsyResp etsy.EtsyShopSectionsResp
	if err := json.NewDecoder(resp.Body).Decode(&etsyResp); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	now := time.Now()
	sections := make([]model.ShopSection, 0, len(etsyResp.Results))
	for _, item := range etsyResp.Results {
		sections = append(sections, model.ShopSection{
			ShopID:             shopID,
			EtsySectionID:      item.ShopSectionID,
			Title:              item.Title,
			Rank:               item.Rank,
			ActiveListingCount: item.ActiveListingCount,
			EtsySyncedAt:       &now,
		})
	}

	return s.sectionRepo.BatchUpsert(ctx, shopID, sections)
}

// CreateSectionToEtsy 创建分区到 Etsy
func (s *ShopService) CreateSectionToEtsy(ctx context.Context, shopID int64, title string) (*dto.ShopSectionResp, error) {
	shop, developer, err := s.getShopWithDeveloper(ctx, shopID)
	if err != nil {
		return nil, err
	}

	if shop.Status != model.ShopStatusActive {
		return nil, errors.New("仅正常状态的店铺可以创建分区")
	}

	etsyReq := etsy.EtsyShopSectionCreateReq{
		Title: title,
	}
	body, err := json.Marshal(etsyReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d/sections", EtsyAPIBaseURL, shop.EtsyShopID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPost, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, shopID, httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, s.parseEtsyError(resp)
	}

	var etsySection etsy.EtsyShopSectionResp
	if err := json.NewDecoder(resp.Body).Decode(&etsySection); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	now := time.Now()
	section := &model.ShopSection{
		ShopID:             shopID,
		EtsySectionID:      etsySection.ShopSectionID,
		Title:              etsySection.Title,
		Rank:               etsySection.Rank,
		ActiveListingCount: etsySection.ActiveListingCount,
		EtsySyncedAt:       &now,
	}

	if err := s.sectionRepo.Create(ctx, section); err != nil {
		return nil, fmt.Errorf("创建本地分区失败: %v", err)
	}

	result := s.convertSectionToResp(section)
	return &result, nil
}

// UpdateSectionToEtsy 更新分区到 Etsy
func (s *ShopService) UpdateSectionToEtsy(ctx context.Context, sectionID int64, title string) error {
	section, err := s.sectionRepo.GetByID(ctx, sectionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("分区不存在")
		}
		return err
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, section.ShopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以修改分区")
	}

	etsyReq := etsy.EtsyShopSectionUpdateReq{
		Title: title,
	}
	body, err := json.Marshal(etsyReq)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}

	url := fmt.Sprintf("%s/shops/%d/sections/%d", EtsyAPIBaseURL, shop.EtsyShopID, section.EtsySectionID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPut, url, bytes.NewReader(body), developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, section.ShopID, httpReq)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	now := time.Now()
	return s.sectionRepo.UpdateFields(ctx, sectionID, map[string]interface{}{
		"title":          title,
		"etsy_synced_at": now,
	})
}

// DeleteSectionFromEtsy 从 Etsy 删除分区
func (s *ShopService) DeleteSectionFromEtsy(ctx context.Context, sectionID int64) error {
	section, err := s.sectionRepo.GetByID(ctx, sectionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("分区不存在")
		}
		return err
	}

	shop, developer, err := s.getShopWithDeveloper(ctx, section.ShopID)
	if err != nil {
		return err
	}

	if shop.Status != model.ShopStatusActive {
		return errors.New("仅正常状态的店铺可以删除分区")
	}

	url := fmt.Sprintf("%s/shops/%d/sections/%d", EtsyAPIBaseURL, shop.EtsyShopID, section.EtsySectionID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodDelete, url, nil, developer.ApiKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %v", err)
	}

	resp, err := s.dispatcher.Send(ctx, section.ShopID, httpReq)
	if err != nil {
		return fmt.Errorf("请求 Etsy API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return s.parseEtsyError(resp)
	}

	return s.sectionRepo.Delete(ctx, sectionID)
}

// ==================== 辅助方法 ====================

func (s *ShopService) getShopWithDeveloper(ctx context.Context, shopID int64) (*model.Shop, *model.Developer, error) {
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

func (s *ShopService) parseEtsyError(resp *http.Response) error {
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

func (s *ShopService) convertToResp(shop *model.Shop) dto.ShopResp {
	resp := dto.ShopResp{
		ID:                   int64(shop.ID),
		EtsyShopID:           shop.EtsyShopID,
		EtsyUserID:           shop.EtsyUserID,
		ShopName:             shop.ShopName,
		Title:                shop.Title,
		Announcement:         shop.Announcement,
		SaleMessage:          shop.SaleMessage,
		DigitalSaleMessage:   shop.DigitalSaleMessage,
		CurrencyCode:         shop.CurrencyCode,
		ListingActiveCount:   shop.ListingActiveCount,
		TransactionSoldCount: shop.TransactionSoldCount,
		ReviewCount:          shop.ReviewCount,
		ReviewAverage:        shop.ReviewAverage,
		TokenStatus:          shop.TokenStatus,
		Status:               shop.Status,
		EtsySyncedAt:         shop.EtsySyncedAt,
		CreatedAt:            shop.CreatedAt,
		UpdatedAt:            shop.UpdatedAt,
		ProxyID:              shop.ProxyID,
		DeveloperID:          shop.DeveloperID,
	}

	switch shop.Status {
	case model.ShopStatusPending:
		resp.StatusText = "待授权"
	case model.ShopStatusActive:
		resp.StatusText = "正常"
	case model.ShopStatusInactive:
		resp.StatusText = "已停用"
	default:
		resp.StatusText = "未知"
	}

	if shop.Proxy != nil {
		resp.ProxyIP = shop.Proxy.IP
		resp.ProxyRegion = shop.Proxy.Region
	}
	if shop.Developer != nil {
		resp.DeveloperName = shop.Developer.Name
	}

	return resp
}

func (s *ShopService) convertSectionToResp(section *model.ShopSection) dto.ShopSectionResp {
	return dto.ShopSectionResp{
		ID:                 int64(section.ID),
		ShopID:             section.ShopID,
		EtsySectionID:      section.EtsySectionID,
		Title:              section.Title,
		Rank:               section.Rank,
		ActiveListingCount: section.ActiveListingCount,
		EtsySyncedAt:       section.EtsySyncedAt,
		CreatedAt:          section.CreatedAt,
		UpdatedAt:          section.UpdatedAt,
	}
}

func (s *ShopService) convertProfileToResp(profile *model.ShippingProfile) dto.ShippingProfileResp {
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

func (s *ShopService) convertPolicyToResp(policy *model.ReturnPolicy) dto.ReturnPolicyResp {
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
