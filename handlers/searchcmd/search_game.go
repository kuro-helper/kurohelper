package searchcmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"

	"kurohelper/cache"
	kurohelperrerrors "kurohelper/errors"
	"kurohelper/navigator"
	"kurohelper/store"
	"kurohelper/utils"
	"kurohelperservice"
	kurohelperdb "kurohelperservice/db"
	"kurohelperservice/provider/erogs"
	"kurohelperservice/provider/seiya"
	"kurohelperservice/provider/vndb"
	"kurohelperservice/provider/ymgal"

	"github.com/bwmarrin/discordgo"
	"github.com/siongui/gojianfan"
	"gorm.io/gorm"
)

const (
	searchGameListItemsPerPage = 10
	searchGameErogsCommandID   = "G2"
	searchGameVndbCommandID    = "G1"
)

var (
	searchGameListColor = 0xF8F8DF
	searchGameColor     = 0x04108e
)

type switchMode struct {
	OptDB      byte
	BehaviorID utils.BehaviorID
}

// 查詢遊戲Handler進入點
func SearchGameV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid == nil {
		optDB, err := utils.GetOptions(i, "查詢資料庫選項")
		if err != nil && errors.Is(err, kurohelperrerrors.ErrOptionTranslateFail) {
			utils.HandleError(err, s, i)
			return
		}
		switch optDB {
		case "1":
			navigator.SearchList(s, i, cache.VndbGameListStore, "vndb查詢遊戲列表", func() ([]vndb.GetVnIDUseListResponse, error) {
				keyword, err := utils.GetOptions(i, "keyword")
				if err != nil {
					return nil, err
				}
				return vndb.GetVnID(keyword)
			}, buildVndbSearchGameComponents)
		case "2":
			erogsSearchGameListV2(s, i)
		default:
			// 預設走批評空間
			erogsSearchGameListV2(s, i)
		}
	} else {
		// 選擇不同行為的進入點
		switch (switchMode{cid.GetCommandID()[1], cid.GetBehaviorID()}) {
		case switchMode{'1', utils.PageBehavior}:
			vndbSearchGameListWithCIDV2(s, i, cid)
		case switchMode{'2', utils.PageBehavior}:
			erogsSearchGameListWithCIDV2(s, i, cid)
		case switchMode{'1', utils.SelectMenuBehavior}:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			vndbSearchGameWithSelectMenuCIDV2(s, i, cid)
		case switchMode{'2', utils.SelectMenuBehavior}:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			erogsSearchGameWithSelectMenuCIDV2(s, i, cid, searchGameErogsCommandID)
		case switchMode{'1', utils.BackToHomeBehavior}:
			navigator.BackToHome(s, i, cid.ToBackToHomeCIDV2(), cache.VndbGameListStore, buildVndbSearchGameComponents)
		case switchMode{'2', utils.BackToHomeBehavior}:
			navigator.BackToHome(s, i, cid.ToBackToHomeCIDV2(), cache.ErogsGameListStore, buildSearchGameComponents)
		default:
			utils.HandleErrorV2(kurohelperrerrors.ErrCIDBehaviorMismatch, s, i, utils.InteractionRespondEditComplex)
		}
	}
}

// 查詢遊戲列表
func erogsSearchGameListV2(s *discordgo.Session, i *discordgo.InteractionCreate) {
	navigator.SearchList(s, i, cache.ErogsGameListStore, "erogs查詢遊戲列表", func() ([]erogs.GameList, error) {
		keyword, err := utils.GetOptions(i, "keyword")
		if err != nil {
			return nil, err
		}
		if utils.IsAllHanziOrDigit(keyword) && strings.EqualFold(os.Getenv("USE_YMGAL_OPTIMIZATION"), "true") {
			slog.Info("ymgal查詢遊戲(跳板)", "keyword", keyword, "guildID", i.GuildID)
			ymgalKeyword, ymgalErr := ymgalGetGameString(keyword)
			if ymgalErr != nil {
				slog.Warn(ymgalErr.Error(), "guildID", i.GuildID)
			}
			if strings.TrimSpace(ymgalKeyword) != "" {
				keyword = ymgalKeyword
			}
		}
		return erogs.SearchGameListByKeyword([]string{keyword, kurohelperservice.ZhTwToJp(keyword)})
	}, buildSearchGameComponents)
}

