package cache

import (
	"sync"
	"time"

	kurohelpercore "kurohelper-core"
	"kurohelper-core/vndb"
)

// cache struct
type Cache[T any] struct {
	Value    T
	ExpireAt time.Time
}

// CacheStore 泛型快取儲存
// T 約束快取中儲存的資料型別
type CacheStore[T any] struct {
	data       map[string]*Cache[T]
	expireTime time.Duration
	mu         sync.RWMutex
}

var (
	// 一般查詢快取 (可儲存多種型別，使用 any)
	SearchCache = NewCacheStore[any](15 * time.Minute)
	// 個人資料快取
	UserInfoCache = NewCacheStore[any](10 * time.Minute)
	// 提交資料快取
	// SubmitDataCache = NewCacheStore[any](240 * time.Minute)

	// 新版查詢公司品牌快取
	SearchBrandCache = NewCacheStore[*vndb.ProducerSearchResponse](2 * time.Hour) // 二小時過期
)

// NewCacheStore 建立新的快取儲存
func NewCacheStore[T any](expireTime time.Duration) *CacheStore[T] {
	return &CacheStore[T]{
		data:       make(map[string]*Cache[T]),
		expireTime: expireTime,
	}
}

// Set 設定快取
func (c *CacheStore[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = &Cache[T]{
		Value:    value,
		ExpireAt: time.Now().Add(c.expireTime),
	}
}

// Get 取得快取
func (c *CacheStore[T]) Get(key string) (T, error) {
	c.mu.RLock()
	item, ok := c.data[key]
	c.mu.RUnlock()

	// 不存在或已過期
	if !ok || time.Now().After(item.ExpireAt) {
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		var zero T
		return zero, kurohelpercore.ErrCacheLost
	}

	return item.Value, nil
}

// Clean 清除所有快取
func (c *CacheStore[T]) Clean() (count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for k := range c.data {
		delete(c.data, k)
		count++
	}

	return
}
