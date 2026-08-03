package cache

/*
 * 保留未來擴充用的二階快取
 */

import (
	"sync"
	"time"
)

var (
	SearchGameCacheStore = NewCacheStoreV2[*InteractionCacheEntity](20 * time.Minute)
)

type InteractionCacheEntity struct {
	mu            sync.Mutex
	erogsGameList string
	erogsGame     string
	vndbGame      string
}

// SetVndbGameKey 安全地設定 VndbGame Key
func (e *InteractionCacheEntity) SetVndbGameKey(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vndbGame = key
}

// SetErogsGameList 安全地設定 ErogsGameList Key
func (e *InteractionCacheEntity) SetErogsGameList(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.erogsGameList = key
}

// SetErogsGame 安全地設定 ErogsGame Key
func (e *InteractionCacheEntity) SetErogsGame(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.erogsGame = key
}

// GetVndbGameKey 安全地取得 VndbGame Key
func (e *InteractionCacheEntity) GetVndbGameKey() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.vndbGame
}

// GetErogsGameList 安全地取得 ErogsGameList Key
func (e *InteractionCacheEntity) GetErogsGameList() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.erogsGameList
}

// GetErogsGame 安全地取得 ErogsGame Key
func (e *InteractionCacheEntity) GetErogsGame() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.erogsGame
}
