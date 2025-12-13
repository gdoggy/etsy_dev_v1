package utils

import (
	"sync"
	"time"
)

// 使用 sync.Map 保证并发安全
var (
	memoryCache sync.Map
)

// cacheItem 内部结构，包含值和过期时间
type cacheItem struct {
	value      string
	expiration int64
}

// SetCache 设置缓存
// key: state
// value: verifier:adapter_id
func SetCache(key string, value string) {
	// 默认 10 分钟过期，足够完成授权流程
	exp := time.Now().Add(10 * time.Minute).Unix()

	memoryCache.Store(key, cacheItem{
		value:      value,
		expiration: exp,
	})
}

// GetCache 获取缓存并验证是否过期
func GetCache(key string) (string, bool) {
	val, ok := memoryCache.Load(key)
	if !ok {
		return "", false
	}

	item := val.(cacheItem)

	// 检查是否过期
	if time.Now().Unix() > item.expiration {
		memoryCache.Delete(key) // 懒删除
		return "", false
	}

	return item.value, true
}

// DeleteCache 删除缓存 (用完即焚)
func DeleteCache(key string) {
	memoryCache.Delete(key)
}
