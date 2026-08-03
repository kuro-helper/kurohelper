package store

import (
	"sync"

	"kurohelperservice/db"
)

var (
	allowListMu           sync.RWMutex
	guildDiscordAllowList = make(map[string]struct{})
	dmDiscordAllowList    = make(map[string]struct{})
	userStoreMu           sync.RWMutex
	userStore             = make(map[string]struct{})
)

func InitAllowList() error {
	guildEntries, err := db.GetDiscordAllowListByKind(db.Dbs, "guild")
	if err != nil {
		return err
	}

	dmEntries, err := db.GetDiscordAllowListByKind(db.Dbs, "dm")
	if err != nil {
		return err
	}

	guilds := make(map[string]struct{}, len(guildEntries))
	for _, g := range guildEntries {
		guilds[g.ID] = struct{}{}
	}
	dms := make(map[string]struct{}, len(dmEntries))
	for _, d := range dmEntries {
		dms[d.ID] = struct{}{}
	}
	allowListMu.Lock()
	guildDiscordAllowList = guilds
	dmDiscordAllowList = dms
	allowListMu.Unlock()
	return nil
}

func GuildAllowed(id string) bool {
	allowListMu.RLock()
	defer allowListMu.RUnlock()
	_, ok := guildDiscordAllowList[id]
	return ok
}

func DMAllowed(id string) bool {
	allowListMu.RLock()
	defer allowListMu.RUnlock()
	_, ok := dmDiscordAllowList[id]
	return ok
}

// 把有存在的User從資料庫載入快取
//
// 目的是檢查使用者的時候不用先檢查他是否在資料庫，可以直接決定要產生User紀錄還是直接抓出資料
func InitUser() error {
	users, err := db.GetAllUsers(db.Dbs)
	if err != nil {
		return err
	}

	loaded := make(map[string]struct{}, len(users))
	for _, e := range users {
		if e.DiscordID == nil || *e.DiscordID == "" {
			continue
		}
		loaded[*e.DiscordID] = struct{}{}
	}
	userStoreMu.Lock()
	userStore = loaded
	userStoreMu.Unlock()
	return nil
}

func HasUser(discordID string) bool {
	userStoreMu.RLock()
	defer userStoreMu.RUnlock()
	_, ok := userStore[discordID]
	return ok
}

func AddUser(discordID string) {
	if discordID == "" {
		return
	}
	userStoreMu.Lock()
	userStore[discordID] = struct{}{}
	userStoreMu.Unlock()
}
