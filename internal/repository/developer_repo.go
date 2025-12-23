package repository

import (
	"context"
	"etsy_dev_v1_202512/internal/model"

	"gorm.io/gorm"
)

type DeveloperRepo struct {
	db *gorm.DB
}

func NewDeveloperRepo(db *gorm.DB) *DeveloperRepo {
	return &DeveloperRepo{db: db}
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
