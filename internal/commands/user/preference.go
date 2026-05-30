package user

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"kurohelper/internal/cache"
	"kurohelper/internal/cid"
	"kurohelper/internal/utils"
	kurohelperdb "kurohelperservice/db"
)

type PreferenceCache struct {
	PrivateGameData bool
	DiscordID       string
}

type Preference struct{}

const preferenceCommandName = "帳號設定"

func (p *Preference) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "帳號設定",
		Description: "調整帳號相關設定",
	}
}

func (p *Preference) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := utils.GetUserID(i)
	user, err := kurohelperdb.GetUserByDiscordID(kurohelperdb.Dbs, userID)
	if err != nil {
		utils.InteractionRespond(s, i, "查無此帳號")
		return
	}

	cacheID := uuid.New().String()
	cache.CIDV3Store.Set(cacheID, PreferenceCache{
		PrivateGameData: user.PrivateGameData,
		DiscordID:       userID,
	})

	privateGameDataLabel := "隱私遊戲資料"
	privateGameDataButtonLabel := "已關閉（公開個人建檔資料）"
	privateGameDataButtonStyle := discordgo.DangerButton
	if user.PrivateGameData {
		privateGameDataButtonLabel = "已啟用（隱藏個人建檔資料）"
		privateGameDataButtonStyle = discordgo.SuccessButton
	}

	userSection := discordgo.Section{
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: fmt.Sprintf("**帳號名稱**\n%s", user.Name)},
		},
	}
	privateGameDataSection := discordgo.Section{
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: fmt.Sprintf("**%s**", privateGameDataLabel)},
		},
		Accessory: discordgo.Button{
			Label:    privateGameDataButtonLabel,
			Style:    privateGameDataButtonStyle,
			CustomID: cid.MakeCIDV3(preferenceCommandName, cacheID),
		},
	}

	var discordUser *discordgo.User
	if i.Interaction != nil {
		discordUser = i.Interaction.User
		if discordUser == nil && i.Interaction.Member != nil {
			discordUser = i.Interaction.Member.User
		}
	}
	if discordUser != nil {
		userSection.Accessory = &discordgo.Thumbnail{
			Media: discordgo.UnfurledMediaItem{URL: utils.GetAvatarURL(discordUser)},
		}
	}

	divider := true
	preferenceColor := 0xB481BB
	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &preferenceColor,
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: "# 帳號設定"},
				discordgo.Separator{Divider: &divider},
				userSection,
				privateGameDataSection,
			},
		},
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		},
	}); err != nil {
		utils.InteractionRespond(s, i, "該功能目前異常，請稍後再嘗試")
	}
}

func (p *Preference) HandleComponentV2(s *discordgo.Session, i *discordgo.InteractionCreate, uuid string) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}); err != nil {
		utils.HandleError(err, s, i)
		return
	}

	cacheValue, err := cache.CIDV3Store.Get(uuid)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.WebhookEditRespond)
		return
	}

	cacheData, ok := cacheValue.(PreferenceCache)
	if !ok {
		utils.HandleErrorV2(fmt.Errorf("preference cache data type mismatch"), s, i, utils.WebhookEditRespond)
		return
	}

	currentDiscordID := utils.GetUserID(i)
	if cacheData.DiscordID != currentDiscordID {
		utils.HandleErrorV2(fmt.Errorf("only the original user can update preference"), s, i, utils.WebhookEditRespond)
		return
	}

	nextPrivateGameData := !cacheData.PrivateGameData
	if err := kurohelperdb.UpdateUserPrivateGameDataByDiscordID(kurohelperdb.Dbs, cacheData.DiscordID, nextPrivateGameData); err != nil {
		utils.HandleErrorV2(err, s, i, utils.WebhookEditRespond)
		return
	}

	successColor := 0x7BA23F
	utils.WebhookEditRespond(s, i, []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &successColor,
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: "✅ 設定更新成功"},
			},
		},
	})
}
