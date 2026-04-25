package commands

import (
	"fmt"
	"kurohelper/internal/utils"
	"log/slog"
	"math/rand"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Define fortune result structure
type fortuneResult struct {
	Name        string
	Description string
	Probability int
	Color       int
}

// Define possible fortunes and their probabilities
// Total probability should be 100
var fortunes = []fortuneResult{
	{"大吉", "太幸運啦！今天將會是個超棒的一天！", 10, 0xEB4537}, // Red
	{"中吉", "很不錯的一天，可能有好事發生喔！", 20, 0xFA7B17},  // Orange
	{"小吉", "平穩順遂，享受生活中的小確幸吧！", 30, 0xF8C10F},  // Yellow
	{"吉", "順順利利，保持平常心就好！", 20, 0x36C159},      // Green
	{"末吉", "腳踏實地，總會有收穫的！", 10, 0x25A1F2},      // Blue
	{"凶", "出門在外多加小心，避免與人起衝突！", 8, 0x8C44F7},   // Purple
	{"大凶", "今日宜低調行事，凡事三思而後行！", 2, 0x1A1A1D},   // Black
}

type Fortune struct{}

func (f *Fortune) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "運勢",
		Description: "抽抽今天的運勢",
	}
}

func (f *Fortune) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Use time to seed the random number generator to ensure stability
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	r := rng.Intn(100)
	var selected fortuneResult

	cumulative := 0
	for _, f := range fortunes {
		cumulative += f.Probability
		if r < cumulative {
			selected = f
			break
		}
	}

	var user *discordgo.User
	if i.Member != nil && i.Member.User != nil {
		user = i.Member.User
	} else {
		user = i.User
	}
	avatarURL := utils.GetAvatarURL(user)

	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: "# 今天的運勢是...",
		},
		discordgo.Separator{Divider: &divider},
		discordgo.Section{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{
					Content: fmt.Sprintf("## **%s**\n%s", selected.Name, selected.Description),
				},
			},
			Accessory: &discordgo.Thumbnail{
				Media: discordgo.UnfurledMediaItem{
					URL: avatarURL,
				},
			},
		},
	}

	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &selected.Color,
			Components:  containerComponents,
		},
	}

	slog.Info(user.Username+"使用了運勢功能", "fortune", selected.Name, "guildID", i.GuildID)

	utils.InteractionRespondV2(s, i, components)
}
