package etsy

// ==========================================
// DTO: 用于接收 Etsy API 返回的原始 JSON 数据
// ==========================================

// EtsyShopResp Etsy 店铺 API 响应
// GET /v3/application/shops/{shop_id}
type EtsyShopResp struct {
	ShopID               int64   `json:"shop_id"`
	UserID               int64   `json:"user_id"`
	ShopName             string  `json:"shop_name"`
	Title                string  `json:"title"`
	Announcement         string  `json:"announcement"`
	SaleMessage          string  `json:"sale_message"`
	DigitalSaleMessage   string  `json:"digital_sale_message"`
	CurrencyCode         string  `json:"currency_code"`
	ListingActiveCount   int     `json:"listing_active_count"`
	TransactionSoldCount int     `json:"transaction_sold_count"`
	ReviewCount          int     `json:"review_count"`
	ReviewAverage        float64 `json:"review_average"`
	URL                  string  `json:"url"`
	ImageURL760x100      string  `json:"image_url_760x100"`
	IconURLFullxFull     string  `json:"icon_url_fullxfull"`
	CreateTimestamp      int64   `json:"create_timestamp"`
	UpdateTimestamp      int64   `json:"update_timestamp"`
	IsVacation           bool    `json:"is_vacation"`
	VacationMessage      string  `json:"vacation_message"`
}

// EtsyShopUpdateReq Etsy 店铺更新请求
// PUT /v3/application/shops/{shop_id}
type EtsyShopUpdateReq struct {
	Title              string `json:"title,omitempty"`
	Announcement       string `json:"announcement,omitempty"`
	SaleMessage        string `json:"sale_message,omitempty"`
	DigitalSaleMessage string `json:"digital_sale_message,omitempty"`
}

// EtsyErrorResp Etsy 通用错误响应
type EtsyErrorResp struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// EtsyPingResp Etsy Ping 响应
// GET /v3/application/openapi-ping
type EtsyPingResp struct {
	ApplicationID int64 `json:"application_id"`
}

// EtsyShopSectionResp Etsy 店铺分区 API 响应
// GET /v3/application/shops/{shop_id}/sections/{shop_section_id}
type EtsyShopSectionResp struct {
	ShopSectionID      int64  `json:"shop_section_id"`
	Title              string `json:"title"`
	Rank               int    `json:"rank"`
	UserID             int64  `json:"user_id"`
	ActiveListingCount int    `json:"active_listing_count"`
}

// EtsyShopSectionsResp Etsy 店铺分区列表 API 响应
// GET /v3/application/shops/{shop_id}/sections
type EtsyShopSectionsResp struct {
	Count   int                   `json:"count"`
	Results []EtsyShopSectionResp `json:"results"`
}

// EtsyShopSectionCreateReq Etsy 创建店铺分区请求
// POST /v3/application/shops/{shop_id}/sections
type EtsyShopSectionCreateReq struct {
	Title string `json:"title"`
}

// EtsyShopSectionUpdateReq Etsy 更新店铺分区请求
// PUT /v3/application/shops/{shop_id}/sections/{shop_section_id}
type EtsyShopSectionUpdateReq struct {
	Title string `json:"title"`
}

// EtsyShippingProfileResp Etsy 运费模板 API 响应
// GET /v3/application/shops/{shop_id}/shipping-profiles/{shipping_profile_id}
type EtsyShippingProfileResp struct {
	ShippingProfileID           int64                         `json:"shipping_profile_id"`
	Title                       string                        `json:"title"`
	UserID                      int64                         `json:"user_id"`
	MinProcessingDays           int                           `json:"min_processing_days"`
	MaxProcessingDays           int                           `json:"max_processing_days"`
	ProcessingDaysDisplayLabel  string                        `json:"processing_days_display_label"`
	OriginCountryISO            string                        `json:"origin_country_iso"`
	OriginPostalCode            string                        `json:"origin_postal_code"`
	ProfileType                 string                        `json:"profile_type"`
	IsDeleted                   bool                          `json:"is_deleted"`
	DomesticHandlingFee         int64                         `json:"domestic_handling_fee"`
	InternationalHandlingFee    int64                         `json:"international_handling_fee"`
	ShippingProfileDestinations []EtsyShippingDestinationResp `json:"shipping_profile_destinations"`
	ShippingProfileUpgrades     []EtsyShippingUpgradeResp     `json:"shipping_profile_upgrades"`
}

