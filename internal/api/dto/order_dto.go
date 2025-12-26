package dto

import "time"

// ==================== 订单列表查询 ====================

// ListOrdersRequest 订单列表请求
type ListOrdersRequest struct {
	ShopID    int64  `form:"shop_id" binding:"required"`
	Status    string `form:"status"`     // pending, processing, shipped, delivered, canceled
	StartDate string `form:"start_date"` // 2024-01-01
	EndDate   string `form:"end_date"`
	Keyword   string `form:"keyword"` // 搜索：订单号、买家名
	Page      int    `form:"page,default=1"`
	PageSize  int    `form:"page_size,default=20"`
}

// ListOrdersResponse 订单列表响应
type ListOrdersResponse struct {
	Total int64           `json:"total"`
	List  []OrderListItem `json:"list"`
}

// OrderListItem 订单列表项
type OrderListItem struct {
	ID              int64      `json:"id"`
	EtsyReceiptID   int64      `json:"etsy_receipt_id"`
	ShopID          int64      `json:"shop_id"`
	ShopName        string     `json:"shop_name"`
	BuyerName       string     `json:"buyer_name"`
	Status          string     `json:"status"`
	EtsyStatus      string     `json:"etsy_status"`
	ItemCount       int        `json:"item_count"`
	TotalAmount     float64    `json:"total_amount"`
	Currency        string     `json:"currency"`
	ShippingCountry string     `json:"shipping_country"`
	HasShipment     bool       `json:"has_shipment"`
	CreatedAt       time.Time  `json:"created_at"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
}

// ==================== 订单详情 ====================

// OrderDetailResponse 订单详情响应
type OrderDetailResponse struct {
	Order           *OrderVO           `json:"order"`
	Items           []OrderItemVO      `json:"items"`
	ShippingAddress *ShippingAddressVO `json:"shipping_address"`
	Shipment        *ShipmentVO        `json:"shipment,omitempty"`
}

// OrderVO 订单视图对象
type OrderVO struct {
	ID                int64      `json:"id"`
	EtsyReceiptID     int64      `json:"etsy_receipt_id"`
	ShopID            int64      `json:"shop_id"`
	ShopName          string     `json:"shop_name"`
	BuyerUserID       int64      `json:"buyer_user_id"`
	BuyerEmail        string     `json:"buyer_email"`
	BuyerName         string     `json:"buyer_name"`
	Status            string     `json:"status"`
	EtsyStatus        string     `json:"etsy_status"`
	MessageFromBuyer  string     `json:"message_from_buyer,omitempty"`
	MessageFromSeller string     `json:"message_from_seller,omitempty"`
	IsGift            bool       `json:"is_gift"`
	GiftMessage       string     `json:"gift_message,omitempty"`
	SubtotalAmount    float64    `json:"subtotal_amount"`
	ShippingAmount    float64    `json:"shipping_amount"`
	TaxAmount         float64    `json:"tax_amount"`
	DiscountAmount    float64    `json:"discount_amount"`
	GrandTotalAmount  float64    `json:"grand_total_amount"`
	Currency          string     `json:"currency"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	ShippedAt         *time.Time `json:"shipped_at,omitempty"`
	EtsySyncedAt      *time.Time `json:"etsy_synced_at,omitempty"`
}

// OrderItemVO 订单项视图对象
type OrderItemVO struct {
	ID                int64   `json:"id"`
	EtsyTransactionID int64   `json:"etsy_transaction_id"`
	ListingID         int64   `json:"listing_id"`
	Title             string  `json:"title"`
	SKU               string  `json:"sku,omitempty"`
	ImageURL          string  `json:"image_url,omitempty"`
	Quantity          int     `json:"quantity"`
	Price             float64 `json:"price"`
	ShippingCost      float64 `json:"shipping_cost"`
	Variations        string  `json:"variations,omitempty"` // JSON string
}

