package searchcmd

import (
	"errors"
	"fmt"
	"kurohelper/cache"
	kurohelperrerrors "kurohelper/errors"
	common "kurohelper/navigator"
	"kurohelper/store"
	"kurohelper/utils"
	"sort"
	"strconv"
	"strings"

	kurohelpercore "kurohelper-core"
	"kurohelper-core/erogs"

	"kurohelper-core/vndb"

	kurohelperdb "kurohelper-db"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	searchBrandItemsPerPage   = 7
	searchBrandVNDBCommandID  = "B1"
	searchBrandErogsCommandID = "B2"
)

var (
	searchBrandColor = 0x00AA90
)

// 查詢公司品牌Handler(新版API)
func SearchBrandV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid == nil {
		optDB, err := utils.GetOptions(i, "查詢資料庫選項")
		if err != nil && errors.Is(err, kurohelperrerrors.ErrOptionTranslateFail) {
			utils.HandleError(err, s, i)
			return
		}
		switch optDB {
		case "1":
			common.SearchList(s, i, cache.VndbBrandStore, "vndb查詢公司品牌", func() (*vndb.ProducerSearchResponse, error) {
				keyword, err := utils.GetOptions(i, "keyword")
				if err != nil {
					return nil, err
				}
				return vndb.GetProducerByFuzzy(keyword, "")
			}, buildSearchBrandComponents)
		case "2":
			erogsSearchBrandV2(s, i)
		default:
			// 預設走批評空間
			erogsSearchBrandV2(s, i)
		}
	} else {
		// 選擇不同行為的進入點
		switch (switchMode{cid.GetCommandID()[1], cid.GetBehaviorID()}) {
		case switchMode{'1', utils.PageBehavior}:
			vndbSearchBrandWithCIDV2(s, i, cid)
		case switchMode{'1', utils.SelectMenuBehavior}:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			vndbSearchBrandWithSelectMenuCIDV2(s, i, cid)
		case switchMode{'1', utils.BackToHomeBehavior}:
			common.BackToHome(s, i, cid.ToBackToHomeCIDV2(), cache.VndbBrandStore, buildSearchBrandComponents)
		case switchMode{'2', utils.PageBehavior}:
			erogsSearchBrandWithCIDV2(s, i, cid)
		case switchMode{'2', utils.SelectMenuBehavior}:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			erogsSearchGameWithSelectMenuCIDV2(s, i, cid, searchBrandErogsCommandID)
		case switchMode{'2', utils.BackToHomeBehavior}:
			common.BackToHome(s, i, cid.ToBackToHomeCIDV2(), cache.ErogsBrandStore, func(cacheValue *erogs.Brand, page int, cacheID string) ([]discordgo.MessageComponent, error) {
				hasPlayedMap, inWishMap := getErogsUserPlayWishMaps(i)
				return buildSearchBrandErogsComponents(cacheValue, page, cacheID, hasPlayedMap, inWishMap)
			})
		default:
			utils.HandleErrorV2(kurohelperrerrors.ErrCIDBehaviorMismatch, s, i, utils.InteractionRespondEditComplex)
			return
		}
	}
}

// vndbSearchBrandWithCIDV2 查詢公司品牌(有CID版本)，目前只有翻頁事件
func vndbSearchBrandWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	common.ChangePage(s, i, pageCID, cache.VndbBrandStore, buildSearchBrandComponents)
}

