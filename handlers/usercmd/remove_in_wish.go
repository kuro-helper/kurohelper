package usercmd

import (
	"fmt"

	kurohelpererrors "kurohelper/errors"
	"kurohelper/utils"

	"github.com/bwmarrin/discordgo"

	kurohelperdb "kurohelperservice/db"
)

type RemoveInWish struct{}

const removeInWishCommandName = "刪除收藏"

func (r *RemoveInWish) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "刪除收藏",
		Description: "刪除個人建檔的收藏資料",
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

func (r *RemoveInWish) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	r.HandleComponent(s, i, nil)
}

func (r *RemoveInWish) HandleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	userID := utils.GetUserID(i)

	if cid != nil {
		if cid.GetBehaviorID() != utils.UserDataOperationBehavior {
			utils.HandleError(kurohelpererrors.ErrCIDBehaviorMismatch, s, i)
			return
		}
		userDataCID, err := cid.ToUserDataOperationCIDV2()
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		// 先拿到該遊戲的名字
		data, err := kurohelperdb.GetUserInWishByUserAndGameID(kurohelperdb.Dbs, userID, userDataCID.Value)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		// 刪除
		if err := kurohelperdb.DeleteUserInWish(kurohelperdb.Dbs, userID, userDataCID.Value); err != nil {
			utils.HandleError(err, s, i)
			return
		}

		embed := &discordgo.MessageEmbed{
			Title: fmt.Sprintf("%s 刪除成功！", data.GameErogs.Name),
			Color: 0x7BA23F,
		}
		utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
	} else {
		opt, err := utils.GetOptions(i, "keyword")
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		data, err := kurohelperdb.GetUserInWishByUserAndGameNameLike(kurohelperdb.Dbs, userID, opt)
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		messageComponent := []discordgo.MessageComponent{
			discordgo.Button{
				Label: "✅",
				Style: discordgo.PrimaryButton,
				// 刪除不需要快取
				CustomID: utils.MakeUserDataOperationCIDV2(removeInWishCommandName, "", data.GameErogs.ID),
			},
		}
		actionsRow := utils.MakeActionsRow(messageComponent)

		embed := &discordgo.MessageEmbed{
			Title: fmt.Sprintf("**%s**", data.GameErogs.Name),
			Color: 0x90B44B,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "確認",
					Value:  "你確定要刪除收藏嗎?",
					Inline: false,
				},
			},
		}
		utils.InteractionEmbedRespondForSelf(s, i, embed, actionsRow, true)
	}
}
