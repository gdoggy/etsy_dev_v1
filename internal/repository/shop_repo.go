package repository

import (
	"context"
	"etsy_dev_v1_202512/internal/model"
	"time"

	"gorm.io/gorm"
)

// ShopListFilter 店铺列表查询过滤条件
type ShopListFilter struct {
	ShopName    string
	Status      int   // -1 表示不筛选
	ProxyID     int64 // 0 表示不筛选
	DeveloperID int64 // 0 表示不筛选
	Page        int
	PageSize    int
}
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

// GetByID 通过内部 ID 查找店铺
func (r *ShopRepo) GetByID(ctx context.Context, shopID int64) (*model.Shop, error) {
	var shop model.Shop
	err := r.db.WithContext(ctx).
		Preload("Developer").
		Preload("Proxy").
		First(&shop, shopID).Error
	if err != nil {
		return nil, err
	}
	return &shop, nil
}

// GetByIDWithRelations 根据ID获取店铺（含全部关联数据）
func (r *ShopRepo) GetByIDWithRelations(ctx context.Context, id int64) (*model.Shop, error) {
	var shop model.Shop
	err := r.db.WithContext(ctx).
		Preload("Proxy").
		Preload("Developer").
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order("rank ASC, id ASC")
		}).
		Preload("ShippingProfiles").
		Preload("ReturnPolicies").
		First(&shop, id).Error
	if err != nil {
		return nil, err
	}
	return &shop, nil
}

// GetByEtsyShopID 根据Etsy店铺ID获取
func (r *ShopRepo) GetByEtsyShopID(ctx context.Context, etsyShopID int64) (*model.Shop, error) {
	var shop model.Shop
	err := r.db.WithContext(ctx).
		Where("etsy_shop_id = ?", etsyShopID).
		First(&shop).Error
	if err != nil {
		return nil, err
	}
	return &shop, nil
}

// List 分页列表查询
func (r *ShopRepo) List(ctx context.Context, filter ShopListFilter) ([]model.Shop, int64, error) {
	var list []model.Shop
	var total int64

	db := r.db.WithContext(ctx).Model(&model.Shop{})

	// 动态构建查询条件
	if filter.ShopName != "" {
		db = db.Where("shop_name LIKE ?", "%"+filter.ShopName+"%")
	}
	if filter.Status >= 0 {
		db = db.Where("status = ?", filter.Status)
	}
	if filter.ProxyID > 0 {
		db = db.Where("proxy_id = ?", filter.ProxyID)
	}
	if filter.DeveloperID > 0 {
		db = db.Where("developer_id = ?", filter.DeveloperID)
	}

	// 计算总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (filter.Page - 1) * filter.PageSize
	err := db.
		Preload("Proxy", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "ip", "port")
		}).
		Preload("Developer", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "api_key")
		}).
		Order("id DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&list).Error

	return list, total, err
}

// Update 更新店铺
func (r *ShopRepo) Update(ctx context.Context, shop *model.Shop) error {
	return r.db.WithContext(ctx).Save(shop).Error
}

// UpdateFields 更新指定字段
func (r *ShopRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.Shop{}).
		Where("id = ?", id).
		Updates(fields).Error
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

// UpdateStatus 更新状态
func (r *ShopRepo) UpdateStatus(ctx context.Context, id int64, status int) error {
	return r.db.WithContext(ctx).
		Model(&model.Shop{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateTokenStatus 更新Token状态
func (r *ShopRepo) UpdateTokenStatus(ctx context.Context, id int64, tokenStatus string) error {
	return r.db.WithContext(ctx).
		Model(&model.Shop{}).
		Where("id = ?", id).
		Update("token_status", tokenStatus).Error
}

// Delete 软删除店铺
func (r *ShopRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Shop{}, id).Error
}

// GetByDeveloperID 根据开发者ID获取所有店铺
func (r *ShopRepo) GetByDeveloperID(ctx context.Context, developerID int64) ([]model.Shop, error) {
	var list []model.Shop
	err := r.db.WithContext(ctx).
		Where("developer_id = ?", developerID).
		Find(&list).Error
	return list, err
}

// GetByProxyID 查代理受灾店铺
func (r *ShopRepo) GetByProxyID(ctx context.Context, proxyID int64) ([]model.Shop, error) {
	var list []model.Shop
	err := r.db.WithContext(ctx).
		Where("proxy_id = ?", proxyID).
		Find(&list).Error
	return list, err
}

// UpdateEtsySyncedAt 更新Etsy同步时间
func (r *ShopRepo) UpdateEtsySyncedAt(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.Shop{}).
		Where("id = ?", id).
		Update("etsy_synced_at", gorm.Expr("NOW()")).Error
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
	if err != nil {
		return nil, err
	}
	return shops, err
}
