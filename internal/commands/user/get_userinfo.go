package user

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"kurohelper/internal/cache"
	kurohelpererrors "kurohelper/internal/errors"
	"kurohelper/internal/utils"

	kurohelperdb "kurohelperservice/db"
)

type UserInfo struct {
	User            kurohelperdb.User
	UserGames       []kurohelperdb.UserGame
	BrandStatistics []kurohelperdb.BrandCount
	Avatar          string
}

type GetUserinfo struct{}

const userInfoCommandName = "個人資料"

func filterDisplayUserGames(userGames []kurohelperdb.UserGame) []kurohelperdb.UserGame {
	filtered := make([]kurohelperdb.UserGame, 0, len(userGames))
	for _, item := range userGames {
		// Empty shell row: status=0 and no marks.
		if item.Status == 0 && !item.BlackListMark && !item.WishListMark {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (g *GetUserinfo) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "個人資料",
		Description: "取得自己的個人資料",
	}
}

func (g *GetUserinfo) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	g.HandleComponent(s, i, nil)
}

func (g *GetUserinfo) HandleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	// 長時間查詢
	if cid == nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
	}

	var messageComponent []discordgo.MessageComponent
	var user kurohelperdb.User
	var brandStatistics []kurohelperdb.BrandCount
	var completedCount int
	var wishCount int
	var avatar string
	listUserGames := make([]string, 0, 10)

	if cid != nil {
		if cid.GetBehaviorID() != utils.PageBehavior {
			utils.HandleError(kurohelpererrors.ErrCIDBehaviorMismatch, s, i)
			return
		}
		pageCID, err := cid.ToPageCIDV2()
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		cacheValue, err := cache.UserInfoCache.Get(pageCID.CacheID)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}
		userInfo := cacheValue.(UserInfo)
		filteredUserGames := filterDisplayUserGames(userInfo.UserGames)

		for _, item := range filteredUserGames {
			if item.Status == 1 {
				completedCount++
			}
			if item.WishListMark {
				wishCount++
			}
		}
		user = userInfo.User
		brandStatistics = userInfo.BrandStatistics
		avatar = userInfo.Avatar

		// 取得資料頁
		pageIndex := pageCID.Value

		var hasMore bool
		userGames, tmpMore := utils.PaginationR(filteredUserGames, pageIndex, true)
		if tmpMore {
			hasMore = true
		}

		if hasMore {
			if pageIndex == 0 {
				messageComponent = []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "▶️",
						Style:    discordgo.PrimaryButton,
						CustomID: utils.MakePageCIDV2(userInfoCommandName, "", 1, pageCID.CacheID, false),
					},
				}
			} else {
				messageComponent = []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "◀️",
						Style:    discordgo.PrimaryButton,
						CustomID: utils.MakePageCIDV2(userInfoCommandName, "", pageIndex-1, pageCID.CacheID, false),
					},
				}
				messageComponent = append(messageComponent, discordgo.Button{
					Label:    "▶️",
					Style:    discordgo.PrimaryButton,
					CustomID: utils.MakePageCIDV2(userInfoCommandName, "", pageIndex+1, pageCID.CacheID, false),
				})
			}
		} else {
			messageComponent = []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "◀️",
					Style:    discordgo.PrimaryButton,
					CustomID: utils.MakePageCIDV2(userInfoCommandName, "", pageIndex-1, pageCID.CacheID, false),
				},
			}
		}

		startNo := pageIndex*10 + 1
		for i, ug := range userGames {
			if i == 10 {
				break
			}

			listUserGames = append(listUserGames, formatUserGameLine(startNo+i, &ug))
		}
	} else {
		userID := utils.GetUserID(i)

		// User資料
		userTmp, err := kurohelperdb.GetUserByDiscordID(kurohelperdb.Dbs, userID)
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

		// UserGame資料（單一列表）
		userGames, err := kurohelperdb.GetUserGameByDiscordID(kurohelperdb.Dbs, userID)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}
		userGames = filterDisplayUserGames(userGames)
		for _, item := range userGames {
			if item.Status == 1 {
				completedCount++
			}
			if item.WishListMark {
				wishCount++
			}
		}

		// Brand資料統計
		brandStatistics, err = kurohelperdb.GetUserHasPlayedBrandCount(userID)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		// 處理翻頁
		if len(userGames) > 10 {
			userInfo := UserInfo{
				User:            user,
				UserGames:       userGames,
				BrandStatistics: brandStatistics,
				Avatar:          avatarURL,
			}

			idStr := uuid.New().String()
			cache.UserInfoCache.Set(idStr, userInfo)

			messageComponent = []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "▶️",
					Style:    discordgo.PrimaryButton,
					CustomID: utils.MakePageCIDV2(userInfoCommandName, "", 1, idStr, false),
				},
			}
		}

		for i, ug := range userGames {
			if i == 10 {
				break
			}
			listUserGames = append(listUserGames, formatUserGameLine(i+1, &ug))
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

	if len(listUserGames) == 0 {
		listUserGames = append(listUserGames, "**無資料**")
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
				Name:   fmt.Sprintf("遊戲列表（✅ 已玩 %d / ❤️ 收藏 %d）", completedCount, wishCount),
				Value:  strings.Join(listUserGames, "\n"),
				Inline: false,
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

func getUserGameRecordTime(ug *kurohelperdb.UserGame) string {
	if ug.FinishedDate != nil {
		return ug.FinishedDate.Format("2006-01-02")
	}
	return ""
}

func formatUserGameLine(index int, ug *kurohelperdb.UserGame) string {
	flags := make([]string, 0, 2)
	if ug.Status == 1 {
		flags = append(flags, "✅")
	}
	if ug.WishListMark {
		flags = append(flags, "❤️")
	}
	prefix := strings.Join(flags, "")
	if prefix == "" {
		prefix = "▫️"
	}

	t := getUserGameRecordTime(ug)
	if t != "" {
		return fmt.Sprintf("%d. %s **%s**  ⏱️%s", index, prefix, ug.GameErogs.Name, t)
	}
	return fmt.Sprintf("%d. %s **%s**", index, prefix, ug.GameErogs.Name)
}
