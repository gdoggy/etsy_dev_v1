package model

import (
	"time"

	"gorm.io/datatypes"
)

// ==================== Shipment 发货记录 ====================

// ShipmentStatus 发货状态
const (
	ShipmentStatusCreated    = "created"    // 已创建
	ShipmentStatusInTransit  = "in_transit" // 运输中
	ShipmentStatusArrived    = "arrived"    // 到达目的国
	ShipmentStatusDelivering = "delivering" // 派送中
	ShipmentStatusDelivered  = "delivered"  // 已签收
	ShipmentStatusException  = "exception"  // 异常
	ShipmentStatusReturned   = "returned"   // 已退回
)

// CarrierCode 物流商代码
const (
	CarrierYanwen  = "yanwen"  // 燕文物流
	CarrierWanbang = "wanbang" // 万邦速达
)

// Shipment 发货记录
type Shipment struct {
	BaseModel
	OrderID int64 `gorm:"uniqueIndex;not null"`

	// Karrio 关联
	KarrioShipmentID string `gorm:"size:64;index"`
	KarrioTrackerID  string `gorm:"size:64;index"`

	// 物流商信息（国际段）
	CarrierCode    string `gorm:"size:32;not null"`
	CarrierName    string `gorm:"size:64"`
	TrackingNumber string `gorm:"size:64;index"`
	ServiceCode    string `gorm:"size:32"`

	// 目的地物流（末端配送）
	DestCarrierCode    string `gorm:"size:32"`
	DestCarrierName    string `gorm:"size:64"`
	DestTrackingNumber string `gorm:"size:64"`

	// 面单
	LabelURL  string `gorm:"size:500"`
	LabelType string `gorm:"size:10"` // PDF, ZPL

	// 包裹信息
	Weight     float64
	WeightUnit string `gorm:"size:10;default:KG"`

	// 状态
	Status string `gorm:"size:32;index;default:created"`

	// Etsy 同步
	EtsySynced    bool `gorm:"default:false"`
	EtsySyncedAt  *time.Time
	EtsySyncError string `gorm:"type:text"`

	// 最后跟踪信息
	LastTrackingStatus   string `gorm:"size:64"`
	LastTrackingTime     *time.Time
	LastTrackingLocation string `gorm:"size:255"`

	// 时间
	ShippedAt   *time.Time
	DeliveredAt *time.Time

	// 关联
	TrackingEvents []TrackingEvent `gorm:"foreignKey:ShipmentID"`
}

func (*Shipment) TableName() string {
	return "shipments"
}

// ShouldSyncToEtsy 是否应同步到 Etsy
func (s *Shipment) ShouldSyncToEtsy() bool {
	return (s.Status == ShipmentStatusDelivering || s.Status == ShipmentStatusDelivered) && !s.EtsySynced
}

// ==================== TrackingEvent 物流轨迹 ====================

// TrackingEvent 物流轨迹事件
type TrackingEvent struct {
	BaseModel
	ShipmentID    int64  `gorm:"index;not null"`
	KarrioEventID string `gorm:"size:64"`

	// 事件信息
	OccurredAt  time.Time `gorm:"index"`
	Status      string    `gorm:"size:32"`
	StatusCode  string    `gorm:"size:32"`
	Description string    `gorm:"size:500"`
	Location    string    `gorm:"size:255"`

	// 原始数据（PostgreSQL JSONB）
	RawPayload datatypes.JSON `gorm:"type:jsonb"`
}

func (*TrackingEvent) TableName() string {
	return "tracking_events"
}
