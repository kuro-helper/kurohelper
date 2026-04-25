package bot

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"

	"kurohelper/internal/commands"
	"kurohelper/internal/commands/random"
	"kurohelper/internal/commands/search"
	"kurohelper/internal/commands/user"
	"kurohelper/internal/commands/vndb"
	kurohelpererrors "kurohelper/internal/errors"
	"kurohelper/internal/utils"
)

type SlashCommand interface {
	Definition() *discordgo.ApplicationCommand
	Handler(s *discordgo.Session, i *discordgo.InteractionCreate)
}

// 使用新版CID的介面
type ComponentV2Handler interface {
	HandleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2)
}

// 選擇性介面：只有需要自動補完的指令才實作此方法
type Autocompleter interface {
	Autocomplete(s *discordgo.Session, i *discordgo.InteractionCreate)
}

// 要使用的指令
var commandMap = map[string]SlashCommand{
	// 主要專用指令
	"查詢遊戲":   &search.SearchGame{},
	"查詢公司品牌": &search.SearchBrand{},
	"查詢創作者":  &search.SearchCreator{},
	"查詢角色":   &search.SearchCharacter{},
	"查詢音樂":   &search.SearchMusic{},
	"查詢歌手":   &search.SearchSinger{},
	// 隨機相關指令
	"隨機遊戲": &random.RandomGame{},
	"隨機角色": &random.RandomCharacter{},
	// 使用者相關指令
	"個人資料": &user.GetUserinfo{},
	"加已玩":  &user.AddHasPlayed{},
	"加收藏":  &user.AddInWish{},
	"刪除已玩": &user.RemoveHasPlayed{},
	"刪除收藏": &user.RemoveInWish{},
	// vndb專用指令
	"vndb統計資料": &vndb.VNDBStats{},
	// 未分類指令
	"運勢": &commands.Fortune{},
	"幫助": &commands.Helper{},
}

// 註冊命令
func RegisterCommand(s *discordgo.Session) {
	for n, cmd := range commandMap {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd.Definition())
		if err != nil {
			slog.Error(fmt.Sprintf("register %s command failed: %s", n, err.Error()))
		}
	}
}

func GetSlashCommand(name string) SlashCommand {
	cmd, ok := commandMap[name]
	if !ok {
		return nil
	}

	return cmd
}

func OnInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	// 一般事件
	case discordgo.InteractionApplicationCommand:
		if cmd := GetSlashCommand(i.ApplicationCommandData().Name); cmd != nil {
			go cmd.Handler(s, i)
		}
	// Autocomplete
	case discordgo.InteractionApplicationCommandAutocomplete:
		if cmd := GetSlashCommand(i.ApplicationCommandData().Name); cmd != nil {
			if auto, ok := cmd.(Autocompleter); ok {
				go auto.Autocomplete(s, i)
			} else {
				slog.Warn(i.ApplicationCommandData().Name + " 沒有實作Autocomplete")
				return
			}
		}

	case discordgo.InteractionMessageComponent:
		cid, err := utils.ParseCIDV2(i.MessageComponentData().CustomID)
		if err != nil {
			utils.HandleError(kurohelpererrors.ErrCIDWrongFormat, s, i)
			return
		}

		// 下拉選單選擇遊戲時，修改Value值
		if cid.GetBehaviorID() == utils.SelectMenuBehavior {
			cid.ChangeValue(i.MessageComponentData().Values[0])
		}

		commandName := cid.GetCommandName()
		if commandName == "" {
			utils.HandleError(kurohelpererrors.ErrCIDWrongFormat, s, i)
			return
		}

		cmd := GetSlashCommand(commandName)
		if cmd == nil {
			slog.Warn(commandName + " 沒有註冊SlashCommand")
			return
		}

		v2cmd, ok := cmd.(ComponentV2Handler)
		if !ok {
			slog.Warn(commandName + " 沒有實作ComponentV2Handler")
			return
		}

		go v2cmd.HandleComponent(s, i, cid)
	default:
		return
	}
}
