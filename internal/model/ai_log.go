package model

// AICallLog AI调用日志
type AICallLog struct {
	BaseModel

	// 关联
	ShopID int64 `gorm:"index;comment:店铺ID"`
	TaskID int64 `gorm:"index;comment:草稿任务ID"`

	// 调用信息
	CallType  string `gorm:"size:32;index;comment:调用类型(text/image)"`
	ModelName string `gorm:"size:64;comment:模型名称"`

	// 用量统计
	InputTokens  int `gorm:"default:0;comment:输入token数"`
	OutputTokens int `gorm:"default:0;comment:输出token数"`
	ImageCount   int `gorm:"default:0;comment:生成图片数量"`

	// 性能与成本
	DurationMs int64   `gorm:"comment:耗时(毫秒)"`
	CostUSD    float64 `gorm:"type:decimal(10,6);default:0;comment:成本(美元)"`

	// 状态
	Status   string `gorm:"size:32;index;default:success;comment:状态(success/failed)"`
	ErrorMsg string `gorm:"size:1024;comment:错误信息"`
}

func (AICallLog) TableName() string {
	return "ai_call_logs"
}

// ==================== 调用类型常量 ====================

const (
	AICallTypeText  = "text"
	AICallTypeImage = "image"
)

// ==================== 状态常量 ====================

const (
	AICallStatusSuccess = "success"
	AICallStatusFailed  = "failed"
)
