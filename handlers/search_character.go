package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	kurohelpercore "kurohelper-core"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"

	"kurohelper/cache"
	kurohelperrerrors "kurohelper/errors"
	"kurohelper/navigator"
	"kurohelper/store"
	"kurohelper/utils"

	"kurohelper-core/vndb"

	"kurohelper-core/bangumi"
)

const (
	searchCharacterListItemsPerPage  = 10
	searchCharacterListVNDBCommandID = "H1"
)

func SearchCharacterV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid == nil {
		optDB, err := utils.GetOptions(i, "查詢資料庫選項")
		if err != nil && errors.Is(err, kurohelperrerrors.ErrOptionTranslateFail) {
			utils.HandleError(err, s, i)
			return
		}
		switch optDB {
		case "1":
			vndbSearchCharacterV2(s, i)
		case "3":
			bangumiSearchCharacter(s, i)
		default:
			// 預設走 vndb 列表
			vndbSearchCharacterV2(s, i)
		}
	} else {
		// 選擇不同行為的進入點
		switch (switchMode{cid.GetCommandID()[1], cid.GetBehaviorID()}) {
		case switchMode{'1', utils.PageBehavior}:
			vndbSearchCharacterWithCIDV2(s, i, cid)
		case switchMode{'1', utils.SelectMenuBehavior}:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			vndbSearchCharacterWithSelectMenuCIDV2(s, i, cid)
		case switchMode{'1', utils.BackToHomeBehavior}:
			navigator.BackToHome(s, i, cid.ToBackToHomeCIDV2(), cache.VndbCharacterListStore, buildSearchCharacterComponents)
		default:
			utils.HandleErrorV2(kurohelperrerrors.ErrCIDBehaviorMismatch, s, i, utils.InteractionRespondEditComplex)
		}
	}
}

func vndbSearchCharacterV2(s *discordgo.Session, i *discordgo.InteractionCreate) {
	navigator.SearchList(s, i, cache.VndbCharacterListStore, "vndb查詢角色列表", func() ([]vndb.CharacterSearchResponse, error) {
		keyword, err := utils.GetOptions(i, "keyword")
		if err != nil {
			return nil, err
		}
		return vndb.GetCharacterListByFuzzy(keyword)
	}, buildSearchCharacterComponents)
}

// buildSearchCharacterComponents 產生 VNDB 角色列表的 V2 元件
func buildSearchCharacterComponents(res []vndb.CharacterSearchResponse, currentPage int, cacheID string) ([]discordgo.MessageComponent, error) {
	totalItems := len(res)
	totalPages := (totalItems + searchCharacterListItemsPerPage - 1) / searchCharacterListItemsPerPage
	if totalPages == 0 {
		totalPages = 1
	}

	start := (currentPage - 1) * searchCharacterListItemsPerPage
	end := min(start+searchCharacterListItemsPerPage, totalItems)
	page := res[start:end]

	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: fmt.Sprintf("# 角色列表搜尋\n搜尋筆數: **%d**", totalItems),
		},
		discordgo.Separator{Divider: &divider},
	}

	menuItems := make([]utils.SelectMenuItem, 0, len(page))

	for idx, r := range page {
		nameData := r.Name
		if r.Original != "" {
			nameData = r.Original
		}
		sort.Slice(r.VNs, func(i, j int) bool {
			return vndb.RolePriority[r.VNs[i].Role] < vndb.RolePriority[r.VNs[j].Role]
		})
		var vnParts []string
		for i, vn := range r.VNs {
			if i >= 2 {
				break
			}
			title := vn.Title
			for _, t := range vn.Titles {
				if t.Main {
					title = t.Title
					break
				}
			}
			vnParts = append(vnParts, fmt.Sprintf("%s (%s)", title, vndb.Role[vn.Role]))
		}
		itemNum := start + idx + 1
		// 角色名稱與第一個遊戲同一行，其餘遊戲換行列出，避免名稱下方被渲染成新段落產生大間距
		var b strings.Builder
		fmt.Fprintf(&b, "**%d. %s**", itemNum, nameData)
		for i, p := range vnParts {
			if i == 0 {
				b.WriteString("　• ")
				b.WriteString(p)
			} else {
				b.WriteString("\n　• ")
				b.WriteString(p)
			}
		}
		itemContent := b.String()

		thumbnailURL := strings.TrimSpace(r.Image.URL)
		if thumbnailURL == "" {
			thumbnailURL = placeholderImageURL
		}

		containerComponents = append(containerComponents, discordgo.Section{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: itemContent},
			},
			Accessory: &discordgo.Thumbnail{
				Media: discordgo.UnfurledMediaItem{URL: thumbnailURL},
			},
		})

		menuItems = append(menuItems, utils.SelectMenuItem{Title: nameData, ID: r.ID})
	}

	pageComponents, err := utils.MakeChangePageComponent(searchCharacterListVNDBCommandID, currentPage, totalPages, cacheID)
	if err != nil {
		return nil, err
	}

	containerComponents = append(containerComponents,
		discordgo.Separator{Divider: &divider},
		utils.MakeSelectMenuComponent(menuItems, searchCharacterListVNDBCommandID, cacheID, "選擇角色查看詳情"),
		pageComponents,
	)

	color := 0xF8F8DF
	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &color,
			Components:  containerComponents,
		},
	}, nil
}

func vndbSearchCharacterWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	navigator.ChangePage(s, i, pageCID, cache.VndbCharacterListStore, buildSearchCharacterComponents)
}

// 查詢單一 VNDB 角色資料(有CID版本，從選單選擇)
func vndbSearchCharacterWithSelectMenuCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
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

	var res *vndb.CharacterSearchResponse
	res, err := cache.VndbCharacterStore.Get(selectMenuCID.Value)
	if err != nil {
		if errors.Is(err, kurohelpercore.ErrCacheLost) {
			logrus.WithField("guildID", i.GuildID).Infof("vndb查詢角色ID: %s", selectMenuCID.Value)
			res, err = vndb.GetCharacterByID(selectMenuCID.Value)
			if err != nil {
				utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
				return
			}
			cache.VndbCharacterStore.Set(selectMenuCID.Value, res)
		} else {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
			return
		}
	}

	// 處理回傳結構
	nameData := res.Name
	if res.Original != "" {
		nameData = fmt.Sprintf("%s (%s)", res.Original, res.Name)
	}
	heightData := "未收錄"
	weightData := "未收錄"
	BWHData := "未收錄"
	ageData := "未收錄"
	birthDayData := "未收錄"
	sexData := "未收錄"
	genderData := "未收錄"
	if res.Height != 0 {
		heightData = strconv.Itoa(res.Height) + "cm"
	}
	if res.Weight != 0 {
		weightData = strconv.Itoa(res.Weight) + "kg"
	}
	if res.Bust != 0 && res.Waist != 0 && res.Hips != 0 {
		BWHData = fmt.Sprintf("%d/%d/%d", res.Bust, res.Waist, res.Hips)
	}
	if res.Age != nil {
		ageData = strconv.Itoa(*res.Age)
	}
	if res.Birthday != [2]int{} {
		birthDayData = fmt.Sprintf("%d月%d號", res.Birthday[0], res.Birthday[1])
	}
	if res.Sex != [2]string{} {
		if res.Sex[0] != res.Sex[1] {
			sexData = fmt.Sprintf("%s/||%s||", vndb.Sex[res.Sex[0]], vndb.Sex[res.Sex[1]])
		} else {
			sexData = vndb.Sex[res.Sex[0]]
		}
	}
	if res.Gender != [2]string{} {
		if res.Gender[0] != res.Gender[1] {
			genderData = fmt.Sprintf("%s/||%s||", vndb.Gender[res.Gender[0]], vndb.Gender[res.Gender[1]])
		} else {
			genderData = vndb.Gender[res.Gender[0]]
		}
	}
	aliasesData := "未收錄"
	if len(res.Aliases) > 0 {
		aliasesData = strings.Join(res.Aliases, "/")
	}
	cvData := strings.Join(res.Vas, "/")
	if cvData == "" {
		cvData = "未收錄"
	}
	bloodTypeData := res.BloodType
	if bloodTypeData == "" {
		bloodTypeData = "未收錄"
	}
	descData := res.Description
	if descData == "" {
		descData = "無角色敘述"
	} else {
		descData = vndb.ConvertBBCodeToMarkdown(descData)
	}
	vnData := make([]string, 0, len(res.VNs))
	if len(res.VNs) == 0 {
		vnData = append(vnData, "未收錄")
	} else {
		sort.Slice(res.VNs, func(a, b int) bool {
			return vndb.RolePriority[res.VNs[a].Role] < vndb.RolePriority[res.VNs[b].Role]
		})
		for _, vn := range res.VNs {
			titleData := vn.Title
			for _, title := range vn.Titles {
				if title.Main {
					titleData = title.Title
					break
				}
			}
			if vn.Spoiler == 0 {
				vnData = append(vnData, fmt.Sprintf("%s (%s)", titleData, vndb.Role[vn.Role]))
			} else {
				vnData = append(vnData, fmt.Sprintf("||%s (%s)||", titleData, vndb.Role[vn.Role]))
			}
		}
	}

	// 構建 Components V2 格式（與 search_game_v2 vndbSearchGameWithSelectMenuCIDV2 版面一致）
	contentParts := []string{}
	contentParts = append(contentParts, fmt.Sprintf("**別名**\n%s", aliasesData))
	contentParts = append(contentParts, fmt.Sprintf("**CV**\n%s", cvData))
	contentParts = append(contentParts, fmt.Sprintf("**生日**\n%s", birthDayData))
	contentParts = append(contentParts, fmt.Sprintf("**生理性別**\n%s", sexData))
	contentParts = append(contentParts, fmt.Sprintf("**性別認同**\n%s", genderData))
	contentParts = append(contentParts, fmt.Sprintf("**身高/體重**\n%s / %s", heightData, weightData))
	contentParts = append(contentParts, fmt.Sprintf("**年齡**\n%s", ageData))
	contentParts = append(contentParts, fmt.Sprintf("**血型**\n%s", bloodTypeData))
	contentParts = append(contentParts, fmt.Sprintf("**三圍**\n%s", BWHData))
	contentParts = append(contentParts, fmt.Sprintf("**角色敘述**\n%s", descData))
	contentParts = append(contentParts, fmt.Sprintf("**登場於**\n%s", strings.Join(vnData, "\n")))

	fullContent := strings.Join(contentParts, "\n\n")

	section := discordgo.Section{
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: fullContent},
		},
	}

	thumbnailURL := strings.TrimSpace(res.Image.URL)
	userID := utils.GetUserID(i)
	if thumbnailURL != "" {
		if i.GuildID != "" {
			if _, ok := store.GuildDiscordAllowList[i.GuildID]; !ok {
				thumbnailURL = ""
			}
		} else {
			if _, ok := store.GuildDiscordAllowList[userID]; !ok {
				thumbnailURL = ""
			}
		}
	}
	if thumbnailURL == "" {
		thumbnailURL = placeholderImageURL
	}

	section.Accessory = &discordgo.Thumbnail{
		Media: discordgo.UnfurledMediaItem{URL: thumbnailURL},
	}

	divider := true
	searchCharacterColor := 0xF8F8DF
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: fmt.Sprintf("# %s", nameData),
		},
		discordgo.Separator{Divider: &divider},
		section,
		discordgo.Separator{Divider: &divider},
	}
	containerComponents = append(containerComponents, utils.MakeBackToHomeComponent(searchCharacterListVNDBCommandID, selectMenuCID.CacheID))

	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchCharacterColor,
			Components:  containerComponents,
		},
	}

	utils.InteractionRespondEditComplex(s, i, components)
}

