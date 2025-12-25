package repository

import (
	"context"
	"errors"
	"etsy_dev_v1_202512/internal/model"

	"gorm.io/gorm"
)

// DeveloperListFilter 列表查询的过滤条件
type DeveloperListFilter struct {
	Name     string // 模糊搜索名称
	Status   int    // 状态筛选 (-1 表示不筛选)
	Page     int
	PageSize int
}
type DeveloperRepo struct {
	db *gorm.DB
}

func NewDeveloperRepo(db *gorm.DB) *DeveloperRepo {
	return &DeveloperRepo{db: db}
}

// Create 新建 developer账号
func (r *DeveloperRepo) Create(ctx context.Context, developer *model.Developer) error {
	return r.db.WithContext(ctx).Create(developer).Error
}

// Update 更新
func (r *DeveloperRepo) Update(ctx context.Context, developer *model.Developer) error {
	return r.db.WithContext(ctx).Save(developer).Error
}

// UpdateStatus 更新状态
func (r *DeveloperRepo) UpdateStatus(ctx context.Context, id int64, status int) error {
	return r.db.WithContext(ctx).
		Model(&model.Developer{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Delete 软删除
func (r *DeveloperRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Developer{}, id).Error
}

// GetByID 根据 ID 获取单条详情（含关联 Shops）
func (r *DeveloperRepo) GetByID(ctx context.Context, id int64) (*model.Developer, error) {
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

// List 分页列表查询
func (r *DeveloperRepo) List(ctx context.Context, filter DeveloperListFilter) ([]model.Developer, int64, error) {
	var list []model.Developer
	var total int64

	db := r.db.WithContext(ctx).Model(&model.Developer{})

	// 动态构建查询条件
	if filter.Name != "" {
		db = db.Where("name LIKE ?", "%"+filter.Name+"%")
	}
	if filter.Status >= 0 {
		db = db.Where("status = ?", filter.Status)
	}

	// 计算总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (filter.Page - 1) * filter.PageSize
	err := db.
		Preload("Shops", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "developer_id", "shop_name", "etsy_shop_id", "token_status")
		}).
		Order("id DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&list).Error

	return list, total, err
}

// FindBestDevByRegion 根据 region 找最优 developer
func (r *DeveloperRepo) FindBestDevByRegion(ctx context.Context, region string) (*model.Developer, error) {
	var dev model.Developer
	// SQL 逻辑：
	// 1. SELECT developers.*: 选择所有字段
	// 2. LEFT JOIN shops: 关联店铺表，用来计数
	// 3. WHERE: 地区匹配 + 状态正常 + 排除已满载(例如 >=2)的
	// 4. GROUP BY: 按代理聚合
	// 5. ORDER BY: 按店铺数量升序 (COUNT(shops.id) ASC)
	// 6. LIMIT 1: 只取最闲的那一个

	err := r.db.WithContext(ctx).
		Table("developers").
		Select("developers.*").
		Joins("LEFT JOIN shops ON shops.developer_id = developers.id").
		Where("shops.region = ? AND developers.status = ?", region, 1). // 1=Normal
		Group("developers.id").
		// HAVING 用于过滤聚合后的结果 (找出绑定数小于 2 的)
		Having("COUNT(developers.id) < ?", 2).
		// 核心：动态计算负载并排序
		Order("COUNT(developers.id) ASC").
		First(&dev).Error

	if err != nil {
		return nil, err
	}
	return &dev, nil
}

// FindByApiKey 根据 apiKey 查重
func (r *DeveloperRepo) FindByApiKey(ctx context.Context, key string) (*model.Developer, error) {
	var dev model.Developer
	err := r.db.WithContext(ctx).
		Where("api_key = ?", key).
		First(&dev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 没找到，说明不重复，是安全的
	}
	if err != nil {
		return nil, err // 数据库错误
	}
	return &dev, nil // 找到了，说明重复
}

// FindByCallbackURL 根据 CallbackURL 查询（用于唯一性校验，可选）
func (r *DeveloperRepo) FindByCallbackURL(ctx context.Context, callbackURL string) (*model.Developer, error) {
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

// GetRandomActiveDomain 随机获取可用根域名
func (r *DeveloperRepo) GetRandomActiveDomain(ctx context.Context) (*model.DomainPool, error) {
	var domain model.DomainPool
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Order("RANDOM()").Take(&domain).Error
	if err != nil {
		return nil, err
	}
	return &domain, err
}

// UnbindShops 解绑该 Developer 下所有关联的 Shop
// 将 Shop.DeveloperID 置为 0，TokenStatus 置为 auth_invalid
func (r *DeveloperRepo) UnbindShops(ctx context.Context, developerID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.Shop{}).
		Where("developer_id = ?", developerID).
		Updates(map[string]interface{}{
			"developer_id": 0,
			"token_status": model.TokenStatusInvalid,
		}).Error
}
