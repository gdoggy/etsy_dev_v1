package repository

import (
	"context"
	"errors"
	"etsy_dev_v1_202512/internal/model"

	"gorm.io/gorm"
)

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

// Delete 软删除
func (r *DeveloperRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Developer{}, id).Error
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

// GetRandomActiveDomain 随机获取可用根域名
func (r *DeveloperRepo) GetRandomActiveDomain(ctx context.Context) (*model.DomainPool, error) {
	var domain model.DomainPool
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Order("RANDOM()").Take(&domain).Error
	if err != nil {
		return nil, err
	}
	return &domain, err
}
