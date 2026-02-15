package usercmd

import (
	"fmt"
	"kurohelper/cache"
	"kurohelper/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	kurohelperdb "kurohelper-db"
)

type userRecordDataCache struct {
	gameName string
	gameID   int
}

func RemoveHasPlayed(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.NewCID) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	userID := utils.GetUserID(i)

	if cid != nil {
		// get cache
		cacheValue, err := cache.UserInfoCache.Get(cid.GetCacheID())
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}
		userRecordDataCache := cacheValue.(userRecordDataCache)

		// 刪除
		kurohelperdb.DeleteUserHasPlayed(userID, userRecordDataCache.gameID)

		embed := &discordgo.MessageEmbed{
			Title: fmt.Sprintf("%s 刪除成功！", userRecordDataCache.gameName),
			Color: 0x7BA23F,
		}
		utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
	} else {
		opt, err := utils.GetOptions(i, "keyword")
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		data, err := kurohelperdb.FindUserHasPlayedByUserAndGameNameLike(userID, opt)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		idStr := uuid.New().String()
		cache.UserInfoCache.Set(idStr, userRecordDataCache{
			gameName: data.GameErogs.Name,
			gameID:   data.GameErogs.ID,
		})

		cidCommandName := utils.MakeCIDCommandName(i.ApplicationCommandData().Name, false, "")
		messageComponent := []discordgo.MessageComponent{utils.MakeCIDCommonComponent("✅", idStr, cidCommandName)}
		actionsRow := utils.MakeActionsRow(messageComponent)

		embed := &discordgo.MessageEmbed{
			Title: fmt.Sprintf("**%s**", data.GameErogs.Name),
			Color: 0x90B44B,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "確認",
					Value:  "你確定要刪除已玩嗎?",
					Inline: false,
				},
			},
		}
		utils.InteractionEmbedRespondForSelf(s, i, embed, actionsRow, true)
	}
}
