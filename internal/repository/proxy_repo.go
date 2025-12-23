package repository

import (
	"context"
	"errors"
	"etsy_dev_v1_202512/internal/model"
	"time"

	"gorm.io/gorm"
)

type ProxyRepo struct {
	db *gorm.DB
}

func NewProxyRepo(db *gorm.DB) *ProxyRepo {
	return &ProxyRepo{db: db}
}

// 1. 增删改

// Create 创建代理
func (r *ProxyRepo) Create(ctx context.Context, proxy *model.Proxy) error {
	return r.db.WithContext(ctx).Create(proxy).Error
}

// Update 更新代理 (全量或部分更新)
func (r *ProxyRepo) Update(ctx context.Context, proxy *model.Proxy) error {
	return r.db.WithContext(ctx).Save(proxy).Error
}

// Delete 软删除代理
func (r *ProxyRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Proxy{}, id).Error
}

// // 2. 查询与关联

// GetByID 获取单条详情
// Preload，为了满足 DTO 中展示 "BoundShops" 和 "BoundDevelopers" 的需求
func (r *ProxyRepo) GetByID(ctx context.Context, id int64) (*model.Proxy, error) {
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

// GetRandomProxy 随机获取一个可用代理
func (r *ProxyRepo) GetRandomProxy(ctx context.Context) (*model.Proxy, error) {
	var proxy model.Proxy
	err := r.db.WithContext(ctx).Where("status = ?", 1).Order("RANDOM()").Take(&proxy).Error
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

// FindByEndpoint 根据 IP 和 Port 查重
// 业务逻辑：创建前检查是否已存在
func (r *ProxyRepo) FindByEndpoint(ctx context.Context, ip, port string) (*model.Proxy, error) {
	var proxy model.Proxy
	err := r.db.WithContext(ctx).
		Where("ip = ? AND port = ?", ip, port).
		First(&proxy).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 没找到，说明不重复，是安全的
	}
	if err != nil {
		return nil, err // 数据库错误
	}
	return &proxy, nil // 找到了，说明重复
}

// 3. 列表搜索

// ProxyListFilter 列表查询的过滤条件
type ProxyListFilter struct {
	IP       string
	Region   string
	Status   int
	Capacity int
	Page     int
	PageSize int
}

// List 获取分页列表
// 同样需要 Preload，因为列表页通常也要显示 "关联店铺数: 5" 这种统计信息
func (r *ProxyRepo) List(ctx context.Context, filter ProxyListFilter) ([]model.Proxy, int64, error) {
	var list []model.Proxy
	var total int64

	db := r.db.WithContext(ctx).Model(&model.Proxy{})

	// --- 动态构建查询条件 ---
	if filter.IP != "" {
		db = db.Where("ip LIKE ?", "%"+filter.IP+"%") // 模糊查 IP
	}
	if filter.Region != "" {
		db = db.Where("region = ?", filter.Region)
	}
	if filter.Status > 0 {
		db = db.Where("status = ?", filter.Status)
	}
	if filter.Capacity > 0 {
		db = db.Where("capacity = ?", filter.Capacity)
	}

	// 1. 计算总数 (用于分页)
	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 2. 查询数据 (带关联)
	offset := (filter.Page - 1) * filter.PageSize
	err = db.
		Preload("Shops", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "proxy_id", "shop_name", "etsy_shop_id", "token_status")
		}).
		Preload("Developers", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "proxy_id", "name", "api_key")
		}).
		Order("id DESC"). // 默认按创建时间倒序
		Limit(filter.PageSize).
		Offset(offset).
		Find(&list).Error

	return list, total, err
}

// 代理查询自救逻辑

func (r *ProxyRepo) FindCheckList(ctx context.Context) ([]model.Proxy, error) {
	var list []model.Proxy
	err := r.db.WithContext(ctx).Model(&model.Proxy{}).Where("status != ?", 3).Find(&list).Error
	return list, err
}

// 查询 region 相同且负载 < 2 的正常代理

func (r *ProxyRepo) FindSpareProxy(ctx context.Context, region string) (*model.Proxy, error) {
	var proxy model.Proxy
	// SQL 逻辑：
	// 1. SELECT proxies.*: 选择代理所有字段
	// 2. LEFT JOIN shops: 关联店铺表，用来计数
	// 3. WHERE: 地区匹配 + 状态正常 + 排除已满载(例如 >=2)的
	// 4. GROUP BY: 按代理聚合
	// 5. ORDER BY: 按店铺数量升序 (COUNT(shops.id) ASC)
	// 6. LIMIT 1: 只取最闲的那一个

	err := r.db.WithContext(ctx).
		Table("proxies").
		Select("proxies.*").
		Joins("LEFT JOIN shops ON shops.proxy_id = proxies.id").
		Where("proxies.region = ? AND proxies.status = ?", region, 1). // 1=Normal
		Group("proxies.id").
		// HAVING 用于过滤聚合后的结果 (找出绑定数小于 2 的)
		Having("COUNT(shops.id) < ?", 2).
		// 核心：动态计算负载并排序
		Order("COUNT(shops.id) ASC").
		First(&proxy).Error

	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

// UpdateStatus 更新状态
func (r *ProxyRepo) UpdateStatus(ctx context.Context, proxyID int64, status int) error {
	return r.db.WithContext(ctx).Model(&model.Proxy{}).Where("id = ?", proxyID).Update("status", status).Error
}

// 更新 monitor 最后检测时间

func (r *ProxyRepo) UpdateLastCheckTime(ctx context.Context, proxyID int64) error {
	return r.db.WithContext(ctx).Model(&model.Proxy{}).Where("id = ?", proxyID).Update("last_check_time", time.Now()).Error
}

func (r *ProxyRepo) UpdateStatusAndCount(ctx context.Context, proxy *model.Proxy) error {
	return r.db.WithContext(ctx).Model(&model.Proxy{}).Where("id = ?", proxy.ID).Updates(model.Proxy{Status: proxy.Status, FailureCount: proxy.FailureCount}).Error
}
