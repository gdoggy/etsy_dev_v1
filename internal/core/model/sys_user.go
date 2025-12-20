package model

// SysUser 系统用户/合作商账号
type SysUser struct {
	BaseModel
	AuditMixin
	// 基础信息
	Username string `gorm:"size:100;uniqueIndex;not null"`
	Password string `gorm:"size:255;not null"` // 哈希密码
	Email    string `gorm:"size:100"`

	// 系统级角色: super_admin (超管), partner (合作商), user (普通员工)
	// 注意区分：这是系统的角色，ShopMember 里的是店铺内的角色
	Role string `gorm:"size:20;default:'user'"`

	IsActive bool `gorm:"default:true"`

	// ==============================
	// 关联关系
	// ==============================

	// 方式 A: 快速查询用户拥有的店铺 (忽略角色)
	// 方便：user.Shops 直接拿到列表
	Shops []Shop `gorm:"many2many:shop_members;"`

	// 方式 B: 查询用户在店铺的权限详情 (包含 Role)
	// 严谨：user.Memberships 拿到 ShopID + Role
	Memberships []ShopMember `gorm:"foreignKey:SysUserID"`
}

func (SysUser) TableName() string {
	return "sys_users"
}

// AuditMixin 审计字段 (只记录，不参与 WHERE 查询权限)
type AuditMixin struct {
	BaseModel
	CreatedBy int64 `gorm:"index"`     // 创建人的 SysUserID
	UpdatedBy int64 `gorm:"index"`     // 最后修改人的 SysUserID
	DeletedBy int64 `gorm:"default:0"` // 删除人的 SysUserID
}

func (AuditMixin) TableName() string { return "audit_mixins" }
