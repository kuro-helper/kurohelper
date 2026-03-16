package searchcmd

import (
	"errors"
	"fmt"
	"log/slog"
	kurohelpercore "kurohelper-core"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"kurohelper/cache"
	kurohelperrerrors "kurohelper/errors"
	"kurohelper/navigator"
	"kurohelper/utils"

	"kurohelper-core/erogs"
)

const (
	searchCreatorListItemsPerPage    = 10
	searchCreatorItemsPerPage        = 7
	searchCreatorListCommandID       = "C2"
	searchCreatorDetailCommandID     = "CD2"
	searchCreatorGameSelectCommandID = "CG2" // 從創作者詳情選遊戲跳轉，回到上一頁用 CD2
)

var searchCreatorColor = 0xF8F8DF

func SearchCreatorV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid == nil {
		navigator.SearchList(s, i, cache.ErogsCreatorListStore, "erogs查詢創作者列表", func() ([]erogs.CreatorList, error) {
			keyword, err := utils.GetOptions(i, "keyword")
			if err != nil {
				return nil, err
			}
			return erogs.SearchCreatorListByKeyword([]string{keyword, kurohelpercore.ZhTwToJp(keyword)})
		}, buildSearchCreatorListComponents)
	} else {
		cmdID, behaviorID := cid.GetCommandID(), cid.GetBehaviorID()
		switch {
		case cmdID == searchCreatorGameSelectCommandID && behaviorID == utils.SelectMenuBehavior:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			erogsSearchGameWithSelectMenuCIDV2(s, i, cid, searchCreatorDetailCommandID)
		case cmdID == searchCreatorDetailCommandID && behaviorID == utils.BackToHomeBehavior:
			navigator.BackToHome(s, i, cid.ToBackToHomeCIDV2(), cache.ErogsCreatorStore, buildSearchCreatorDetailComponents)
		case behaviorID == utils.PageBehavior:
			if cmdID == searchCreatorDetailCommandID {
				erogsSearchCreatorDetailWithCIDV2(s, i, cid)
			} else {
				erogsSearchCreatorListWithCIDV2(s, i, cid)
			}
		case behaviorID == utils.DetailBtnBehavior:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			erogsSearchCreatorWithSelectMenuCIDV2(s, i, cid)
		case behaviorID == utils.BackToHomeBehavior:
			navigator.BackToHome(s, i, cid.ToBackToHomeCIDV2(), cache.ErogsCreatorListStore, buildSearchCreatorListComponents)
		default:
			utils.HandleErrorV2(kurohelperrerrors.ErrCIDBehaviorMismatch, s, i, utils.InteractionRespondEditComplex)
		}
	}
}

// erogsSearchCreatorListWithCIDV2 創作者列表翻頁
func erogsSearchCreatorListWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	navigator.ChangePage(s, i, pageCID, cache.ErogsCreatorListStore, buildSearchCreatorListComponents)
}

// erogsSearchCreatorDetailWithCIDV2 創作者詳情歷代作品翻頁（僅詳情，與列表完全無關）
func erogsSearchCreatorDetailWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	navigator.ChangePage(s, i, pageCID, cache.ErogsCreatorStore, buildSearchCreatorDetailComponents)
}

// erogsSearchCreatorWithSelectMenuCIDV2 以 CID 的 value 作為查詢 id 顯示創作者詳情（選單或按鈕「查看詳情」進入，統一取 cid value）
func erogsSearchCreatorWithSelectMenuCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	detailCID := cid.ToDetailBtnCIDV2()
	creatorKey := detailCID.Value

	utils.WebhookEditRespond(s, i, []discordgo.MessageComponent{
		discordgo.Container{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: "# ⌛ 正在跳轉，請稍候...",
				},
			},
		},
	})

	res, err := cache.ErogsCreatorStore.Get(creatorKey)
	if err != nil {
		if errors.Is(err, kurohelpercore.ErrCacheLost) {
			slog.Info("erogs查詢創作者", "creatorKey", creatorKey)
			cleanStr := strings.TrimPrefix(creatorKey, "E")
			cleanStr = strings.TrimPrefix(cleanStr, "e")
			creatorID, err := strconv.Atoi(cleanStr)
			if err != nil {
				utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
				return
			}
			res, err = erogs.SearchCreatorByID(creatorID)
			if err != nil {
				utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
				return
			}
			cache.ErogsCreatorStore.Set(creatorKey, res)
		} else {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
			return
		}
	}

	// 選擇後與原列表脫鉤，僅用 PageCID：cacheID 只存 creatorKey，後續翻頁完全獨立
	detailCacheID := uuid.New().String()
	cache.CIDStore.Set(detailCacheID, creatorKey)

	components, err := buildSearchCreatorDetailComponents(res, 1, detailCacheID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	utils.InteractionRespondEditComplex(s, i, components)
}

