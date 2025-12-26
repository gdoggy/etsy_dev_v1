package repository

import (
	"context"

	"gorm.io/gorm"

	"etsy_dev_v1_202512/internal/model"
)

// ==================== 接口定义 ====================

// ShopRepository 店铺仓储接口
type ShopRepository interface {
	Create(ctx context.Context, shop *model.Shop) error
	GetByID(ctx context.Context, id int64) (*model.Shop, error)
	GetByIDWithRelations(ctx context.Context, id int64) (*model.Shop, error)
	GetByEtsyShopID(ctx context.Context, etsyShopID int64) (*model.Shop, error)
	Update(ctx context.Context, shop *model.Shop) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter ShopFilter) ([]model.Shop, int64, error)

	// Proxy 相关
	GetByProxyID(ctx context.Context, proxyID int64) ([]model.Shop, error)

	// Token 相关
	FindExpiringShops(ctx context.Context) ([]model.Shop, error)
	UpdateToken(ctx context.Context, id int64, accessToken, refreshToken string, expiresAt int64) error

	// 活跃店铺
	ListActiveShops(ctx context.Context) ([]*model.Shop, error)

	// 开发者关联
	GetDeveloperByShopID(ctx context.Context, shopID int64) (*model.Developer, error)
}

// ==================== 过滤条件 ====================

// ShopFilter 店铺过滤条件
type ShopFilter struct {
	UserID      int64
	ShopName    string
	Status      int   // -1 表示不筛选
	ProxyID     int64 // 0 表示不筛选
	DeveloperID int64 // 0 表示不筛选
	TokenStatus int
	Page        int
	PageSize    int
}

// ==================== 仓储实现 ====================

// shopRepo 店铺仓储实现
type shopRepo struct {
	db *gorm.DB
}

// NewShopRepository 创建店铺仓储
func NewShopRepository(db *gorm.DB) ShopRepository {
	return &shopRepo{db: db}
}

func (r *shopRepo) Create(ctx context.Context, shop *model.Shop) error {
	return r.db.WithContext(ctx).Create(shop).Error
}

func (r *shopRepo) GetByID(ctx context.Context, id int64) (*model.Shop, error) {
	var shop model.Shop
	if err := r.db.WithContext(ctx).First(&shop, id).Error; err != nil {
		return nil, err
	}
	return &shop, nil
}

func (r *shopRepo) GetByIDWithRelations(ctx context.Context, id int64) (*model.Shop, error) {
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

func (r *shopRepo) GetByEtsyShopID(ctx context.Context, etsyShopID int64) (*model.Shop, error) {
	var shop model.Shop
	if err := r.db.WithContext(ctx).Where("etsy_shop_id = ?", etsyShopID).First(&shop).Error; err != nil {
		return nil, err
	}
	return &shop, nil
}

func (r *shopRepo) Update(ctx context.Context, shop *model.Shop) error {
	return r.db.WithContext(ctx).Save(shop).Error
}

func (r *shopRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Shop{}).Where("id = ?", id).Updates(fields).Error
}

func (r *shopRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Shop{}, id).Error
}

func (r *shopRepo) List(ctx context.Context, filter ShopFilter) ([]model.Shop, int64, error) {
	var shops []model.Shop
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Shop{})

	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.ShopName != "" {
		query = query.Where("shop_name LIKE ?", "%"+filter.ShopName+"%")
	}
	if filter.Status >= 0 {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.ProxyID > 0 {
		query = query.Where("proxy_id = ?", filter.ProxyID)
	}
	if filter.DeveloperID > 0 {
		query = query.Where("developer_id = ?", filter.DeveloperID)
	}
	if filter.TokenStatus > 0 {
		query = query.Where("token_status = ?", filter.TokenStatus)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("created_at DESC").Limit(filter.PageSize).Offset(offset).Find(&shops).Error; err != nil {
		return nil, 0, err
	}

	return shops, total, nil
}

func (r *shopRepo) GetByProxyID(ctx context.Context, proxyID int64) ([]model.Shop, error) {
	var shops []model.Shop
	err := r.db.WithContext(ctx).
		Model(&model.Shop{}).
		Where("proxy_id = ?", proxyID).
		Find(&shops).Error
	return shops, err
}

// FindExpiringShops 查找即将过期的店铺（Token 有效但即将过期）
func (r *shopRepo) FindExpiringShops(ctx context.Context) ([]model.Shop, error) {
	var shops []model.Shop
	// Token 状态为活跃，且 Token 即将过期（30分钟内）
	err := r.db.WithContext(ctx).
		Model(&model.Shop{}).
		Where("token_status = ?", model.ShopTokenStatusValid).
		Find(&shops).Error
	return shops, err
}

// UpdateToken 更新 Token
func (r *shopRepo) UpdateToken(ctx context.Context, id int64, accessToken, refreshToken string, expiresAt int64) error {
	return r.db.WithContext(ctx).
		Model(&model.Shop{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"token_expires": expiresAt,
			"token_status":  model.ShopTokenStatusExpired,
		}).Error
}

// ListActiveShops 获取所有活跃店铺
func (r *shopRepo) ListActiveShops(ctx context.Context) ([]*model.Shop, error) {
	var shops []*model.Shop
	err := r.db.WithContext(ctx).
		Where("token_status = ?", model.ShopTokenStatusValid).
		Find(&shops).Error
	return shops, err
}

// GetDeveloperByShopID 根据店铺ID获取开发者信息
func (r *shopRepo) GetDeveloperByShopID(ctx context.Context, shopID int64) (*model.Developer, error) {
	var shop model.Shop
	if err := r.db.WithContext(ctx).First(&shop, shopID).Error; err != nil {
		return nil, err
	}

	var developer model.Developer
	if err := r.db.WithContext(ctx).First(&developer, shop.DeveloperID).Error; err != nil {
		return nil, err
	}

	return &developer, nil
}

// GetWithDeveloper 获取店铺及其开发者信息
func (r *shopRepo) GetWithDeveloper(ctx context.Context, id int64) (*model.Shop, error) {
	var shop model.Shop
	if err := r.db.WithContext(ctx).Preload("Developer").First(&shop, id).Error; err != nil {
		return nil, err
	}
	return &shop, nil
}
