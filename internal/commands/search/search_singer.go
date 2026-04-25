package search

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"kurohelper/internal/cache"
	kurohelperrerrors "kurohelper/internal/errors"
	"kurohelper/internal/executor"
	"kurohelper/internal/utils"
	"kurohelperservice"

	"kurohelperservice/provider/erogs"
)

const (
	searchSingerItemsPerPage        = 10
	searchSingerDetailItemsPerPage  = 7
	searchSingerCommandName         = "查詢歌手"
	searchSingerListRouteKey        = "list"
	searchSingerDetailRouteKey      = "detail"
	searchSingerMusicSelectRouteKey = "music_select"
)

var searchSingerColor = 0x7DD3FC

type SearchSinger struct{}

func (ss *SearchSinger) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "查詢歌手",
		Description: "[新]根據關鍵字查詢歌手相關資料(批評空間)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "keyword",
				Description: "關鍵字",
				Required:    true,
			},
		},
	}
}

func (ss *SearchSinger) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ss.HandleComponent(s, i, nil)
}

func (ss *SearchSinger) HandleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid == nil {
		executor.SearchList(s, i, cache.ErogsSingerListStore, "erogs查詢歌手列表", func() ([]erogs.CreatorList, error) {
			keyword, err := utils.GetOptions(i, "keyword")
			if err != nil {
				return nil, err
			}
			return erogs.SearchSingerListByKeyword([]string{keyword, kurohelperservice.ZhTwToJp(keyword)})
		}, buildSearchSingerListComponents)
	} else {
		routeKey, behaviorID := cid.GetRouteKey(), cid.GetBehaviorID()
		switch {
		case routeKey == searchSingerMusicSelectRouteKey && behaviorID == utils.SelectMenuBehavior:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			erogsSearchMusicWithSelectMenuCIDV2(s, i, cid, searchSingerCommandName, searchSingerDetailRouteKey)
		case routeKey == searchSingerDetailRouteKey && behaviorID == utils.BackToHomeBehavior:
			executor.BackToHome(s, i, cid.ToBackToHomeCIDV2(), cache.ErogsSingerStore, buildSearchSingerDetailComponents)
		case behaviorID == utils.PageBehavior:
			if routeKey == searchSingerDetailRouteKey {
				erogsSearchSingerDetailWithCIDV2(s, i, cid)
			} else {
				erogsSearchSingerListWithCIDV2(s, i, cid)
			}
		case behaviorID == utils.DetailBtnBehavior:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			erogsSearchSingerWithSelectMenuCIDV2(s, i, cid)
		case behaviorID == utils.BackToHomeBehavior:
			executor.BackToHome(s, i, cid.ToBackToHomeCIDV2(), cache.ErogsSingerListStore, buildSearchSingerListComponents)
		default:
			utils.HandleErrorV2(kurohelperrerrors.ErrCIDBehaviorMismatch, s, i, utils.InteractionRespondEditComplex)
		}
	}
}

func erogsSearchSingerListWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	executor.ChangePage(s, i, pageCID, cache.ErogsSingerListStore, buildSearchSingerListComponents)
}

func erogsSearchSingerDetailWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	executor.ChangePage(s, i, pageCID, cache.ErogsSingerStore, buildSearchSingerDetailComponents)
}

func erogsSearchSingerWithSelectMenuCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	detailCID := cid.ToDetailBtnCIDV2()
	singerKey := detailCID.Value

	utils.WebhookEditRespond(s, i, []discordgo.MessageComponent{
		discordgo.Container{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: "# ⌛ 正在跳轉，請稍候...",
				},
			},
		},
	})

	res, err := cache.ErogsSingerStore.Get(singerKey)
	if err != nil {
		if errors.Is(err, kurohelperservice.ErrCacheLost) {
			slog.Info("erogs查詢歌手", "singerKey", singerKey)
			cleanStr := strings.TrimPrefix(singerKey, "E")
			cleanStr = strings.TrimPrefix(cleanStr, "e")
			singerID, convErr := strconv.Atoi(cleanStr)
			if convErr != nil {
				utils.HandleErrorV2(convErr, s, i, utils.InteractionRespondEditComplex)
				return
			}
			res, err = erogs.SearchSingerByKeyword(singerID)
			if err != nil {
				utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
				return
			}
			cache.ErogsSingerStore.Set(singerKey, res)
		} else {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
			return
		}
	}

	detailCacheID := uuid.New().String()
	cache.CIDStore.Set(detailCacheID, singerKey)

	components, err := buildSearchSingerDetailComponents(res, 1, detailCacheID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	utils.InteractionRespondEditComplex(s, i, components)
}

