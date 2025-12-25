package repository

import (
	"context"
	"etsy_dev_v1_202512/internal/model"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProductRepo struct {
	db *gorm.DB
}

func NewProductRepo(db *gorm.DB) *ProductRepo {
	return &ProductRepo{db: db}
}

// ==================== 基础 CRUD ====================

// Create 创建单个商品
func (r *ProductRepo) Create(ctx context.Context, product *model.Product) error {
	return r.db.WithContext(ctx).Create(product).Error
}

// GetByID 根据 ID 获取商品
func (r *ProductRepo) GetByID(ctx context.Context, id int64) (*model.Product, error) {
	var product model.Product
	err := r.db.WithContext(ctx).
		Preload("Variants").
		Preload("Images").
		First(&product, id).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

// GetByListingID 根据 Etsy ListingID 获取商品
func (r *ProductRepo) GetByListingID(ctx context.Context, listingID int64) (*model.Product, error) {
	var product model.Product
	err := r.db.WithContext(ctx).
		Preload("Variants").
		Preload("Images").
		Where("listing_id = ?", listingID).
		First(&product).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

// Update 更新商品
func (r *ProductRepo) Update(ctx context.Context, product *model.Product) error {
	return r.db.WithContext(ctx).Save(product).Error
}

// Delete 软删除商品 (标记状态为 removed)
func (r *ProductRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("id = ?", id).
		Update("state", model.ProductStateRemoved).Error
}

// HardDelete 物理删除商品
func (r *ProductRepo) HardDelete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Product{}, id).Error
}

// ==================== 列表查询 ====================

// ListByShop 分页查询店铺商品
func (r *ProductRepo) ListByShop(ctx context.Context, shopID int64, page, pageSize int) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("shop_id = ?", shopID).
		Where("state != ?", model.ProductStateRemoved)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Order("updated_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&products).Error

	return products, total, err
}

// ListByShopAndState 按店铺和状态查询
func (r *ProductRepo) ListByShopAndState(ctx context.Context, shopID int64, state model.ProductState, page, pageSize int) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("shop_id = ? AND state = ?", shopID, state)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Order("updated_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&products).Error

	return products, total, err
}

// ListBySyncStatus 按同步状态查询 (用于后台任务)
func (r *ProductRepo) ListBySyncStatus(ctx context.Context, status model.ProductSyncStatus, limit int) ([]model.Product, error) {
	var products []model.Product
	err := r.db.WithContext(ctx).
		Preload("Shop").
		Preload("Shop.Developer").
		Preload("Variants").
		Preload("Images").
		Where("sync_status = ?", status).
		Limit(limit).
		Find(&products).Error
	return products, err
}

// ListByEditStatus 按编辑状态查询 (AI草稿流程)
func (r *ProductRepo) ListByEditStatus(ctx context.Context, shopID int64, status model.ProductEditStatus) ([]model.Product, error) {
	var products []model.Product
	err := r.db.WithContext(ctx).
		Where("shop_id = ? AND edit_status = ?", shopID, status).
		Order("created_at DESC").
		Find(&products).Error
	return products, err
}

// ==================== 批量操作 ====================

// BatchUpsert 批量插入或更新 (按 listing_id 冲突检测)
func (r *ProductRepo) BatchUpsert(ctx context.Context, products []model.Product) error {
	if len(products) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "listing_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"title", "description", "state", "url",
			"price_amount", "price_divisor", "currency_code",
			"quantity", "tags", "materials", "styles",
			"views", "num_favorers",
			"etsy_last_modified_ts", "etsy_state_ts",
			"sync_status", "updated_at",
		}),
	}).Create(&products).Error
}

// BatchUpdateSyncStatus 批量更新同步状态
func (r *ProductRepo) BatchUpdateSyncStatus(ctx context.Context, ids []int64, status model.ProductSyncStatus, errMsg string) error {
	return r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"sync_status": status,
			"sync_error":  errMsg,
		}).Error
}

// ==================== 变体操作 ====================

// CreateVariant 创建变体
func (r *ProductRepo) CreateVariant(ctx context.Context, variant *model.ProductVariant) error {
	return r.db.WithContext(ctx).Create(variant).Error
}

// BatchUpsertVariants 批量更新变体
func (r *ProductRepo) BatchUpsertVariants(ctx context.Context, variants []model.ProductVariant) error {
	if len(variants) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "product_id"}, {Name: "etsy_product_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"etsy_offering_id", "property_values",
			"price_amount", "price_divisor", "currency_code",
			"quantity", "is_enabled", "etsy_sku", "updated_at",
		}),
	}).Create(&variants).Error
}

// DeleteVariantsByProductID 删除商品所有变体
func (r *ProductRepo) DeleteVariantsByProductID(ctx context.Context, productID int64) error {
	return r.db.WithContext(ctx).
		Where("product_id = ?", productID).
		Delete(&model.ProductVariant{}).Error
}

// ==================== 图片操作 ====================

// CreateImage 创建图片记录
func (r *ProductRepo) CreateImage(ctx context.Context, image *model.ProductImage) error {
	return r.db.WithContext(ctx).Create(image).Error
}

// UpdateImage 更新图片记录
func (r *ProductRepo) UpdateImage(ctx context.Context, image *model.ProductImage) error {
	return r.db.WithContext(ctx).Save(image).Error
}

// GetImagesByProductID 获取商品所有图片
func (r *ProductRepo) GetImagesByProductID(ctx context.Context, productID int64) ([]model.ProductImage, error) {
	var images []model.ProductImage
	err := r.db.WithContext(ctx).
		Where("product_id = ?", productID).
		Order("rank ASC").
		Find(&images).Error
	return images, err
}

// DeleteImage 删除图片
func (r *ProductRepo) DeleteImage(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ProductImage{}, id).Error
}

// BatchUpsertImages 批量更新图片
func (r *ProductRepo) BatchUpsertImages(ctx context.Context, images []model.ProductImage) error {
	if len(images) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "product_id"}, {Name: "etsy_image_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"rank", "etsy_url", "alt_text", "hex_code",
			"height", "width", "sync_status", "updated_at",
		}),
	}).Create(&images).Error
}

// ==================== 统计查询 ====================

// CountByShopAndState 统计店铺各状态商品数量
func (r *ProductRepo) CountByShopAndState(ctx context.Context, shopID int64) (map[model.ProductState]int64, error) {
	type result struct {
		State model.ProductState
		Count int64
	}
	var results []result

	err := r.db.WithContext(ctx).
		Model(&model.Product{}).
		Select("state, COUNT(*) as count").
		Where("shop_id = ?", shopID).
		Group("state").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	stats := make(map[model.ProductState]int64)
	for _, r := range results {
		stats[r.State] = r.Count
	}
	return stats, nil
}

// ==================== 搜索 ====================

// SearchByTitle 按标题模糊搜索
func (r *ProductRepo) SearchByTitle(ctx context.Context, shopID int64, keyword string, page, pageSize int) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("shop_id = ?", shopID).
		Where("state != ?", model.ProductStateRemoved).
		Where("title ILIKE ?", fmt.Sprintf("%%%s%%", keyword))

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Order("updated_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&products).Error

	return products, total, err
}

// ==================== 事务支持 ====================

// WithTx 返回带事务的 Repo
func (r *ProductRepo) WithTx(tx *gorm.DB) *ProductRepo {
	return &ProductRepo{db: tx}
}

// Transaction 执行事务
func (r *ProductRepo) Transaction(ctx context.Context, fn func(txRepo *ProductRepo) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(r.WithTx(tx))
	})
}
