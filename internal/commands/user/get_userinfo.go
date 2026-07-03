package user

import (
	"errors"
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
		Description: "取得個人資料",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "discord_id",
				Description: "要查詢的使用者 Discord ID（選填）",
				Required:    false,
			},
		},
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
		requesterID := utils.GetUserID(i)
		targetDiscordID := requesterID

		targetUserIDOption, err := utils.GetOptions(i, "discord_id")
		if err != nil && !errors.Is(err, kurohelpererrors.ErrOptionNotFound) {
			utils.HandleError(err, s, i)
			return
		}
		if strings.TrimSpace(targetUserIDOption) != "" {
			targetDiscordID = strings.TrimSpace(targetUserIDOption)
		}

		// User資料
		userTmp, err := kurohelperdb.GetUserByDiscordID(kurohelperdb.Dbs, targetDiscordID)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}
		user = userTmp

		if targetDiscordID != requesterID && user.PrivateGameData {
			utils.HandleError(kurohelpererrors.ErrPrivateGameData, s, i)
			return
		}

		// 取得使用者照片
		avatarURL := discordgo.EndpointDefaultUserAvatar(0)
		if discordUser, err := s.User(targetDiscordID); err == nil {
			avatarURL = utils.GetAvatarURL(discordUser)
		}
		avatar = avatarURL

		// UserGame資料（單一列表）
		userGames, err := kurohelperdb.GetUserGameByDiscordID(kurohelperdb.Dbs, targetDiscordID)
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
		brandStatistics, err = kurohelperdb.GetUserHasPlayedBrandCount(targetDiscordID)
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

	// 依遊玩數量分星級（相同數量的公司合併在同一行），最多顯示 5 個星級
	listData := make([]string, 0, 5)
	tier := 0
	for i := 0; i < len(brandStatistics); {
		count := brandStatistics[i].Count
		names := make([]string, 0)
		// brandStatistics 已依 count 由大到小排序，相同數量必相鄰
		for i < len(brandStatistics) && brandStatistics[i].Count == count {
			names = append(names, brandStatistics[i].BrandName)
			i++
		}
		star := strings.Repeat("⭐", 5-tier)
		listData = append(listData, fmt.Sprintf("%s **%s: (%d)**", star, strings.Join(names, " | "), count))
		tier++
		if tier >= 5 {
			break
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
	prefix := utils.FormatGameFlags(ug.Status, ug.WishListMark)
	if prefix == "" {
		prefix = "▫️"
	}

	t := getUserGameRecordTime(ug)
	if t != "" {
		return fmt.Sprintf("%d. %s **%s**  ⏱️%s", index, prefix, ug.GameErogs.Name, t)
	}
	return fmt.Sprintf("%d. %s **%s**", index, prefix, ug.GameErogs.Name)
}