// ShippingAddressVO 收货地址视图对象
type ShippingAddressVO struct {
	Name        string `json:"name"`
	FirstLine   string `json:"first_line"`
	SecondLine  string `json:"second_line,omitempty"`
	City        string `json:"city"`
	State       string `json:"state,omitempty"`
	PostalCode  string `json:"postal_code"`
	CountryCode string `json:"country_code"`
	CountryName string `json:"country_name"`
	Phone       string `json:"phone,omitempty"`
}

// ShipmentVO 发货信息视图对象
type ShipmentVO struct {
	ID                 int64      `json:"id"`
	CarrierCode        string     `json:"carrier_code"`
	CarrierName        string     `json:"carrier_name"`
	TrackingNumber     string     `json:"tracking_number"`
	DestCarrierCode    string     `json:"dest_carrier_code,omitempty"`
	DestTrackingNumber string     `json:"dest_tracking_number,omitempty"`
	LabelURL           string     `json:"label_url,omitempty"`
	Status             string     `json:"status"`
	EtsySynced         bool       `json:"etsy_synced"`
	EtsySyncedAt       *time.Time `json:"etsy_synced_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// ==================== 订单同步 ====================

// SyncOrdersRequest 同步订单请求
type SyncOrdersRequest struct {
	ShopID     int64  `json:"shop_id" binding:"required"`
	MinCreated string `json:"min_created,omitempty"` // Unix timestamp
	MaxCreated string `json:"max_created,omitempty"`
	ForceSync  bool   `json:"force_sync,omitempty"` // 强制全量同步
}

// SyncOrdersResponse 同步订单响应
type SyncOrdersResponse struct {
	TotalFetched  int      `json:"total_fetched"`
	NewOrders     int      `json:"new_orders"`
	UpdatedOrders int      `json:"updated_orders"`
	Errors        []string `json:"errors,omitempty"`
}

// ==================== 订单状态更新 ====================

// UpdateOrderStatusRequest 更新订单状态请求
type UpdateOrderStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending processing shipped delivered canceled"`
}

// UpdateOrderNoteRequest 更新订单备注请求
type UpdateOrderNoteRequest struct {
	MessageFromSeller string `json:"message_from_seller"`
}

// ==================== 批量操作 ====================

// BatchOrderIDsRequest 批量订单ID请求
type BatchOrderIDsRequest struct {
	OrderIDs []int64 `json:"order_ids" binding:"required,min=1,max=100"`
}

// BatchOperationResponse 批量操作响应
type BatchOperationResponse struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

// ==================== 订单统计 ====================

// OrderStatsRequest 订单统计请求
type OrderStatsRequest struct {
	ShopID    int64  `form:"shop_id" binding:"required"`
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
}

// OrderStatsResponse 订单统计响应
type OrderStatsResponse struct {
	TotalOrders      int     `json:"total_orders"`
	TotalAmount      float64 `json:"total_amount"`
	Currency         string  `json:"currency"`
	PendingOrders    int     `json:"pending_orders"`
	ProcessingOrders int     `json:"processing_orders"`
	ShippedOrders    int     `json:"shipped_orders"`
	DeliveredOrders  int     `json:"delivered_orders"`
	CanceledOrders   int     `json:"canceled_orders"`
	AvgOrderValue    float64 `json:"avg_order_value"`
}

// ==================== Etsy Receipt 映射 ====================

