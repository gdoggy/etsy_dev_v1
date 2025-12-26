package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"etsy_dev_v1_202512/internal/model"
)

// ==================== 接口定义 ====================

// DeveloperRepository 开发者仓储接口
type DeveloperRepository interface {
	// 基础 CRUD
	Create(ctx context.Context, developer *model.Developer) error
	GetByID(ctx context.Context, id int64) (*model.Developer, error)
	Update(ctx context.Context, developer *model.Developer) error
	UpdateStatus(ctx context.Context, id int64, status int) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter DeveloperFilter) ([]model.Developer, int64, error)

	// 查询
	FindByApiKey(ctx context.Context, apiKey string) (*model.Developer, error)
	FindByCallbackURL(ctx context.Context, callbackURL string) (*model.Developer, error)
	FindBestByRegion(ctx context.Context, region string) (*model.Developer, error)

	// 关联操作
	UnbindShops(ctx context.Context, developerID int64) error

	// 域名池
	GetRandomActiveDomain(ctx context.Context) (*model.DomainPool, error)
}

// ==================== 过滤条件 ====================

// DeveloperFilter 开发者过滤条件
type DeveloperFilter struct {
	Name     string // 模糊搜索名称
	Status   int    // 状态筛选 (-1 表示不筛选)
	Page     int
	PageSize int
}

// ==================== 仓储实现 ====================

type developerRepo struct {
	db *gorm.DB
}

// NewDeveloperRepository 创建开发者仓储
func NewDeveloperRepository(db *gorm.DB) DeveloperRepository {
	return &developerRepo{db: db}
}

func (r *developerRepo) Create(ctx context.Context, developer *model.Developer) error {
	return r.db.WithContext(ctx).Create(developer).Error
}

func (r *developerRepo) GetByID(ctx context.Context, id int64) (*model.Developer, error) {
	var dev model.Developer
	err := r.db.WithContext(ctx).
		Preload("Shops", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "developer_id", "shop_name", "etsy_shop_id", "token_status")
		}).
		First(&dev, id).Error
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

func (r *developerRepo) Update(ctx context.Context, developer *model.Developer) error {
	return r.db.WithContext(ctx).Save(developer).Error
}

func (r *developerRepo) UpdateStatus(ctx context.Context, id int64, status int) error {
	return r.db.WithContext(ctx).
		Model(&model.Developer{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *developerRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Developer{}, id).Error
}

func (r *developerRepo) List(ctx context.Context, filter DeveloperFilter) ([]model.Developer, int64, error) {
	var list []model.Developer
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Developer{})

	if filter.Name != "" {
		query = query.Where("name LIKE ?", "%"+filter.Name+"%")
	}
	if filter.Status >= 0 {
		query = query.Where("status = ?", filter.Status)
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
	err := query.
		Preload("Shops", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "developer_id", "shop_name", "etsy_shop_id", "token_status")
		}).
		Order("id DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&list).Error

	return list, total, err
}

func (r *developerRepo) FindByApiKey(ctx context.Context, apiKey string) (*model.Developer, error) {
	var dev model.Developer
	err := r.db.WithContext(ctx).
		Where("api_key = ?", apiKey).
		First(&dev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

func (r *developerRepo) FindByCallbackURL(ctx context.Context, callbackURL string) (*model.Developer, error) {
	var dev model.Developer
	err := r.db.WithContext(ctx).
		Where("callback_url = ?", callbackURL).
		First(&dev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

func (r *developerRepo) FindBestByRegion(ctx context.Context, region string) (*model.Developer, error) {
	var dev model.Developer
	err := r.db.WithContext(ctx).
		Table("developers").
		Select("developers.*").
		Joins("LEFT JOIN shops ON shops.developer_id = developers.id").
		Where("shops.region = ? AND developers.status = ?", region, 1).
		Group("developers.id").
		Having("COUNT(developers.id) < ?", 2).
		Order("COUNT(developers.id) ASC").
		First(&dev).Error
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

func (r *developerRepo) UnbindShops(ctx context.Context, developerID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.Shop{}).
		Where("developer_id = ?", developerID).
		Updates(map[string]interface{}{
			"developer_id": 0,
			"token_status": model.ShopTokenStatusInvalid,
		}).Error
}

func (r *developerRepo) GetRandomActiveDomain(ctx context.Context) (*model.DomainPool, error) {
	var domain model.DomainPool
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("RANDOM()").
		Take(&domain).Error
	if err != nil {
		return nil, err
	}
	return &domain, nil
}
