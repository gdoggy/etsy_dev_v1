package service

import (
	"context"
	"errors"
	"etsy_dev_v1_202512/internal/api/dto"
	"time"

	"golang.org/x/crypto/bcrypt"

	"etsy_dev_v1_202512/internal/middleware"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
)

// ==================== UserService 用户服务 ====================

// UserService 用户服务
type UserService struct {
	userRepo repository.UserRepository
}

// NewUserService 创建用户服务
func NewUserService(userRepo repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

// ==================== 认证相关 ====================

// Login 用户登录
func (s *UserService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	// 查找用户
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	// 检查状态
	if user.Status != model.UserStatusActive {
		return nil, ErrUserDisabled
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// 生成 Token
	accessToken, refreshToken, err := middleware.GenerateTokenPair(user.ID, user.Username, string(user.Role))
	if err != nil {
		return nil, err
	}

	// 更新最后登录时间
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	cfg := middleware.GetJWTConfig()
	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(cfg.AccessTokenTTL),
		User:         s.toUserInfo(user),
	}, nil
}

// RefreshToken 刷新 Token
func (s *UserService) RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error) {
	// 解析 Refresh Token
	claims, err := middleware.ParseToken(req.RefreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// 验证是否为 Refresh Token
	if claims.Subject != "refresh" {
		return nil, ErrInvalidToken
	}

	// 获取用户信息（确保用户仍然有效）
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Status != model.UserStatusActive {
		return nil, ErrUserDisabled
	}

	// 生成新 Token
	accessToken, refreshToken, err := middleware.GenerateTokenPair(user.ID, user.Username, string(user.Role))
	if err != nil {
		return nil, err
	}

	cfg := middleware.GetJWTConfig()
	return &dto.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(cfg.AccessTokenTTL),
	}, nil
}

// ChangePassword 修改密码
func (s *UserService) ChangePassword(ctx context.Context, userID int64, req *dto.ChangePasswordRequest) error {
	// 获取用户
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return ErrInvalidOldPassword
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 更新密码
	return s.userRepo.UpdatePassword(ctx, userID, string(hashedPassword))
}

// GetProfile 获取当前用户信息
func (s *UserService) GetProfile(ctx context.Context, userID int64) (*dto.UserInfo, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return s.toUserInfo(user), nil
}

// ==================== 用户管理（管理员） ====================

// CreateUser 创建用户
func (s *UserService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserInfo, error) {
	// 检查用户名是否存在
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	// 检查邮箱是否存在
	if req.Email != "" {
		exists, err = s.userRepo.ExistsByEmail(ctx, req.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrEmailExists
		}
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 创建用户
	user := &model.SysUser{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Role:     model.UserRole(req.Role),
		Status:   model.UserStatusActive,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return s.toUserInfo(user), nil
}

// UpdateUser 更新用户
func (s *UserService) UpdateUser(ctx context.Context, userID int64, req *dto.UpdateUserRequest) (*dto.UserInfo, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	// 检查邮箱是否被其他用户使用
	if req.Email != "" && req.Email != user.Email {
		existing, err := s.userRepo.GetByEmail(ctx, req.Email)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != userID {
			return nil, ErrEmailExists
		}
		user.Email = req.Email
	}

	if req.Role != "" {
		user.Role = model.UserRole(req.Role)
	}

	if req.Status != nil {
		user.Status = model.UserStatus(*req.Status)
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return s.toUserInfo(user), nil
}

// ResetPassword 重置密码（管理员）
func (s *UserService) ResetPassword(ctx context.Context, userID int64, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePassword(ctx, userID, string(hashedPassword))
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(ctx context.Context, userID int64) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	// 不允许删除 admin 用户
	if user.Role == model.UserRoleAdmin {
		return ErrCannotDeleteAdmin
	}

	return s.userRepo.Delete(ctx, userID)
}

// ListUsers 用户列表
func (s *UserService) ListUsers(ctx context.Context, req *dto.UserListRequest) (*dto.UserListResponse, error) {
	users, total, err := s.userRepo.List(ctx, repository.UserFilter{
		Keyword:  req.Keyword,
		Role:     req.Role,
		Status:   req.Status,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}

	list := make([]*dto.UserInfo, len(users))
	for i, u := range users {
		list[i] = s.toUserInfo(&u)
	}

	return &dto.UserListResponse{
		List:  list,
		Total: total,
	}, nil
}

// GetUserByID 获取用户详情
func (s *UserService) GetUserByID(ctx context.Context, userID int64) (*dto.UserInfo, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return s.toUserInfo(user), nil
}

// ==================== 辅助方法 ====================

// toUserInfo 转换为 DTO
func (s *UserService) toUserInfo(user *model.SysUser) *dto.UserInfo {
	info := &dto.UserInfo{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      string(user.Role),
		Status:    int(user.Status),
		CreatedAt: user.CreatedAt,
	}
	if user.LastLoginAt != nil {
		info.LastLoginAt = *user.LastLoginAt
	}
	return info
}

// ==================== 错误定义 ====================

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserDisabled       = errors.New("用户已禁用")
	ErrInvalidToken       = errors.New("Token 无效")
	ErrUserNotFound       = errors.New("用户不存在")
	ErrInvalidOldPassword = errors.New("旧密码错误")
	ErrUsernameExists     = errors.New("用户名已存在")
	ErrEmailExists        = errors.New("邮箱已存在")
	ErrCannotDeleteAdmin  = errors.New("不能删除管理员用户")
)
