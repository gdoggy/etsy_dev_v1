package repository

import (
	"context"
	"etsy_dev_v1_202512/internal/core/model"
	"time"

	"gorm.io/gorm"
)

type ShopRepo struct {
	db *gorm.DB
}

func NewShopRepo(db *gorm.DB) *ShopRepo {
	return &ShopRepo{db: db}
}

// Create 创建或保存店铺信息
func (r *ShopRepo) Create(ctx context.Context, shop *model.Shop) error {
	return r.db.WithContext(ctx).Create(shop).Error
}

// GetShopByID 通过内部 ID 查找店铺
func (r *ShopRepo) GetShopByID(ctx context.Context, shopID int64) (*model.Shop, error) {
	var shop *model.Shop
	err := r.db.WithContext(ctx).Preload("Developer").Preload("Proxy").First(&shop, shopID).Error
	if err != nil {
		return nil, err
	}
	return shop, nil
}

// SaveOrUpdate 保存或更新店铺信息
// 解决重复授权时的唯一键冲突问题
func (r *ShopRepo) SaveOrUpdate(ctx context.Context, shop *model.Shop) error {
	var existingShop model.Shop
	// 1. 根据业务唯一键  Etsy Shop ID 查询
	err := r.db.WithContext(ctx).Where("etsy_shop_id = ?", shop.EtsyShopID).First(&existingShop).Error
	if err == nil {
		// Case A: 记录已存在 -> 执行更新
		// 关键步骤：必须把数据库里的主键 ID 赋给当前的 shop 对象
		// 否则 GORM 不知道更新哪一行，会尝试创建新行
		shop.ID = existingShop.ID
		shop.CreatedAt = existingShop.CreatedAt
		// Save 会更新所有字段 (包括 AccessToken, RefreshToken 等)
		return r.db.Save(shop).Error
	}
	// Case B: 记录不存在 -> 执行创建
	return r.db.Create(shop).Error
}

func (r *ShopRepo) UpdateTokenStatus(ctx context.Context, shopID int64, status string) error {
	return r.db.WithContext(ctx).Model(&model.Shop{}).Where("id = ?", shopID).Update("token_status", status).Error
}

// GetShopByEtsyShopID 根据 Etsy 官方 ShopID 查找
func (r *ShopRepo) GetShopByEtsyShopID(ctx context.Context, etsyShopID string) (*model.Shop, error) {
	var shop *model.Shop
	err := r.db.WithContext(ctx).Where("etsy_shop_id = ?", etsyShopID).First(&shop).Error
	if err != nil {
		return nil, err
	}
	return shop, nil
}

// GetShopsByProxyID 查代理受灾店铺
func (r *ShopRepo) GetShopsByProxyID(ctx context.Context, proxyID int64) ([]model.Shop, error) {
	var shops []model.Shop
	err := r.db.WithContext(ctx).Where("proxy_id = ?", proxyID).Find(&shops).Error
	return shops, err
}

// UpdateProxyBinding 迁移绑定关系
func (r *ShopRepo) UpdateProxyBinding(ctx context.Context, shopID int64, proxyID int64) error {
	return r.db.WithContext(ctx).Model(&model.Shop{}).Where("id = ?", shopID).Update("proxy_id", proxyID).Error
}

func (r *ShopRepo) UnbindProxy(ctx context.Context, shopID int64) error {
	return r.db.WithContext(ctx).Model(&model.Shop{}).Where("id = ?", shopID).Update("proxy_id", nil).Error
}

// FindExpiringShops 查询 Token 即将过期店铺  (ExpiresAt < Now + 1h)
func (r *ShopRepo) FindExpiringShops(ctx context.Context) ([]model.Shop, error) {
	// 查询条件：
	// 1. 快过期 (ExpiresAt < Now + 1h)
	// 2. 状态不是 'auth_invalid' (如果已经坏了，就不浪费资源去刷了，等人工处理)
	threshold := time.Now().Add(1 * time.Hour)
	var shops []model.Shop
	err := r.db.WithContext(ctx).Preload("Developer").Preload("Proxy").
		Where("token_expires_at < ? AND token_status != ?", threshold, model.TokenStatusInvalid).
		Find(&shops).Error
	return shops, err
}
