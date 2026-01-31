package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"kurohelper/cache"
	"kurohelper/store"
	"kurohelper/utils"
	"sort"
	"strings"

	kurohelpercore "kurohelper-core"

	"kurohelper-core/vndb"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	searchBrandItemsPerPage = 7
	searchBrandCommandID    = "B1"
)

var (
	searchBrandColor = 0x00AA90
)

// 查詢公司品牌Handler(新版API)
func SearchBrandV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid == nil {
		vndbSearchBrandV2(s, i)
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		switch cid.GetBehaviorID() {
		case utils.PageBehavior:
			vndbSearchBrandWithCIDV2(s, i, cid)
		case utils.SelectMenuBehavior:
			// 查單一遊戲資料
			vndbSearchBrandWithSelectMenuCIDV2(s, i, cid)
		case utils.BackToHomeBehavior:
			vndbSearchBrandWithBackToHomeCIDV2(s, i, cid)
		}
	}
}

// 查詢公司品牌
func vndbSearchBrandV2(s *discordgo.Session, i *discordgo.InteractionCreate) {
	keyword, err := utils.GetOptions(i, "keyword")
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondV2)
		return
	}

	idStr := uuid.New().String()

	// 將 keyword 轉成 base64 作為快取鍵
	cacheKey := base64.RawURLEncoding.EncodeToString([]byte(keyword))

	// 檢查快取是否存在
	cacheValue, err := cache.VndbBrandStore.Get(cacheKey)
	if err == nil {
		// 存入CID與關鍵字的對應快取
		cache.CIDStore.Set(idStr, cacheKey)

		// 快取存在，直接使用，不需要延遲傳送
		components, err := buildSearchBrandComponents(cacheValue, 1, idStr)
		if err != nil {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondV2)
			return
		}
		utils.InteractionRespondV2(s, i, components)
		return
	}

	// 快取不存在，需要查詢資料
	// 先發送延遲回應
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	logrus.WithField("interaction", i).Infof("vndb查詢公司品牌: %s", keyword)

	res, err := vndb.GetProducerByFuzzy(keyword, "")
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.WebhookEditRespond)
		return
	}

	// 將查詢結果存入快取
	cache.VndbBrandStore.Set(cacheKey, res)

	// 存入CID與關鍵字的對應快取
	cache.CIDStore.Set(idStr, cacheKey)

	components, err := buildSearchBrandComponents(res, 1, idStr)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.WebhookEditRespond)
		return
	}

	utils.WebhookEditRespond(s, i, components)
}

// 產生查詢公司品牌(有CID版本)
//
// 目前只有翻頁事件
func vndbSearchBrandWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid.GetBehaviorID() != utils.PageBehavior {
		utils.HandleErrorV2(errors.New("handlers: cid behavior id error"), s, i, utils.InteractionRespondEditComplex)
		return
	}

	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	cidCacheValue, err := cache.CIDStore.Get(pageCID.CacheID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	cacheValue, err := cache.VndbBrandStore.Get(cidCacheValue)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	components, err := buildSearchBrandComponents(cacheValue, pageCID.Value, pageCID.CacheID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	utils.InteractionRespondEditComplex(s, i, components)
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
						URL: placeholderImageURL,
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
	selectMenuComponents := utils.MakeSelectMenuComponent(searchBrandCommandID, cacheID, brandMenuItems)

	// 產生翻頁組件
	pageComponents, err := utils.MakeChangePageComponent(searchBrandCommandID, currentPage, totalPages, cacheID)
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

	characterMap := make(map[string]CharacterData) // map[characterID]CharacterData
	for _, va := range res.Results[0].Va {
		characterName := va.Character.Original
		if characterName == "" {
			characterName = va.Character.Name
		}
		for _, vn := range va.Character.Vns {
			if vn.ID == res.Results[0].ID {
				characterMap[va.Character.ID] = CharacterData{
					Name: characterName,
					Role: vn.Role,
				}
				break
			}
		}
	}

	// 將 map 轉為 slice 並排序
	characterList := make([]CharacterData, 0, len(characterMap))
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
		thumbnailURL = placeholderImageURL
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
		utils.MakeBackToHomeComponent(searchBrandCommandID, selectMenuCID.CacheID),
	}

	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchBrandColor,
			Components:  containerComponents,
		},
	}

	utils.InteractionRespondEditComplex(s, i, components)
}

func vndbSearchBrandWithBackToHomeCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid.GetBehaviorID() != utils.BackToHomeBehavior {
		utils.HandleErrorV2(errors.New("handlers: cid behavior id error"), s, i, utils.InteractionRespondEditComplex)
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	backToHomeCID := cid.ToBackToHomeCIDV2()

	cidCacheValue, err := cache.CIDStore.Get(backToHomeCID.CacheID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	cacheValue, err := cache.VndbBrandStore.Get(cidCacheValue)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	components, err := buildSearchBrandComponents(cacheValue, 1, backToHomeCID.CacheID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	utils.InteractionRespondEditComplex(s, i, components)
}
