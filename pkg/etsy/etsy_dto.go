package etsy

// ==========================================
// DTO: 用于接收 Etsy API 返回的原始 JSON 数据
// ==========================================

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
