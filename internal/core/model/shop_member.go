package model

// ShopMember 定义用户和店铺的关联关系及权限
// GORM 自定义连接表 (Join Table)
type ShopMember struct {
	BaseModel
	// 联合唯一索引
	// 确保一个用户在一个店铺里只有一条记录
	SysUserID int64 `gorm:"index;uniqueIndex:idx_user_shop;not null"`
	ShopID    int64 `gorm:"index;uniqueIndex:idx_user_shop;not null"`

	// 权限控制
	// 角色: owner, manager, editor, viewer
	Role string `gorm:"size:20;default:'viewer'"`

	// 关联对象 (Belongs To)
	SysUser *SysUser `gorm:"foreignKey:SysUserID"`
	Shop    *Shop    `gorm:"foreignKey:ShopID"`
}

func (ShopMember) TableName() string {
	return "shop_members"
}
