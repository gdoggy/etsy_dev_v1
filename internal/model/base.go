package model

import (
	"time"

	"gorm.io/gorm"
)

type BaseModel struct {
	ID        int64          `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// --- 审计字段 ---
	CreatedBy int64 `gorm:"comment:创建人ID" json:"created_by"`
	UpdatedBy int64 `gorm:"comment:更新人ID" json:"updated_by"`
}
