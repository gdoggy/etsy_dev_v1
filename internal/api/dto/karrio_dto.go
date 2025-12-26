package dto

import "time"

// ==================== Karrio 通用结构 ====================

// KarrioAddress 地址
type KarrioAddress struct {
	PersonName   string `json:"person_name,omitempty"`
	Company      string `json:"company_name,omitempty"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2,omitempty"`
	City         string `json:"city"`
	StateCode    string `json:"state_code,omitempty"`
	PostalCode   string `json:"postal_code"`
	CountryCode  string `json:"country_code"`
	Phone        string `json:"phone_number,omitempty"`
	Email        string `json:"email,omitempty"`
	Residential  bool   `json:"residential,omitempty"`
}

// KarrioParcel 包裹
type KarrioParcel struct {
	Weight        float64 `json:"weight"`
	WeightUnit    string  `json:"weight_unit"` // KG, LB
	Length        float64 `json:"length,omitempty"`
	Width         float64 `json:"width,omitempty"`
	Height        float64 `json:"height,omitempty"`
	DimensionUnit string  `json:"dimension_unit,omitempty"` // CM, IN
	PackagingType string  `json:"packaging_type,omitempty"`
	Description   string  `json:"description,omitempty"`
}

// KarrioCustomsItem 海关申报项
type KarrioCustomsItem struct {
	Description   string  `json:"description"`
	Quantity      int     `json:"quantity"`
	Value         float64 `json:"value_amount"`
	Currency      string  `json:"value_currency"`
	Weight        float64 `json:"weight,omitempty"`
	WeightUnit    string  `json:"weight_unit,omitempty"`
	OriginCountry string  `json:"origin_country,omitempty"`
	HSCode        string  `json:"hs_code,omitempty"`
	SKU           string  `json:"sku,omitempty"`
}

// KarrioCustoms 海关信息
type KarrioCustoms struct {
	ContentType      string              `json:"content_type,omitempty"` // merchandise, documents, gift, sample
	Incoterm         string              `json:"incoterm,omitempty"`     // DDU, DDP
	Commodities      []KarrioCustomsItem `json:"commodities"`
	DeclaredValue    float64             `json:"invoice,omitempty"`
	DeclaredCurrency string              `json:"invoice_currency,omitempty"`
}

// ==================== Shipment 运单 ====================

// CreateShipmentRequest 创建运单请求
type CreateShipmentRequest struct {
	CarrierIDs  []string       `json:"carrier_ids"`
	ServiceCode string         `json:"service,omitempty"`
	Shipper     KarrioAddress  `json:"shipper"`
	Recipient   KarrioAddress  `json:"recipient"`
	Parcels     []KarrioParcel `json:"parcels"`
	Customs     *KarrioCustoms `json:"customs,omitempty"`
	Reference   string         `json:"reference,omitempty"`
	LabelType   string         `json:"label_type,omitempty"` // PDF, ZPL
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ShipmentResponse 运单响应
type ShipmentResponse struct {
	ID             string          `json:"id"`
	CarrierID      string          `json:"carrier_id"`
	CarrierName    string          `json:"carrier_name"`
	TrackingNumber string          `json:"tracking_number"`
	ShipmentID     string          `json:"shipment_identifier"`
	LabelURL       string          `json:"label_url,omitempty"`
	LabelType      string          `json:"label_type,omitempty"`
	Status         string          `json:"status"`
	Service        string          `json:"service,omitempty"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	Messages       []KarrioMessage `json:"messages,omitempty"`
}

// KarrioMessage 消息/错误
type KarrioMessage struct {
	CarrierName string `json:"carrier_name,omitempty"`
	CarrierID   string `json:"carrier_id,omitempty"`
	Code        string `json:"code,omitempty"`
	Message     string `json:"message"`
}

// ==================== Tracker 跟踪 ====================

// CreateTrackerRequest 创建跟踪器请求
type CreateTrackerRequest struct {
	TrackingNumber string         `json:"tracking_number"`
	CarrierName    string         `json:"carrier_name"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// BatchCreateTrackersRequest 批量创建跟踪器
type BatchCreateTrackersRequest struct {
	Trackers []CreateTrackerRequest `json:"trackers"`
}

// TrackerResponse 跟踪器响应
type TrackerResponse struct {
	ID                string          `json:"id"`
	TrackingNumber    string          `json:"tracking_number"`
	CarrierID         string          `json:"carrier_id"`
	CarrierName       string          `json:"carrier_name"`
	Status            string          `json:"status"` // pending, in_transit, out_for_delivery, delivered, ...
	EstimatedDelivery *time.Time      `json:"estimated_delivery,omitempty"`
	Events            []TrackingEvent `json:"events,omitempty"`
	Metadata          map[string]any  `json:"metadata,omitempty"`
}

// TrackingEvent 跟踪事件
type TrackingEvent struct {
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	Location    string    `json:"location,omitempty"`
	Code        string    `json:"code,omitempty"`
	Time        string    `json:"time,omitempty"`
}

// ==================== Rate 运费报价 ====================

// RateRequest 运费查询请求
type RateRequest struct {
	CarrierIDs []string       `json:"carrier_ids"`
	Shipper    KarrioAddress  `json:"shipper"`
	Recipient  KarrioAddress  `json:"recipient"`
	Parcels    []KarrioParcel `json:"parcels"`
	Services   []string       `json:"services,omitempty"`
}

// RateResponse 运费响应
type RateResponse struct {
	Rates []Rate `json:"rates"`
}

// Rate 单个运费报价
type Rate struct {
	ID          string  `json:"id"`
	CarrierID   string  `json:"carrier_id"`
	CarrierName string  `json:"carrier_name"`
	Service     string  `json:"service"`
	TotalCharge float64 `json:"total_charge"`
	Currency    string  `json:"currency"`
	TransitDays int     `json:"transit_days,omitempty"`
}

// ==================== Webhook ====================

// KarrioWebhookPayload Webhook 载荷
type KarrioWebhookPayload struct {
	Event     string         `json:"event"` // tracking.updated, shipment.purchased, ...
	Data      map[string]any `json:"data"`
	TestMode  bool           `json:"test_mode"`
	CreatedAt time.Time      `json:"created_at"`
}

// TrackingWebhookData 跟踪 Webhook 数据
type TrackingWebhookData struct {
	TrackerID      string          `json:"id"`
	TrackingNumber string          `json:"tracking_number"`
	CarrierName    string          `json:"carrier_name"`
	Status         string          `json:"status"`
	Events         []TrackingEvent `json:"events"`
}

// ==================== Connection 连接配置 ====================

// CarrierConnection 物流商连接
type CarrierConnection struct {
	ID          string         `json:"id"`
	CarrierID   string         `json:"carrier_id"`
	CarrierName string         `json:"carrier_name"`
	TestMode    bool           `json:"test_mode"`
	Active      bool           `json:"active"`
	Credentials map[string]any `json:"credentials,omitempty"`
}

// CreateConnectionRequest 创建连接请求
type CreateConnectionRequest struct {
	CarrierName string         `json:"carrier_name"`
	CarrierID   string         `json:"carrier_id"`
	Credentials map[string]any `json:"credentials"`
	TestMode    bool           `json:"test_mode,omitempty"`
}

// ==================== 通用响应 ====================

// KarrioErrorResponse 错误响应
type KarrioErrorResponse struct {
	Errors []KarrioMessage `json:"errors"`
}

// KarrioListResponse 列表响应
type KarrioListResponse[T any] struct {
	Count   int `json:"count"`
	Results []T `json:"results"`
}
