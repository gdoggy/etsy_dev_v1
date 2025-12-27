package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ==================== JWT 配置 ====================

// JWTConfig JWT 配置
type JWTConfig struct {
	SecretKey       string        // 签名密钥
	AccessTokenTTL  time.Duration // Access Token 有效期
	RefreshTokenTTL time.Duration // Refresh Token 有效期
	Issuer          string        // 签发者
}

// DefaultJWTConfig 默认配置
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		SecretKey:       "etsy-erp-secret-key-change-in-production",
		AccessTokenTTL:  2 * time.Hour,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "etsy-erp",
	}
}

// 全局配置
var jwtConfig = DefaultJWTConfig()

// SetJWTConfig 设置 JWT 配置
func SetJWTConfig(cfg *JWTConfig) {
	jwtConfig = cfg
}

// GetJWTConfig 获取 JWT 配置
func GetJWTConfig() *JWTConfig {
	return jwtConfig
}

// ==================== Claims 定义 ====================

// UserClaims 用户声明
type UserClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// ==================== Token 生成 ====================

// GenerateAccessToken 生成 Access Token
func GenerateAccessToken(userID int64, username, role string) (string, error) {
	now := time.Now()
	claims := &UserClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtConfig.Issuer,
			Subject:   "access",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(jwtConfig.AccessTokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtConfig.SecretKey))
}

// GenerateRefreshToken 生成 Refresh Token
func GenerateRefreshToken(userID int64, username, role string) (string, error) {
	now := time.Now()
	claims := &UserClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtConfig.Issuer,
			Subject:   "refresh",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(jwtConfig.RefreshTokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtConfig.SecretKey))
}

// GenerateTokenPair 生成 Token 对
func GenerateTokenPair(userID int64, username, role string) (accessToken, refreshToken string, err error) {
	accessToken, err = GenerateAccessToken(userID, username, role)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = GenerateRefreshToken(userID, username, role)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ==================== Token 解析 ====================

// ParseToken 解析 Token
func ParseToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(jwtConfig.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ==================== Gin 中间件 ====================

// Context Keys
const (
	ContextKeyUserID   = "user_id"
	ContextKeyUsername = "username"
	ContextKeyRole     = "role"
	ContextKeyClaims   = "claims"
)

// JWTAuth JWT 认证中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取 Authorization Header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未提供认证信息",
			})
			c.Abort()
			return
		}

		// 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "认证格式错误，应为 Bearer {token}",
			})
			c.Abort()
			return
		}

		// 解析 Token
		claims, err := ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Token 无效或已过期",
			})
			c.Abort()
			return
		}

		// 检查是否为 Access Token
		if claims.Subject != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Token 类型错误",
			})
			c.Abort()
			return
		}

		// 注入用户信息到 Context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRole, claims.Role)
		c.Set(ContextKeyClaims, claims)

		c.Next()
	}
}

// RequireRole 角色权限校验中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(ContextKeyRole)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未获取到用户角色",
			})
			c.Abort()
			return
		}

		userRole := role.(string)
		for _, r := range roles {
			if userRole == r {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权限访问",
		})
		c.Abort()
	}
}

// OptionalAuth 可选认证中间件（不强制登录）
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := ParseToken(parts[1])
		if err != nil || claims.Subject != "access" {
			c.Next()
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRole, claims.Role)
		c.Set(ContextKeyClaims, claims)

		c.Next()
	}
}

// ==================== 辅助函数 ====================

// GetUserID 从 Context 获取用户 ID
func GetUserID(c *gin.Context) int64 {
	if id, exists := c.Get(ContextKeyUserID); exists {
		return id.(int64)
	}
	return 0
}

// GetUsername 从 Context 获取用户名
func GetUsername(c *gin.Context) string {
	if name, exists := c.Get(ContextKeyUsername); exists {
		return name.(string)
	}
	return ""
}

// GetUserRole 从 Context 获取用户角色
func GetUserRole(c *gin.Context) string {
	if role, exists := c.Get(ContextKeyRole); exists {
		return role.(string)
	}
	return ""
}

// GetUserClaims 从 Context 获取完整 Claims
func GetUserClaims(c *gin.Context) *UserClaims {
	if claims, exists := c.Get(ContextKeyClaims); exists {
		return claims.(*UserClaims)
	}
	return nil
}
