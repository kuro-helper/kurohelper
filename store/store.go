package store

import (
	"log/slog"
	"os"

	"kurohelperservice/db"
)

var (
	GuildDiscordAllowList = make(map[string]struct{})
	DmDiscordAllowList    = make(map[string]struct{})

	UserStore = make(map[string]struct{})
)

func InitAllowList() {
	guildDiscordAllowList, err := db.GetDiscordAllowListByKind(db.Dbs, "guild")
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	dmDiscordAllowList, err := db.GetDiscordAllowListByKind(db.Dbs, "dm")
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	// 存進快取
	for _, g := range guildDiscordAllowList {
		GuildDiscordAllowList[g.ID] = struct{}{}
	}
	for _, d := range dmDiscordAllowList {
		GuildDiscordAllowList[d.ID] = struct{}{}
	}
}

// 把有存在的User從資料庫載入快取
//
// 目的是檢查使用者的時候不用先檢查他是否在資料庫，可以直接決定要產生User紀錄還是直接抓出資料
func InitUser() {
	user, err := db.GetAllUsers(db.Dbs)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	// 存進快取
	for _, e := range user {
		UserStore[e.ID] = struct{}{}
	}
}
