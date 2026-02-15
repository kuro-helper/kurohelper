package cache

import (
	"sync"
	"time"

	"kurohelper-core/bangumi"
	"kurohelper-core/erogs"
	"kurohelper-core/vndb"

	kurohelpercore "kurohelper-core"
)

// CID快取
//
// 對每一次查詢建立CID以及關鍵字的關聯
// 因為CID不允許過長字元，所以遇到很長的關鍵字時會直接丟錯，所以才需要這層快取
var (
	CIDStore = NewCacheStoreV2[string](time.Hour)
)

// 批評空間快取
var (
	// 使用關鍵字Base64作為鍵
	ErogsGameListStore = NewCacheStoreV2[[]erogs.GameList](2 * time.Hour)
	// 使用批評空間ID作為鍵
	ErogsGameStore = NewCacheStoreV2[*erogs.Game](2 * time.Hour)
	// 使用關鍵字Base64作為鍵
	ErogsMusicListStore = NewCacheStoreV2[[]erogs.MusicList](2 * time.Hour)
	// 使用批評空間ID作為鍵
	ErogsMusicStore = NewCacheStoreV2[*erogs.Music](2 * time.Hour)
	// 使用關鍵字Base64作為鍵
	ErogsBrandStore = NewCacheStoreV2[*erogs.Brand](2 * time.Hour)
	// 創作者列表：使用關鍵字 Base64 作為鍵
	ErogsCreatorListStore = NewCacheStoreV2[[]erogs.CreatorList](2 * time.Hour)
	// 創作者詳情：使用 "e" + 創作者 ID 作為鍵
	ErogsCreatorStore = NewCacheStoreV2[*erogs.Creator](2 * time.Hour)
)

// VNDB快取
var (
	// 使用關鍵字Base64作為鍵
	VndbGameListStore = NewCacheStoreV2[[]vndb.GetVnIDUseListResponse](2 * time.Hour)
	// 使用VNDB ID作為鍵 (遊戲詳細資料,可被遊戲搜尋和品牌搜尋共用)
	VndbGameStore = NewCacheStoreV2[*vndb.BasicResponse[vndb.GetVnUseIDResponse]](2 * time.Hour)
	// 使用關鍵字Base64作為鍵
	VndbBrandStore = NewCacheStoreV2[*vndb.ProducerSearchResponse](2 * time.Hour)
	// 角色列表：使用關鍵字 Base64 作為鍵
	VndbCharacterListStore = NewCacheStoreV2[[]vndb.CharacterSearchResponse](2 * time.Hour)
	// 角色詳情：使用 VNDB 角色 ID（如 c123）作為鍵
	VndbCharacterStore = NewCacheStoreV2[*vndb.CharacterSearchResponse](2 * time.Hour)
)

// Bangumi快取(因為沒有任何CID事件，所以直接拿搜尋關鍵字做 base64 對應實際資料)
var (
	BangumiCharacterStore = NewCacheStoreV2[*bangumi.Character](2 * time.Hour)
)

// 月幕快取
// var (
// 	YmgalGame = NewCacheStoreV2[*ymgal.SearchGameResp](time.Hour)
// )

// cache struct
type CacheV2[T any] struct {
	Value    T
	ExpireAt time.Time
}

// CacheStoreV2 泛型快取儲存
// data的鍵值為UUID
type CacheStoreV2[T any] struct {
	data       map[string]*CacheV2[T]
	expireTime time.Duration
	mu         sync.RWMutex
}

// NewCacheStore 建立新的快取儲存
func NewCacheStoreV2[T any](expireTime time.Duration) *CacheStoreV2[T] {
	return &CacheStoreV2[T]{
		data:       make(map[string]*CacheV2[T]),
		expireTime: expireTime,
	}
}

// Set 設定快取
func (c *CacheStoreV2[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = &CacheV2[T]{
		Value:    value,
		ExpireAt: time.Now().Add(c.expireTime),
	}
}

// Get 從快取中取得資料
func (c *CacheStoreV2[T]) Get(key string) (T, error) {
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

// Clean 清除過期快取
func (c *CacheStoreV2[T]) Clean() (deleteCount int, total int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	total = len(c.data)
	for k, d := range c.data {
		if time.Now().After(d.ExpireAt) {
			delete(c.data, k)
			deleteCount++
		}
	}

	return
}

// CleanAll 清除所有快取
func (c *CacheStoreV2[T]) CleanAll() (count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for k := range c.data {
		delete(c.data, k)
		count++
	}

	return
}
