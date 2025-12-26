package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"etsy_dev_v1_202512/internal/model"
)

// ==================== 接口定义 ====================

// ProductRepository 商品仓储接口
type ProductRepository interface {
	// 基础 CRUD
	Create(ctx context.Context, product *model.Product) error
	GetByID(ctx context.Context, id int64) (*model.Product, error)
	GetByListingID(ctx context.Context, listingID int64) (*model.Product, error)
	Update(ctx context.Context, product *model.Product) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	HardDelete(ctx context.Context, id int64) error
	List(ctx context.Context, filter ProductFilter) ([]model.Product, int64, error)

	// 列表查询
	ListByShop(ctx context.Context, shopID int64, page, pageSize int) ([]model.Product, int64, error)
	ListByShopAndState(ctx context.Context, shopID int64, state model.ProductState, page, pageSize int) ([]model.Product, int64, error)
	ListBySyncStatus(ctx context.Context, status model.ProductSyncStatus, limit int) ([]model.Product, error)
	ListByEditStatus(ctx context.Context, shopID int64, status model.ProductEditStatus) ([]model.Product, error)
	SearchByTitle(ctx context.Context, shopID int64, keyword string, page, pageSize int) ([]model.Product, int64, error)

	// 批量操作
	BatchUpsert(ctx context.Context, products []model.Product) error
	BatchUpdateSyncStatus(ctx context.Context, ids []int64, status model.ProductSyncStatus, errMsg string) error

	// 变体操作
	CreateVariant(ctx context.Context, variant *model.ProductVariant) error
	BatchUpsertVariants(ctx context.Context, variants []model.ProductVariant) error
	DeleteVariantsByProductID(ctx context.Context, productID int64) error

	// 图片操作
	CreateImage(ctx context.Context, image *model.ProductImage) error
	UpdateImage(ctx context.Context, image *model.ProductImage) error
	GetImagesByProductID(ctx context.Context, productID int64) ([]model.ProductImage, error)
	DeleteImage(ctx context.Context, id int64) error
	BatchUpsertImages(ctx context.Context, images []model.ProductImage) error

	// 统计
	CountByShopAndState(ctx context.Context, shopID int64) (map[model.ProductState]int64, error)

	// 事务
	WithTx(tx *gorm.DB) ProductRepository
	Transaction(ctx context.Context, fn func(txRepo ProductRepository) error) error
}

// ==================== 过滤条件 ====================

// ProductFilter 商品过滤条件
type ProductFilter struct {
	ShopID     int64
	State      model.ProductState
	SyncStatus int
	EditStatus model.ProductEditStatus
	Keyword    string
	Page       int
	PageSize   int
}

// ==================== 仓储实现 ====================

type productRepo struct {
	db *gorm.DB
}

// NewProductRepository 创建商品仓储
func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepo{db: db}
}

func (r *productRepo) Create(ctx context.Context, product *model.Product) error {
	return r.db.WithContext(ctx).Create(product).Error
}

func (r *productRepo) GetByID(ctx context.Context, id int64) (*model.Product, error) {
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

func (r *productRepo) GetByListingID(ctx context.Context, listingID int64) (*model.Product, error) {
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

func (r *productRepo) Update(ctx context.Context, product *model.Product) error {
	return r.db.WithContext(ctx).Save(product).Error
}

func (r *productRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *productRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("id = ?", id).
		Update("state", model.ProductStateRemoved).Error
}

func (r *productRepo) HardDelete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Product{}, id).Error
}

func (r *productRepo) List(ctx context.Context, filter ProductFilter) ([]model.Product, int64, error) {
	var products []model.Product
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Product{})

	if filter.ShopID > 0 {
		query = query.Where("shop_id = ?", filter.ShopID)
	}
	if filter.State != "" {
		query = query.Where("state = ?", filter.State)
	} else {
		query = query.Where("state != ?", model.ProductStateRemoved)
	}
	if filter.SyncStatus >= 0 {
		query = query.Where("sync_status = ?", filter.SyncStatus)
	}
	if filter.Keyword != "" {
		query = query.Where("title ILIKE ?", "%"+filter.Keyword+"%")
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
		Order("updated_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&products).Error

	return products, total, err
}

func (r *productRepo) ListByShop(ctx context.Context, shopID int64, page, pageSize int) ([]model.Product, int64, error) {
	return r.List(ctx, ProductFilter{
		ShopID:   shopID,
		Page:     page,
		PageSize: pageSize,
	})
}

func (r *productRepo) ListByShopAndState(ctx context.Context, shopID int64, state model.ProductState, page, pageSize int) ([]model.Product, int64, error) {
	return r.List(ctx, ProductFilter{
		ShopID:   shopID,
		State:    state,
		Page:     page,
		PageSize: pageSize,
	})
}

func (r *productRepo) ListBySyncStatus(ctx context.Context, status model.ProductSyncStatus, limit int) ([]model.Product, error) {
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

func (r *productRepo) ListByEditStatus(ctx context.Context, shopID int64, status model.ProductEditStatus) ([]model.Product, error) {
	var products []model.Product
	err := r.db.WithContext(ctx).
		Where("shop_id = ? AND edit_status = ?", shopID, status).
		Order("created_at DESC").
		Find(&products).Error
	return products, err
}

func (r *productRepo) SearchByTitle(ctx context.Context, shopID int64, keyword string, page, pageSize int) ([]model.Product, int64, error) {
	return r.List(ctx, ProductFilter{
		ShopID:   shopID,
		Keyword:  keyword,
		Page:     page,
		PageSize: pageSize,
	})
}

func (r *productRepo) BatchUpsert(ctx context.Context, products []model.Product) error {
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

func (r *productRepo) BatchUpdateSyncStatus(ctx context.Context, ids []int64, status model.ProductSyncStatus, errMsg string) error {
	return r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"sync_status": status,
			"sync_error":  errMsg,
		}).Error
}

func (r *productRepo) CreateVariant(ctx context.Context, variant *model.ProductVariant) error {
	return r.db.WithContext(ctx).Create(variant).Error
}

func (r *productRepo) BatchUpsertVariants(ctx context.Context, variants []model.ProductVariant) error {
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

func (r *productRepo) DeleteVariantsByProductID(ctx context.Context, productID int64) error {
	return r.db.WithContext(ctx).
		Where("product_id = ?", productID).
		Delete(&model.ProductVariant{}).Error
}

func (r *productRepo) CreateImage(ctx context.Context, image *model.ProductImage) error {
	return r.db.WithContext(ctx).Create(image).Error
}

func (r *productRepo) UpdateImage(ctx context.Context, image *model.ProductImage) error {
	return r.db.WithContext(ctx).Save(image).Error
}

func (r *productRepo) GetImagesByProductID(ctx context.Context, productID int64) ([]model.ProductImage, error) {
	var images []model.ProductImage
	err := r.db.WithContext(ctx).
		Where("product_id = ?", productID).
		Order("rank ASC").
		Find(&images).Error
	return images, err
}

func (r *productRepo) DeleteImage(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ProductImage{}, id).Error
}

func (r *productRepo) BatchUpsertImages(ctx context.Context, images []model.ProductImage) error {
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

func (r *productRepo) CountByShopAndState(ctx context.Context, shopID int64) (map[model.ProductState]int64, error) {
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

func (r *productRepo) WithTx(tx *gorm.DB) ProductRepository {
	return &productRepo{db: tx}
}

func (r *productRepo) Transaction(ctx context.Context, fn func(txRepo ProductRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(r.WithTx(tx))
	})
}