func buildSearchSingerDetailComponents(res *erogs.Singer, currentPage int, pageCacheID string) ([]discordgo.MessageComponent, error) {
	if res == nil {
		return nil, errors.New("handlers: singer res is nil")
	}
	musicInfos := res.MusicInfo
	sort.Slice(musicInfos, func(i, j int) bool {
		return musicInfos[i].MusicAvgScore > musicInfos[j].MusicAvgScore
	})

	totalItems := len(musicInfos)
	totalPages := (totalItems + searchSingerDetailItemsPerPage - 1) / searchSingerDetailItemsPerPage
	if totalPages == 0 {
		totalPages = 1
	}

	start := (currentPage - 1) * searchSingerDetailItemsPerPage
	end := min(start+searchSingerDetailItemsPerPage, totalItems)
	paged := musicInfos[start:end]

	link := ""
	if t := strings.TrimSpace(res.Twitter); t != "" {
		if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") {
			link += fmt.Sprintf("[Twitter](%s) ", t)
		} else {
			t = strings.TrimPrefix(t, "@")
			link += fmt.Sprintf("[Twitter](https://x.com/%s) ", t)
		}
	}
	if b := strings.TrimSpace(res.Blog); b != "" {
		if strings.HasPrefix(b, "http://") || strings.HasPrefix(b, "https://") {
			link += fmt.Sprintf("[Blog](%s) ", b)
		}
	}
	if p := strings.TrimSpace(res.Pixiv); p != "" {
		if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
			link += fmt.Sprintf("[Pixiv](%s) ", p)
		} else {
			link += fmt.Sprintf("[Pixiv](https://www.pixiv.net/users/%s) ", p)
		}
	}

	divider := true
	headerContent := fmt.Sprintf("# 🎤 %s\n🎵 音樂作品 **%d** 筆（依評分排序）\n⭐ 平均得分　📊 投票數", res.SingerName, totalItems)
	if strings.TrimSpace(link) != "" {
		headerContent += "\n" + link
	}

	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: headerContent,
		},
		discordgo.Separator{Divider: &divider},
	}

	musicMenuItems := make([]utils.SelectMenuItem, 0, len(paged))
	for idx, m := range paged {
		itemNum := start + idx + 1
		releaseDisplay := m.ReleaseDate
		if releaseDisplay == "0001-01-01" {
			releaseDisplay = "無紀錄"
		}
		itemContent := fmt.Sprintf("🎹 **%d.** %s\n⭐ **%.2f** / 📊 **%d** / 📅 %s\n🎮 %s",
			itemNum, m.MusicName, m.MusicAvgScore, m.MusicVoteCount, releaseDisplay, m.GameName)

		thumbnailURL := ""
		if strings.TrimSpace(m.DMM) != "" {
			thumbnailURL = erogs.MakeDMMImageURL(m.DMM)
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
		musicMenuItems = append(musicMenuItems, utils.SelectMenuItem{
			Title: m.MusicName + " (" + releaseDisplay + ")",
			ID:    "e" + strconv.Itoa(m.MusicID),
		})
	}

	if len(musicMenuItems) > 0 {
		selectMenuComponents := utils.MakeSelectMenuComponent(musicMenuItems, searchSingerCommandName, searchSingerMusicSelectRouteKey, pageCacheID, "選擇音樂查看詳細")
		containerComponents = append(containerComponents, discordgo.Separator{Divider: &divider}, selectMenuComponents)
	}

	if totalItems > searchSingerDetailItemsPerPage {
		pageComponents, err := utils.MakeChangePageComponent(searchSingerCommandName, searchSingerDetailRouteKey, currentPage, totalPages, pageCacheID)
		if err != nil {
			return nil, err
		}
		containerComponents = append(containerComponents, discordgo.Separator{Divider: &divider}, pageComponents)
	}

	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchSingerColor,
			Components:  containerComponents,
		},
	}, nil
}

func buildSearchSingerListComponents(res []erogs.CreatorList, currentPage int, cacheID string) ([]discordgo.MessageComponent, error) {
	if res == nil {
		return nil, errors.New("handlers: singer list res is nil")
	}
	totalItems := len(res)
	totalPages := (totalItems + searchSingerItemsPerPage - 1) / searchSingerItemsPerPage
	if totalPages == 0 {
		totalPages = 1
	}

	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: fmt.Sprintf("# 歌手列表搜尋\n搜尋筆數: **%d**", totalItems),
		},
		discordgo.Separator{Divider: &divider},
	}

	start := (currentPage - 1) * searchSingerItemsPerPage
	end := min(start+searchSingerItemsPerPage, totalItems)
	pagedResults := res[start:end]

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
				CustomID: utils.MakeDetailBtnCIDV2(searchSingerCommandName, searchSingerListRouteKey, cacheID, "e"+strconv.Itoa(r.ID)),
			},
		})
	}

	pageComponents, err := utils.MakeChangePageComponent(searchSingerCommandName, searchSingerListRouteKey, currentPage, totalPages, cacheID)
	if err != nil {
		return nil, err
	}

	containerComponents = append(containerComponents,
		discordgo.Separator{Divider: &divider},
		pageComponents,
	)

	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchSingerColor,
			Components:  containerComponents,
		},
	}, nil
}
