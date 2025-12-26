package repository

import (
	"context"
	"time"

	"etsy_dev_v1_202512/internal/model"

	"gorm.io/gorm"
)

// ==================== 过滤条件 ====================

// OrderFilter 订单过滤条件
type OrderFilter struct {
	ShopID     int64
	Status     string
	EtsyStatus string
	StartDate  *time.Time
	EndDate    *time.Time
	Keyword    string
	IsPaid     *bool
	IsShipped  *bool
	Page       int
	PageSize   int
}

// ==================== OrderRepository 订单仓库 ====================

// OrderRepository 订单仓库接口
type OrderRepository interface {
	Create(ctx context.Context, order *model.Order) error
	GetByID(ctx context.Context, id int64) (*model.Order, error)
	GetByEtsyReceiptID(ctx context.Context, receiptID int64) (*model.Order, error)
	GetByIDWithRelations(ctx context.Context, id int64) (*model.Order, error)
	List(ctx context.Context, filter OrderFilter) ([]model.Order, int64, error)
	Update(ctx context.Context, order *model.Order) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	BatchUpdateStatus(ctx context.Context, ids []int64, status string) (int64, error)
	Delete(ctx context.Context, id int64) error

	// 统计
	CountByShopAndStatus(ctx context.Context, shopID int64, status string) (int64, error)
	GetStats(ctx context.Context, shopID int64, startDate, endDate time.Time) (*OrderStats, error)

	// 同步相关
	GetPendingSyncOrders(ctx context.Context, shopID int64, limit int) ([]model.Order, error)
	GetOrdersNeedingShipmentSync(ctx context.Context, limit int) ([]model.Order, error)
}

// OrderStats 订单统计
type OrderStats struct {
	TotalOrders      int64
	TotalAmount      int64
	PendingOrders    int64
	ProcessingOrders int64
	ShippedOrders    int64
	DeliveredOrders  int64
	CanceledOrders   int64
}

// ==================== 实现 ====================

type orderRepository struct {
	db *gorm.DB
}

// NewOrderRepository 创建订单仓库
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) Create(ctx context.Context, order *model.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *orderRepository) GetByID(ctx context.Context, id int64) (*model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).First(&order, id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *orderRepository) GetByEtsyReceiptID(ctx context.Context, receiptID int64) (*model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).Where("etsy_receipt_id = ?", receiptID).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *orderRepository) GetByIDWithRelations(ctx context.Context, id int64) (*model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("Shipment").
		Preload("Shipment.TrackingEvents", func(db *gorm.DB) *gorm.DB {
			return db.Order("occurred_at DESC").Limit(20)
		}).
		First(&order, id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *orderRepository) List(ctx context.Context, filter OrderFilter) ([]model.Order, int64, error) {
	var orders []model.Order
	var total int64

	db := r.db.WithContext(ctx).Model(&model.Order{})

	// 应用过滤条件
	if filter.ShopID > 0 {
		db = db.Where("shop_id = ?", filter.ShopID)
	}
	if filter.Status != "" {
		db = db.Where("status = ?", filter.Status)
	}
	if filter.EtsyStatus != "" {
		db = db.Where("etsy_status = ?", filter.EtsyStatus)
	}
	if filter.StartDate != nil {
		db = db.Where("created_at >= ?", filter.StartDate)
	}
	if filter.EndDate != nil {
		db = db.Where("created_at <= ?", filter.EndDate)
	}
	if filter.IsPaid != nil {
		db = db.Where("is_paid = ?", *filter.IsPaid)
	}
	if filter.IsShipped != nil {
		db = db.Where("is_shipped = ?", *filter.IsShipped)
	}
	if filter.Keyword != "" {
		keyword := "%" + filter.Keyword + "%"
		db = db.Where("buyer_name LIKE ? OR buyer_email LIKE ? OR CAST(etsy_receipt_id AS CHAR) LIKE ?",
			keyword, keyword, keyword)
	}

	// 计算总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	err := db.
		Preload("Items").
		Preload("Shipment").
		Order("created_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&orders).Error

	return orders, total, err
}

func (r *orderRepository) Update(ctx context.Context, order *model.Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

func (r *orderRepository) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Order{}).Where("id = ?", id).Updates(fields).Error
}

func (r *orderRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).Model(&model.Order{}).Where("id = ?", id).Update("status", status).Error
}

