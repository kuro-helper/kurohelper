package bot

import (
	"strings"

	"github.com/bwmarrin/discordgo"

	kurohelpererrors "kurohelper/errors"
	"kurohelper/handlers"
	"kurohelper/utils"
)

func OnInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		onInteractionApplicationCommand(s, i)
	case discordgo.InteractionMessageComponent:
		onInteractionMessageComponent(s, i)
	}
}

// 事件是InteractionApplicationCommand(使用斜線命令)的處理
func onInteractionApplicationCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.ApplicationCommandData().Name {
	case "幫助":
		go handlers.Helper(s, i)
	case "vndb統計資料":
		go handlers.VndbStats(s, i)
	case "查詢遊戲v2":
		go handlers.SearchGameV2(s, i, nil)
	case "查詢公司品牌v2":
		go handlers.SearchBrandV2(s, i, nil)
	case "查詢創作者":
		go handlers.SearchCreatorV2(s, i, nil)
	case "查詢音樂":
		go handlers.SearchMusicV2(s, i, nil)
	case "查詢角色":
		go handlers.SearchCharacterV2(s, i, nil)
	case "加已玩":
		go handlers.AddHasPlayed(s, i, nil)
	case "加收藏":
		go handlers.AddInWish(s, i, nil)
	case "隨機角色":
		go handlers.RandomCharacter(s, i)
	case "隨機遊戲":
		go handlers.RandomGame(s, i)
	case "個人資料":
		go handlers.GetUserinfo(s, i, nil)
	case "刪除已玩":
		go handlers.RemoveHasPlayed(s, i, nil)
	case "刪除收藏":
		go handlers.RemoveInWish(s, i, nil)
	}
}

// 事件是InteractionMessageComponent(點擊按鈕)的處理
func onInteractionMessageComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	cidStringSlice := strings.Split(i.MessageComponentData().CustomID, "|")
	// 舊版CID，其餘當成新版CID(V2)去嘗試解析
	if len(cidStringSlice) > 1 {
		cid := utils.NewCID(cidStringSlice)
		switch cid.GetCommandName() {
		case "加已玩":
			go handlers.AddHasPlayed(s, i, &cid)
		case "加收藏":
			go handlers.AddInWish(s, i, &cid)
		case "個人資料":
			go handlers.GetUserinfo(s, i, &cid)
		case "刪除已玩":
			go handlers.RemoveHasPlayed(s, i, &cid)
		case "刪除收藏":
			go handlers.RemoveInWish(s, i, &cid)
		}
	} else { // 新版CID(V2)
		cid, err := utils.ParseCIDV2(i.MessageComponentData().CustomID)
		if err != nil {
			utils.HandleError(kurohelpererrors.ErrCIDWrongFormat, s, i)
			return
		}

		// 下拉選單選擇遊戲時，修改Value值
		if cid.GetBehaviorID() == utils.SelectMenuBehavior {
			cid.ChangeValue(i.MessageComponentData().Values[0])
		}

		commandID := cid.GetCommandID()
		// CID不合法(commandID的部分)
		if len(commandID) < 2 || len(commandID) > 3 {
			utils.HandleError(kurohelpererrors.ErrCIDWrongFormat, s, i)
			return
		}
		switch commandID[0] {
		case 'G':
			go handlers.SearchGameV2(s, i, cid)
		case 'B':
			go handlers.SearchBrandV2(s, i, cid)
		case 'M':
			go handlers.SearchMusicV2(s, i, cid)
		case 'C':
			go handlers.SearchCreatorV2(s, i, cid)
		case 'H':
			go handlers.SearchCharacterV2(s, i, cid)
		default:
			utils.HandleError(kurohelpererrors.ErrCIDWrongFormat, s, i)
			return
		}
	}
}
