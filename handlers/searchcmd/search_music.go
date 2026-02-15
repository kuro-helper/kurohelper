package searchcmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"

	"kurohelper/cache"
	kurohelperrerrors "kurohelper/errors"
	"kurohelper/navigator"
	"kurohelper/utils"

	"kurohelper-core/erogs"

	kurohelpercore "kurohelper-core"
)

const searchMusicCommandID = "M2"

var searchMusicColor = 0xF8F8DF

// 查詢音樂指令入口
func SearchMusicV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid == nil {
		navigator.SearchList(s, i, cache.ErogsMusicListStore, "erogs查詢音樂列表", func() ([]erogs.MusicList, error) {
			keyword, err := utils.GetOptions(i, "keyword")
			if err != nil {
				return nil, err
			}
			return erogs.SearchMusicListByKeyword([]string{keyword, kurohelpercore.ZhTwToJp(keyword)})
		}, buildSearchMusicComponents)
	} else {
		switch cid.GetBehaviorID() {
		case utils.PageBehavior:
			erogsSearchMusicListWithCIDV2(s, i, cid)
		case utils.SelectMenuBehavior:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			erogsSearchMusicWithSelectMenuCIDV2(s, i, cid)
		case utils.BackToHomeBehavior:
			navigator.BackToHome(s, i, cid.ToBackToHomeCIDV2(), cache.ErogsMusicListStore, buildSearchMusicComponents)
		default:
			utils.HandleErrorV2(kurohelperrerrors.ErrCIDBehaviorMismatch, s, i, utils.InteractionRespondEditComplex)
		}
	}
}

// 查詢音樂列表(有CID版本)
func erogsSearchMusicListWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	navigator.ChangePage(s, i, pageCID, cache.ErogsMusicListStore, buildSearchMusicComponents)
}

// 查詢指定音樂(有CID版本)
func erogsSearchMusicWithSelectMenuCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid.GetBehaviorID() != utils.SelectMenuBehavior {
		utils.HandleErrorV2(errors.New("handlers: cid behavior id error"), s, i, utils.InteractionRespondEditComplex)
		return
	}

	selectMenuCID := cid.ToSelectMenuCIDV2()

	utils.WebhookEditRespond(s, i, []discordgo.MessageComponent{
		discordgo.Container{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: "# ⌛ 正在跳轉，請稍候...",
				},
			},
		},
	})

	res, err := cache.ErogsMusicStore.Get(selectMenuCID.Value)
	if err != nil {
		if errors.Is(err, kurohelpercore.ErrCacheLost) {
			logrus.WithField("interaction", i).Infof("erogs查詢音樂: %s", selectMenuCID.Value)

			cleanStr := strings.TrimPrefix(selectMenuCID.Value, "E")
			cleanStr = strings.TrimPrefix(cleanStr, "e")
			erogsID, err := strconv.Atoi(cleanStr)
			if err != nil {
				utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
				return
			}

			res, err = erogs.SearchMusicByID(erogsID)
			if err != nil {
				utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
				return
			}

			cache.ErogsMusicStore.Set(selectMenuCID.Value, res)

		} else {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
			return
		}
	}

	// 處理資料
	if res.PlayTime == "00:00:00" {
		res.PlayTime = "未收錄"
	}
	if res.ReleaseDate == "0001-01-01" {
		res.ReleaseDate = "未收錄"
	}

	singerList := strings.Split(res.Singers, ",")
	arrangementList := strings.Split(res.Arrangments, ",")
	lyricList := strings.Split(res.Lyrics, ",")
	compositionList := strings.Split(res.Compositions, ",")
	albumList := strings.Split(res.Album, ",")

	musicData := []string{}
	thumbnailURL := ""
	for _, m := range res.GameCategories {
		musicData = append(musicData, fmt.Sprintf("%s (%s) (%s)", m.GameName, m.GameModel, m.Category))
		if thumbnailURL == "" && strings.TrimSpace(m.GameDMM) != "" {
			thumbnailURL = erogs.MakeDMMImageURL(m.GameDMM)
		}
	}

	if thumbnailURL == "" {
		thumbnailURL = utils.PlaceholderImageURL
	}

	// 構建 Components
	divider := true
	contentParts := []string{}

	// 基本資訊
	contentParts = append(contentParts,
		fmt.Sprintf("**音樂時長**\n%s", res.PlayTime),
		fmt.Sprintf("**發行日期**\n%s", res.ReleaseDate),
		fmt.Sprintf("**平均分數/樣本數**\n%.2f / %d", res.AvgTokuten, res.TokutenCount),
	)

	// 歌手
	if len(singerList) > 0 && singerList[0] != "" {
		contentParts = append(contentParts, fmt.Sprintf("**歌手**\n%s", strings.Join(singerList, " / ")))
	}
	// 作詞
	if len(lyricList) > 0 && lyricList[0] != "" {
		contentParts = append(contentParts, fmt.Sprintf("**作詞**\n%s", strings.Join(lyricList, " / ")))
	}
	// 作曲
	if len(compositionList) > 0 && compositionList[0] != "" {
		contentParts = append(contentParts, fmt.Sprintf("**作曲**\n%s", strings.Join(compositionList, " / ")))
	}
	// 編曲
	if len(arrangementList) > 0 && arrangementList[0] != "" {
		contentParts = append(contentParts, fmt.Sprintf("**編曲**\n%s", strings.Join(arrangementList, " / ")))
	}

	// 遊戲收錄
	if len(musicData) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**遊戲收錄**\n%s", strings.Join(musicData, "\n")))
	}

	// 專輯
	if len(albumList) > 0 && albumList[0] != "" {
		contentParts = append(contentParts, fmt.Sprintf("**收錄專輯**\n%s", strings.Join(albumList, "\n")))
	}

	fullContent := strings.Join(contentParts, "\n\n")

	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: fmt.Sprintf("# %s", res.MusicName),
		},
		discordgo.Separator{Divider: &divider},
		discordgo.Section{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: fullContent,
				},
			},
			Accessory: &discordgo.Thumbnail{
				Media: discordgo.UnfurledMediaItem{
					URL: thumbnailURL,
				},
			},
		},
		discordgo.Separator{Divider: &divider},
	}

	containerComponents = append(containerComponents, utils.MakeBackToHomeComponent(searchMusicCommandID, selectMenuCID.CacheID))

	utils.InteractionRespondEditComplex(s, i, []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchMusicColor,
			Components:  containerComponents,
		},
	})
}