// 查詢遊戲列表(有CID版本)
func erogsSearchGameListWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	navigator.ChangePage(s, i, pageCID, cache.ErogsGameListStore, buildSearchGameComponents)
}

// 查詢單一遊戲資料(有CID版本，從選單選擇)
func erogsSearchGameWithSelectMenuCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2, cidNamePrefix string) {
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

	res, err := cache.ErogsGameStore.Get(selectMenuCID.Value)
	if err != nil {
		if errors.Is(err, kurohelperservice.ErrCacheLost) {
			slog.Info("erogs查詢遊戲", "gameID", selectMenuCID.Value, "guildID", i.GuildID)

			cleanStr := strings.TrimPrefix(selectMenuCID.Value, "E")
			cleanStr = strings.TrimPrefix(cleanStr, "e")
			erogsID, err := strconv.Atoi(cleanStr)
			if err != nil {
				utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
				return
			}

			res, err = erogs.SearchGameByID(erogsID)
			if err != nil {
				utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
				return
			}

			cache.ErogsGameStore.Set(selectMenuCID.Value, res)

		} else {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
			return
		}
	}

	// 處理使用者資訊
	userID := utils.GetUserID(i)
	var userData string
	_, err = kurohelperdb.GetUserHasPlayedByUserAndGameID(kurohelperdb.Dbs, userID, res.ID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
			return
		}
	} else {
		userData += "✅"
	}
	_, err = kurohelperdb.GetUserInWishByUserAndGameID(kurohelperdb.Dbs, userID, res.ID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
			return
		}
	} else {
		userData += "❤️"
	}

	// 獲取 VNDB 資料
	vndbRating := 0.0
	vndbVotecount := 0
	var resVndb *vndb.BasicResponse[vndb.GetVnUseIDResponse]
	if strings.TrimSpace(res.VndbId) != "" {
		resVndb, err = vndb.GetVNByID(res.VndbId)
		if err != nil {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
			return
		}
		vndbRating = resVndb.Results[0].Rating
		vndbVotecount = resVndb.Results[0].Votecount
	}

	// 處理 shubetu 資料
	shubetuData := make(map[int]map[int][]string) // map[shubetu_type]map[shubetu_detail]][]creator name + shubetu_detail_name

	for typeIdx := 1; typeIdx <= 6; typeIdx++ {
		shubetuData[typeIdx] = make(map[int][]string)
		for detailIdx := 1; detailIdx <= 3; detailIdx++ {
			shubetuData[typeIdx][detailIdx] = make([]string, 0)
		}
	}
	for _, shubetu := range res.CreatorShubetu {
		shubetuType := shubetu.ShubetuType
		detailType := shubetu.ShubetuDetailType
		creatorData := ""
		if shubetu.ShubetuDetailName == "" {
			creatorData = shubetu.CreatorName
		} else {
			creatorData = shubetu.CreatorName + " (" + shubetu.ShubetuDetailName + ")"
		}
		if shubetu.ShubetuType != 5 {
			shubetuData[shubetuType][1] = append(shubetuData[shubetuType][1], creatorData)
		} else {
			if detailType == 2 || detailType == 3 {
				shubetuData[shubetuType][2] = append(shubetuData[shubetuType][2], creatorData)
			} else {
				shubetuData[shubetuType][1] = append(shubetuData[shubetuType][1], creatorData)
			}
		}
	}

	// 處理其他資訊
	switch res.Okazu {
	case "true":
		res.Okazu = "拔作"
	case "false":
		res.Okazu = "非拔作"
	default:
		res.Okazu = ""
	}

	switch res.Erogame {
	case "true":
		res.Erogame = "18禁"
	case "false":
		res.Erogame = "全年齡"
	default:
		res.Erogame = ""
	}

	otherInfo := ""
	if res.Erogame == "" && res.Okazu == "" {
		otherInfo = "無"
	} else if res.Erogame == "" || res.Okazu == "" {
		otherInfo = res.Erogame + res.Okazu
	} else {
		otherInfo = res.Okazu + " / " + res.Erogame
	}

	// 處理排名和顏色
	junni := 0x04108e
	rank := ""
	if res.Junni == 0 || res.Junni > 500 {
		junni = 0x04108e // Default
	} else if res.Junni <= 50 {
		junni = 0xFFD700 // Gold
		rank = "批評空間 TOP 50"
	} else if res.Junni <= 100 {
		junni = 0xC0C0C0 // Silver
		rank = "批評空間 TOP 100"
	} else {
		junni = 0xCD7F32 // Bronze
		rank = "批評空間 TOP 500"
	}

	// 用批評空間回來的遊戲名對誠也做模糊搜尋
	seiyaURL := seiya.GetGuideURL(res.Gamename)
	if seiyaURL != "" {
		rank += "  " + fmt.Sprintf("[誠也攻略](%s)", seiyaURL)
	}
	erogsURL := "https://erogamescape.dyndns.org/~ap2/ero/toukei_kaiseki/game.php?game=" + fmt.Sprint(res.ID)
	rank += "  " + fmt.Sprintf("[批評空間](%s)", erogsURL)
	if res.VndbId != "" {
		vndbURL := "https://vndb.org/" + res.VndbId
		rank += "  " + fmt.Sprintf("[VNDB](%s)", vndbURL)
	}

	vndbData := "無"
	if vndbVotecount != 0 {
		vndbData = fmt.Sprintf("%.1f/%d", vndbRating, vndbVotecount)
	}

	// 過濾圖片 - 使用 DMM 字段
	imageURL := ""
	if strings.TrimSpace(res.DMM) != "" {
		imageURL = erogs.MakeDMMImageURL(res.DMM)
		// 檢查是否允許顯示圖片
		if i.GuildID != "" {
			// guild
			if _, ok := store.GuildDiscordAllowList[i.GuildID]; !ok {
				imageURL = ""
			}
		} else {
			// DM
			if _, ok := store.GuildDiscordAllowList[userID]; !ok {
				imageURL = ""
			}
		}
	}

	// 構建 Components V2 格式
	divider := true
	contentParts := []string{}

	// 品牌名稱
	if strings.TrimSpace(res.BrandName) != "" {
		contentParts = append(contentParts, fmt.Sprintf("**品牌名稱**\n%s", res.BrandName))
	}

	// 排名和連結
	if strings.TrimSpace(rank) != "" {
		contentParts = append(contentParts, rank)
	}

	// 劇本
	if len(shubetuData[2][1]) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**劇本**\n%s", strings.Join(shubetuData[2][1], " / ")))
	}

	// 原畫
	if len(shubetuData[1][1]) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**原畫**\n%s", strings.Join(shubetuData[1][1], " / ")))
	}

	// 主角群CV
	if len(shubetuData[5][1]) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**主角群CV**\n%s", strings.Join(shubetuData[5][1], " / ")))
	}

	// 配角群CV
	if len(shubetuData[5][2]) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**配角群CV**\n%s", strings.Join(shubetuData[5][2], " / ")))
	}

	// 歌手
	if len(shubetuData[6][1]) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**歌手**\n%s", strings.Join(shubetuData[6][1], " / ")))
	}

	// 音樂
	if len(shubetuData[3][1]) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**音樂**\n%s", strings.Join(shubetuData[3][1], " / ")))
	}

	// 分數資訊
	evaluationText := fmt.Sprintf("**批評空間分數/樣本數**\n%s / %s", res.Median, res.TokutenCount)
	vndbText := fmt.Sprintf("**vndb分數/樣本數**\n%s", vndbData)
	contentParts = append(contentParts, evaluationText, vndbText)

	// 遊玩時數
	if strings.TrimSpace(res.TotalPlayTimeMedian) != "" {
		contentParts = append(contentParts, fmt.Sprintf("**遊玩時數**\n%s", res.TotalPlayTimeMedian))
	}

	// 開始理解遊戲樂趣時數
	if strings.TrimSpace(res.TimeBeforeUnderstandingFunMedian) != "" {
		contentParts = append(contentParts, fmt.Sprintf("**開始理解遊戲樂趣時數**\n%s", res.TimeBeforeUnderstandingFunMedian))
	}

	// 發行機種
	if strings.TrimSpace(res.Model) != "" {
		contentParts = append(contentParts, fmt.Sprintf("**發行機種**\n%s", res.Model))
	}

	// 類型
	if strings.TrimSpace(res.Genre) != "" {
		contentParts = append(contentParts, fmt.Sprintf("**類型**\n%s", res.Genre))
	}

	// 其他資訊
	contentParts = append(contentParts, fmt.Sprintf("**其他資訊**\n%s", otherInfo))

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
			Content: fmt.Sprintf("# %s**%s(%s)**", userData, res.Gamename, res.SellDay),
		},
		discordgo.Separator{Divider: &divider},
		section,
		discordgo.Separator{Divider: &divider},
	}

	containerComponents = append(containerComponents, utils.MakeBackToHomeComponent(cidNamePrefix, selectMenuCID.CacheID))

	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &junni,
			Components:  containerComponents,
		},
	}

	utils.InteractionRespondEditComplex(s, i, components)
}

