package dto

import "time"

// Request DTO (前端传进来的数据)

// CreateDeveloperReq 创建开发者账号请求
type CreateDeveloperReq struct {
	// 基础信息
	Name       string `json:"name" binding:"omitempty,max=50"`       // 备注名称
	LoginEmail string `json:"login_email" binding:"omitempty,email"` // 登录邮箱
	LoginPwd   string `json:"login_pwd" binding:"omitempty"`         // 登录密码

	// API 凭证
	ApiKey       string `json:"api_key" binding:"required"`       // Keystring
	SharedSecret string `json:"shared_secret" binding:"required"` // Shared Secret
}

// UpdateDeveloperReq 更新开发者账号请求 (仅允许修改备注与密码/密钥)
type UpdateDeveloperReq struct {
	Name       string `json:"name" binding:"omitempty,max=50"`       // 备注名称
	LoginEmail string `json:"login_email" binding:"omitempty,email"` // 登录邮箱
	LoginPwd   string `json:"login_pwd" binding:"omitempty"`         // 登录密码
	// API 凭证
	ApiKey       string `json:"api_key" binding:"omitempty"`       // Keystring
	SharedSecret string `json:"shared_secret" binding:"omitempty"` // Shared Secret
}

// UpdateDevStatusReq 状态变更请求 (如手动停用)
type UpdateDevStatusReq struct {
	Status int `json:"status" binding:"oneof=0 1 2"` // 0:Init, 1:Active, 2:Banned
}

// DeveloperResp 列表与详情通用响应
type DeveloperResp struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`

	// 基础展示
	Name       string `json:"name"`
	LoginEmail string `json:"login_email"`
	// 注意：Login Pwd 绝不返回

	// 业务核心
	ApiKey string `json:"api_key"`
	// SharedSecret 通常在列表页脱敏显示，详情页可能需要单独接口查看，此处视 ERP 内部安全级别而定
	SharedSecret string `json:"shared_secret"`

	// 防关联生成结果 (前端需要复制此链接到 Etsy)
	CallbackURL string `json:"callback_url"`

	// 状态 (前端根据此字段显示 待配置/正常/封禁)
	Status     int    `json:"status"`
	StatusText string `json:"status_text"` // 可选：后端处理好文本直接给前端
}