// EtsyShippingProfilesResp Etsy 运费模板列表 API 响应
// GET /v3/application/shops/{shop_id}/shipping-profiles
type EtsyShippingProfilesResp struct {
	Count   int                       `json:"count"`
	Results []EtsyShippingProfileResp `json:"results"`
}

// EtsyShippingProfileCreateReq Etsy 创建运费模板请求
// POST /v3/application/shops/{shop_id}/shipping-profiles
type EtsyShippingProfileCreateReq struct {
	Title                 string `json:"title"`
	OriginCountryISO      string `json:"origin_country_iso"`
	OriginPostalCode      string `json:"origin_postal_code,omitempty"`
	MinProcessingDays     int    `json:"min_processing_time"`
	MaxProcessingDays     int    `json:"max_processing_time"`
	PrimaryCost           int64  `json:"primary_cost"`
	SecondaryCost         int64  `json:"secondary_cost"`
	DestinationCountryISO string `json:"destination_country_iso"`
	DestinationRegion     string `json:"destination_region,omitempty"`
	ShippingCarrierID     int64  `json:"shipping_carrier_id,omitempty"`
	MailClass             string `json:"mail_class,omitempty"`
	MinDeliveryDays       int    `json:"min_delivery_days,omitempty"`
	MaxDeliveryDays       int    `json:"max_delivery_days,omitempty"`
}

// EtsyShippingProfileUpdateReq Etsy 更新运费模板请求
// PUT /v3/application/shops/{shop_id}/shipping-profiles/{shipping_profile_id}
type EtsyShippingProfileUpdateReq struct {
	Title             string `json:"title,omitempty"`
	OriginCountryISO  string `json:"origin_country_iso,omitempty"`
	OriginPostalCode  string `json:"origin_postal_code,omitempty"`
	MinProcessingDays int    `json:"min_processing_time,omitempty"`
	MaxProcessingDays int    `json:"max_processing_time,omitempty"`
}

// EtsyShippingDestinationResp Etsy 运费目的地 API 响应
type EtsyShippingDestinationResp struct {
	ShippingProfileDestinationID int64     `json:"shipping_profile_destination_id"`
	ShippingProfileID            int64     `json:"shipping_profile_id"`
	OriginCountryISO             string    `json:"origin_country_iso"`
	DestinationCountryISO        string    `json:"destination_country_iso"`
	DestinationRegion            string    `json:"destination_region"`
	PrimaryCost                  EtsyMoney `json:"primary_cost"`
	SecondaryCost                EtsyMoney `json:"secondary_cost"`
	ShippingCarrierID            int64     `json:"shipping_carrier_id"`
	MailClass                    string    `json:"mail_class"`
	MinDeliveryDays              int       `json:"min_delivery_days"`
	MaxDeliveryDays              int       `json:"max_delivery_days"`
}

// EtsyShippingDestinationsResp Etsy 运费目的地列表 API 响应
// GET /v3/application/shops/{shop_id}/shipping-profiles/{shipping_profile_id}/destinations
type EtsyShippingDestinationsResp struct {
	Count   int                           `json:"count"`
	Results []EtsyShippingDestinationResp `json:"results"`
}

