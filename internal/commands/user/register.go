package user

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"kurohelper/internal/utils"

	kurohelperdb "kurohelperservice/db"
)

type Register struct{}

func (r *Register) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "註冊帳號",
		Description: "註冊KuroHelper網站帳號",
	}
}

func (r *Register) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	discordID := utils.GetUserID(i)
	user, err := kurohelperdb.EnsureDiscordUser(kurohelperdb.Dbs, discordID, utils.GetUsername(i))
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	_, err = kurohelperdb.GetUserAuthByUserID(kurohelperdb.Dbs, user.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		utils.HandleError(err, s, i)
		return
	}

	// 已有正式帳號（user_auth）
	if err == nil {
		embed := &discordgo.MessageEmbed{
			Title:       "你已經完成註冊",
			Color:       0xB481BB,
			Description: "目前帳號已綁定，不需要再次申辦",
		}
		utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
		return
	}

	// 虛擬帳號（user）已建立，尚未建立正式帳號（user_auth）：產生註冊連結
	registerID := uuid.NewString()
	if err := kurohelperdb.CreateRegisterCache(kurohelperdb.Dbs, registerID, discordID, 30*time.Minute); err != nil {
		utils.HandleError(err, s, i)
		return
	}

	registerURL, err := buildRegisterURL(registerID)
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}
	embed := &discordgo.MessageEmbed{
		Title:       "註冊連結已產生",
		Color:       0x7BA23F,
		Description: "請使用以下私人連結完成註冊（30 分鐘內有效）",
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "註冊連結",
				Value:  registerURL,
				Inline: false,
			},
		},
	}
	utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
}

func buildRegisterURL(registerID string) (string, error) {
	baseURL := strings.TrimSpace(os.Getenv("REGISTER_PAGE_BASE_URL"))
	if baseURL == "" {
		return "", errors.New("REGISTER_PAGE_BASE_URL is not set")
	}
	return fmt.Sprintf("%s/%s", strings.TrimRight(baseURL, "/"), registerID), nil
}
