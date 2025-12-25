package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// ==================== 状态常量 ====================

const (
	// 任务状态
	TaskStatusPending    = "pending"
	TaskStatusProcessing = "processing"
	TaskStatusDraft      = "draft"
	TaskStatusConfirmed  = "confirmed"
	TaskStatusExpired    = "expired"
	TaskStatusFailed     = "failed"

	// AI 状态
	AIStatusPending    = "pending"
	AIStatusProcessing = "processing"
	AIStatusDone       = "done"
	AIStatusFailed     = "failed"

	// 草稿商品状态
	DraftStatusDraft     = "draft"
	DraftStatusConfirmed = "confirmed"
	DraftStatusSubmitted = "submitted"
	DraftStatusFailed    = "failed"

	// 同步状态
	DraftSyncStatusNone    = 0 // 未同步
	DraftSyncStatusPending = 1 // 待同步
	DraftSyncStatusDone    = 2 // 已同步
	DraftSyncStatusFailed  = 3 // 同步失败

	// 图片状态
	ImageStatusPending = "pending"
	ImageStatusReady   = "ready"
	ImageStatusFailed  = "failed"
)

// ==================== JSON 类型 ====================

// StringSlice 字符串切片（JSON 存储）
type StringSlice []string

func (s *StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	return json.Marshal(s)
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, s)
}

// JSONMap JSON对象（map 存储）
type JSONMap map[string]interface{}

func (m *JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	return json.Marshal(m)
}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(map[string]interface{})
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, m)
}

// ==================== 数据库模型 ====================

// DraftTask 草稿任务
type DraftTask struct {
	ID             int64          `gorm:"primaryKey;autoIncrement"`
	CreatedAt      time.Time      `gorm:"index"`
	UpdatedAt      time.Time      `gorm:"index"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
	UserID         int64          `gorm:"index;not null;comment:用户ID"`
	SourceURL      string         `gorm:"size:2048;not null;comment:源商品URL"`
	SourcePlatform string         `gorm:"size:32;index;comment:来源平台"`
	SourceItemID   string         `gorm:"size:64;index;comment:来源商品ID"`
	SourceData     JSONMap        `gorm:"type:json;comment:抓取的原始数据"`
	ImageCount     int            `gorm:"default:20;comment:生成图片数量"`
	StyleHint      string         `gorm:"size:255;comment:风格提示"`
	ExtraPrompt    string         `gorm:"type:text;comment:额外提示词"`
	Status         string         `gorm:"size:32;index;default:pending;comment:任务状态"`
	AIStatus       string         `gorm:"size:32;index;default:pending;comment:AI处理状态"`
	AITextResult   JSONMap        `gorm:"type:json;comment:AI文案结果"`
	AIImages       StringSlice    `gorm:"type:json;comment:AI生成图片URL"`
	AIErrorMessage string         `gorm:"size:1024;comment:AI处理错误信息"`
}

func (*DraftTask) TableName() string {
	return "draft_tasks"
}

// DraftProduct 草稿商品
type DraftProduct struct {
	ID                int64          `gorm:"primaryKey;autoIncrement"`
	CreatedAt         time.Time      `gorm:"index"`
	UpdatedAt         time.Time      `gorm:"index"`
	DeletedAt         gorm.DeletedAt `gorm:"index"`
	TaskID            int64          `gorm:"index;not null;comment:任务ID"`
	ShopID            int64          `gorm:"index;not null;comment:店铺ID"`
	Title             string         `gorm:"size:140;comment:商品标题"`
	Description       string         `gorm:"type:text;comment:商品描述"`
	Tags              StringSlice    `gorm:"type:json;comment:标签"`
	PriceAmount       int64          `gorm:"comment:价格(分)"`
	PriceDivisor      int64          `gorm:"default:100;comment:价格除数"`
	CurrencyCode      string         `gorm:"size:3;default:USD;comment:货币代码"`
	Quantity          int            `gorm:"default:1;comment:库存数量"`
	TaxonomyID        int64          `gorm:"comment:Etsy分类ID"`
	ShippingProfileID int64          `gorm:"comment:运费模板ID"`
	ReturnPolicyID    int64          `gorm:"comment:退货政策ID"`
	SelectedImages    StringSlice    `gorm:"type:json;comment:选中的图片URL"`
	Status            string         `gorm:"size:32;index;default:draft;comment:状态"`
	SyncStatus        int            `gorm:"default:0;index;comment:同步状态"`
	SyncError         string         `gorm:"size:1024;comment:同步错误信息"`
	ProductID         int64          `gorm:"index;comment:同步后的产品ID"`
	ListingID         int64          `gorm:"index;comment:Etsy listing ID"`

	// 关联
	Task *DraftTask `gorm:"foreignKey:TaskID"`
}

func (*DraftProduct) TableName() string {
	return "draft_products"
}

// DraftImage 草稿图片
type DraftImage struct {
	ID           int64          `gorm:"primaryKey;autoIncrement"`
	CreatedAt    time.Time      `gorm:"index"`
	UpdatedAt    time.Time      `gorm:"index"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	TaskID       int64          `gorm:"index;not null;comment:任务ID"`
	GroupIndex   int            `gorm:"index;comment:分组索引"`
	ImageIndex   int            `gorm:"comment:组内索引"`
	Prompt       string         `gorm:"type:text;comment:生成提示词"`
	StorageURL   string         `gorm:"size:2048;comment:存储URL"`
	ThumbnailURL string         `gorm:"size:2048;comment:缩略图URL"`
	Width        int            `gorm:"comment:图片宽度"`
	Height       int            `gorm:"comment:图片高度"`
	Status       string         `gorm:"size:32;default:pending;comment:状态"`
	ErrorMessage string         `gorm:"size:1024;comment:错误信息"`

	// 关联
	Task *DraftTask `gorm:"foreignKey:TaskID"`
}

func (*DraftImage) TableName() string {
	return "draft_images"
}

// ==================== 辅助方法 ====================

// GetPrice 获取价格（浮点数）
func (p *DraftProduct) GetPrice() float64 {
	if p.PriceDivisor == 0 {
		p.PriceDivisor = 100
	}
	return float64(p.PriceAmount) / float64(p.PriceDivisor)
}

// SetPrice 设置价格（浮点数）
func (p *DraftProduct) SetPrice(price float64) {
	p.PriceDivisor = 100
	p.PriceAmount = int64(price * 100)
}

// CanConfirm 检查是否可以确认
func (p *DraftProduct) CanConfirm() error {
	if p.Status != DraftStatusDraft {
		return errors.New("当前状态不允许确认")
	}
	if p.Title == "" {
		return errors.New("标题不能为空")
	}
	if p.TaxonomyID == 0 {
		return errors.New("请选择商品分类")
	}
	if p.ShippingProfileID == 0 {
		return errors.New("请选择运费模板")
	}
	return nil
}

// MarkConfirmed 标记为已确认
func (p *DraftProduct) MarkConfirmed() {
	p.Status = DraftStatusConfirmed
	p.SyncStatus = DraftSyncStatusPending
}

// MarkSyncSuccess 标记同步成功
func (p *DraftProduct) MarkSyncSuccess(listingID int64) {
	p.Status = DraftStatusSubmitted
	p.SyncStatus = DraftSyncStatusDone
	p.ListingID = listingID
	p.SyncError = ""
}

// MarkSyncFailed 标记同步失败
func (p *DraftProduct) MarkSyncFailed(err string) {
	p.SyncStatus = DraftSyncStatusFailed
	p.SyncError = err
}