// 產生查詢遊戲列表的Components
func buildSearchGameComponents(res []erogs.GameList, currentPage int, cacheID string) ([]discordgo.MessageComponent, error) {
	totalItems := len(res)
	totalPages := (totalItems + searchGameListItemsPerPage - 1) / searchGameListItemsPerPage

	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: fmt.Sprintf("# 遊戲搜尋\n搜尋筆數: **%d**\n⭐: 批評空間分數 📊: 投票人數 ⏱️: 遊玩時數 🥰: 開始理解遊戲樂趣時數", totalItems),
		},
		discordgo.Separator{Divider: &divider},
	}

	// 計算當前頁的範圍
	start := (currentPage - 1) * searchGameListItemsPerPage
	end := min(start+searchGameListItemsPerPage, totalItems)
	pagedResults := res[start:end]

	gameMenuItems := []utils.SelectMenuItem{}

	// 產生遊戲列表組件
	for idx, r := range pagedResults {
		itemNum := start + idx + 1
		itemContent := fmt.Sprintf("**%d. %s (%s)**\n⭐ **%s** / 📊 **%s**", itemNum, r.Name, r.Category, r.Median, r.TokutenCount)
		if strings.TrimSpace(r.TotalPlayTimeMedian) != "" {
			itemContent += fmt.Sprintf(" / ⏱️ **%s**", r.TotalPlayTimeMedian)
		}
		if strings.TrimSpace(r.TimeBeforeUnderstandingFunMedian) != "" {
			itemContent += fmt.Sprintf(" / 🥰 **%s**", r.TimeBeforeUnderstandingFunMedian)
		}

		// 處理圖片 URL
		thumbnailURL := ""
		if strings.TrimSpace(r.DMM) != "" {
			thumbnailURL = erogs.MakeDMMImageURL(r.DMM)
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
			Title: r.Name + " (" + r.Category + ")",
			ID:    "e" + strconv.Itoa(r.ID),
		})
	}

	// 產生選單組件
	selectMenuComponents := utils.MakeSelectMenuComponent(gameMenuItems, searchGameErogsCommandID, cacheID, "選擇遊戲查看詳細")

	// 產生翻頁組件
	pageComponents, err := utils.MakeChangePageComponent(searchGameErogsCommandID, currentPage, totalPages, cacheID)
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
			AccentColor: &searchGameListColor,
			Components:  containerComponents,
		},
	}, nil
}