// EtsyShippingDestinationCreateReq Etsy 创建运费目的地请求
// POST /v3/application/shops/{shop_id}/shipping-profiles/{shipping_profile_id}/destinations
type EtsyShippingDestinationCreateReq struct {
	DestinationCountryISO string `json:"destination_country_iso"`
	DestinationRegion     string `json:"destination_region,omitempty"`
	PrimaryCost           int64  `json:"primary_cost"`
	SecondaryCost         int64  `json:"secondary_cost"`
	ShippingCarrierID     int64  `json:"shipping_carrier_id,omitempty"`
	MailClass             string `json:"mail_class,omitempty"`
	MinDeliveryDays       int    `json:"min_delivery_days,omitempty"`
	MaxDeliveryDays       int    `json:"max_delivery_days,omitempty"`
}

// EtsyShippingDestinationUpdateReq Etsy 更新运费目的地请求
// PUT /v3/application/shops/{shop_id}/shipping-profiles/{shipping_profile_id}/destinations/{shipping_profile_destination_id}
type EtsyShippingDestinationUpdateReq struct {
	DestinationCountryISO string `json:"destination_country_iso,omitempty"`
	DestinationRegion     string `json:"destination_region,omitempty"`
	PrimaryCost           int64  `json:"primary_cost,omitempty"`
	SecondaryCost         int64  `json:"secondary_cost,omitempty"`
	ShippingCarrierID     int64  `json:"shipping_carrier_id,omitempty"`
	MailClass             string `json:"mail_class,omitempty"`
	MinDeliveryDays       int    `json:"min_delivery_days,omitempty"`
	MaxDeliveryDays       int    `json:"max_delivery_days,omitempty"`
}

// EtsyShippingUpgradeResp Etsy 加急配送选项 API 响应
type EtsyShippingUpgradeResp struct {
	UpgradeID         int64     `json:"upgrade_id"`
	ShippingProfileID int64     `json:"shipping_profile_id"`
	UpgradeName       string    `json:"upgrade_name"`
	Type              int       `json:"type"`
	Price             EtsyMoney `json:"price"`
	SecondaryCost     EtsyMoney `json:"secondary_cost"`
	ShippingCarrierID int64     `json:"shipping_carrier_id"`
	MailClass         string    `json:"mail_class"`
	MinDeliveryDays   int       `json:"min_delivery_days"`
	MaxDeliveryDays   int       `json:"max_delivery_days"`
}

// EtsyShippingUpgradesResp Etsy 加急配送选项列表 API 响应
// GET /v3/application/shops/{shop_id}/shipping-profiles/{shipping_profile_id}/upgrades
type EtsyShippingUpgradesResp struct {
	Count   int                       `json:"count"`
	Results []EtsyShippingUpgradeResp `json:"results"`
}

// EtsyShippingUpgradeCreateReq Etsy 创建加急配送选项请求
// POST /v3/application/shops/{shop_id}/shipping-profiles/{shipping_profile_id}/upgrades
type EtsyShippingUpgradeCreateReq struct {
	UpgradeName       string `json:"upgrade_name"`
	Type              int    `json:"type"`
	Price             int64  `json:"price"`
	SecondaryCost     int64  `json:"secondary_cost"`
	ShippingCarrierID int64  `json:"shipping_carrier_id,omitempty"`
	MailClass         string `json:"mail_class,omitempty"`
	MinDeliveryDays   int    `json:"min_delivery_days,omitempty"`
	MaxDeliveryDays   int    `json:"max_delivery_days,omitempty"`
}

// EtsyShippingUpgradeUpdateReq Etsy 更新加急配送选项请求
// PUT /v3/application/shops/{shop_id}/shipping-profiles/{shipping_profile_id}/upgrades/{upgrade_id}
type EtsyShippingUpgradeUpdateReq struct {
	UpgradeName       string `json:"upgrade_name,omitempty"`
	Type              int    `json:"type,omitempty"`
	Price             int64  `json:"price,omitempty"`
	SecondaryCost     int64  `json:"secondary_cost,omitempty"`
	ShippingCarrierID int64  `json:"shipping_carrier_id,omitempty"`
	MailClass         string `json:"mail_class,omitempty"`
	MinDeliveryDays   int    `json:"min_delivery_days,omitempty"`
	MaxDeliveryDays   int    `json:"max_delivery_days,omitempty"`
}

