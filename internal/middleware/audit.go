package middleware

import (
	"context"
	"reflect"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ==================== 审计上下文 ====================

// AuditContext Key
type auditContextKey struct{}

// AuditInfo 审计信息
type AuditInfo struct {
	UserID   int64
	Username string
}

// WithAuditInfo 注入审计信息到 context
func WithAuditInfo(ctx context.Context, userID int64, username string) context.Context {
	return context.WithValue(ctx, auditContextKey{}, &AuditInfo{
		UserID:   userID,
		Username: username,
	})
}

// GetAuditInfo 从 context 获取审计信息
func GetAuditInfo(ctx context.Context) *AuditInfo {
	if info, ok := ctx.Value(auditContextKey{}).(*AuditInfo); ok {
		return info
	}
	return nil
}

// GetAuditUserID 从 context 获取审计用户 ID
func GetAuditUserID(ctx context.Context) int64 {
	if info := GetAuditInfo(ctx); info != nil {
		return info.UserID
	}
	return 0
}

// ==================== Gin 中间件 ====================

// AuditContext 审计上下文中间件
// 将 JWT 中的用户信息注入到 request context，供 GORM 回调使用
func AuditContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		username := GetUsername(c)

		if userID > 0 {
			ctx := WithAuditInfo(c.Request.Context(), userID, username)
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
	}
}

// ==================== GORM 回调 ====================

// RegisterAuditCallbacks 注册 GORM 审计回调
// 在 Create/Update 时自动填充 CreatedBy/UpdatedBy
func RegisterAuditCallbacks(db *gorm.DB) {
	// Create 回调
	db.Callback().Create().Before("gorm:create").Register("audit:create", func(tx *gorm.DB) {
		if tx.Statement.Context == nil {
			return
		}

		userID := GetAuditUserID(tx.Statement.Context)
		if userID == 0 {
			return
		}

		// 设置 CreatedBy 和 UpdatedBy
		setAuditField(tx, "CreatedBy", userID)
		setAuditField(tx, "UpdatedBy", userID)
	})

	// Update 回调
	db.Callback().Update().Before("gorm:update").Register("audit:update", func(tx *gorm.DB) {
		if tx.Statement.Context == nil {
			return
		}

		userID := GetAuditUserID(tx.Statement.Context)
		if userID == 0 {
			return
		}

		// 仅设置 UpdatedBy
		setAuditField(tx, "UpdatedBy", userID)
	})
}

// setAuditField 设置审计字段
func setAuditField(tx *gorm.DB, fieldName string, value int64) {
	if tx.Statement.Schema == nil {
		return
	}

	field := tx.Statement.Schema.LookUpField(fieldName)
	if field == nil {
		return
	}

	switch tx.Statement.ReflectValue.Kind() {
	case reflect.Struct:
		// 单个对象
		if _, isZero := field.ValueOf(tx.Statement.Context, tx.Statement.ReflectValue); isZero {
			_ = field.Set(tx.Statement.Context, tx.Statement.ReflectValue, value)
		}
	case reflect.Slice:
		// 批量插入
		for i := 0; i < tx.Statement.ReflectValue.Len(); i++ {
			rv := tx.Statement.ReflectValue.Index(i)
			if _, isZero := field.ValueOf(tx.Statement.Context, rv); isZero {
				_ = field.Set(tx.Statement.Context, rv, value)
			}
		}
	}
}