// 月幕查詢遊戲名稱處理
func ymgalGetGameString(keyword string) (string, error) {
	slog.Debug("ymgal查詢遊戲", "keyword", keyword)

	searchGameRes, err := ymgal.SearchGame(gojianfan.T2S(keyword))
	if err != nil {
		return "", err
	}

	if len(searchGameRes.Result) == 0 {
		return "", kurohelperservice.ErrSearchNoContent
	}

	sort.Slice(searchGameRes.Result, func(i, j int) bool {
		return searchGameRes.Result[i].Weights > searchGameRes.Result[j].Weights
	})

	return searchGameRes.Result[0].Name, nil
}

// 查詢 VNDB 遊戲列表(有CID版本)
func vndbSearchGameListWithCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	pageCID, err := cid.ToPageCIDV2()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	navigator.ChangePage(s, i, pageCID, cache.VndbGameListStore, buildVndbSearchGameComponents)
}

// 查詢單一 VNDB 遊戲資料(有CID版本，從選單選擇)
func vndbSearchGameWithSelectMenuCIDV2(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
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

	res, err := cache.VndbGameStore.Get(selectMenuCID.Value)
	if err != nil {
		if errors.Is(err, kurohelperservice.ErrCacheLost) {
			slog.Info("vndb查詢遊戲", "vnID", selectMenuCID.Value, "guildID", i.GuildID)

			res, err = vndb.GetVNByID(selectMenuCID.Value)
			if err != nil {
				utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
				return
			}

			cache.VndbGameStore.Set(selectMenuCID.Value, res)

		} else {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
			return
		}
	}

	// 處理回傳結構
	gameTitle := res.Results[0].Alttitle
	if strings.TrimSpace(gameTitle) == "" {
		gameTitle = res.Results[0].Title
	}

	brandTitle := ""
	if len(res.Results[0].Developers) > 0 {
		brandTitle = res.Results[0].Developers[0].Original
		if strings.TrimSpace(brandTitle) != "" {
			brandTitle += fmt.Sprintf("(%s)", res.Results[0].Developers[0].Name)
		} else {
			brandTitle = res.Results[0].Developers[0].Name
		}
	}

	// staff block
	var scenario []string
	var art []string
	var songs []string

	for _, staff := range res.Results[0].Staff {
		staffName := staff.Original
		if staffName == "" {
			staffName = staff.Name
		}
		tmpAlias := ""
		if len(staff.Aliases) > 0 {
			aliases := make([]string, 0, len(staff.Aliases))
			for _, alias := range staff.Aliases {
				if alias.IsMain {
					staffName = alias.Name
				} else {
					aliases = append(aliases, alias.Name)
				}
			}
			if len(aliases) > 0 {
				tmpAlias = "(" + strings.Join(aliases, ", ") + ")"
			}
		}

		staffDisplay := staffName
		if tmpAlias != "" {
			staffDisplay += " " + tmpAlias
		}

		switch staff.Role {
		case "scenario":
			scenario = append(scenario, staffDisplay)
		case "art":
			art = append(art, staffDisplay)
		case "songs":
			songs = append(songs, staffDisplay)
		}
	}

	// character block
	characterMap := make(map[string]utils.CharacterData)
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

	// 構建 Components V2 格式
	divider := true
	contentParts := []string{}

	// 品牌名稱
	if strings.TrimSpace(brandTitle) != "" {
		contentParts = append(contentParts, fmt.Sprintf("**品牌(公司)名稱**\n%s", brandTitle))
	}

	// 劇本
	if len(scenario) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**劇本**\n%s", strings.Join(scenario, "\n")))
	}

	// 美術
	if len(art) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**美術**\n%s", strings.Join(art, "\n")))
	}

	// 音樂
	if len(songs) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**音樂**\n%s", strings.Join(songs, "\n")))
	}

	// 評價資訊
	evaluationText := fmt.Sprintf("**評價(平均/貝式平均/樣本數)**\n%.1f / %.1f / %d",
		res.Results[0].Average, res.Results[0].Rating, res.Results[0].Votecount)
	contentParts = append(contentParts, evaluationText)

	// 遊玩時數
	if res.Results[0].LengthMinutes > 0 {
		lengthText := fmt.Sprintf("**平均遊玩時數/樣本數**\n%d(H) / %d",
			res.Results[0].LengthMinutes/60, res.Results[0].LengthVotes)
		contentParts = append(contentParts, lengthText)
	}

	// 角色列表
	if len(characters) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**角色列表**\n%s", strings.Join(characters, " / ")))
	}

	// 相關遊戲
	relationsGameDisplay := strings.Join(relationsGame, ", ")
	if strings.TrimSpace(relationsGameDisplay) == "" {
		relationsGameDisplay = "無"
	}
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

	// 處理圖片
	thumbnailURL := res.Results[0].Image.Url
	// 過濾色情/暴力圖片
	if res.Results[0].Image.Sexual >= 1 || res.Results[0].Image.Violence >= 1 {
		thumbnailURL = ""
		slog.Info("封面已過濾圖片顯示", "gameTitle", gameTitle, "guildID", i.GuildID)
	}

	// 檢查是否允許顯示圖片
	userID := utils.GetUserID(i)
	if strings.TrimSpace(thumbnailURL) != "" {
		if i.GuildID != "" {
			// guild
			if _, ok := store.GuildDiscordAllowList[i.GuildID]; !ok {
				thumbnailURL = ""
			}
		} else {
			// DM
			if _, ok := store.GuildDiscordAllowList[userID]; !ok {
				thumbnailURL = ""
			}
		}
	}

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
	}

	containerComponents = append(containerComponents, utils.MakeBackToHomeComponent(searchGameVndbCommandID, selectMenuCID.CacheID))

	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &searchGameColor,
			Components:  containerComponents,
		},
	}

	utils.InteractionRespondEditComplex(s, i, components)
}

