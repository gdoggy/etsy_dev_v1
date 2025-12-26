package repository

import (
	"context"
	"time"

	"etsy_dev_v1_202512/internal/model"

	"gorm.io/gorm"
)

// ==================== ShipmentFilter 过滤条件 ====================

// ShipmentFilter 发货记录过滤条件
type ShipmentFilter struct {
	OrderID        int64
	CarrierCode    string
	Status         string
	EtsySynced     *bool
	TrackingNumber string
	StartDate      *time.Time
	EndDate        *time.Time
	Page           int
	PageSize       int
}

// ==================== ShipmentRepository 发货仓库 ====================

// ShipmentRepository 发货仓库接口
type ShipmentRepository interface {
	Create(ctx context.Context, shipment *model.Shipment) error
	GetByID(ctx context.Context, id int64) (*model.Shipment, error)
	GetByOrderID(ctx context.Context, orderID int64) (*model.Shipment, error)
	GetByTrackingNumber(ctx context.Context, trackingNumber string) (*model.Shipment, error)
	GetByKarrioTrackerID(ctx context.Context, trackerID string) (*model.Shipment, error)
	GetByIDWithEvents(ctx context.Context, id int64) (*model.Shipment, error)
	List(ctx context.Context, filter ShipmentFilter) ([]model.Shipment, int64, error)
	Update(ctx context.Context, shipment *model.Shipment) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateTrackingInfo(ctx context.Context, id int64, status, location string, eventTime *time.Time) error
	MarkEtsySynced(ctx context.Context, id int64) error
	MarkEtsySyncFailed(ctx context.Context, id int64, errMsg string) error
	Delete(ctx context.Context, id int64) error

	// 批量操作
	GetPendingTrackingShipments(ctx context.Context, limit int) ([]model.Shipment, error)
	GetPendingEtsySyncShipments(ctx context.Context, limit int) ([]model.Shipment, error)
	BatchUpdateStatus(ctx context.Context, ids []int64, status string) (int64, error)
}

type shipmentRepository struct {
	db *gorm.DB
}

// NewShipmentRepository 创建发货仓库
func NewShipmentRepository(db *gorm.DB) ShipmentRepository {
	return &shipmentRepository{db: db}
}

func (r *shipmentRepository) Create(ctx context.Context, shipment *model.Shipment) error {
	return r.db.WithContext(ctx).Create(shipment).Error
}

func (r *shipmentRepository) GetByID(ctx context.Context, id int64) (*model.Shipment, error) {
	var shipment model.Shipment
	err := r.db.WithContext(ctx).First(&shipment, id).Error
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}

func (r *shipmentRepository) GetByOrderID(ctx context.Context, orderID int64) (*model.Shipment, error) {
	var shipment model.Shipment
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&shipment).Error
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}

func (r *shipmentRepository) GetByTrackingNumber(ctx context.Context, trackingNumber string) (*model.Shipment, error) {
	var shipment model.Shipment
	err := r.db.WithContext(ctx).Where("tracking_number = ?", trackingNumber).First(&shipment).Error
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}

func (r *shipmentRepository) GetByKarrioTrackerID(ctx context.Context, trackerID string) (*model.Shipment, error) {
	var shipment model.Shipment
	err := r.db.WithContext(ctx).Where("karrio_tracker_id = ?", trackerID).First(&shipment).Error
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}

func (r *shipmentRepository) GetByIDWithEvents(ctx context.Context, id int64) (*model.Shipment, error) {
	var shipment model.Shipment
	err := r.db.WithContext(ctx).
		Preload("TrackingEvents", func(db *gorm.DB) *gorm.DB {
			return db.Order("occurred_at DESC")
		}).
		First(&shipment, id).Error
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}

func (r *shipmentRepository) List(ctx context.Context, filter ShipmentFilter) ([]model.Shipment, int64, error) {
	var shipments []model.Shipment
	var total int64

	db := r.db.WithContext(ctx).Model(&model.Shipment{})

	if filter.OrderID > 0 {
		db = db.Where("order_id = ?", filter.OrderID)
	}
	if filter.CarrierCode != "" {
		db = db.Where("carrier_code = ?", filter.CarrierCode)
	}
	if filter.Status != "" {
		db = db.Where("status = ?", filter.Status)
	}
	if filter.EtsySynced != nil {
		db = db.Where("etsy_synced = ?", *filter.EtsySynced)
	}
	if filter.TrackingNumber != "" {
		db = db.Where("tracking_number LIKE ?", "%"+filter.TrackingNumber+"%")
	}
	if filter.StartDate != nil {
		db = db.Where("created_at >= ?", filter.StartDate)
	}
	if filter.EndDate != nil {
		db = db.Where("created_at <= ?", filter.EndDate)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	err := db.
		Preload("TrackingEvents", func(db *gorm.DB) *gorm.DB {
			return db.Order("occurred_at DESC").Limit(5)
		}).
		Order("created_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&shipments).Error

	return shipments, total, err
}

func (r *shipmentRepository) Update(ctx context.Context, shipment *model.Shipment) error {
	return r.db.WithContext(ctx).Save(shipment).Error
}

func (r *shipmentRepository) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Shipment{}).Where("id = ?", id).Updates(fields).Error
}

