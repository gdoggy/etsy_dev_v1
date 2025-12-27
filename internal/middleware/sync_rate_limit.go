package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ==================== 同步限流中间件 ====================

// SyncRateLimit 同步限流中间件
// 按店铺 + 同步类型维度进行限流
//
// 使用示例:
//
//	router.POST("/api/v1/shops/:id/sync",
//	    middleware.SyncRateLimit(middleware.SyncTypeShop, 0),
//	    controller.TriggerShopSync,
//	)
//
// 参数:
//   - syncType: 同步类型
//   - interval: 冷却间隔，0 表示使用默认值
func SyncRateLimit(syncType SyncType, interval time.Duration) gin.HandlerFunc {
	if interval == 0 {
		interval = GetInterval(syncType)
	}

	return func(c *gin.Context) {
		// 获取店铺 ID
		shopIDStr := c.Param("id")
		if shopIDStr == "" {
			shopIDStr = c.Param("shop_id")
		}
		if shopIDStr == "" {
			shopIDStr = c.Query("shop_id")
		}

		var key string
		if shopIDStr != "" {
			shopID, err := strconv.ParseInt(shopIDStr, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    400,
					"message": "无效的店铺 ID",
				})
				c.Abort()
				return
			}
			key = ShopSyncKey(shopID, syncType)
		} else {
			// 无店铺 ID，使用全局限流
			key = GlobalSyncKey(syncType)
		}

		// 检查限流
		result := GetLimiter().Check(key, interval)
		if !result.Allowed {
			retryAfter := int(result.RetryAfter.Seconds())
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": formatRetryMessage(result.RetryAfter),
				"data": gin.H{
					"retry_after": retryAfter,
					"sync_type":   syncType,
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GlobalSyncRateLimit 全局同步限流中间件
// 用于"同步所有店铺"等全局操作
func GlobalSyncRateLimit(syncType SyncType, interval time.Duration) gin.HandlerFunc {
	if interval == 0 {
		interval = GetInterval(syncType)
	}

	return func(c *gin.Context) {
		key := GlobalSyncKey(syncType)

		result := GetLimiter().Check(key, interval)
		if !result.Allowed {
			retryAfter := int(result.RetryAfter.Seconds())
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": formatRetryMessage(result.RetryAfter),
				"data": gin.H{
					"retry_after": retryAfter,
					"sync_type":   syncType,
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ==================== 辅助函数 ====================

// formatRetryMessage 格式化重试提示信息
func formatRetryMessage(d time.Duration) string {
	seconds := int(d.Seconds())

	if seconds < 60 {
		return fmt.Sprintf("同步冷却中，请 %d 秒后重试", seconds)
	}

	minutes := seconds / 60
	remainingSeconds := seconds % 60

	if remainingSeconds == 0 {
		return fmt.Sprintf("同步冷却中，请 %d 分钟后重试", minutes)
	}

	return fmt.Sprintf("同步冷却中，请 %d 分 %d 秒后重试", minutes, remainingSeconds)
}

// ==================== 手动限流检查（供 Service 层使用）====================

// CheckSyncAllowed 检查同步是否允许（不更新时间）
func CheckSyncAllowed(shopID int64, syncType SyncType) (bool, time.Duration) {
	key := ShopSyncKey(shopID, syncType)
	interval := GetInterval(syncType)
	result := GetLimiter().CheckOnly(key, interval)
	return result.Allowed, result.RetryAfter
}

// MarkSyncExecuted 标记同步已执行
func MarkSyncExecuted(shopID int64, syncType SyncType) {
	key := ShopSyncKey(shopID, syncType)
	GetLimiter().MarkExecuted(key)
}

// ResetSyncLimit 重置同步限流（管理员使用）
func ResetSyncLimit(shopID int64, syncType SyncType) {
	key := ShopSyncKey(shopID, syncType)
	GetLimiter().Reset(key)
}