// 產生查詢公司品牌的Components
func buildSearchBrandComponents(res *vndb.ProducerSearchResponse, currentPage int, cacheID string) ([]discordgo.MessageComponent, error) {
	producerName := res.Producer.Results[0].Name
	totalItems := len(res.Vn.Results)
	totalPages := (totalItems + searchBrandItemsPerPage - 1) / searchBrandItemsPerPage

	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: fmt.Sprintf("# %s\n遊戲筆數: **%d**\n⭐: vndb分數 📊:投票人數 🕒: 遊玩時間(小時)", producerName, totalItems),
		},
		discordgo.Separator{Divider: &divider},
	}

	// 計算當前頁的範圍
	start := (currentPage - 1) * searchBrandItemsPerPage
	end := min(start+searchBrandItemsPerPage, totalItems)
	pagedResults := res.Vn.Results[start:end]

	brandMenuItems := []utils.SelectMenuItem{}

	// 產生遊戲組件
	for idx, item := range pagedResults {
		itemNum := start + idx + 1
		title := item.Title
		if strings.TrimSpace(item.Alttitle) != "" {
			title = item.Alttitle
		}
		hours := item.LengthMinutes / 60
		itemContent := fmt.Sprintf("**%d. %s**\n⭐**%.1f**/📊**%d**/🕒**%02d**", itemNum, title, item.Rating, item.Votecount, hours)

		if strings.TrimSpace(item.Image.Thumbnail) == "" {
			containerComponents = append(containerComponents, discordgo.Section{
				Components: []discordgo.MessageComponent{
					discordgo.TextDisplay{
						Content: itemContent,
					},
				},
				Accessory: &discordgo.Thumbnail{
					Media: discordgo.UnfurledMediaItem{
						URL: utils.PlaceholderImageURL,
					},
				},
			})
		} else {
			containerComponents = append(containerComponents, discordgo.Section{
				Components: []discordgo.MessageComponent{
					discordgo.TextDisplay{
						Content: itemContent,
					},
				},
				Accessory: &discordgo.Thumbnail{
					Media: discordgo.UnfurledMediaItem{
						URL: item.Image.Thumbnail,
					},
				},
			})
		}

		brandMenuItems = append(brandMenuItems, utils.SelectMenuItem{
			Title: title,
			ID:    item.ID,
		})
	}

	// 產生選單組件
	selectMenuComponents := utils.MakeSelectMenuComponent(brandMenuItems, searchBrandVNDBCommandID, cacheID, "選擇遊戲查看詳細")

	// 產生翻頁組件
	pageComponents, err := utils.MakeChangePageComponent(searchBrandVNDBCommandID, currentPage, totalPages, cacheID)
	if err != nil {
		return nil, err
	} else {
		containerComponents = append(containerComponents,
			discordgo.Separator{Divider: &divider},
			selectMenuComponents,
			pageComponents,
		)
	}

	// 組成完整組件回傳
	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchBrandColor,
			Components:  containerComponents,
		},
	}, nil
}

func vndbSearchBrandWithSelectMenuCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid.GetBehaviorID() != utils.SelectMenuBehavior {
		utils.HandleErrorV2(errors.New("handlers: cid behavior id error"), s, i, utils.InteractionRespondEditComplex)
		return
	}

	selectMenuCID := cid.ToSelectMenuCIDV2()

	// 檢查 CID 快取是否存在
	if _, err := cache.CIDStore.Get(selectMenuCID.CacheID); err != nil {
		utils.HandleErrorV2(kurohelpercore.ErrCacheLost, s, i, utils.InteractionRespondEditComplex)
		return
	}

	utils.WebhookEditRespond(s, i, []discordgo.MessageComponent{
		discordgo.Container{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: "# ⌛ 正在跳轉,請稍候...",
				},
			},
		},
	})

	// 嘗試從快取取得單一遊戲資料
	res, err := cache.VndbGameStore.Get(selectMenuCID.Value)
	if err != nil {
		if errors.Is(err, kurohelpercore.ErrCacheLost) {
			logrus.WithField("guildID", i.GuildID).Infof("vndb搜尋遊戲: %s", selectMenuCID.Value)

			res, err = vndb.GetVNByFuzzy(selectMenuCID.Value)
			if err != nil {
				utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
				return
			}

			// 將查詢結果存入快取
			cache.VndbGameStore.Set(selectMenuCID.Value, res)

		} else {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
			return
		}
	}
	/* 處理回傳結構 */

	gameTitle := res.Results[0].Alttitle
	if strings.TrimSpace(gameTitle) == "" {
		gameTitle = res.Results[0].Title
	}
	brandTitle := res.Results[0].Developers[0].Original
	if strings.TrimSpace(brandTitle) != "" {
		brandTitle += fmt.Sprintf("(%s)", res.Results[0].Developers[0].Name)
	} else {
		brandTitle = res.Results[0].Developers[0].Name
	}

	// staff block
	var scenario string
	var art string
	var songs string
	var tmpAlias string
	for _, staff := range res.Results[0].Staff {
		staffName := staff.Original
		if staffName == "" {
			staffName = staff.Name
		}
		if len(staff.Aliases) > 0 {
			aliases := make([]string, 0, len(staff.Aliases))
			for _, alias := range staff.Aliases {
				if alias.IsMain {
					staffName = alias.Name
				} else {
					aliases = append(aliases, alias.Name)
				}
			}
			tmpAlias = "(" + strings.Join(aliases, ", ") + ")"
			if len(aliases) == 0 {
				tmpAlias = ""
			}
		}

		switch staff.Role {
		case "scenario":
			scenario += fmt.Sprintf("%s %s\n", staffName, tmpAlias)
		case "art":
			art += fmt.Sprintf("%s %s\n", staffName, tmpAlias)
		case "songs":
			songs += fmt.Sprintf("%s %s\n", staffName, tmpAlias)
		}
	}

	// character block

	characterMap := make(map[string]utils.CharacterData) // map[characterID]CharacterData
	for _, va := range res.Results[0].Va {
		characterName := va.Character.Original
		if characterName == "" {
			characterName = va.Character.Name
		}
		for _, vn := range va.Character.Vns {
			if vn.ID == res.Results[0].ID {
				characterMap[va.Character.ID] = utils.CharacterData{
					Name: characterName,
					Role: vn.Role,
				}
				break
			}
		}
	}

	// 將 map 轉為 slice 並排序
	characterList := make([]utils.CharacterData, 0, len(characterMap))
	for _, character := range characterMap {
		characterList = append(characterList, character)
	}
	sort.Slice(characterList, func(i, j int) bool {
		return characterList[i].Role < characterList[j].Role
	})

	// 格式化輸出
	characters := make([]string, 0, len(characterList))
	for _, character := range characterList {
		characters = append(characters, fmt.Sprintf("**%s** (%s)", character.Name, vndb.Role[character.Role]))
	}

	// relations block
	relationsGame := make([]string, 0, len(res.Results[0].Relations))
	for _, rg := range res.Results[0].Relations {
		titleName := ""
		for _, title := range rg.Titles {
			if title.Main {
				titleName = title.Title
			}
		}
		relationsGame = append(relationsGame, fmt.Sprintf("%s(%s)", titleName, rg.ID))
	}
	relationsGameDisplay := strings.Join(relationsGame, ", ")
	if strings.TrimSpace(relationsGameDisplay) == "" {
		relationsGameDisplay = "無"
	}

	// 過濾色情/暴力圖片
	imageURL := res.Results[0].Image.Url
	if res.Results[0].Image.Sexual >= 1 || res.Results[0].Image.Violence >= 1 {
		imageURL = ""
		logrus.WithField("guildID", i.GuildID).Infof("%s 封面已過濾圖片顯示", gameTitle)
	} else {
		// 檢查是否允許顯示圖片
		if i.GuildID != "" {
			// guild
			if _, ok := store.GuildDiscordAllowList[i.GuildID]; !ok {
				imageURL = ""
			}
		} else {
			// DM
			userID := utils.GetUserID(i)
			if _, ok := store.GuildDiscordAllowList[userID]; !ok {
				imageURL = ""
			}
		}
	}

	// 構建 Components V2 格式 - 將所有內容合併到一個 Section
	divider := true
	contentParts := []string{}

	// 品牌(公司)名稱
	if strings.TrimSpace(brandTitle) != "" {
		contentParts = append(contentParts, fmt.Sprintf("**品牌(公司)名稱**\n%s", brandTitle))
	}

	// 劇本
	if strings.TrimSpace(scenario) != "" {
		contentParts = append(contentParts, fmt.Sprintf("**劇本**\n%s", strings.TrimSpace(scenario)))
	}

	// 美術
	if strings.TrimSpace(art) != "" {
		contentParts = append(contentParts, fmt.Sprintf("**美術**\n%s", strings.TrimSpace(art)))
	}

	// 音樂
	if strings.TrimSpace(songs) != "" {
		contentParts = append(contentParts, fmt.Sprintf("**音樂**\n%s", strings.TrimSpace(songs)))
	}

	// 評價和遊玩時數
	evaluationText := fmt.Sprintf("**評價(平均/貝式平均/樣本數)**\n%.1f/%.1f/%d", res.Results[0].Average, res.Results[0].Rating, res.Results[0].Votecount)
	playtimeText := fmt.Sprintf("**平均遊玩時數/樣本數**\n%d(H)/%d", res.Results[0].LengthMinutes/60, res.Results[0].LengthVotes)
	contentParts = append(contentParts, evaluationText, playtimeText)

	// 角色列表
	if len(characters) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**角色列表**\n%s", strings.Join(characters, " / ")))
	}

	// 相關遊戲
	contentParts = append(contentParts, fmt.Sprintf("**相關遊戲**\n%s", relationsGameDisplay))

	// 合併所有內容
	fullContent := strings.Join(contentParts, "\n\n")

	// 構建單一 Section，包含所有內容
	section := discordgo.Section{
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{
				Content: fullContent,
			},
		},
	}

	// 如果有圖片，使用真實圖片；沒有圖片則使用占位符
	thumbnailURL := imageURL
	if strings.TrimSpace(thumbnailURL) == "" {
		thumbnailURL = utils.PlaceholderImageURL
	}

	section.Accessory = &discordgo.Thumbnail{
		Media: discordgo.UnfurledMediaItem{
			URL: thumbnailURL,
		},
	}

	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: fmt.Sprintf("# %s", gameTitle),
		},
		discordgo.Separator{Divider: &divider},
		section,
		discordgo.Separator{Divider: &divider},
		utils.MakeBackToHomeComponent(searchBrandVNDBCommandID, selectMenuCID.CacheID),
	}

	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchBrandColor,
			Components:  containerComponents,
		},
	}

	utils.InteractionRespondEditComplex(s, i, components)
}

