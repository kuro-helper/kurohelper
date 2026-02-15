package usercmd

import (
	"fmt"
	"strings"

	kurohelperdb "kurohelper-db"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"kurohelper/cache"
	"kurohelper/utils"
)

type UserInfo struct {
	User            kurohelperdb.User
	HasPlayed       []kurohelperdb.UserHasPlayed
	InWish          []kurohelperdb.UserInWish
	BrandStatistics []kurohelperdb.BrandCount
	Avatar          string
}

func GetUserinfo(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.NewCID) {
	// 長時間查詢
	if cid == nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
	}

	var messageComponent []discordgo.MessageComponent
	var user kurohelperdb.User
	var brandStatistics []kurohelperdb.BrandCount
	var hasPlayedCount int
	var inWishCount int
	var avatar string
	listHasPlayed := make([]string, 0, 10)
	listInWish := make([]string, 0, 10)

	if cid != nil {
		// 處理CID
		pageCID := utils.PageCID{
			NewCID: *cid,
		}

		cacheValue, err := cache.UserInfoCache.Get(pageCID.GetCacheID())
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}
		userInfo := cacheValue.(UserInfo)

		hasPlayedCount = len(userInfo.HasPlayed)
		inWishCount = len(userInfo.InWish)
		user = userInfo.User
		brandStatistics = userInfo.BrandStatistics
		avatar = userInfo.Avatar

		// 取得資料頁
		pageIndex, err := pageCID.GetPageIndex()
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		var hasMore bool
		hasPlayed, tmpMore := utils.PaginationR(userInfo.HasPlayed, pageIndex, true)
		if tmpMore {
			hasMore = true
		}

		inWish, tmpMore := utils.PaginationR(userInfo.InWish, pageIndex, true)
		if tmpMore {
			hasMore = true
		}

		cidCommandName := utils.MakeCIDCommandName(cid.GetCommandName(), true, "")
		if hasMore {
			if pageIndex == 0 {
				messageComponent = []discordgo.MessageComponent{utils.MakeCIDPageComponent("▶️", pageCID.GetCacheID(), 1, cidCommandName)}
			} else {
				messageComponent = []discordgo.MessageComponent{utils.MakeCIDPageComponent("◀️", pageCID.GetCacheID(), pageIndex-1, cidCommandName)}
				messageComponent = append(messageComponent, utils.MakeCIDPageComponent("▶️", pageCID.GetCacheID(), pageIndex+1, cidCommandName))
			}
		} else {
			messageComponent = []discordgo.MessageComponent{utils.MakeCIDPageComponent("◀️", pageCID.GetCacheID(), pageIndex-1, cidCommandName)}
		}

		for i, hp := range hasPlayed {
			if i == 10 {
				break
			}

			t := getUserPlayRecordTime(&hp)
			if t != "" {
				listHasPlayed = append(listHasPlayed, fmt.Sprintf("* **%s**/*⏱️%s*", hp.GameErogs.Name, t))
			} else {
				listHasPlayed = append(listHasPlayed, fmt.Sprintf("* **%s**", hp.GameErogs.Name))
			}
		}

		for i, iw := range inWish {
			if i == 10 {
				break
			}

			listInWish = append(listInWish, fmt.Sprintf("%d. %s", i+1, iw.GameErogs.Name))
		}
	} else {
		userID := utils.GetUserID(i)

		// User資料
		userTmp, err := kurohelperdb.GetUser(userID)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}
		user = userTmp

		// 取得使用者照片
		discordUser := i.Interaction.User
		if discordUser == nil && i.Interaction.Member != nil {
			// 如果互動在 guild 裡，使用 Member.User
			discordUser = i.Interaction.Member.User
		}
		avatarURL := utils.GetAvatarURL(discordUser)
		avatar = avatarURL

		// 已玩資料
		userHasPlayed, err := kurohelperdb.SelectUserHasPlayed(userID)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}
		hasPlayedCount = len(userHasPlayed)

		// 收藏資料
		userInWish, err := kurohelperdb.SelectUserInWish(userID)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}
		inWishCount = len(userInWish)

		// Brand資料統計
		brandStatistics, err = kurohelperdb.GetUserHasPlayedBrandCount(userID)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		// 處理翻頁
		if len(userInWish) > 10 || len(userHasPlayed) > 10 {
			userInfo := UserInfo{
				User:            user,
				HasPlayed:       userHasPlayed,
				InWish:          userInWish,
				BrandStatistics: brandStatistics,
				Avatar:          avatarURL,
			}

			idStr := uuid.New().String()
			cache.UserInfoCache.Set(idStr, userInfo)

			cidCommandName := utils.MakeCIDCommandName(i.ApplicationCommandData().Name, false, "")
			messageComponent = []discordgo.MessageComponent{utils.MakeCIDPageComponent("▶️", idStr, 1, cidCommandName)}
		}

		for i, hp := range userHasPlayed {
			if i == 10 {
				break
			}

			t := getUserPlayRecordTime(&hp)
			if t != "" {
				listHasPlayed = append(listHasPlayed, fmt.Sprintf("* **%s**/*⏱️%s*", hp.GameErogs.Name, t))
			} else {
				listHasPlayed = append(listHasPlayed, fmt.Sprintf("* **%s**", hp.GameErogs.Name))
			}
		}

		for i, iw := range userInWish {
			if i == 10 {
				break
			}

			listInWish = append(listInWish, fmt.Sprintf("* %s", iw.GameErogs.Name))
		}
	}

	listData := make([]string, 0, len(brandStatistics))
	for i, b := range brandStatistics {
		if i >= 5 { // 已經到第六筆，直接跳出
			break
		}
		if i <= 4 {
			star := strings.Repeat("⭐", 5-i)
			listData = append(listData, fmt.Sprintf("%s **%s: (%d)**", star, b.BrandName, b.Count))
		}
	}

	if len(listHasPlayed) == 0 {
		listHasPlayed = append(listHasPlayed, "**此頁無資料**")
	}

	if len(listInWish) == 0 {
		listInWish = append(listInWish, "**此頁無資料**")
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("**%s 的個人資料**", user.Name),
		Color:       0xB481BB,
		Description: fmt.Sprintf("資料建檔日期: %s", user.CreatedAt.Format("2006-01-02")),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: avatar,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "玩過最多(公司品牌)",
				Value:  strings.Join(listData, "\n"),
				Inline: false,
			},
			{
				Name:   fmt.Sprintf("最近遊玩完畢(%d)", hasPlayedCount),
				Value:  strings.Join(listHasPlayed, "\n"),
				Inline: true,
			},
			{
				Name:   fmt.Sprintf("最近收藏(%d)", inWishCount),
				Value:  strings.Join(listInWish, "\n"),
				Inline: true,
			},
		},
	}

	actionsRow := utils.MakeActionsRow(messageComponent)

	if cid == nil {
		utils.InteractionEmbedRespond(s, i, embed, actionsRow, true)
	} else {
		utils.EditEmbedRespond(s, i, embed, actionsRow)
	}
}

func getUserPlayRecordTime(hp *kurohelperdb.UserHasPlayed) string {
	if hp.CompletedAt != nil {
		return hp.CompletedAt.Format("2006-01-02")
	}
	return ""
}