// EtsyMoney Etsy 金额结构
type EtsyMoney struct {
	Amount       int    `json:"amount"`
	Divisor      int    `json:"divisor"`
	CurrencyCode string `json:"currency_code"`
}

// ToInt64 转换为分（最小单位）
func (m EtsyMoney) ToInt64() int64 {
	if m.Divisor == 0 {
		return int64(m.Amount)
	}
	return int64(m.Amount)
}

// EtsyShippingCarrierResp Etsy 物流承运商 API 响应
// GET /v3/application/shipping-carriers
type EtsyShippingCarrierResp struct {
	ShippingCarrierID    int64               `json:"shipping_carrier_id"`
	Name                 string              `json:"name"`
	DomesticClasses      []EtsyMailClassResp `json:"domestic_classes"`
	InternationalClasses []EtsyMailClassResp `json:"international_classes"`
}

// EtsyShippingCarriersResp Etsy 物流承运商列表 API 响应
type EtsyShippingCarriersResp struct {
	Count   int                       `json:"count"`
	Results []EtsyShippingCarrierResp `json:"results"`
}

// EtsyMailClassResp Etsy 邮寄类型响应
type EtsyMailClassResp struct {
	MailClassKey string `json:"mail_class_key"`
	Name         string `json:"name"`
}

// EtsyReturnPolicyResp Etsy 退货政策 API 响应
// GET /v3/application/shops/{shop_id}/policies/return/{return_policy_id}
type EtsyReturnPolicyResp struct {
	ReturnPolicyID   int64 `json:"return_policy_id"`
	ShopID           int64 `json:"shop_id"`
	AcceptsReturns   bool  `json:"accepts_returns"`
	AcceptsExchanges bool  `json:"accepts_exchanges"`
	ReturnDeadline   int   `json:"return_deadline"`
}

// EtsyReturnPoliciesResp Etsy 退货政策列表 API 响应
// GET /v3/application/shops/{shop_id}/policies/return
type EtsyReturnPoliciesResp struct {
	Count   int                    `json:"count"`
	Results []EtsyReturnPolicyResp `json:"results"`
}

// EtsyReturnPolicyCreateReq Etsy 创建退货政策请求
// POST /v3/application/shops/{shop_id}/policies/return
type EtsyReturnPolicyCreateReq struct {
	AcceptsReturns   bool `json:"accepts_returns"`
	AcceptsExchanges bool `json:"accepts_exchanges"`
	ReturnDeadline   int  `json:"return_deadline,omitempty"`
}

// EtsyReturnPolicyUpdateReq Etsy 更新退货政策请求
// PUT /v3/application/shops/{shop_id}/policies/return/{return_policy_id}
type EtsyReturnPolicyUpdateReq struct {
	AcceptsReturns   bool `json:"accepts_returns"`
	AcceptsExchanges bool `json:"accepts_exchanges"`
	ReturnDeadline   int  `json:"return_deadline,omitempty"`
}

// EtsyReturnPolicyConsolidateReq Etsy 合并退货政策请求
// POST /v3/application/shops/{shop_id}/policies/return/consolidate
type EtsyReturnPolicyConsolidateReq struct {
	SourceReturnPolicyID      int64 `json:"source_return_policy_id"`
	DestinationReturnPolicyID int64 `json:"destination_return_policy_id"`
}

// ======================= old ======================

// PriceDTO 1. 价格嵌套结构
type PriceDTO struct {
	Amount       int64  `json:"amount"`
	Divisor      int64  `json:"divisor"`
	CurrencyCode string `json:"currency_code"`
}