// 批評空間

func erogsSearchBrandV2(s *discordgo.Session, i *discordgo.InteractionCreate) {
	common.SearchList(s, i, cache.ErogsBrandStore, "erogs查詢公司品牌", func() (*erogs.Brand, error) {
		keyword, err := utils.GetOptions(i, "keyword")
		if err != nil {
			return nil, err
		}
		return erogs.SearchBrandByKeyword([]string{keyword})
	}, func(cacheValue *erogs.Brand, page int, cacheID string) ([]discordgo.MessageComponent, error) {
		hasPlayedMap, inWishMap := getErogsUserPlayWishMaps(i)
		return buildSearchBrandErogsComponents(cacheValue, page, cacheID, hasPlayedMap, inWishMap)
	})
}

func erogsSearchBrandWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	common.ChangePage(s, i, pageCID, cache.ErogsBrandStore, func(cacheValue *erogs.Brand, page int, cacheID string) ([]discordgo.MessageComponent, error) {
		hasPlayedMap, inWishMap := getErogsUserPlayWishMaps(i)
		return buildSearchBrandErogsComponents(cacheValue, page, cacheID, hasPlayedMap, inWishMap)
	})
}

// getErogsUserPlayWishMaps 依互動取得使用者的已玩／願望清單對應的 GameErogsID set，供品牌頁顯示 ✅／❤️。
func getErogsUserPlayWishMaps(i *discordgo.InteractionCreate) (hasPlayedMap, inWishMap map[int]struct{}) {
	hasPlayedMap = make(map[int]struct{})
	inWishMap = make(map[int]struct{})
	userID := utils.GetUserID(i)
	if strings.TrimSpace(userID) == "" {
		return hasPlayedMap, inWishMap
	}
	if _, ok := store.UserStore[userID]; !ok {
		return hasPlayedMap, inWishMap
	}
	userHasPlayed, err := kurohelperdb.SelectUserHasPlayed(userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return hasPlayedMap, inWishMap
	}
	for _, item := range userHasPlayed {
		hasPlayedMap[item.GameErogsID] = struct{}{}
	}
	userInWish, err := kurohelperdb.SelectUserInWish(userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return hasPlayedMap, inWishMap
	}
	for _, item := range userInWish {
		inWishMap[item.GameErogsID] = struct{}{}
	}
	return hasPlayedMap, inWishMap
}

