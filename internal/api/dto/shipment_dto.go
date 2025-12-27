package dto

import "time"

// ==================== 请求 DTO ====================

// CreateEtsyShipmentRequest 创建发货请求
type CreateEtsyShipmentRequest struct {
	OrderID        int64   `json:"order_id" binding:"required"`
	CarrierCode    string  `json:"carrier_code" binding:"required"` // yanwen, wanbang
	ServiceCode    string  `json:"service_code"`                    // 服务代码
	TrackingNumber string  `json:"tracking_number"`                 // 可选，手动填写
	Weight         float64 `json:"weight"`                          // 重量 KG
	WeightUnit     string  `json:"weight_unit"`                     // KG, LB
}

// UpdateEtsyShipmentRequest 更新发货请求
type UpdateEtsyShipmentRequest struct {
	CarrierCode        string  `json:"carrier_code"`
	TrackingNumber     string  `json:"tracking_number"`
	DestCarrierCode    string  `json:"dest_carrier_code"`
	DestTrackingNumber string  `json:"dest_tracking_number"`
	Weight             float64 `json:"weight"`
	Status             string  `json:"status"`
}

// ListEtsyShipmentsRequest 发货列表请求 todo tags 是 form
type ListEtsyShipmentsRequest struct {
	OrderID        int64      `form:"order_id"`
	CarrierCode    string     `form:"carrier_code"`
	Status         string     `form:"status"`
	EtsySynced     *bool      `form:"etsy_synced"`
	TrackingNumber string     `form:"tracking_number"`
	StartDate      *time.Time `form:"start_date"`
	EndDate        *time.Time `form:"end_date"`
	Page           int        `form:"page"`
	PageSize       int        `form:"page_size"`
}

// SyncEtsyTrackingRequest 同步 Etsy 物流请求
type SyncEtsyTrackingRequest struct {
	ShipmentID int64 `json:"shipment_id" binding:"required"`
}

// ==================== 响应 DTO ====================

// EtsyShipmentResponse 发货响应
type EtsyShipmentResponse struct {
	ID                 int64   `json:"id"`
	OrderID            int64   `json:"order_id"`
	CarrierCode        string  `json:"carrier_code"`
	CarrierName        string  `json:"carrier_name"`
	TrackingNumber     string  `json:"tracking_number"`
	ServiceCode        string  `json:"service_code"`
	DestCarrierCode    string  `json:"dest_carrier_code,omitempty"`
	DestCarrierName    string  `json:"dest_carrier_name,omitempty"`
	DestTrackingNumber string  `json:"dest_tracking_number,omitempty"`
	LabelURL           string  `json:"label_url,omitempty"`
	Weight             float64 `json:"weight"`
	WeightUnit         string  `json:"weight_unit"`
	Status             string  `json:"status"`
	StatusText         string  `json:"status_text"`

	// Etsy 同步
	EtsySynced    bool    `json:"etsy_synced"`
	EtsySyncedAt  *string `json:"etsy_synced_at,omitempty"`
	EtsySyncError string  `json:"etsy_sync_error,omitempty"`

	// 最新跟踪
	LastTrackingStatus   string  `json:"last_tracking_status,omitempty"`
	LastTrackingTime     *string `json:"last_tracking_time,omitempty"`
	LastTrackingLocation string  `json:"last_tracking_location,omitempty"`

	// 时间
	ShippedAt   *string `json:"shipped_at,omitempty"`
	DeliveredAt *string `json:"delivered_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`

	// 轨迹
	TrackingEvents []TrackingEventResponse `json:"tracking_events,omitempty"`
}

// TrackingEventResponse 物流轨迹响应
type TrackingEventResponse struct {
	ID          int64  `json:"id"`
	OccurredAt  string `json:"occurred_at"`
	Status      string `json:"status"`
	StatusCode  string `json:"status_code"`
	Description string `json:"description"`
	Location    string `json:"location"`
}

// EtsyShipmentListResponse 发货列表响应
type EtsyShipmentListResponse struct {
	List     []EtsyShipmentResponse `json:"list"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// EtsyShipmentDetailResponse 发货详情响应
type EtsyShipmentDetailResponse struct {
	Shipment       *EtsyShipmentResponse   `json:"shipment"`
	Order          *OrderBriefResponse     `json:"order,omitempty"`
	TrackingEvents []TrackingEventResponse `json:"tracking_events"`
}

// OrderBriefResponse 订单简要信息
type OrderBriefResponse struct {
	ID            int64   `json:"id"`
	EtsyReceiptID int64   `json:"etsy_receipt_id"`
	BuyerName     string  `json:"buyer_name"`
	GrandTotal    float64 `json:"grand_total"`
	Currency      string  `json:"currency"`
	Status        string  `json:"status"`
}

// ==================== Karrio 集成 DTO ====================

// CreateLabelRequest 创建面单请求
type CreateLabelRequest struct {
	OrderID     int64  `json:"order_id" binding:"required"`
	CarrierCode string `json:"carrier_code" binding:"required"`
	ServiceCode string `json:"service_code" binding:"required"`
}

// CreateLabelResponse 创建面单响应
type CreateLabelResponse struct {
	ShipmentID     int64  `json:"shipment_id"`
	TrackingNumber string `json:"tracking_number"`
	LabelURL       string `json:"label_url"`
	LabelType      string `json:"label_type"`
}

// ==================== 物流商配置 DTO ====================

// CarrierInfo 物流商信息
type CarrierInfo struct {
	Code     string        `json:"code"`
	Name     string        `json:"name"`
	Services []ServiceInfo `json:"services"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CarrierListResponse 物流商列表响应
type CarrierListResponse struct {
	Carriers []CarrierInfo `json:"carriers"`
}

// ==================== 统计 DTO ====================

// ShipmentStatsResponse 发货统计响应
type ShipmentStatsResponse struct {
	Total       int64            `json:"total"`
	ByStatus    map[string]int64 `json:"by_status"`
	ByCarrier   map[string]int64 `json:"by_carrier"`
	EtsySynced  int64            `json:"etsy_synced"`
	EtsyPending int64            `json:"etsy_pending"`
}