// 產生查詢音樂列表的Components
func buildSearchMusicComponents(res []erogs.MusicList, currentPage int, cacheID string) ([]discordgo.MessageComponent, error) {
	totalItems := len(res)
	totalPages := (totalItems + searchGameListItemsPerPage - 1) / searchGameListItemsPerPage

	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: fmt.Sprintf("# 音樂搜尋\n搜尋筆數: **%d**", totalItems),
		},
		discordgo.Separator{Divider: &divider},
	}

	// 計算當前頁的範圍
	start := (currentPage - 1) * searchGameListItemsPerPage
	end := min(start+searchGameListItemsPerPage, totalItems)
	pagedResults := (res)[start:end]

	gameMenuItems := []utils.SelectMenuItem{}

	// 產生遊戲列表組件
	for idx, r := range pagedResults {
		itemNum := start + idx + 1
		category := ""
		if strings.TrimSpace(r.Category) != "" {
			category = fmt.Sprintf("(%s)", r.Category)
		}
		itemContent := fmt.Sprintf("**%d. %s%s**", itemNum, r.Name, category)

		// 處理Games資訊
		thumbnailURL := "" // 圖片 URL(取第一個)
		if len(r.Games) > 0 {
			var names []string
			for _, g := range r.Games {
				if strings.TrimSpace(g.Name) != "" {
					names = append(names, g.Name)
				}
			}

			cleanDMM := strings.TrimSpace(r.Games[0].DMM)
			if cleanDMM != "" {
				thumbnailURL = erogs.MakeDMMImageURL(cleanDMM)
			}

			if len(names) > 0 {
				itemContent += "\n收錄作品: " + strings.Join(names, ", ")
			}
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
			Title: r.Name + category,
			ID:    "e" + strconv.Itoa(r.ID),
		})
	}

	// 產生選單組件
	selectMenuComponents := utils.MakeSelectMenuComponent(gameMenuItems, searchMusicCommandID, cacheID, "選擇音樂查看詳細")

	// 產生翻頁組件
	pageComponents, err := utils.MakeChangePageComponent(searchMusicCommandID, currentPage, totalPages, cacheID)
	if err != nil {
		return nil, err
	}

	containerComponents = append(containerComponents,
		discordgo.Separator{Divider: &divider},
		selectMenuComponents,
		pageComponents,
	)

	// 組成完整組件回傳
	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchMusicColor,
			Components:  containerComponents,
		},
	}, nil
}
