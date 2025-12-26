package dto

// ==================== 通用结构 ====================

// Address 地址（Karrio 通用）
type Address struct {
	PersonName   string `json:"person_name,omitempty"`
	CompanyName  string `json:"company_name,omitempty"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2,omitempty"`
	City         string `json:"city"`
	StateCode    string `json:"state_code,omitempty"`
	PostalCode   string `json:"postal_code"`
	CountryCode  string `json:"country_code"`
	Phone        string `json:"phone_number,omitempty"`
	Email        string `json:"email,omitempty"`
}

// Parcel 包裹
type Parcel struct {
	Weight        float64 `json:"weight"`
	WeightUnit    string  `json:"weight_unit"` // KG, LB
	Length        float64 `json:"length,omitempty"`
	Width         float64 `json:"width,omitempty"`
	Height        float64 `json:"height,omitempty"`
	DimensionUnit string  `json:"dimension_unit,omitempty"` // CM, IN
}

// ==================== Shipment 运单 ====================

// CreateShipmentRequest 创建运单请求（Karrio API）
type CreateShipmentRequest struct {
	CarrierName string            `json:"carrier_name"`
	ServiceCode string            `json:"service_code"`
	Shipper     *Address          `json:"shipper"`
	Recipient   *Address          `json:"recipient"`
	Parcels     []Parcel          `json:"parcels"`
	Options     map[string]string `json:"options,omitempty"`
	Reference   string            `json:"reference,omitempty"`
	LabelType   string            `json:"label_type,omitempty"` // PDF, ZPL
}

// ShipmentResponse 运单响应（Karrio API）
type ShipmentResponse struct {
	ID             string   `json:"id"`
	Status         string   `json:"status"`
	CarrierName    string   `json:"carrier_name"`
	CarrierID      string   `json:"carrier_id"`
	ServiceCode    string   `json:"service"`
	TrackingNumber string   `json:"tracking_number"`
	ShipmentID     string   `json:"shipment_identifier"`
	LabelURL       string   `json:"label_url"`
	LabelType      string   `json:"label_type"`
	TrackingURL    string   `json:"tracking_url,omitempty"`
	Documents      []string `json:"docs,omitempty"`
	Meta           any      `json:"meta,omitempty"`
	CreatedAt      string   `json:"created_at"`
}

// ==================== Tracker 跟踪 ====================

// CreateTrackerRequest 创建跟踪器请求（Karrio API）
type CreateTrackerRequest struct {
	TrackingNumber string            `json:"tracking_number"`
	CarrierName    string            `json:"carrier_name"`
	Reference      string            `json:"reference,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// BatchCreateTrackersRequest 批量创建跟踪器请求
type BatchCreateTrackersRequest struct {
	Trackers []CreateTrackerRequest `json:"trackers"`
}

// TrackerResponse 跟踪器响应（Karrio API）
type TrackerResponse struct {
	ID                string          `json:"id"`
	TrackingNumber    string          `json:"tracking_number"`
	CarrierName       string          `json:"carrier_name"`
	CarrierID         string          `json:"carrier_id"`
	Status            string          `json:"status"`
	Delivered         string          `json:"delivered"` // in_transit, delivered, etc.
	EstimatedDelivery string          `json:"estimated_delivery,omitempty"`
	Events            []TrackingEvent `json:"events"`
	Meta              any             `json:"meta,omitempty"`
	CreatedAt         string          `json:"created_at"`
	UpdatedAt         string          `json:"updated_at"`
}

// TrackingEvent 跟踪事件（Karrio API）
type TrackingEvent struct {
	Date        string `json:"date"`
	Time        string `json:"time"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Code        string `json:"code"`
}

// ==================== Rate 运费报价 ====================

// RateRequest 运费报价请求
type RateRequest struct {
	Shipper   *Address `json:"shipper"`
	Recipient *Address `json:"recipient"`
	Parcels   []Parcel `json:"parcels"`
	Services  []string `json:"services,omitempty"`
	Carriers  []string `json:"carrier_ids,omitempty"`
}

// RateResponse 运费报价响应
type RateResponse struct {
	Rates []Rate `json:"rates"`
}

// Rate 单个运费报价
type Rate struct {
	ID           string  `json:"id"`
	CarrierName  string  `json:"carrier_name"`
	CarrierID    string  `json:"carrier_id"`
	ServiceCode  string  `json:"service"`
	ServiceName  string  `json:"service_name"`
	TotalCharge  float64 `json:"total_charge"`
	Currency     string  `json:"currency"`
	TransitDays  int     `json:"transit_days,omitempty"`
	ExtraCharges []struct {
		Name   string  `json:"name"`
		Amount float64 `json:"amount"`
	} `json:"extra_charges,omitempty"`
}

// ==================== Connection 连接管理 ====================

// CreateConnectionRequest 创建连接请求
type CreateConnectionRequest struct {
	CarrierName string         `json:"carrier_name"`
	CarrierID   string         `json:"carrier_id"`
	Credentials map[string]any `json:"credentials"`
	Config      map[string]any `json:"config,omitempty"`
	Active      bool           `json:"active"`
}

// CarrierConnection 物流商连接
type CarrierConnection struct {
	ID          string         `json:"id"`
	CarrierName string         `json:"carrier_name"`
	CarrierID   string         `json:"carrier_id"`
	Active      bool           `json:"active"`
	Config      map[string]any `json:"config,omitempty"`
	CreatedAt   string         `json:"created_at"`
}

// ==================== 通用响应 ====================

// KarrioListResponse 列表响应
type KarrioListResponse[T any] struct {
	Count    int    `json:"count"`
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
	Results  []T    `json:"results"`
}

// KarrioErrorResponse 错误响应
type KarrioErrorResponse struct {
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details any    `json:"details,omitempty"`
	} `json:"errors"`
}
