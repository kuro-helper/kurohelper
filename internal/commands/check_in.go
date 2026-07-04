package commands

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"kurohelper/internal/utils"

	kurohelperdb "kurohelperservice/db"

	"github.com/bwmarrin/discordgo"
)

type CheckIn struct{}

type checkInFortune struct {
	Type        kurohelperdb.FortuneType
	Name        string
	Description string
	Probability int
	Color       int
}

var checkInFortunes = []checkInFortune{
	{kurohelperdb.FortuneTypeGreatBlessing, "大吉", "太幸運啦！今天將會是個超棒的一天！", 10, 0xEB4537},
	{kurohelperdb.FortuneTypeMiddleBlessing, "中吉", "很不錯的一天，可能有好事發生喔！", 20, 0xFA7B17},
	{kurohelperdb.FortuneTypeSmallBlessing, "小吉", "平穩順遂，享受生活中的小確幸吧！", 30, 0xF8C10F},
	{kurohelperdb.FortuneTypeBlessing, "吉", "順順利利，保持平常心就好！", 20, 0x36C159},
	{kurohelperdb.FortuneTypeFutureBlessing, "末吉", "腳踏實地，總會有收穫的！", 10, 0x25A1F2},
	{kurohelperdb.FortuneTypeBadLuck, "凶", "出門在外多加小心，避免與人起衝突！", 8, 0x8C44F7},
	{kurohelperdb.FortuneTypeGreatBadLuck, "大凶", "今日宜低調行事，凡事三思而後行！", 2, 0x1A1A1D},
}

func (c *CheckIn) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "簽到",
		Description: "每日簽到並抽取今天的運勢",
	}
}

func (c *CheckIn) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		slog.Error("failed to defer check-in interaction", "error", err, "guildID", i.GuildID)
		return
	}

	var discordUser *discordgo.User
	if i.Member != nil && i.Member.User != nil {
		discordUser = i.Member.User
	} else {
		discordUser = i.User
	}
	if discordUser == nil {
		utils.HandleErrorV2(fmt.Errorf("check-in: discord user not found"), s, i, utils.WebhookEditRespond)
		return
	}

	if err := kurohelperdb.EnsureDiscordUser(kurohelperdb.Dbs, discordUser.ID, discordUser.Username); err != nil {
		utils.HandleErrorV2(err, s, i, utils.WebhookEditRespond)
		return
	}
	user, err := kurohelperdb.GetUserByDiscordID(kurohelperdb.Dbs, discordUser.ID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.WebhookEditRespond)
		return
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	candidate := c.drawFortune(rng)
	checkIn, err := kurohelperdb.CheckInUser(kurohelperdb.Dbs, user.ID, candidate.Type, time.Now())
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.WebhookEditRespond)
		return
	}

	selected, ok := c.fortuneByType(checkIn.State.LastFortune)
	if !ok {
		utils.HandleErrorV2(fmt.Errorf("check-in: unknown persisted fortune type %d", checkIn.State.LastFortune), s, i, utils.WebhookEditRespond)
		return
	}

	header := "# 簽到成功！"
	if checkIn.AlreadyCheckedIn {
		header = "# 今天已經簽到過囉！"
	}

	avatarURL := utils.GetAvatarURL(discordUser)
	divider := true
	components := []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &selected.Color,
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: header},
				discordgo.Separator{Divider: &divider},
				discordgo.Section{
					Components: []discordgo.MessageComponent{
						discordgo.TextDisplay{
							Content: fmt.Sprintf("## 今日運勢：**%s**\n%s\n\n已連續簽到 **%d** 天", selected.Name, selected.Description, checkIn.State.CurrentStreak),
						},
					},
					Accessory: &discordgo.Thumbnail{
						Media: discordgo.UnfurledMediaItem{URL: avatarURL},
					},
				},
			},
		},
	}

	slog.Info(discordUser.Username+"使用了簽到功能",
		"fortune", selected.Name,
		"streak", checkIn.State.CurrentStreak,
		"alreadyCheckedIn", checkIn.AlreadyCheckedIn,
		"guildID", i.GuildID,
	)

	utils.WebhookEditRespond(s, i, components)
}

func (*CheckIn) drawFortune(rng *rand.Rand) checkInFortune {
	roll := rng.Intn(100)
	cumulative := 0
	for _, fortune := range checkInFortunes {
		cumulative += fortune.Probability
		if roll < cumulative {
			return fortune
		}
	}
	return checkInFortunes[len(checkInFortunes)-1]
}

func (*CheckIn) fortuneByType(fortuneType kurohelperdb.FortuneType) (checkInFortune, bool) {
	for _, fortune := range checkInFortunes {
		if fortune.Type == fortuneType {
			return fortune, true
		}
	}
	return checkInFortune{}, false
}
