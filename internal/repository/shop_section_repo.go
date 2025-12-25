package repository

import (
	"context"
	"etsy_dev_v1_202512/internal/model"

	"gorm.io/gorm"
)

// ShopSectionRepo 店铺分区
type ShopSectionRepo struct {
	db *gorm.DB
}

func NewShopSectionRepo(db *gorm.DB) *ShopSectionRepo {
	return &ShopSectionRepo{db: db}
}

// Create 创建店铺分区
func (r *ShopSectionRepo) Create(ctx context.Context, section *model.ShopSection) error {
	return r.db.WithContext(ctx).Create(section).Error
}

// GetByID 根据ID获取分区
func (r *ShopSectionRepo) GetByID(ctx context.Context, id int64) (*model.ShopSection, error) {
	var section model.ShopSection
	err := r.db.WithContext(ctx).First(&section, id).Error
	if err != nil {
		return nil, err
	}
	return &section, nil
}

// GetByShopID 根据店铺ID获取所有分区
func (r *ShopSectionRepo) GetByShopID(ctx context.Context, shopID int64) ([]model.ShopSection, error) {
	var list []model.ShopSection
	err := r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Order("rank ASC, id ASC").
		Find(&list).Error
	return list, err
}

// GetByEtsySectionID 根据Etsy分区ID获取
func (r *ShopSectionRepo) GetByEtsySectionID(ctx context.Context, shopID int64, etsySectionID int64) (*model.ShopSection, error) {
	var section model.ShopSection
	err := r.db.WithContext(ctx).
		Where("shop_id = ? AND etsy_section_id = ?", shopID, etsySectionID).
		First(&section).Error
	if err != nil {
		return nil, err
	}
	return &section, nil
}

// Update 更新分区
func (r *ShopSectionRepo) Update(ctx context.Context, section *model.ShopSection) error {
	return r.db.WithContext(ctx).Save(section).Error
}

// UpdateFields 更新指定字段
func (r *ShopSectionRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.ShopSection{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete 删除分区
func (r *ShopSectionRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ShopSection{}, id).Error
}

// DeleteByShopID 根据店铺ID删除所有分区
func (r *ShopSectionRepo) DeleteByShopID(ctx context.Context, shopID int64) error {
	return r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Delete(&model.ShopSection{}).Error
}

// BatchCreate 批量创建分区
func (r *ShopSectionRepo) BatchCreate(ctx context.Context, sections []model.ShopSection) error {
	if len(sections) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&sections).Error
}

// BatchUpsert 批量更新或创建（用于同步）
func (r *ShopSectionRepo) BatchUpsert(ctx context.Context, shopID int64, sections []model.ShopSection) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, section := range sections {
			section.ShopID = shopID
			err := tx.Where("shop_id = ? AND etsy_section_id = ?", shopID, section.EtsySectionID).
				Assign(section).
				FirstOrCreate(&section).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateEtsySyncedAt 更新Etsy同步时间
func (r *ShopSectionRepo) UpdateEtsySyncedAt(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.ShopSection{}).
		Where("id = ?", id).
		Update("etsy_synced_at", gorm.Expr("NOW()")).Error
}

// Count 统计店铺分区数量
func (r *ShopSectionRepo) Count(ctx context.Context, shopID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ShopSection{}).
		Where("shop_id = ?", shopID).
		Count(&count).Error
	return count, err
}
