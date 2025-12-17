package etsy

// CreateListingReq 创建草稿请求 DTO
type CreateListingReq struct {
	Quantity    int     `json:"quantity" binding:"required,min=1"`
	Title       string  `json:"title" binding:"required,max=140"`
	Description string  `json:"description" binding:"required"`
	Price       float64 `json:"price" binding:"required,gt=0"`

	// --- 必填枚举 ---
	WhoMade           string `json:"who_made" binding:"required"`  // i_did, collective, someone_else
	WhenMade          string `json:"when_made" binding:"required"` // made_to_order, 2020_2025...
	ShippingProfileID int64  `json:"shipping_profile_id" binding:"required"`
	// --- 核心关联 ID ---
	TaxonomyID int64 `json:"taxonomy_id" binding:"required"`

	// 图片 ID 列表 (创建 Draft 时可选，如果用户已经上传了图片则传此 ID)
	ImageIDs []int64 `json:"image_ids"`

	Type string `json:"type" binding:"required"` // physical, download

	ReadinessStateID int64 `json:"readiness_state_id"`

	Tags []string `json:"tags" binding:"max=13"`

	// 3. AI 效率工具开关
	EnableAI        bool   `json:"enable_ai"`         // 总开关
	AIKeyword       string `json:"ai_keyword"`        // AI 关键词 (如不填则取 Title)
	AIExtraPrompt   string `json:"ai_extra_prompt"`   // 用户额外要求 (如 "语气幽默")
	ReferenceImgURL string `json:"reference_img_url"` // 视觉逆向参考图 (用于生成 Prompt 或生图)
}
