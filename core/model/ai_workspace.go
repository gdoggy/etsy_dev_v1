package model

import "gorm.io/gorm"

// 任务状态枚举
const (
	TaskStatusPending    = "pending"    // 队列中
	TaskStatusProcessing = "processing" // AI 生成中
	TaskStatusPartial    = "partial"    // 生成了部分（例如文案好了，图还没好）
	TaskStatusFinished   = "finished"   // 全部生成完毕，等待用户审核
	TaskStatusFailed     = "failed"     // 任务彻底失败
	TaskStatusCompleted  = "completed"  // 用户已确认并转为正式商品
)

// 资源类型枚举
const (
	CandidateTypeText  = "text_set" // 文案套件 (Title + Desc + Tags)
	CandidateTypeImage = "image"    // 图片
	CandidateTypeVideo = "video"    // 视频
)

// AIGenTask 一次 AI 生成任务会话 (父表)
type AIGenTask struct {
	gorm.Model

	// --- 归属 ---
	ShopID uint `gorm:"index;not null" json:"shop_id"`
	UserID uint `gorm:"index;not null" json:"user_id"` // 记录是谁发起的任务

	// --- 输入快照 (保留现场) ---
	Keyword      string `gorm:"size:255" json:"keyword"`
	ExtraPrompt  string `gorm:"type:text" json:"extra_prompt"`  // 用户的额外要求
	InputImgPath string `gorm:"size:500" json:"input_img_path"` // 用户上传的白底图路径
	InputImgURL  string `gorm:"size:500" json:"input_img_url"`  // 对应的公网 URL (传给 AI 用)

	// --- 任务控制 ---
	// 期望生成的数量 (用于断点续传/补货)
	TargetImageCount int `gorm:"default:10" json:"target_image_count"`
	TargetTextCount  int `gorm:"default:2" json:"target_text_count"`
	TargetVideoCount int `gorm:"default:1" json:"target_video_count"`

	// --- 状态 ---
	Status       string `gorm:"size:20;index;default:'pending'" json:"status"`
	ErrorMessage string `gorm:"type:text" json:"error_message"` // 记录 AI 报错信息
}

// AIGenCandidate AI 生成的候选资源 (子表)
// 存放生成的 2套文案、10张图、1个视频
type AIGenCandidate struct {
	gorm.Model

	// --- 归属 (级联删除) ---
	TaskID uint      `gorm:"index;not null" json:"task_id"`
	Task   AIGenTask `gorm:"foreignKey:TaskID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`

	// --- 内容 ---
	Type string `gorm:"size:20;index" json:"type"` // image, video, text_set

	// 如果是 image/video，这里存 URL
	// 如果是 text_set，这里存 JSON 字符串 {"title": "...", "desc": "...", "tags": [...]}
	Content string `gorm:"type:text" json:"content"`

	// 缩略图 (如果是视频，存封面图；如果是图片，存缩略图，提升前端加载速度)
	ThumbnailUrl string `gorm:"size:500" json:"thumbnail_url"`

	// --- 审核状态 ---
	// pending (默认), selected (用户选中), rejected (用户废弃)
	SelectionStatus string `gorm:"size:20;index;default:'pending'" json:"selection_status"`

	// 排序 (用户可能想调整图片的顺序，比如把某张图设为主图)
	SortOrder int `gorm:"default:0" json:"sort_order"`
}
