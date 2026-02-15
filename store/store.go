package store

import (
	kurohelperdb "kurohelper-db"

	"github.com/sirupsen/logrus"
)

var (
	GuildDiscordAllowList = make(map[string]struct{})
	DmDiscordAllowList    = make(map[string]struct{})

	UserStore = make(map[string]struct{})
)

func InitAllowList() {
	guildDiscordAllowList, err := kurohelperdb.GetDiscordAllowListByKind("guild")
	if err != nil {
		logrus.Fatal(err)
	}

	dmDiscordAllowList, err := kurohelperdb.GetDiscordAllowListByKind("dm")
	if err != nil {
		logrus.Fatal(err)
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
	user, err := kurohelperdb.GetUsers()
	if err != nil {
		logrus.Fatal(err)
	}

	// 存進快取
	for _, e := range user {
		UserStore[e.ID] = struct{}{}
	}
}
