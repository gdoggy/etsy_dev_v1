package dto

import "time"

// ================== Shop && Shipping DTO ==================

// ShopListReq 店铺列表请求
type ShopListReq struct {
	Page        int    `form:"page,default=1"`
	PageSize    int    `form:"page_size,default=20"`
	ShopName    string `form:"shop_name"`
	Status      int    `form:"status,default=-1"`
	ProxyID     int64  `form:"proxy_id"`
	DeveloperID int64  `form:"developer_id"`
}

// ShopUpdateToEtsyReq 推送到 Etsy（仅 Etsy 可写字段）
type ShopUpdateToEtsyReq struct {
	Title              string `json:"title"`
	Announcement       string `json:"announcement"`
	SaleMessage        string `json:"sale_message"`
	DigitalSaleMessage string `json:"digital_sale_message"`
}

// ShopStopReq 停用店铺请求（可选备注）
type ShopStopReq struct {
	Reason string `json:"reason"` // 停用原因（可选）
}

// ShopSyncResp 同步结果响应
type ShopSyncResp struct {
	Success      bool       `json:"success"`
	Message      string     `json:"message"`
	SyncedAt     *time.Time `json:"synced_at"`
	NextSyncTime *time.Time `json:"next_sync_time"`
}

// ShopResp 店铺响应
type ShopResp struct {
	ID                   int64      `json:"id"`
	EtsyShopID           int64      `json:"etsy_shop_id"`
	EtsyUserID           int64      `json:"etsy_user_id"`
	ShopName             string     `json:"shop_name"`
	Title                string     `json:"title"`
	Announcement         string     `json:"announcement"`
	SaleMessage          string     `json:"sale_message"`
	DigitalSaleMessage   string     `json:"digital_sale_message"`
	CurrencyCode         string     `json:"currency_code"`
	ListingActiveCount   int        `json:"listing_active_count"`
	TransactionSoldCount int        `json:"transaction_sold_count"`
	ReviewCount          int        `json:"review_count"`
	ReviewAverage        float64    `json:"review_average"`
	TokenStatus          string     `json:"token_status"`
	Status               int        `json:"status"`
	StatusText           string     `json:"status_text"`
	EtsySyncedAt         *time.Time `json:"etsy_synced_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	// 关联信息
	ProxyID       int64  `json:"proxy_id"`
	ProxyIP       string `json:"proxy_ip"`
	ProxyRegion   string `json:"proxy_region"`
	DeveloperID   int64  `json:"developer_id"`
	DeveloperName string `json:"developer_name"`
}

// ShopDetailResp 店铺详情响应（含关联数据）
type ShopDetailResp struct {
	ShopResp
	Sections         []ShopSectionResp     `json:"sections"`
	ShippingProfiles []ShippingProfileResp `json:"shipping_profiles"`
	ReturnPolicies   []ReturnPolicyResp    `json:"return_policies"`
}

// ShopListResp 店铺列表响应
type ShopListResp struct {
	Total int64      `json:"total"`
	List  []ShopResp `json:"list"`
}

// ShopSectionListReq 店铺分区列表请求
type ShopSectionListReq struct {
	ShopID int64 `form:"shop_id" binding:"required"`
}

// ShopSectionCreateReq 店铺分区创建请求
type ShopSectionCreateReq struct {
	ShopID int64  `json:"shop_id" binding:"required"`
	Title  string `json:"title" binding:"required,max=255"`
}

// ShopSectionUpdateReq 店铺分区更新请求
type ShopSectionUpdateReq struct {
	Title string `json:"title" binding:"required,max=255"`
	Rank  int    `json:"rank"`
}

// ShopSectionResp 店铺分区响应
type ShopSectionResp struct {
	ID                 int64      `json:"id"`
	ShopID             int64      `json:"shop_id"`
	EtsySectionID      int64      `json:"etsy_section_id"`
	Title              string     `json:"title"`
	Rank               int        `json:"rank"`
	ActiveListingCount int        `json:"active_listing_count"`
	EtsySyncedAt       *time.Time `json:"etsy_synced_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// ShopSectionListResp 店铺分区列表响应
type ShopSectionListResp struct {
	Total int64             `json:"total"`
	List  []ShopSectionResp `json:"list"`
}

// ================== ShippingProfile DTO ==================

// ShippingProfileListReq 运费模板列表请求
type ShippingProfileListReq struct {
	ShopID int64 `form:"shop_id" binding:"required"`
}

// ShippingProfileCreateReq 运费模板创建请求
type ShippingProfileCreateReq struct {
	ShopID            int64  `json:"shop_id" binding:"required"`
	Title             string `json:"title" binding:"required,max=255"`
	OriginCountryISO  string `json:"origin_country_iso" binding:"required,len=2"`
	OriginPostalCode  string `json:"origin_postal_code"`
	ProcessingDaysMin int    `json:"processing_days_min" binding:"gte=1"`
	ProcessingDaysMax int    `json:"processing_days_max" binding:"gtefield=ProcessingDaysMin"`
}

// ShippingProfileUpdateReq 运费模板更新请求
type ShippingProfileUpdateReq struct {
	Title             string `json:"title" binding:"max=255"`
	OriginCountryISO  string `json:"origin_country_iso" binding:"omitempty,len=2"`
	OriginPostalCode  string `json:"origin_postal_code"`
	ProcessingDaysMin int    `json:"processing_days_min"`
	ProcessingDaysMax int    `json:"processing_days_max"`
}

// ShippingProfileResp 运费模板响应
type ShippingProfileResp struct {
	ID                int64      `json:"id"`
	ShopID            int64      `json:"shop_id"`
	EtsyProfileID     int64      `json:"etsy_profile_id"`
	Title             string     `json:"title"`
	OriginCountryISO  string     `json:"origin_country_iso"`
	OriginPostalCode  string     `json:"origin_postal_code"`
	ProcessingDaysMin int        `json:"processing_days_min"`
	ProcessingDaysMax int        `json:"processing_days_max"`
	EtsySyncedAt      *time.Time `json:"etsy_synced_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// ShippingProfileDetailResp 运费模板详情响应（含关联数据）
type ShippingProfileDetailResp struct {
	ShippingProfileResp
	Destinations []ShippingDestinationResp `json:"destinations"`
	Upgrades     []ShippingUpgradeResp     `json:"upgrades"`
}

// ShippingProfileListResp 运费模板列表响应
type ShippingProfileListResp struct {
	Total int64                 `json:"total"`
	List  []ShippingProfileResp `json:"list"`
}

// ================== ShippingDestination DTO ==================

// ShippingDestinationListReq 运费目的地列表请求
type ShippingDestinationListReq struct {
	ShippingProfileID int64 `form:"shipping_profile_id" binding:"required"`
}

// ShippingDestinationCreateReq 运费目的地创建请求
type ShippingDestinationCreateReq struct {
	ShippingProfileID     int64  `json:"shipping_profile_id" binding:"required"`
	DestinationCountryISO string `json:"destination_country_iso" binding:"required,len=2"`
	DestinationRegion     string `json:"destination_region"`
	PrimaryCost           int64  `json:"primary_cost" binding:"gte=0"`
	SecondaryCost         int64  `json:"secondary_cost" binding:"gte=0"`
	CurrencyCode          string `json:"currency_code" binding:"required,len=3"`
	ShippingCarrierID     int64  `json:"shipping_carrier_id"`
	MailClass             string `json:"mail_class"`
	DeliveryDaysMin       int    `json:"delivery_days_min" binding:"gte=0"`
	DeliveryDaysMax       int    `json:"delivery_days_max" binding:"gtefield=DeliveryDaysMin"`
}

// ShippingDestinationUpdateReq 运费目的地更新请求
type ShippingDestinationUpdateReq struct {
	DestinationCountryISO string `json:"destination_country_iso" binding:"omitempty,len=2"`
	DestinationRegion     string `json:"destination_region"`
	PrimaryCost           int64  `json:"primary_cost" binding:"gte=0"`
	SecondaryCost         int64  `json:"secondary_cost" binding:"gte=0"`
	CurrencyCode          string `json:"currency_code" binding:"omitempty,len=3"`
	ShippingCarrierID     int64  `json:"shipping_carrier_id"`
	MailClass             string `json:"mail_class"`
	DeliveryDaysMin       int    `json:"delivery_days_min"`
	DeliveryDaysMax       int    `json:"delivery_days_max"`
}

// ShippingDestinationResp 运费目的地响应
type ShippingDestinationResp struct {
	ID                    int64     `json:"id"`
	ShippingProfileID     int64     `json:"shipping_profile_id"`
	EtsyDestinationID     int64     `json:"etsy_destination_id"`
	DestinationCountryISO string    `json:"destination_country_iso"`
	DestinationRegion     string    `json:"destination_region"`
	PrimaryCost           int64     `json:"primary_cost"`
	SecondaryCost         int64     `json:"secondary_cost"`
	CurrencyCode          string    `json:"currency_code"`
	ShippingCarrierID     int64     `json:"shipping_carrier_id"`
	MailClass             string    `json:"mail_class"`
	DeliveryDaysMin       int       `json:"delivery_days_min"`
	DeliveryDaysMax       int       `json:"delivery_days_max"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// ShippingDestinationListResp 运费目的地列表响应
type ShippingDestinationListResp struct {
	Total int64                     `json:"total"`
	List  []ShippingDestinationResp `json:"list"`
}

// ================== ShippingUpgrade DTO ==================

// ShippingUpgradeListReq 加急配送选项列表请求
type ShippingUpgradeListReq struct {
	ShippingProfileID int64 `form:"shipping_profile_id" binding:"required"`
}

// ShippingUpgradeCreateReq 加急配送选项创建请求
type ShippingUpgradeCreateReq struct {
	ShippingProfileID int64  `json:"shipping_profile_id" binding:"required"`
	UpgradeName       string `json:"upgrade_name" binding:"required,max=100"`
	Type              int    `json:"type" binding:"oneof=0 1"`
	Price             int64  `json:"price" binding:"gte=0"`
	SecondaryCost     int64  `json:"secondary_cost" binding:"gte=0"`
	CurrencyCode      string `json:"currency_code" binding:"required,len=3"`
	ShippingCarrierID int64  `json:"shipping_carrier_id"`
	MailClass         string `json:"mail_class"`
	DeliveryDaysMin   int    `json:"delivery_days_min" binding:"gte=0"`
	DeliveryDaysMax   int    `json:"delivery_days_max" binding:"gtefield=DeliveryDaysMin"`
}

// ShippingUpgradeUpdateReq 加急配送选项更新请求
type ShippingUpgradeUpdateReq struct {
	UpgradeName       string `json:"upgrade_name" binding:"max=100"`
	Type              int    `json:"type" binding:"oneof=0 1"`
	Price             int64  `json:"price" binding:"gte=0"`
	SecondaryCost     int64  `json:"secondary_cost" binding:"gte=0"`
	CurrencyCode      string `json:"currency_code" binding:"omitempty,len=3"`
	ShippingCarrierID int64  `json:"shipping_carrier_id"`
	MailClass         string `json:"mail_class"`
	DeliveryDaysMin   int    `json:"delivery_days_min"`
	DeliveryDaysMax   int    `json:"delivery_days_max"`
}

// ShippingUpgradeResp 加急配送选项响应
type ShippingUpgradeResp struct {
	ID                int64     `json:"id"`
	ShippingProfileID int64     `json:"shipping_profile_id"`
	EtsyUpgradeID     int64     `json:"etsy_upgrade_id"`
	UpgradeName       string    `json:"upgrade_name"`
	Type              int       `json:"type"`
	TypeText          string    `json:"type_text"`
	Price             int64     `json:"price"`
	SecondaryCost     int64     `json:"secondary_cost"`
	CurrencyCode      string    `json:"currency_code"`
	ShippingCarrierID int64     `json:"shipping_carrier_id"`
	MailClass         string    `json:"mail_class"`
	DeliveryDaysMin   int       `json:"delivery_days_min"`
	DeliveryDaysMax   int       `json:"delivery_days_max"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ShippingUpgradeListResp 加急配送选项列表响应
type ShippingUpgradeListResp struct {
	Total int64                 `json:"total"`
	List  []ShippingUpgradeResp `json:"list"`
}

// ================== ReturnPolicy DTO ==================

// ReturnPolicyListReq 退货政策列表请求
type ReturnPolicyListReq struct {
	ShopID int64 `form:"shop_id" binding:"required"`
}

// ReturnPolicyCreateReq 退货政策创建请求
type ReturnPolicyCreateReq struct {
	ShopID           int64 `json:"shop_id" binding:"required"`
	AcceptsReturns   bool  `json:"accepts_returns"`
	AcceptsExchanges bool  `json:"accepts_exchanges"`
	ReturnDeadline   int   `json:"return_deadline" binding:"gte=0"`
}

// ReturnPolicyUpdateReq 退货政策更新请求
type ReturnPolicyUpdateReq struct {
	AcceptsReturns   bool `json:"accepts_returns"`
	AcceptsExchanges bool `json:"accepts_exchanges"`
	ReturnDeadline   int  `json:"return_deadline" binding:"gte=0"`
}

// ReturnPolicyResp 退货政策响应
type ReturnPolicyResp struct {
	ID               int64      `json:"id"`
	ShopID           int64      `json:"shop_id"`
	EtsyPolicyID     int64      `json:"etsy_policy_id"`
	AcceptsReturns   bool       `json:"accepts_returns"`
	AcceptsExchanges bool       `json:"accepts_exchanges"`
	ReturnDeadline   int        `json:"return_deadline"`
	EtsySyncedAt     *time.Time `json:"etsy_synced_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ReturnPolicyListResp 退货政策列表响应
type ReturnPolicyListResp struct {
	Total int64              `json:"total"`
	List  []ReturnPolicyResp `json:"list"`
}