// 產生查詢 VNDB 遊戲列表的Components
func buildVndbSearchGameComponents(res []vndb.GetVnIDUseListResponse, currentPage int, cacheID string) ([]discordgo.MessageComponent, error) {
	totalItems := len(res)
	totalPages := (totalItems + searchGameListItemsPerPage - 1) / searchGameListItemsPerPage

	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: fmt.Sprintf("# VNDB 遊戲搜尋\n搜尋筆數: **%d**\n⭐: VNDB分數 📊: 投票人數 ⏱️: 遊玩時數", totalItems),
		},
		discordgo.Separator{Divider: &divider},
	}

	// 計算當前頁的範圍
	start := (currentPage - 1) * searchGameListItemsPerPage
	end := min(start+searchGameListItemsPerPage, totalItems)
	pagedResults := res[start:end]

	gameMenuItems := []utils.SelectMenuItem{}

	// 產生遊戲列表組件
	for idx, r := range pagedResults {
		itemNum := start + idx + 1
		var title string
		if strings.TrimSpace(r.Alttitle) != "" {
			title = r.Alttitle
		} else {
			title = r.Title
		}

		var ratingStr string
		if r.Average != nil {
			ratingStr = fmt.Sprintf("%.1f", *r.Average)
		} else {
			ratingStr = "無"
		}
		if r.Rating != nil {
			ratingStr += fmt.Sprintf("/%.1f", *r.Rating)
		} else {
			ratingStr += "/無"
		}

		lengthHour := "無"
		if r.LengthMinutes != nil {
			lengthHour = fmt.Sprintf("%.1fh", float64(*r.LengthMinutes)/60.0)
		}

		itemContent := fmt.Sprintf("**%d. %s**\n⭐ **%s** 📊 **%d** ⏱️ **%s**", itemNum, title, ratingStr, r.VoteCount, lengthHour)

		// // 處理圖片 URL
		var thumbnailURL string
		if r.Image != nil && strings.TrimSpace(r.Image.Thumbnail) != "" {
			thumbnailURL = r.Image.Thumbnail
		} else {
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
			Title: r.Title,
			ID:    r.ID,
		})
	}

	// 產生選單組件
	selectMenuComponents := utils.MakeSelectMenuComponent(gameMenuItems, searchGameVndbCommandID, cacheID, "選擇遊戲查看詳細")

	// 產生翻頁組件
	pageComponents, err := utils.MakeChangePageComponent(searchGameVndbCommandID, currentPage, totalPages, cacheID)
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
			AccentColor: &searchGameListColor,
			Components:  containerComponents,
		},
	}, nil
}

func HandleGameAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	// 找出目前使用者正在打字的那個選項
	var focusedOption *discordgo.ApplicationCommandInteractionDataOption
	for _, opt := range data.Options {
		if opt.Focused {
			focusedOption = opt
			break
		}
	}

	if focusedOption == nil {
		return
	}

	// 取得目前輸入的文字
	userInput := focusedOption.StringValue()
	userInput = strings.ToLower(userInput)
	queryRunes := []rune(userInput)

	if len(queryRunes) < 2 {
		return
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	if len(erogs.GamesName) == 0 {
		slog.Warn("ErogsGameAutoComplete has not been initialized...")
		return
	}

	targetIndices, ok := erogs.InvertedIndex[queryRunes[0]]
	if !ok {
		return
	}

	limit := 15

	for _, idx := range targetIndices {
		name := erogs.GamesName[idx]
		if strings.Contains(strings.ToLower(name), userInput) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  name,
				Value: name,
			})
		}

		if len(choices) >= limit {
			break
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}