func (r *shipmentRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if status == model.ShipmentStatusDelivered {
		now := time.Now()
		updates["delivered_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&model.Shipment{}).Where("id = ?", id).Updates(updates).Error
}

func (r *shipmentRepository) UpdateTrackingInfo(ctx context.Context, id int64, status, location string, eventTime *time.Time) error {
	updates := map[string]interface{}{
		"last_tracking_status":   status,
		"last_tracking_location": location,
		"last_tracking_time":     eventTime,
		"updated_at":             time.Now(),
	}
	return r.db.WithContext(ctx).Model(&model.Shipment{}).Where("id = ?", id).Updates(updates).Error
}

func (r *shipmentRepository) MarkEtsySynced(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.Shipment{}).Where("id = ?", id).Updates(map[string]interface{}{
		"etsy_synced":     true,
		"etsy_synced_at":  &now,
		"etsy_sync_error": "",
	}).Error
}

func (r *shipmentRepository) MarkEtsySyncFailed(ctx context.Context, id int64, errMsg string) error {
	return r.db.WithContext(ctx).Model(&model.Shipment{}).Where("id = ?", id).Updates(map[string]interface{}{
		"etsy_sync_error": errMsg,
	}).Error
}

func (r *shipmentRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Shipment{}, id).Error
}

func (r *shipmentRepository) GetPendingTrackingShipments(ctx context.Context, limit int) ([]model.Shipment, error) {
	var shipments []model.Shipment
	// 获取需要跟踪的发货记录：状态不是已签收/已退回，且有跟踪号
	err := r.db.WithContext(ctx).
		Where("status NOT IN ?", []string{model.ShipmentStatusDelivered, model.ShipmentStatusReturned}).
		Where("tracking_number != ''").
		Where("karrio_tracker_id != ''").
		Order("last_tracking_time ASC NULLS FIRST").
		Limit(limit).
		Find(&shipments).Error
	return shipments, err
}

func (r *shipmentRepository) GetPendingEtsySyncShipments(ctx context.Context, limit int) ([]model.Shipment, error) {
	var shipments []model.Shipment
	// 获取需要同步到 Etsy 的发货记录：状态为派送中或已签收，且未同步
	err := r.db.WithContext(ctx).
		Where("etsy_synced = ?", false).
		Where("status IN ?", []string{model.ShipmentStatusDelivering, model.ShipmentStatusDelivered}).
		Where("tracking_number != ''").
		Order("created_at ASC").
		Limit(limit).
		Find(&shipments).Error
	return shipments, err
}

func (r *shipmentRepository) BatchUpdateStatus(ctx context.Context, ids []int64, status string) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.Shipment{}).Where("id IN ?", ids).Update("status", status)
	return result.RowsAffected, result.Error
}

// ==================== TrackingEventRepository 物流轨迹仓库 ====================

// TrackingEventRepository 物流轨迹仓库接口
type TrackingEventRepository interface {
	Create(ctx context.Context, event *model.TrackingEvent) error
	CreateBatch(ctx context.Context, events []model.TrackingEvent) error
	GetByShipmentID(ctx context.Context, shipmentID int64) ([]model.TrackingEvent, error)
	GetLatestByShipmentID(ctx context.Context, shipmentID int64) (*model.TrackingEvent, error)
	DeleteByShipmentID(ctx context.Context, shipmentID int64) error
}

type trackingEventRepository struct {
	db *gorm.DB
}

// NewTrackingEventRepository 创建物流轨迹仓库
func NewTrackingEventRepository(db *gorm.DB) TrackingEventRepository {
	return &trackingEventRepository{db: db}
}

func (r *trackingEventRepository) Create(ctx context.Context, event *model.TrackingEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *trackingEventRepository) CreateBatch(ctx context.Context, events []model.TrackingEvent) error {
	if len(events) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(events, 100).Error
}

func (r *trackingEventRepository) GetByShipmentID(ctx context.Context, shipmentID int64) ([]model.TrackingEvent, error) {
	var events []model.TrackingEvent
	err := r.db.WithContext(ctx).
		Where("shipment_id = ?", shipmentID).
		Order("occurred_at DESC").
		Find(&events).Error
	return events, err
}

func (r *trackingEventRepository) GetLatestByShipmentID(ctx context.Context, shipmentID int64) (*model.TrackingEvent, error) {
	var event model.TrackingEvent
	err := r.db.WithContext(ctx).
		Where("shipment_id = ?", shipmentID).
		Order("occurred_at DESC").
		First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *trackingEventRepository) DeleteByShipmentID(ctx context.Context, shipmentID int64) error {
	return r.db.WithContext(ctx).Where("shipment_id = ?", shipmentID).Delete(&model.TrackingEvent{}).Error
}
