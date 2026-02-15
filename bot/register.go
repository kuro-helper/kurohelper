package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// 註冊命令
func RegisterCommand(s *discordgo.Session) {
	var globalCmds []*discordgo.ApplicationCommand
	globalCmds = append(globalCmds, searchCommands()...)

	for _, cmd := range globalCmds {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd)
		if err != nil {
			logrus.Errorf("register global slash command failed: %s", err)
		}
	}
}

// 主要專用指令(全域)
func searchCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "查詢遊戲",
			Description: "根據關鍵字查詢遊戲資料",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "keyword",
					Description: "關鍵字",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "查詢資料庫選項",
					Description: "選擇查詢的資料庫",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "VNDB",
							Value: "1",
						},
						{
							Name:  "erogamescape",
							Value: "2",
						},
					},
				},
			},
		},
		{
			Name:        "查詢公司品牌",
			Description: "根據關鍵字查詢公司品牌資料(VNDB)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "keyword",
					Description: "關鍵字",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "查詢資料庫選項",
					Description: "選擇查詢的資料庫",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "VNDB",
							Value: "1",
						},
						{
							Name:  "erogamescape",
							Value: "2",
						},
					},
				},
			},
		},
		{
			Name:        "查詢音樂",
			Description: "根據關鍵字查詢音樂資料(ErogameScape)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "keyword",
					Description: "關鍵字",
					Required:    true,
				},
			},
		},
		{
			Name:        "查詢創作者",
			Description: "根據關鍵字查詢創作者資料(ErogameScape)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "keyword",
					Description: "關鍵字",
					Required:    true,
				},
			},
		},
		{
			Name:        "查詢角色",
			Description: "根據關鍵字查詢角色資料",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "keyword",
					Description: "關鍵字",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "查詢資料庫選項",
					Description: "選擇查詢的資料庫",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "VNDB",
							Value: "1",
						},
						// 資料庫資料太少且不齊全，所以暫停使用
						// {
						// 	Name:  "erogamescape",
						// 	Value: "2",
						// },
						{
							Name:  "Bangumi",
							Value: "3",
						},
					},
				},
			},
		},
	}
}

// 隨機相關指令(全域)
func randomCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "隨機遊戲",
			Description: "隨機一部Galgame",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "查詢資料庫選項",
					Description: "選擇查詢的資料庫",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "VNDB",
							Value: "1",
						},
						{
							Name:  "ymgal",
							Value: "2",
						},
					},
				},
			},
		},
		{
			Name:        "隨機角色",
			Description: "隨機一個Galgame角色(VNDB)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "隨機角色的身分",
					Description: "選擇隨機角色的身分",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "主角",
							Value: "1",
						},
						{
							Name:  "配角",
							Value: "2",
						},
					},
				},
			},
		},
	}
}

// 使用者相關指令(全域)
func userCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "個人資料",
			Description: "取得自己的個人資料(KuroHelper)",
		},
		{
			Name:        "加已玩",
			Description: "把遊戲加到已玩(ErogameScape)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "keyword",
					Description: "關鍵字",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "complete_date",
					Description: "遊玩結束日期",
					Required:    false,
				},
			},
		},
		{
			Name:        "加收藏",
			Description: "把遊戲加到收藏(ErogameScape)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "keyword",
					Description: "關鍵字",
					Required:    true,
				},
			},
		},
		{
			Name:        "刪除已玩",
			Description: "刪除個人建檔的已玩資料",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "keyword",
					Description: "關鍵字",
					Required:    true,
				},
			},
		},
		{
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
		},
	}
}

// vndb專用指令(全域)
func vndbCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "vndb統計資料",
			Description: "取得VNDB統計資料(VNDB)",
		},
	}
}

// 未分類指令(全域)
func commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "幫助",
			Description: "獲取機器人相關使用教學",
		},
	}

}