// EtsyReceiptData Etsy 订单原始数据（用于同步解析）
type EtsyReceiptData struct {
	ReceiptID          int64                 `json:"receipt_id"`
	ReceiptType        int                   `json:"receipt_type"`
	SellerUserID       int64                 `json:"seller_user_id"`
	SellerEmail        string                `json:"seller_email"`
	BuyerUserID        int64                 `json:"buyer_user_id"`
	BuyerEmail         string                `json:"buyer_email"`
	Name               string                `json:"name"`
	FirstLine          string                `json:"first_line"`
	SecondLine         string                `json:"second_line"`
	City               string                `json:"city"`
	State              string                `json:"state"`
	Zip                string                `json:"zip"`
	Status             string                `json:"status"`
	FormattedAddress   string                `json:"formatted_address"`
	CountryISO         string                `json:"country_iso"`
	PaymentMethod      string                `json:"payment_method"`
	PaymentEmail       string                `json:"payment_email"`
	MessageFromSeller  string                `json:"message_from_seller"`
	MessageFromBuyer   string                `json:"message_from_buyer"`
	MessageFromPayment string                `json:"message_from_payment"`
	IsPaid             bool                  `json:"is_paid"`
	IsShipped          bool                  `json:"is_shipped"`
	CreateTimestamp    int64                 `json:"create_timestamp"`
	CreatedTimestamp   int64                 `json:"created_timestamp"`
	UpdateTimestamp    int64                 `json:"update_timestamp"`
	UpdatedTimestamp   int64                 `json:"updated_timestamp"`
	IsGift             bool                  `json:"is_gift"`
	GiftMessage        string                `json:"gift_message"`
	GrandTotal         EtsyMoney             `json:"grandtotal"`
	Subtotal           EtsyMoney             `json:"subtotal"`
	TotalPrice         EtsyMoney             `json:"total_price"`
	TotalShippingCost  EtsyMoney             `json:"total_shipping_cost"`
	TotalTaxCost       EtsyMoney             `json:"total_tax_cost"`
	TotalVatCost       EtsyMoney             `json:"total_vat_cost"`
	DiscountAmt        EtsyMoney             `json:"discount_amt"`
	GiftWrapPrice      EtsyMoney             `json:"gift_wrap_price"`
	Shipments          []EtsyShipmentData    `json:"shipments"`
	Transactions       []EtsyTransactionData `json:"transactions"`
}

// EtsyMoney Etsy 金额
type EtsyMoney struct {
	Amount       int    `json:"amount"`
	Divisor      int    `json:"divisor"`
	CurrencyCode string `json:"currency_code"`
}

// ToFloat 转换为浮点数
func (m EtsyMoney) ToFloat() float64 {
	if m.Divisor == 0 {
		return 0
	}
	return float64(m.Amount) / float64(m.Divisor)
}

// EtsyShipmentData Etsy 发货数据
type EtsyShipmentData struct {
	ReceiptShippingID             int64  `json:"receipt_shipping_id"`
	ShipmentNotificationTimestamp int64  `json:"shipment_notification_timestamp"`
	CarrierName                   string `json:"carrier_name"`
	TrackingCode                  string `json:"tracking_code"`
}

// EtsyTransactionData Etsy 交易数据（订单项）
type EtsyTransactionData struct {
	TransactionID    int64             `json:"transaction_id"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	SellerUserID     int64             `json:"seller_user_id"`
	BuyerUserID      int64             `json:"buyer_user_id"`
	CreateTimestamp  int64             `json:"create_timestamp"`
	CreatedTimestamp int64             `json:"created_timestamp"`
	PaidTimestamp    int64             `json:"paid_timestamp"`
	ShippedTimestamp int64             `json:"shipped_timestamp"`
	Quantity         int               `json:"quantity"`
	ListingImageID   int64             `json:"listing_image_id"`
	ReceiptID        int64             `json:"receipt_id"`
	IsDigital        bool              `json:"is_digital"`
	FileData         string            `json:"file_data"`
	ListingID        int64             `json:"listing_id"`
	SKU              string            `json:"sku"`
	ProductID        int64             `json:"product_id"`
	TransactionType  string            `json:"transaction_type"`
	Price            EtsyMoney         `json:"price"`
	ShippingCost     EtsyMoney         `json:"shipping_cost"`
	Variations       []EtsyVariation   `json:"variations"`
	ProductData      []EtsyProductData `json:"product_data"`
}

// EtsyVariation Etsy 变体
type EtsyVariation struct {
	PropertyID     int64  `json:"property_id"`
	ValueID        int64  `json:"value_id"`
	FormattedName  string `json:"formatted_name"`
	FormattedValue string `json:"formatted_value"`
}

// EtsyProductData Etsy 产品数据
type EtsyProductData struct {
	PropertyID   int64    `json:"property_id"`
	PropertyName string   `json:"property_name"`
	ValueIDs     []int64  `json:"value_ids"`
	Values       []string `json:"values"`
}
