package user

import (
	"fmt"
	"kurohelper/internal/cache"
	kurohelpererrors "kurohelper/internal/errors"
	"kurohelper/internal/utils"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"gorm.io/gorm"

	kurohelperdb "kurohelperservice/db"
)

type RemoveUserGame struct{}

type removeUserGameCacheData struct {
	OwnerDiscordID string
	Game           kurohelperdb.UserGame
}

const removeUserGameCommandName = "刪除使用者遊戲資料"

const (
	removeModeHasPlayed = 1
	removeModeInWish    = 2
)

func (r *RemoveUserGame) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "刪除使用者遊戲資料",
		Description: "刪除個人建檔的遊戲資料",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "keyword",
				Description: "關鍵字",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "remove_mode",
				Description: "移除類型",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "刪除已玩", Value: "played"},
					{Name: "刪除最愛", Value: "inWish"},
				},
			},
		},
	}
}

func (r *RemoveUserGame) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	userID := utils.GetUserID(i)

	opt, err := utils.GetOptions(i, "keyword")
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}
	modeText, err := utils.GetOptions(i, "remove_mode")
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	removeMode := 0
	switch modeText {
	case "played":
		removeMode = removeModeHasPlayed
	case "inWish":
		removeMode = removeModeInWish
	default:
		utils.HandleError(gorm.ErrRecordNotFound, s, i)
		return
	}

	userGames, err := kurohelperdb.GetUserGameByDiscordID(kurohelperdb.Dbs, userID)
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	var target *kurohelperdb.UserGame
	for idx := range userGames {
		name := userGames[idx].GameErogs.Name
		if !strings.Contains(strings.ToLower(name), strings.ToLower(opt)) {
			continue
		}
		if removeMode == removeModeHasPlayed && userGames[idx].Status != 1 {
			continue
		}
		if removeMode == removeModeInWish && !userGames[idx].WishListMark {
			continue
		}
		target = &userGames[idx]
		break
	}
	if target == nil {
		utils.HandleError(gorm.ErrRecordNotFound, s, i)
		return
	}

	cacheID := uuid.New().String()
	cache.UserInfoCache.Set(cacheID, removeUserGameCacheData{
		OwnerDiscordID: userID,
		Game:           *target,
	})

	messageComponent := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    "✅",
			Style:    discordgo.PrimaryButton,
			CustomID: utils.MakeUserDataOperationCIDV2(removeUserGameCommandName, cacheID, removeMode),
		},
	}
	actionsRow := utils.MakeActionsRow(messageComponent)

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("**%s**", target.GameErogs.Name),
		Color: 0x90B44B,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "確認",
				Value:  "你確定要刪除此遊戲資料嗎?",
				Inline: false,
			},
		},
	}
	utils.InteractionEmbedRespondForSelf(s, i, embed, actionsRow, true)
}

func (r *RemoveUserGame) HandleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	if cid == nil {
		utils.HandleError(gorm.ErrRecordNotFound, s, i)
		return
	}

	userID := utils.GetUserID(i)
	if cid.GetBehaviorID() != utils.UserDataOperationBehavior {
		utils.HandleError(kurohelpererrors.ErrCIDBehaviorMismatch, s, i)
		return
	}
	userDataCID, err := cid.ToUserDataOperationCIDV2()
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	cacheValue, err := cache.UserInfoCache.Get(userDataCID.CacheID)
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}
	cacheData, ok := cacheValue.(removeUserGameCacheData)
	if !ok {
		utils.HandleError(fmt.Errorf("remove user game cache data type mismatch"), s, i)
		return
	}
	if cacheData.OwnerDiscordID != userID {
		utils.HandleError(fmt.Errorf("only the original user can confirm this deletion"), s, i)
		return
	}

	switch userDataCID.Value {
	case removeModeHasPlayed:
		if err := kurohelperdb.UpdateUserGameStatus(kurohelperdb.Dbs, cacheData.Game.UserID, cacheData.Game.GameErogsID, 0); err != nil {
			utils.HandleError(err, s, i)
			return
		}
	case removeModeInWish:
		if err := kurohelperdb.UpdateUserGameWishListMark(kurohelperdb.Dbs, cacheData.Game.UserID, cacheData.Game.GameErogsID, false); err != nil {
			utils.HandleError(err, s, i)
			return
		}
	default:
		utils.HandleError(gorm.ErrRecordNotFound, s, i)
		return
	}

	successText := ""
	if userDataCID.Value == removeModeHasPlayed {
		successText = "已玩移除成功！"
	}
	if userDataCID.Value == removeModeInWish {
		successText = "最愛移除成功！"
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("%s %s", cacheData.Game.GameErogs.Name, successText),
		Color: 0x7BA23F,
	}
	utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
	slog.Info("刪除使用者遊戲資料成功", "使用者ID", userID, "遊戲ID", cacheData.Game.GameErogsID, "遊戲名稱", cacheData.Game.GameErogs.Name, "移除模式", userDataCID.Value)
}
