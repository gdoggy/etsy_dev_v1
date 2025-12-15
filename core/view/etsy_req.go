package view

// CreateListingReq 创建草稿请求 DTO
type CreateListingReq struct {
	Title       string  `json:"title" binding:"required,max=140"`
	Description string  `json:"description" binding:"required"`
	Quantity    int     `json:"quantity" binding:"required,min=1"`
	Price       float64 `json:"price" binding:"required,gt=0"`

	// --- 核心关联 ID ---
	TaxonomyID        int64 `json:"taxonomy_id" binding:"required"`
	ShippingProfileID int64 `json:"shipping_profile_id" binding:"required"`

	// --- 必填枚举 ---
	WhoMade  string `json:"who_made" binding:"required"`  // i_did, collective, someone_else
	WhenMade string `json:"when_made" binding:"required"` // made_to_order, 2020_2025...
	Type     string `json:"type" binding:"required"`      // physical, download

	// 1. 图片 ID 列表 (创建 Draft 时可选，如果用户已经上传了图片则传此 ID)
	ImageIDs []int64 `json:"image_ids"`

	// 2. 只有实物才需要，但现在的 API 确实要求传这个或 when_made，
	// 为了保险，我们可以让前端传，或者后端根据 when_made 自动处理。
	// 这里先加上，让前端传更灵活。
	ReadinessStateID int64 `json:"readiness_state_id"`

	Tags []string `json:"tags" binding:"max=13"`
}
