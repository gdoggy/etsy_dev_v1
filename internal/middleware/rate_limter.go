package middleware

import (
	"fmt"
	"sync"
	"time"
)

// ==================== SyncRateLimiter 同步限流器 ====================

// SyncRateLimiter 同步任务限流器
// 防止用户频繁触发手动同步导致 Etsy API 限流
type SyncRateLimiter struct {
	locks sync.Map // key -> *lockEntry
}

// lockEntry 锁条目
type lockEntry struct {
	lastTime time.Time
	mu       sync.Mutex
}

// 全局限流器实例
var globalLimiter = &SyncRateLimiter{}

// GetLimiter 获取全局限流器
func GetLimiter() *SyncRateLimiter {
	return globalLimiter
}

// ==================== 限流检查 ====================

// CheckResult 检查结果
type CheckResult struct {
	Allowed    bool          // 是否允许
	RetryAfter time.Duration // 剩余冷却时间
}

// Check 检查是否允许执行
// key: 限流键，如 "shop:123:order_sync"
// interval: 冷却间隔
func (r *SyncRateLimiter) Check(key string, interval time.Duration) CheckResult {
	// 获取或创建锁条目
	actual, _ := r.locks.LoadOrStore(key, &lockEntry{})
	entry := actual.(*lockEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(entry.lastTime)

	if elapsed < interval {
		return CheckResult{
			Allowed:    false,
			RetryAfter: interval - elapsed,
		}
	}

	// 更新最后执行时间
	entry.lastTime = now
	return CheckResult{
		Allowed:    true,
		RetryAfter: 0,
	}
}

// CheckOnly 仅检查，不更新时间
func (r *SyncRateLimiter) CheckOnly(key string, interval time.Duration) CheckResult {
	actual, ok := r.locks.Load(key)
	if !ok {
		return CheckResult{Allowed: true}
	}

	entry := actual.(*lockEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	elapsed := time.Since(entry.lastTime)
	if elapsed < interval {
		return CheckResult{
			Allowed:    false,
			RetryAfter: interval - elapsed,
		}
	}

	return CheckResult{Allowed: true}
}

// MarkExecuted 标记已执行（用于异步任务完成后标记）
func (r *SyncRateLimiter) MarkExecuted(key string) {
	actual, _ := r.locks.LoadOrStore(key, &lockEntry{})
	entry := actual.(*lockEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.lastTime = time.Now()
}

// Reset 重置指定 key 的限流
func (r *SyncRateLimiter) Reset(key string) {
	r.locks.Delete(key)
}

// ==================== Key 生成工具 ====================

// SyncType 同步类型
type SyncType string

const (
	SyncTypeShop     SyncType = "shop"
	SyncTypeProduct  SyncType = "product"
	SyncTypeOrder    SyncType = "order"
	SyncTypeTracking SyncType = "tracking"
)

// ShopSyncKey 生成店铺级同步 Key
func ShopSyncKey(shopID int64, syncType SyncType) string {
	return fmt.Sprintf("shop:%d:%s", shopID, syncType)
}

// GlobalSyncKey 生成全局同步 Key
func GlobalSyncKey(syncType SyncType) string {
	return fmt.Sprintf("global:%s", syncType)
}

// ==================== 默认限流间隔 ====================

// DefaultIntervals 默认限流间隔配置
var DefaultIntervals = map[SyncType]time.Duration{
	SyncTypeShop:     10 * time.Minute, // 店铺同步：5 分钟
	SyncTypeProduct:  10 * time.Minute, // 商品同步：5 分钟
	SyncTypeOrder:    5 * time.Minute,  // 订单同步：3 分钟
	SyncTypeTracking: 3 * time.Minute,  // 物流同步：2 分钟
}

// GetInterval 获取同步类型的默认间隔
func GetInterval(syncType SyncType) time.Duration {
	if interval, ok := DefaultIntervals[syncType]; ok {
		return interval
	}
	return 5 * time.Minute // 默认 5 分钟
}
