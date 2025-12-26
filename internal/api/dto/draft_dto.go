package dto

import "time"

// ==================== 请求 DTO ====================

// CreateDraftRequest 创建草稿请求
type CreateDraftRequest struct {
	UserID      int64   `json:"user_id" binding:"required"`
	SourceURL   string  `json:"source_url" binding:"required"`
	ShopIDs     []int64 `json:"shop_ids" binding:"required,min=1,max=10"`
	ImageCount  int     `json:"image_count"`                       // 1-20, 默认20
	Quantity    int     `json:"quantity" binding:"required,min=1"` // 库存数量
	StyleHint   string  `json:"style_hint"`
	ExtraPrompt string  `json:"extra_prompt"`
}

// UpdateDraftProductRequest 更新草稿商品请求
type UpdateDraftProductRequest struct {
	Title             *string  `json:"title,omitempty"`
	Description       *string  `json:"description,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	Price             *float64 `json:"price,omitempty"`
	SelectedImages    []string `json:"selected_images,omitempty"`
	Quantity          *int     `json:"quantity,omitempty"`
	TaxonomyID        *int64   `json:"taxonomy_id,omitempty"`
	ShippingProfileID *int64   `json:"shipping_profile_id,omitempty"`
}

// RegenerateImagesRequest 重新生成图片请求
type RegenerateImagesRequest struct {
	GroupIndex *int   `json:"group_index,omitempty"`
	Count      int    `json:"count"`
	StyleHint  string `json:"style_hint,omitempty"`
}

// ListDraftTasksRequest 任务列表请求
type ListDraftTasksRequest struct {
	UserID   int64  `form:"user_id"`
	Status   string `form:"status"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"page_size,default=20"`
}

// ==================== 响应 DTO ====================

// DraftTaskResponse 任务列表响应项
type DraftTaskResponse struct {
	TaskID       int64  `json:"task_id"`
	Status       string `json:"status"`
	AIStatus     string `json:"ai_status"`
	SourceURL    string `json:"source_url"`
	Platform     string `json:"platform"`
	CreatedAt    string `json:"created_at"`
	ProductCount int    `json:"product_count"`
}

// DraftDetailResponse 草稿详情响应
type DraftDetailResponse struct {
	Task          *DraftTaskVO      `json:"task"`
	SourceProduct *ScrapedProductVO `json:"source_product"`
	AIResult      *AIGenerateResult `json:"ai_result"`
	Products      []DraftProductVO  `json:"products"`
}

// DraftTaskVO 任务视图对象
type DraftTaskVO struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	AIStatus     string `json:"ai_status"`
	SourceURL    string `json:"source_url"`
	Platform     string `json:"platform"`
	ItemID       string `json:"item_id"`
	ImageCount   int    `json:"image_count"`
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// DraftProductVO 草稿商品视图对象
type DraftProductVO struct {
	ID                int64    `json:"id"`
	ShopID            int64    `json:"shop_id"`
	ShopName          string   `json:"shop_name"`
	Status            string   `json:"status"`
	SyncStatus        int      `json:"sync_status"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Tags              []string `json:"tags"`
	Price             float64  `json:"price"`
	CurrencyCode      string   `json:"currency_code"`
	Quantity          int      `json:"quantity"`
	TaxonomyID        int64    `json:"taxonomy_id"`
	ShippingProfileID int64    `json:"shipping_profile_id"`
	ReturnPolicyID    int64    `json:"return_policy_id"`
	SelectedImages    []string `json:"selected_images"`
	ListingID         int64    `json:"listing_id,omitempty"`
	SyncError         string   `json:"sync_error,omitempty"`
}

// ScrapedProductVO 抓取商品视图对象
type ScrapedProductVO struct {
	Platform    string   `json:"platform"`
	ItemID      string   `json:"item_id"`
	Title       string   `json:"title"`
	Price       float64  `json:"price"`
	Currency    string   `json:"currency"`
	Images      []string `json:"images"`
	Description string   `json:"description"`
	Attributes  []Attr   `json:"attributes,omitempty"`
}

type Attr struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AIGenerateResult AI生成结果
type AIGenerateResult struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Images      []string `json:"images"`
}

// DraftImageVO 草稿图片视图对象
type DraftImageVO struct {
	ID           int64  `json:"id"`
	GroupIndex   int    `json:"group_index"`
	ImageIndex   int    `json:"image_index"`
	StorageURL   string `json:"storage_url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Status       string `json:"status"`
}

// ==================== 进度事件 ====================

// ProgressEvent SSE进度事件
type ProgressEvent struct {
	TaskID   int64       `json:"task_id"`
	Stage    string      `json:"stage"` // fetching, generating_text, generating_images, saving, done, failed
	Progress int         `json:"progress"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data,omitempty"`
}

// ==================== 创建结果 ====================

// CreateDraftResult 创建草稿结果
type CreateDraftResult struct {
	TaskID    int64     `json:"task_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ==================== 支持的平台 ====================

// PlatformInfo 平台信息
type PlatformInfo struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	URLPatterns []string `json:"url_patterns"`
}

// SupportedPlatformsResponse 支持的平台响应
type SupportedPlatformsResponse struct {
	Platforms []PlatformInfo `json:"platforms"`
}
