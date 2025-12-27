package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"etsy_dev_v1_202512/internal/model"
)

// ==================== UserRepository 用户仓库 ====================

// UserRepository 用户仓库接口
type UserRepository interface {
	Create(ctx context.Context, user *model.SysUser) error
	GetByID(ctx context.Context, id int64) (*model.SysUser, error)
	GetByUsername(ctx context.Context, username string) (*model.SysUser, error)
	GetByEmail(ctx context.Context, email string) (*model.SysUser, error)
	Update(ctx context.Context, user *model.SysUser) error
	UpdatePassword(ctx context.Context, id int64, hashedPassword string) error
	UpdateLastLogin(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter UserFilter) ([]model.SysUser, int64, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

// UserFilter 用户筛选条件
type UserFilter struct {
	Keyword  string
	Role     string
	Status   *int
	Page     int
	PageSize int
}

// ==================== 实现 ====================

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓库
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create 创建用户
func (r *userRepository) Create(ctx context.Context, user *model.SysUser) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetByID 根据 ID 获取用户
func (r *userRepository) GetByID(ctx context.Context, id int64) (*model.SysUser, error) {
	var user model.SysUser
	err := r.db.WithContext(ctx).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

// GetByUsername 根据用户名获取用户
func (r *userRepository) GetByUsername(ctx context.Context, username string) (*model.SysUser, error) {
	var user model.SysUser
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

// GetByEmail 根据邮箱获取用户
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.SysUser, error) {
	var user model.SysUser
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

// Update 更新用户
func (r *userRepository) Update(ctx context.Context, user *model.SysUser) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// UpdatePassword 更新密码
func (r *userRepository) UpdatePassword(ctx context.Context, id int64, hashedPassword string) error {
	return r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("id = ?", id).
		Update("password", hashedPassword).Error
}

// UpdateLastLogin 更新最后登录时间
func (r *userRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("id = ?", id).
		Update("last_login_at", gorm.Expr("NOW()")).Error
}

// Delete 删除用户（软删除）
func (r *userRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.SysUser{}, id).Error
}

// List 用户列表
func (r *userRepository) List(ctx context.Context, filter UserFilter) ([]model.SysUser, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SysUser{})

	// 关键词搜索
	if filter.Keyword != "" {
		keyword := "%" + filter.Keyword + "%"
		query = query.Where("username LIKE ? OR email LIKE ?", keyword, keyword)
	}

	// 角色筛选
	if filter.Role != "" {
		query = query.Where("role = ?", filter.Role)
	}

	// 状态筛选
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	var users []model.SysUser
	err := query.
		Order("id DESC").
		Offset(offset).
		Limit(filter.PageSize).
		Find(&users).Error

	return users, total, err
}

// ExistsByUsername 检查用户名是否存在
func (r *userRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("username = ?", username).
		Count(&count).Error
	return count > 0, err
}

// ExistsByEmail 检查邮箱是否存在
func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("email = ?", email).
		Count(&count).Error
	return count > 0, err
}
