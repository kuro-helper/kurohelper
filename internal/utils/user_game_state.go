package utils

import (
	"errors"
	"strings"

	"kurohelper/internal/store"
	kurohelperdb "kurohelperservice/db"

	"gorm.io/gorm"
)

// LoadGameStateMaps 取得使用者的遊玩狀態與願望清單，並以 GameErogsID 建立索引
func LoadGameStateMaps(discordID string) (statusMap map[int]kurohelperdb.UserGameStatus, inWishMap map[int]struct{}, err error) {
	statusMap = make(map[int]kurohelperdb.UserGameStatus)
	inWishMap = make(map[int]struct{})

	if strings.TrimSpace(discordID) == "" {
		return statusMap, inWishMap, nil
	}
	if _, ok := store.UserStore[discordID]; !ok {
		return statusMap, inWishMap, nil
	}

	userGames, err := kurohelperdb.GetUserGameByDiscordID(kurohelperdb.Dbs, discordID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return statusMap, inWishMap, nil
	}
	if err != nil {
		return nil, nil, err
	}

	for _, item := range userGames {
		if item.Status != kurohelperdb.UserGameStatusNone {
			statusMap[item.GameErogsID] = item.Status
		}
		if item.WishListMark {
			inWishMap[item.GameErogsID] = struct{}{}
		}
	}

	return statusMap, inWishMap, nil
}

// FormatGameFlags 將遊玩狀態與願望清單轉換成 Discord 顯示圖示
func FormatGameFlags(status kurohelperdb.UserGameStatus, inWish bool) string {
	flags := make([]string, 0, 2)
	switch status {
	case kurohelperdb.UserGameStatusFinished:
		flags = append(flags, "✅")
	case kurohelperdb.UserGameStatusPlaying:
		flags = append(flags, "🎮")
	case kurohelperdb.UserGameStatusStalled:
		flags = append(flags, "⏸️")
	case kurohelperdb.UserGameStatusDropped:
		flags = append(flags, "🗑️")
	}
	if inWish {
		flags = append(flags, "❤️")
	}
	return strings.Join(flags, " ")
}
