package utils

/*
 * 暫放在這
 */

import (
	"kurohelper/store"

	"github.com/bwmarrin/discordgo"
)

type CharacterData struct {
	Name string
	Role string
}

const (
	Played = 1 << iota
	Wish
)

const (
	PlaceholderImageURL = "https://image.kurohelper.com/docs/neneGIF.gif"
)

// 資料分頁
func Pagination[T any](result *[]T, page int, useCache bool) bool {
	resultLen := len(*result)
	expectedMin := page * 10
	expectedMax := page*10 + 10

	if !useCache || page == 0 {
		if resultLen > 10 {
			*result = (*result)[:10]
			return true
		}
		return false
	} else {
		if resultLen > expectedMax {
			*result = (*result)[expectedMin:expectedMax]
			return true
		} else {
			*result = (*result)[expectedMin:]
			return false
		}
	}
}

// 資料分頁(回傳切片本身版本)
func PaginationR[T any](result []T, page int, useCache bool) ([]T, bool) {
	resultLen := len(result)
	expectedMin := page * 10
	expectedMax := page*10 + 10

	if !useCache || page == 0 {
		if resultLen > 10 {
			return result[:10], true
		}
		return result, false
	} else {
		if resultLen > expectedMax {
			return result[expectedMin:expectedMax], true
		} else {
			return result[min(expectedMin, resultLen):], false
		}
	}
}

// 產生顯示圖片，會檢查白名單來判斷要不要顯示
func GenerateImage(i *discordgo.InteractionCreate, url string) *discordgo.MessageEmbedImage {
	var image *discordgo.MessageEmbedImage
	if i.GuildID != "" {
		// guild
		if _, ok := store.GuildDiscordAllowList[i.GuildID]; ok {
			image = &discordgo.MessageEmbedImage{
				URL: url,
			}
		}
	} else {
		// DM
		userID := GetUserID(i)
		if _, ok := store.GuildDiscordAllowList[userID]; ok {
			image = &discordgo.MessageEmbedImage{
				URL: url,
			}
		}
	}
	return image
}
