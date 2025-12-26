package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ==================== 订单状态常量 ====================

// OrderStatus ERP 订单状态
const (
	OrderStatusPending    = "pending"    // 待处理
	OrderStatusProcessing = "processing" // 处理中（已打单）
	OrderStatusShipped    = "shipped"    // 已发货
	OrderStatusDelivering = "delivering" // 派送中
	OrderStatusDelivered  = "delivered"  // 已签收
	OrderStatusCanceled   = "canceled"   // 已取消
)

// EtsyOrderStatus Etsy 订单状态
const (
	EtsyStatusPaid              = "paid"               // 已支付
	EtsyStatusCompleted         = "completed"          // 已完成
	EtsyStatusOpen              = "open"               // 进行中
	EtsyStatusPaymentProcessing = "payment_processing" // 支付处理中
	EtsyStatusCanceled          = "canceled"           // 已取消
)

// ==================== Order 订单主表 ====================

// Order 订单
type Order struct {
	ID            int64 `gorm:"primaryKey;autoIncrement"`
	EtsyReceiptID int64 `gorm:"uniqueIndex;not null"`
	ShopID        int64 `gorm:"index;not null"`

	// 买家信息
	BuyerUserID int64
	BuyerEmail  string `gorm:"size:255"`
	BuyerName   string `gorm:"size:255"`

	// 状态
	Status     string `gorm:"size:32;index;default:pending"`
	EtsyStatus string `gorm:"size:32"`

	// 消息
	MessageFromBuyer  string `gorm:"type:text"`
	MessageFromSeller string `gorm:"type:text"`

	// 礼物
	IsGift      bool   `gorm:"default:false"`
	GiftMessage string `gorm:"type:text"`

	// 收货地址（PostgreSQL JSONB）
	ShippingAddress datatypes.JSONMap `gorm:"type:jsonb"`

	// 金额（分为单位存储）
	SubtotalAmount   int64
	ShippingAmount   int64
	TaxAmount        int64
	DiscountAmount   int64
	GrandTotalAmount int64
	Currency         string `gorm:"size:10;default:USD"`

	// 支付
	PaymentMethod string `gorm:"size:64"`
	IsPaid        bool   `gorm:"default:false"`
	PaidAt        *time.Time

	// 发货
	IsShipped bool `gorm:"default:false"`
	ShippedAt *time.Time

	// Etsy 原始数据（PostgreSQL JSONB）
	EtsyRawData datatypes.JSON `gorm:"type:jsonb"`

	// 同步时间
	EtsyCreatedAt *time.Time
	EtsyUpdatedAt *time.Time
	EtsySyncedAt  *time.Time

	// 审计字段
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// 关联
	Items    []OrderItem `gorm:"foreignKey:OrderID"`
	Shipment *Shipment   `gorm:"foreignKey:OrderID"`
}

func (*Order) TableName() string {
	return "orders"
}

// GetSubtotal 获取小计金额（元）
func (o *Order) GetSubtotal() float64 {
	return float64(o.SubtotalAmount) / 100
}

// GetShipping 获取运费（元）
func (o *Order) GetShipping() float64 {
	return float64(o.ShippingAmount) / 100
}

// GetTax 获取税费（元）
func (o *Order) GetTax() float64 {
	return float64(o.TaxAmount) / 100
}

// GetDiscount 获取折扣（元）
func (o *Order) GetDiscount() float64 {
	return float64(o.DiscountAmount) / 100
}

// GetGrandTotal 获取总金额（元）
func (o *Order) GetGrandTotal() float64 {
	return float64(o.GrandTotalAmount) / 100
}

// CanProcess 检查是否可以处理（打单）
func (o *Order) CanProcess() bool {
	return o.Status == OrderStatusPending && o.IsPaid
}

// CanShip 检查是否可以发货
func (o *Order) CanShip() bool {
	return o.Status == OrderStatusProcessing
}

// CanCancel 检查是否可以取消
func (o *Order) CanCancel() bool {
	return o.Status == OrderStatusPending || o.Status == OrderStatusProcessing
}

// GetShippingAddressField 获取收货地址字段
func (o *Order) GetShippingAddressField(key string) string {
	if o.ShippingAddress == nil {
		return ""
	}
	if v, ok := o.ShippingAddress[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ==================== OrderItem 订单项 ====================

// OrderItem 订单项
type OrderItem struct {
	ID                int64 `gorm:"primaryKey;autoIncrement"`
	OrderID           int64 `gorm:"index;not null"`
	EtsyTransactionID int64 `gorm:"uniqueIndex;not null"`

	// 商品信息
	ListingID int64 `gorm:"index"`
	ProductID int64
	Title     string `gorm:"size:500"`
	SKU       string `gorm:"size:100;index"`

	// 数量与价格
	Quantity     int `gorm:"default:1"`
	PriceAmount  int64
	ShippingCost int64
	Currency     string `gorm:"size:10"`

	// 变体信息（PostgreSQL JSONB）
	Variations datatypes.JSONMap `gorm:"type:jsonb"`

	// 图片
	ListingImageID int64
	ImageURL       string `gorm:"size:500"`

	// 数字商品
	IsDigital bool `gorm:"default:false"`

	// 时间戳
	PaidAt    *time.Time
	ShippedAt *time.Time

	// 审计
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (*OrderItem) TableName() string {
	return "order_items"
}

// GetPrice 获取单价（元）
func (i *OrderItem) GetPrice() float64 {
	return float64(i.PriceAmount) / 100
}

// GetShippingCost 获取运费（元）
func (i *OrderItem) GetShippingCost() float64 {
	return float64(i.ShippingCost) / 100
}

// GetTotalPrice 获取总价（元）
func (i *OrderItem) GetTotalPrice() float64 {
	return float64(i.PriceAmount*int64(i.Quantity)) / 100
}

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
	ID      int64 `gorm:"primaryKey;autoIncrement"`
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

	// 审计
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// 关联
	TrackingEvents []TrackingEvent `gorm:"foreignKey:ShipmentID"`
}

func (Shipment) TableName() string {
	return "shipments"
}

// ShouldSyncToEtsy 是否应同步到 Etsy
func (s *Shipment) ShouldSyncToEtsy() bool {
	return (s.Status == ShipmentStatusDelivering || s.Status == ShipmentStatusDelivered) && !s.EtsySynced
}

// ==================== TrackingEvent 物流轨迹 ====================

// TrackingEvent 物流轨迹事件
type TrackingEvent struct {
	ID            int64  `gorm:"primaryKey;autoIncrement"`
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

	// 审计
	CreatedAt time.Time
}

func (TrackingEvent) TableName() string {
	return "tracking_events"
}
