package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"etsy_dev_v1_202512/internal/model"
)

// ==================== 接口定义 ====================

// ProxyRepository 代理仓储接口
type ProxyRepository interface {
	// 基础 CRUD
	Create(ctx context.Context, proxy *model.Proxy) error
	GetByID(ctx context.Context, id int64) (*model.Proxy, error)
	Update(ctx context.Context, proxy *model.Proxy) error
	UpdateStatus(ctx context.Context, proxyID int64, status int) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter ProxyFilter) ([]model.Proxy, int64, error)

	// 查询
	FindByEndpoint(ctx context.Context, ip, port string) (*model.Proxy, error)
	GetRandomProxy(ctx context.Context) (*model.Proxy, error)
	FindSpareProxy(ctx context.Context, region string) (*model.Proxy, error)
	FindCheckList(ctx context.Context) ([]model.Proxy, error)

	// 更新
	UpdateLastCheckTime(ctx context.Context, proxyID int64) error
	UpdateStatusAndCount(ctx context.Context, proxy *model.Proxy) error
}

// ==================== 过滤条件 ====================

// ProxyFilter 代理过滤条件
type ProxyFilter struct {
	IP       string
	Region   string
	Status   int
	Capacity int
	Page     int
	PageSize int
}

// ==================== 仓储实现 ====================

type proxyRepo struct {
	db *gorm.DB
}

// NewProxyRepository 创建代理仓储
func NewProxyRepository(db *gorm.DB) ProxyRepository {
	return &proxyRepo{db: db}
}

func (r *proxyRepo) Create(ctx context.Context, proxy *model.Proxy) error {
	return r.db.WithContext(ctx).Create(proxy).Error
}

func (r *proxyRepo) GetByID(ctx context.Context, id int64) (*model.Proxy, error) {
	var proxy model.Proxy
	err := r.db.WithContext(ctx).
		Preload("Shops", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "proxy_id", "shop_name", "etsy_shop_id", "token_status")
		}).
		Preload("Developers", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "proxy_id", "name", "api_key")
		}).
		First(&proxy, id).Error
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

func (r *proxyRepo) Update(ctx context.Context, proxy *model.Proxy) error {
	return r.db.WithContext(ctx).Save(proxy).Error
}

func (r *proxyRepo) UpdateStatus(ctx context.Context, proxyID int64, status int) error {
	return r.db.WithContext(ctx).
		Model(&model.Proxy{}).
		Where("id = ?", proxyID).
		Update("status", status).Error
}

func (r *proxyRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Proxy{}, id).Error
}

func (r *proxyRepo) List(ctx context.Context, filter ProxyFilter) ([]model.Proxy, int64, error) {
	var list []model.Proxy
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Proxy{})

	if filter.IP != "" {
		query = query.Where("ip LIKE ?", "%"+filter.IP+"%")
	}
	if filter.Region != "" {
		query = query.Where("region = ?", filter.Region)
	}
	if filter.Status > 0 {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Capacity > 0 {
		query = query.Where("capacity = ?", filter.Capacity)
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
			return db.Select("id", "proxy_id", "shop_name", "etsy_shop_id", "token_status")
		}).
		Preload("Developers", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "proxy_id", "name", "api_key")
		}).
		Order("id DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&list).Error

	return list, total, err
}

func (r *proxyRepo) FindByEndpoint(ctx context.Context, ip, port string) (*model.Proxy, error) {
	var proxy model.Proxy
	err := r.db.WithContext(ctx).
		Where("ip = ? AND port = ?", ip, port).
		First(&proxy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

func (r *proxyRepo) GetRandomProxy(ctx context.Context) (*model.Proxy, error) {
	var proxy model.Proxy
	err := r.db.WithContext(ctx).
		Where("status = ?", 1).
		Order("RANDOM()").
		Take(&proxy).Error
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

func (r *proxyRepo) FindSpareProxy(ctx context.Context, region string) (*model.Proxy, error) {
	var proxy model.Proxy
	err := r.db.WithContext(ctx).
		Table("proxies").
		Select("proxies.*").
		Joins("LEFT JOIN shops ON shops.proxy_id = proxies.id").
		Where("proxies.region = ? AND proxies.status = ?", region, 1).
		Group("proxies.id").
		Having("COUNT(shops.id) < ?", 2).
		Order("COUNT(shops.id) ASC").
		First(&proxy).Error
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

func (r *proxyRepo) FindCheckList(ctx context.Context) ([]model.Proxy, error) {
	var list []model.Proxy
	err := r.db.WithContext(ctx).
		Model(&model.Proxy{}).
		Where("status != ?", 3).
		Find(&list).Error
	return list, err
}

func (r *proxyRepo) UpdateLastCheckTime(ctx context.Context, proxyID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.Proxy{}).
		Where("id = ?", proxyID).
		Update("last_check_time", time.Now()).Error
}

func (r *proxyRepo) UpdateStatusAndCount(ctx context.Context, proxy *model.Proxy) error {
	return r.db.WithContext(ctx).
		Model(&model.Proxy{}).
		Where("id = ?", proxy.ID).
		Updates(model.Proxy{
			Status:       proxy.Status,
			FailureCount: proxy.FailureCount,
		}).Error
}
