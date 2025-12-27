package model

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// UserRole 用户角色
type UserRole string

const (
	UserRoleAdmin    UserRole = "admin"    // 管理员
	UserRoleOperator UserRole = "operator" // 操作员
	UserRoleViewer   UserRole = "viewer"   // 只读用户
)

// UserStatus 用户状态
type UserStatus int

const (
	UserStatusActive   UserStatus = 1 // 启用
	UserStatusDisabled UserStatus = 0 // 禁用
)

// SysUser 系统用户
type SysUser struct {
	BaseModel

	Username    string     `gorm:"size:50;uniqueIndex;not null;comment:用户名" json:"username"`
	Password    string     `gorm:"size:100;not null;comment:密码哈希" json:"-"`
	Nickname    string     `gorm:"size:50;comment:昵称" json:"nickname"`
	Email       string     `gorm:"size:100;index;comment:邮箱" json:"email"`
	Phone       string     `gorm:"size:20;comment:手机号" json:"phone"`
	Avatar      string     `gorm:"size:255;comment:头像URL" json:"avatar"`
	Role        UserRole   `gorm:"size:20;default:operator;comment:角色" json:"role"`
	Status      UserStatus `gorm:"default:1;comment:状态 1启用 0禁用" json:"status"`
	LastLoginAt *time.Time `gorm:"comment:最后登录时间" json:"last_login_at"`
	LastLoginIP string     `gorm:"size:50;comment:最后登录IP" json:"last_login_ip"`

	// 关联 - 用户管理的店铺
	ShopMembers []ShopMember `gorm:"foreignKey:UserID"`       // Has Many
	Shops       []Shop       `gorm:"many2many:shop_members;"` // Many to Many
}

func (*SysUser) TableName() string {
	return "sys_users"
}

// SetPassword 设置密码（加密）
func (u *SysUser) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hash)
	return nil
}

// CheckPassword 校验密码
func (u *SysUser) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// IsAdmin 是否管理员
func (u *SysUser) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

// IsActive 是否启用
func (u *SysUser) IsActive() bool {
	return u.Status == UserStatusActive
}

// ShopMember 店铺成员（用户-店铺关联）
type ShopMember struct {
	BaseModel

	UserID int64    `gorm:"index:idx_user_shop,unique;not null;comment:用户ID" json:"user_id"`
	User   *SysUser `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ShopID int64    `gorm:"index:idx_user_shop,unique;not null;comment:店铺ID" json:"shop_id"`
	Shop   *Shop    `gorm:"foreignKey:ShopID" json:"shop,omitempty"`
	Role   string   `gorm:"size:20;default:viewer;comment:店铺角色 owner/manager/viewer" json:"role"`
}

func (ShopMember) TableName() string {
	return "shop_members"
}

// ShopMemberRole 店铺成员角色
const (
	ShopMemberRoleOwner   = "owner"   // 店铺所有者
	ShopMemberRoleManager = "manager" // 店铺管理员
	ShopMemberRoleViewer  = "viewer"  // 只读
)
