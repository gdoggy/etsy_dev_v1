package model

import "gorm.io/gorm"

// SysUser 系统用户/合作商账号
type SysUser struct {
	gorm.Model
	Username string `gorm:"size:100;uniqueIndex;not null"`
	Password string `gorm:"size:255;not null"` // 存哈希后的密码
	Email    string `gorm:"size:100"`
	Role     string `gorm:"size:20;default:'user'"` // super_admin, partner, user
	IsActive bool   `gorm:"default:true"`

	// 一个用户可以是多个店铺的成员
	// 这里通过中间表关联
	Shops []Shop `gorm:"many2many:shop_members;"`
}