// buildSearchCreatorDetailComponents 產生創作者詳情（歷代作品分頁）的 Components
func buildSearchCreatorDetailComponents(res *erogs.Creator, currentPage int, pageCacheID string) ([]discordgo.MessageComponent, error) {
	if res == nil {
		return nil, errors.New("handlers: creator res is nil")
	}
	games := res.Games
	sort.Slice(games, func(i, j int) bool {
		return games[i].Median > games[j].Median
	})

	totalItems := len(games)
	totalPages := (totalItems + searchCreatorItemsPerPage - 1) / searchCreatorItemsPerPage
	if totalPages == 0 {
		totalPages = 1
	}

	start := (currentPage - 1) * searchCreatorItemsPerPage
	end := min(start+searchCreatorItemsPerPage, totalItems)
	pagedGames := games[start:end]

	link := ""
	if res.TwitterUsername != "" {
		link += fmt.Sprintf("[Twitter](https://x.com/%s) ", res.TwitterUsername)
	}
	if res.Pixiv != nil {
		link += fmt.Sprintf("[Pixiv](https://www.pixiv.net/users/%d) ", *res.Pixiv)
	}

	divider := true
	countInner := 0
	for _, inner := range res.Games {
		countInner += len(inner.Shokushu)
	}
	headerContent := fmt.Sprintf("# %s\n歷代作品 **%d(%d)** 筆（遊戲評價排序）\n⭐: 批評空間分數 📊: 投票人數", res.Name, totalItems, countInner)
	if strings.TrimSpace(link) != "" {
		headerContent += "\n" + link
	}

	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: headerContent,
		},
		discordgo.Separator{Divider: &divider},
	}

	gameMenuItems := make([]utils.SelectMenuItem, 0, len(pagedGames))
	for idx, g := range pagedGames {
		shokushu := make([]string, 0, len(g.Shokushu))
		for _, s := range g.Shokushu {
			if s.Shubetu != 7 {
				shokushu = append(shokushu, fmt.Sprintf("*%s*", erogs.ShubetuMap[s.Shubetu]))
			} else {
				shokushu = append(shokushu, fmt.Sprintf("*%s*", s.ShubetuDetailName))
			}
		}
		shokushuStr := strings.Join(shokushu, ", ")
		itemNum := start + idx + 1
		itemContent := fmt.Sprintf("**%d. %s** (%s)\n⭐ **%d** / 📊 **%d** / %s", itemNum, g.Gamename, shokushuStr, g.Median, g.CountAll, g.SellDay)

		thumbnailURL := ""
		if strings.TrimSpace(g.DMM) != "" {
			thumbnailURL = erogs.MakeDMMImageURL(g.DMM)
		}
		if strings.TrimSpace(thumbnailURL) == "" {
			thumbnailURL = utils.PlaceholderImageURL
		}

		containerComponents = append(containerComponents, discordgo.Section{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: itemContent,
				},
			},
			Accessory: &discordgo.Thumbnail{
				Media: discordgo.UnfurledMediaItem{
					URL: thumbnailURL,
				},
			},
		})
		gameMenuItems = append(gameMenuItems, utils.SelectMenuItem{
			Title: g.Gamename + " (" + g.SellDay + ")",
			ID:    "e" + strconv.Itoa(g.ID),
		})
	}

	// 與 search_game_v2 相同：選單選擇遊戲可跳轉遊戲詳情，並可回到上一頁（創作者詳情）
	selectMenuComponents := utils.MakeSelectMenuComponent(gameMenuItems, searchCreatorGameSelectCommandID, pageCacheID, "選擇遊戲查看詳細")
	containerComponents = append(containerComponents, discordgo.Separator{Divider: &divider}, selectMenuComponents)

	if totalItems > searchCreatorItemsPerPage {
		pageComponents, err := utils.MakeChangePageComponent(searchCreatorDetailCommandID, currentPage, totalPages, pageCacheID)
		if err != nil {
			return nil, err
		}
		containerComponents = append(containerComponents, discordgo.Separator{Divider: &divider}, pageComponents)
	}

	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchCreatorColor,
			Components:  containerComponents,
		},
	}, nil
}

// buildSearchCreatorListComponents 產生創作者列表的 Components
func buildSearchCreatorListComponents(res []erogs.CreatorList, currentPage int, cacheID string) ([]discordgo.MessageComponent, error) {
	if res == nil {
		return nil, errors.New("handlers: creator list res is nil")
	}
	totalItems := len(res)
	totalPages := (totalItems + searchCreatorListItemsPerPage - 1) / searchCreatorListItemsPerPage
	if totalPages == 0 {
		totalPages = 1
	}

	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: fmt.Sprintf("# 創作者列表搜尋\n搜尋筆數: **%d**", totalItems),
		},
		discordgo.Separator{Divider: &divider},
	}

	start := (currentPage - 1) * searchCreatorListItemsPerPage
	end := min(start+searchCreatorListItemsPerPage, totalItems)
	pagedResults := res[start:end]

	creatorMenuItems := make([]utils.SelectMenuItem, 0, len(pagedResults))
	for idx, r := range pagedResults {
		itemNum := start + idx + 1
		containerComponents = append(containerComponents, discordgo.Section{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: fmt.Sprintf("**%d. e%-5s　%s**", itemNum, strconv.Itoa(r.ID), r.Name),
				},
			},
			Accessory: discordgo.Button{
				Label:    "查看詳情",
				Style:    discordgo.PrimaryButton,
				CustomID: utils.MakeDetailBtnCIDV2(searchCreatorListCommandID, cacheID, "e"+strconv.Itoa(r.ID)),
			},
		})
		creatorMenuItems = append(creatorMenuItems, utils.SelectMenuItem{
			Title: r.Name,
			ID:    "e" + strconv.Itoa(r.ID),
		})
	}
	pageComponents, err := utils.MakeChangePageComponent(searchCreatorListCommandID, currentPage, totalPages, cacheID)
	if err != nil {
		return nil, err
	}

	containerComponents = append(containerComponents,
		discordgo.Separator{Divider: &divider},
		pageComponents,
	)

	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchCreatorColor,
			Components:  containerComponents,
		},
	}, nil
}