func (r *orderRepository) BatchUpdateStatus(ctx context.Context, ids []int64, status string) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.Order{}).Where("id IN ?", ids).Update("status", status)
	return result.RowsAffected, result.Error
}

func (r *orderRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Order{}, id).Error
}

func (r *orderRepository) CountByShopAndStatus(ctx context.Context, shopID int64, status string) (int64, error) {
	var count int64
	db := r.db.WithContext(ctx).Model(&model.Order{}).Where("shop_id = ?", shopID)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	err := db.Count(&count).Error
	return count, err
}

func (r *orderRepository) GetStats(ctx context.Context, shopID int64, startDate, endDate time.Time) (*OrderStats, error) {
	var stats OrderStats

	db := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("shop_id = ?", shopID).
		Where("created_at BETWEEN ? AND ?", startDate, endDate)

	// 总订单数和金额
	var result struct {
		Count  int64
		Amount int64
	}
	if err := db.Select("COUNT(*) as count, COALESCE(SUM(grand_total_amount), 0) as amount").Scan(&result).Error; err != nil {
		return nil, err
	}
	stats.TotalOrders = result.Count
	stats.TotalAmount = result.Amount

	// 各状态订单数
	type StatusCount struct {
		Status string
		Count  int64
	}
	var statusCounts []StatusCount
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("shop_id = ?", shopID).
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, err
	}

	for _, sc := range statusCounts {
		switch sc.Status {
		case model.OrderStatusPending:
			stats.PendingOrders = sc.Count
		case model.OrderStatusProcessing:
			stats.ProcessingOrders = sc.Count
		case model.OrderStatusShipped:
			stats.ShippedOrders = sc.Count
		case model.OrderStatusDelivered:
			stats.DeliveredOrders = sc.Count
		case model.OrderStatusCanceled:
			stats.CanceledOrders = sc.Count
		}
	}

	return &stats, nil
}

func (r *orderRepository) GetPendingSyncOrders(ctx context.Context, shopID int64, limit int) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Where("etsy_synced_at IS NULL OR etsy_synced_at < updated_at").
		Order("created_at ASC").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

func (r *orderRepository) GetOrdersNeedingShipmentSync(ctx context.Context, limit int) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.WithContext(ctx).
		Joins("JOIN shipments ON shipments.order_id = orders.id").
		Where("shipments.etsy_synced = ?", false).
		Where("shipments.status IN ?", []string{model.ShipmentStatusDelivering, model.ShipmentStatusDelivered}).
		Preload("Shipment").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

// ==================== OrderItemRepository 订单项仓库 ====================

// OrderItemRepository 订单项仓库接口
type OrderItemRepository interface {
	Create(ctx context.Context, item *model.OrderItem) error
	CreateBatch(ctx context.Context, items []model.OrderItem) error
	GetByOrderID(ctx context.Context, orderID int64) ([]model.OrderItem, error)
	GetByEtsyTransactionID(ctx context.Context, transactionID int64) (*model.OrderItem, error)
	Update(ctx context.Context, item *model.OrderItem) error
	DeleteByOrderID(ctx context.Context, orderID int64) error
}

type orderItemRepository struct {
	db *gorm.DB
}

// NewOrderItemRepository 创建订单项仓库
func NewOrderItemRepository(db *gorm.DB) OrderItemRepository {
	return &orderItemRepository{db: db}
}

func (r *orderItemRepository) Create(ctx context.Context, item *model.OrderItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *orderItemRepository) CreateBatch(ctx context.Context, items []model.OrderItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(items, 100).Error
}

func (r *orderItemRepository) GetByOrderID(ctx context.Context, orderID int64) ([]model.OrderItem, error) {
	var items []model.OrderItem
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Find(&items).Error
	return items, err
}

func (r *orderItemRepository) GetByEtsyTransactionID(ctx context.Context, transactionID int64) (*model.OrderItem, error) {
	var item model.OrderItem
	err := r.db.WithContext(ctx).Where("etsy_transaction_id = ?", transactionID).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *orderItemRepository) Update(ctx context.Context, item *model.OrderItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *orderItemRepository) DeleteByOrderID(ctx context.Context, orderID int64) error {
	return r.db.WithContext(ctx).Where("order_id = ?", orderID).Delete(&model.OrderItem{}).Error
}
