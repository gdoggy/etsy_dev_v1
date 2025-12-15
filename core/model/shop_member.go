package model

import (
	"time"
)

// ShopMember 定义用户和店铺的关联关系及权限
// 这是一个中间表 (Join Table)，但也包含额外的权限字段
type ShopMember struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	// 联合唯一索引：一个用户在一个店铺里只能有一种角色
	SysUserID uint `gorm:"index;uniqueIndex:idx_user_shop"`
	ShopID    uint `gorm:"index;uniqueIndex:idx_user_shop"`

	// --- 权限控制 ---
	// 角色: owner (所有者), manager (管理员), editor (编辑), viewer (只读)
	Role string `gorm:"size:20;default:'viewer'"`

	// 细粒度权限 (可选，如果 Role 不够用的话)
	// CanPublish bool `gorm:"default:false"`
	// CanDelete  bool `gorm:"default:false"`

	// 关联对象 (方便查询)
	SysUser SysUser `gorm:"foreignKey:SysUserID"`
	Shop    Shop    `gorm:"foreignKey:ShopID"`
}