// ProductListingDTO 2. 单个商品结构 (完全对应 Etsy JSON 的字段)
type ProductListingDTO struct {
	ListingID                   int64    `json:"listing_id"`
	UserID                      int64    `json:"user_id"`
	ShopID                      int64    `json:"shop_id"`
	Title                       string   `json:"title"`
	Description                 string   `json:"description"`
	State                       string   `json:"state"`
	CreationTimestamp           int64    `json:"creation_timestamp"`
	CreatedTimestamp            int64    `json:"created_timestamp"`
	EndingTimestamp             int64    `json:"ending_timestamp"`
	OriginalCreationTimestamp   int64    `json:"original_creation_timestamp"`
	LastModifiedTimestamp       int64    `json:"last_modified_timestamp"`
	UpdatedTimestamp            int64    `json:"updated_timestamp"`
	StateTimestamp              int64    `json:"state_timestamp"`
	Quantity                    int      `json:"quantity"`
	ShopSectionID               int64    `json:"shop_section_id"`
	FeaturedRank                int      `json:"featured_rank"`
	URL                         string   `json:"url"`
	NumFavorers                 int      `json:"num_favorers"`
	NonTaxable                  bool     `json:"non_taxable"`
	IsTaxable                   bool     `json:"is_taxable"`
	IsCustomizable              bool     `json:"is_customizable"`
	IsPersonalizable            bool     `json:"is_personalizable"`
	PersonalizationIsRequired   bool     `json:"personalization_is_required"`
	PersonalizationCharCountMax int      `json:"personalization_char_count_max"`
	PersonalizationInstructions string   `json:"personalization_instructions"`
	ListingType                 string   `json:"listing_type"`
	Tags                        []string `json:"tags"`
	Materials                   []string `json:"materials"`
	ShippingProfileID           int64    `json:"shipping_profile_id"`
	ReturnPolicyID              int64    `json:"return_policy_id"`
	ProcessingMin               int      `json:"processing_min"`
	ProcessingMax               int      `json:"processing_max"`
	WhoMade                     string   `json:"who_made"`
	WhenMade                    string   `json:"when_made"`
	IsSupply                    bool     `json:"is_supply"`
	ItemWeight                  float64  `json:"item_weight"`
	ItemWeightUnit              string   `json:"item_weight_unit"`
	ItemLength                  float64  `json:"item_length"`
	ItemWidth                   float64  `json:"item_width"`
	ItemHeight                  float64  `json:"item_height"`
	ItemDimensionsUnit          string   `json:"item_dimensions_unit"`
	IsPrivate                   bool     `json:"is_private"`
	Style                       []string `json:"style"`
	FileData                    string   `json:"file_data"`
	HasVariations               bool     `json:"has_variations"`
	ShouldAutoRenew             bool     `json:"should_auto_renew"`
	Language                    string   `json:"language"`
	Price                       PriceDTO `json:"price"`
	TaxonomyID                  int64    `json:"taxonomy_id"`
	ReadinessStateID            int64    `json:"readiness_state_id"`
	SuggestedTitle              string   `json:"suggested_title"`
}

// ProductListingsResp 3. 列表响应结构
type ProductListingsResp struct {
	Count   int                 `json:"count"`
	Results []ProductListingDTO `json:"results"` // 这里用 DTO 接收
}

type InventoryDTO struct {
	Products []struct {
		ProductID int64  `json:"product_id"` // Etsy Variant ID
		Sku       string `json:"sku"`
		Offerings []struct {
			OfferingID int64 `json:"offering_id"`
			Price      struct {
				Amount       int64  `json:"amount"`
				Divisor      int64  `json:"divisor"`
				CurrencyCode string `json:"currency_code"`
			} `json:"price"`
			Quantity  int  `json:"quantity"`
			IsEnabled bool `json:"is_enabled"`
		} `json:"offerings"`
		PropertyValues []struct {
			PropertyID   int64    `json:"property_id"`
			PropertyName string   `json:"property_name"`
			Values       []string `json:"values"` // 比如 ["Red"]
		} `json:"property_values"`
	} `json:"products"`

	// 这里的 int 数组其实存的是 PropertyID
	PriceOnProperty    []int64 `json:"price_on_property"`
	QuantityOnProperty []int64 `json:"quantity_on_property"`
	SkuOnProperty      []int64 `json:"sku_on_property"`
}

