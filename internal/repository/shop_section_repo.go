package repository

import (
	"context"

	"gorm.io/gorm"

	"etsy_dev_v1_202512/internal/model"
)

// ==================== 接口定义 ====================

// ShopSectionRepository 店铺分区仓储接口
type ShopSectionRepository interface {
	// 基础 CRUD
	Create(ctx context.Context, section *model.ShopSection) error
	GetByID(ctx context.Context, id int64) (*model.ShopSection, error)
	Update(ctx context.Context, section *model.ShopSection) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Delete(ctx context.Context, id int64) error

	// 查询
	GetByShopID(ctx context.Context, shopID int64) ([]model.ShopSection, error)
	GetByEtsySectionID(ctx context.Context, shopID int64, etsySectionID int64) (*model.ShopSection, error)
	Count(ctx context.Context, shopID int64) (int64, error)

	// 批量操作
	BatchCreate(ctx context.Context, sections []model.ShopSection) error
	BatchUpsert(ctx context.Context, shopID int64, sections []model.ShopSection) error
	DeleteByShopID(ctx context.Context, shopID int64) error

	// 同步
	UpdateEtsySyncedAt(ctx context.Context, id int64) error
}

// ==================== 仓储实现 ====================

type shopSectionRepo struct {
	db *gorm.DB
}

// NewShopSectionRepository 创建店铺分区仓储
func NewShopSectionRepository(db *gorm.DB) ShopSectionRepository {
	return &shopSectionRepo{db: db}
}

func (r *shopSectionRepo) Create(ctx context.Context, section *model.ShopSection) error {
	return r.db.WithContext(ctx).Create(section).Error
}

func (r *shopSectionRepo) GetByID(ctx context.Context, id int64) (*model.ShopSection, error) {
	var section model.ShopSection
	err := r.db.WithContext(ctx).First(&section, id).Error
	if err != nil {
		return nil, err
	}
	return &section, nil
}

func (r *shopSectionRepo) Update(ctx context.Context, section *model.ShopSection) error {
	return r.db.WithContext(ctx).Save(section).Error
}

func (r *shopSectionRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.ShopSection{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *shopSectionRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ShopSection{}, id).Error
}

func (r *shopSectionRepo) GetByShopID(ctx context.Context, shopID int64) ([]model.ShopSection, error) {
	var list []model.ShopSection
	err := r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Order("rank ASC, id ASC").
		Find(&list).Error
	return list, err
}

func (r *shopSectionRepo) GetByEtsySectionID(ctx context.Context, shopID int64, etsySectionID int64) (*model.ShopSection, error) {
	var section model.ShopSection
	err := r.db.WithContext(ctx).
		Where("shop_id = ? AND etsy_section_id = ?", shopID, etsySectionID).
		First(&section).Error
	if err != nil {
		return nil, err
	}
	return &section, nil
}

func (r *shopSectionRepo) Count(ctx context.Context, shopID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ShopSection{}).
		Where("shop_id = ?", shopID).
		Count(&count).Error
	return count, err
}

func (r *shopSectionRepo) BatchCreate(ctx context.Context, sections []model.ShopSection) error {
	if len(sections) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&sections).Error
}

func (r *shopSectionRepo) BatchUpsert(ctx context.Context, shopID int64, sections []model.ShopSection) error {
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

func (r *shopSectionRepo) DeleteByShopID(ctx context.Context, shopID int64) error {
	return r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Delete(&model.ShopSection{}).Error
}

func (r *shopSectionRepo) UpdateEtsySyncedAt(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.ShopSection{}).
		Where("id = ?", id).
		Update("etsy_synced_at", gorm.Expr("NOW()")).Error
}
