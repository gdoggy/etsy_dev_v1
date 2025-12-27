package dto

// Request DTO (前端传进来的数据)

// CreateProxyReq 创建代理
type CreateProxyReq struct {
	IP       string `json:"ip" binding:"required"`
	Port     string `json:"port" binding:"required"`
	Username string `json:"username"`
	Password string `json:"password"`
	// 协议限制校验
	Protocol string `json:"protocol" binding:"omitempty,oneof=http https socks5"`
	Region   string `json:"region" binding:"required"` // 如 "US"
}

// UpdateProxyReq 更新代理请求
type UpdateProxyReq struct {
	ID       int64  `json:"id" binding:"required"` // 更新必传 ID
	IP       string `json:"ip"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Protocol string `json:"protocol" binding:"omitempty,oneof=http https socks5"`
	Region   string `json:"region"`
	Status   int    `json:"status" binding:"omitempty,oneof=1 2 3 4"`
}

// Response DTO (返回给前端的数据)

// ProxyResp 代理列表/详情返回结构
type ProxyResp struct {
	ID            int64  `json:"id"`
	IP            string `json:"ip"`
	Port          string `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	Protocol      string `json:"protocol"`
	Region        string `json:"region"`
	Capacity      int    `json:"capacity"`
	Status        int    `json:"status"`
	FailureCount  int    `json:"failure_count"`
	LastCheckTime int64  `json:"last_check_time"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     int64  `json:"created_at"` // 建议转为时间戳返回，前端格式化

	// --- 审计信息 ---
	CreatedBy     int64  `json:"created_by"`
	CreatedByName string `json:"created_by_name"` // 需要 Service 层填充名字

	// --- 核心业务：绑定关系展示 ---
	// 界面上直观看到：这个代理下挂了多少个店，多少个号
	ShopCount      int `json:"shop_count"`
	DeveloperCount int `json:"developer_count"`

	// 具体的绑定列表 (前端展开详情时用)
	// 使用精简结构体，防止循环嵌套过深
	BoundShops      []BoundShopItem      `json:"bound_shops,omitempty"`
	BoundDevelopers []BoundDeveloperItem `json:"bound_developers,omitempty"`
}

// BoundShopItem 代理下的店铺简要信息
type BoundShopItem struct {
	ShopID     int64  `json:"shop_id"`      // ERP 内部 ID
	ShopName   string `json:"shop_name"`    // 店铺名
	EtsyShopID int64  `json:"etsy_shop_id"` // Etsy 侧 ID
	Status     string `json:"status"`       // 店铺状态
}

// BoundDeveloperItem 代理下的开发者账号简要信息
type BoundDeveloperItem struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`    // 开发者应用备注名
	APIKey string `json:"api_key"` // KeyString
}