type ShopDTO struct {
	ShopID                         int64    `json:"shop_id"`
	UserID                         int64    `json:"user_id"`
	ShopName                       string   `json:"shop_name"`
	CreateDate                     int64    `json:"create_date"`
	CreatedTimestamp               int64    `json:"created_timestamp"`
	Title                          string   `json:"title"`
	Announcement                   string   `json:"announcement"`
	CurrencyCode                   string   `json:"currency_code"`
	IsVacation                     bool     `json:"is_vacation"`
	VacationMessage                string   `json:"vacation_message"`
	SaleMessage                    string   `json:"sale_message"`
	DigitalSaleMessage             string   `json:"digital_sale_message"`
	UpdateDate                     int64    `json:"update_date"`
	UpdatedTimestamp               int64    `json:"updated_timestamp"`
	ListingActiveCount             int      `json:"listing_active_count"`
	DigitalListingCount            int      `json:"digital_listing_count"`
	LoginName                      string   `json:"login_name"`
	AcceptsCustomRequests          bool     `json:"accepts_custom_requests"`
	PolicyWelcome                  string   `json:"policy_welcome"`
	PolicyPayment                  string   `json:"policy_payment"`
	PolicyShipping                 string   `json:"policy_shipping"`
	PolicyRefunds                  string   `json:"policy_refunds"`
	PolicyAdditional               string   `json:"policy_additional"`
	PolicySellerInfo               string   `json:"policy_seller_info"`
	PolicyUpdateDate               int64    `json:"policy_update_date"`
	PolicyHasPrivateReceiptInfo    bool     `json:"policy_has_private_receipt_info"`
	HasUnstructuredPolicies        bool     `json:"has_unstructured_policies"`
	PolicyPrivacy                  string   `json:"policy_privacy"`
	VacationAutoreply              string   `json:"vacation_autoreply"`
	URL                            string   `json:"url"`
	ImageURL760x100                string   `json:"image_url_760x100"`
	NumFavorers                    int      `json:"num_favorers"`
	Languages                      []string `json:"languages"`
	IconURLFullxfull               string   `json:"icon_url_fullxfull"`
	IsUsingStructuredPolicies      bool     `json:"is_using_structured_policies"`
	HasOnboardedStructuredPolicies bool     `json:"has_onboarded_structured_policies"`
	IncludeDisputeFormLink         bool     `json:"include_dispute_form_link"`
	IsDirectCheckoutOnboarded      bool     `json:"is_direct_checkout_onboarded"`
	IsEtsyPaymentsOnboarded        bool     `json:"is_etsy_payments_onboarded"`
	IsCalculatedEligible           bool     `json:"is_calculated_eligible"`
	IsOptedInToBuyerPromise        bool     `json:"is_opted_in_to_buyer_promise"`
	IsShopUSBased                  bool     `json:"is_shop_us_based"`
	TransactionSoldCount           int64    `json:"transaction_sold_count"`
	ShippingFromCountryISO         string   `json:"shipping_from_country_iso"`
	ShopLocationCountryISO         string   `json:"shop_location_country_iso"`
	ReviewCount                    int      `json:"review_count"`
	ReviewAverage                  float64  `json:"review_average"`
}

// EtsyUserResp Etsy 用户 API 响应
// GET /v3/application/users/me
type EtsyUserResp struct {
	UserID        int64  `json:"user_id"`
	PrimaryEmail  string `json:"primary_email"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	ImageURL75x75 string `json:"image_url_75x75"`
}

// EtsyUserShopsResp Etsy 用户店铺列表 API 响应
// GET /v3/application/users/{user_id}/shops
type EtsyUserShopsResp struct {
	Count   int            `json:"count"`
	Results []EtsyShopResp `json:"results"`
}

// ShopListResp 对应 /v3/application/users/{id}/shops 的响应
type ShopListResp struct {
	Results []ShopDTO `json:"results"`
	Count   int       `json:"count"`
}