func buildSearchBrandErogsComponents(res *erogs.Brand, currentPage int, cacheID string, hasPlayedMap, inWishMap map[int]struct{}) ([]discordgo.MessageComponent, error) {
	if hasPlayedMap == nil {
		hasPlayedMap = make(map[int]struct{})
	}
	if inWishMap == nil {
		inWishMap = make(map[int]struct{})
	}
	totalItems := len(res.GameList)
	totalPages := (totalItems + searchBrandItemsPerPage - 1) / searchBrandItemsPerPage

	// 品牌標題（解散時加註）、官網／Twitter 連結
	brandTitle := res.BrandName
	if res.Lost {
		brandTitle += " (解散)"
	}
	linkLine := ""
	if strings.TrimSpace(res.URL) != "" {
		linkLine += fmt.Sprintf("[官網](%s) ", res.URL)
	}
	if strings.TrimSpace(res.Twitter) != "" {
		linkLine += fmt.Sprintf("[Twitter](https://x.com/%s) ", res.Twitter)
	}
	linkSection := ""
	if linkLine != "" {
		linkSection = linkLine + "\n"
	}
	headerContent := fmt.Sprintf("# %s\n%s遊戲筆數: **%d**\n✅: 已玩 ❤️: 願望清單\n⭐: 批評空間分數(中位數/樣本差) 📊:投票人數 📅: 發售日期", brandTitle, linkSection, totalItems)

	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: headerContent,
		},
		discordgo.Separator{Divider: &divider},
	}

	// 計算當前頁的範圍
	start := (currentPage - 1) * searchBrandItemsPerPage
	end := min(start+searchBrandItemsPerPage, totalItems)
	pagedResults := res.GameList[start:end]

	brandMenuItems := []utils.SelectMenuItem{}

	// 產生遊戲組件
	for idx, item := range pagedResults {
		itemNum := start + idx + 1
		var prefix string
		if _, exists := hasPlayedMap[item.ID]; exists {
			prefix += "✅"
		}
		if _, exists := inWishMap[item.ID]; exists {
			prefix += "❤️"
		}
		itemContent := prefix + fmt.Sprintf("**%d. %s**\n⭐**%d/%d** / 📊**%d**/📅**%s** (%s)", itemNum, item.GameName, item.Median, item.Stdev, item.Count2, item.SellDay, item.Model)

		thumbnailURL := ""
		if strings.TrimSpace(item.DMM) != "" {
			thumbnailURL = erogs.MakeDMMImageURL(item.DMM)
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

		brandMenuItems = append(brandMenuItems, utils.SelectMenuItem{
			Title: item.GameName + " (" + item.Category + ")",
			ID:    "e" + strconv.Itoa(item.ID),
		})
	}

	// 產生選單組件
	selectMenuComponents := utils.MakeSelectMenuComponent(brandMenuItems, searchBrandErogsCommandID, cacheID, "選擇遊戲查看詳細")

	// 產生翻頁組件
	pageComponents, err := utils.MakeChangePageComponent(searchBrandErogsCommandID, currentPage, totalPages, cacheID)
	if err != nil {
		return nil, err
	} else {
		containerComponents = append(containerComponents,
			discordgo.Separator{Divider: &divider},
			selectMenuComponents,
			pageComponents,
		)
	}

	// 組成完整組件回傳
	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchBrandColor,
			Components:  containerComponents,
		},
	}, nil
}
