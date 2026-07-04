package commands

import (
	"kurohelper/internal/utils"
	kurohelperdb "kurohelperservice/db"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

type Helper struct{}

func (h *Helper) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "幫助",
		Description: "獲取機器人相關使用教學",
	}
}

func (h *Helper) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	serverLink, err := kurohelperdb.GetAppConfigByKey(kurohelperdb.Dbs, "SERVER_LINK")
	if err != nil {
		slog.Warn(err.Error())
		serverLink.ConfigValue = "目前群組連結不公開"
	}

	embed := &discordgo.MessageEmbed{
		Title: "幫助",
		Color: 0xF19483,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "使用說明/文檔",
				Value:  "https://docs.kurohelper.com/docs",
				Inline: true,
			},
			{
				Name:   "邀請至伺服器",
				Value:  "https://discord.com/oauth2/authorize?client_id=1418225729241612298&permissions=3941734153714752&integration_type=0&scope=bot",
				Inline: true,
			},
			{
				Name:   "聯繫我們/加入群組",
				Value:  serverLink.ConfigValue,
				Inline: true,
			},
		},
	}
	utils.InteractionEmbedRespond(s, i, embed, nil, false)
}