// Bangumi查詢角色處理
func bangumiSearchCharacter(s *discordgo.Session, i *discordgo.InteractionCreate) {
	keyword, err := utils.GetOptions(i, "keyword")
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondV2)
		return
	}

	// 將 keyword 轉成 base64 作為快取鍵（無 CID 事件，直接用關鍵字對應實際資料）
	cacheKey := base64.RawURLEncoding.EncodeToString([]byte(keyword))

	var res *bangumi.Character
	cacheValue, err := cache.BangumiCharacterStore.Get(cacheKey)
	if err == nil {
		res = cacheValue
	} else {
		res, err = bangumi.GetCharacterByFuzzy(keyword)
		if err != nil {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondV2)
			return
		}
		logrus.WithField("guildID", i.GuildID).Infof("Bangumi查詢角色: %s", keyword)
		cache.BangumiCharacterStore.Set(cacheKey, res)
	}

	nameData := res.Name
	if res.NameCN != "" {
		nameData = fmt.Sprintf("%s (%s)", res.Name, res.NameCN)
	}
	aliasesData := "未收錄"
	if len(res.Aliases) > 0 {
		aliasesData = strings.Join(res.Aliases, "/")
	}
	cvData := strings.Join(res.CV, "/")
	if cvData == "" {
		cvData = "未收錄"
	}
	birthDayData := res.BirthDay
	if birthDayData == "" {
		birthDayData = "未收錄"
	}
	genderData := res.Gender
	if genderData == "" {
		genderData = "未收錄"
	}
	heightWeightData := fmt.Sprintf("%s / %s", res.Height, res.Weight)
	if res.Height == "" && res.Weight == "" {
		heightWeightData = "未收錄"
	}
	ageData := res.Age
	if ageData == "" {
		ageData = "未收錄"
	}
	bloodTypeData := res.BloodType
	if bloodTypeData == "" {
		bloodTypeData = "未收錄"
	}
	bwhData := res.BWH
	if bwhData == "" {
		bwhData = "未收錄"
	}
	descData := res.Summary
	if descData == "" {
		descData = "無角色敘述"
	}
	gameData := "未收錄"
	if len(res.Game) > 0 {
		gameData = strings.Join(res.Game, "\n")
	}
	otherData := "未收錄"
	if len(res.Other) > 0 {
		otherData = strings.Join(res.Other, "\n")
	}

	contentParts := []string{
		fmt.Sprintf("**別名**\n%s", aliasesData),
		fmt.Sprintf("**CV**\n%s", cvData),
		fmt.Sprintf("**生日**\n%s", birthDayData),
		fmt.Sprintf("**性別**\n%s", genderData),
		fmt.Sprintf("**身高/體重**\n%s", heightWeightData),
		fmt.Sprintf("**年齡**\n%s", ageData),
		fmt.Sprintf("**血型**\n%s", bloodTypeData),
		fmt.Sprintf("**三圍**\n%s", bwhData),
		fmt.Sprintf("**角色敘述**\n%s", descData),
		fmt.Sprintf("**登場於**\n%s", gameData),
		fmt.Sprintf("**其他**\n%s", otherData),
	}
	fullContent := strings.Join(contentParts, "\n\n")

	section := discordgo.Section{
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: fullContent},
		},
	}
	thumbnailURL := strings.TrimSpace(res.Image)
	userID := utils.GetUserID(i)
	if thumbnailURL != "" {
		if i.GuildID != "" {
			if _, ok := store.GuildDiscordAllowList[i.GuildID]; !ok {
				thumbnailURL = ""
			}
		} else {
			if _, ok := store.GuildDiscordAllowList[userID]; !ok {
				thumbnailURL = ""
			}
		}
	}
	if thumbnailURL == "" {
		thumbnailURL = placeholderImageURL
	}
	section.Accessory = &discordgo.Thumbnail{
		Media: discordgo.UnfurledMediaItem{URL: thumbnailURL},
	}

	divider := true
	searchCharacterColor := 0xF8F8DF
	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchCharacterColor,
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: fmt.Sprintf("# %s", nameData)},
				discordgo.Separator{Divider: &divider},
				section,
			},
		},
	}
	utils.InteractionRespondV2(s, i, components)
}
