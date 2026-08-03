package kuro

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	servicekuro "kurohelperservice/airuntime"
)

func loadKuroMemorySource(session *discordgo.Session, memory *servicekuro.Memory) string {
	if session == nil || memory == nil {
		return "來源對話：沒有可用的 Discord 來源資訊。"
	}
	channelID := strings.TrimSpace(strings.TrimPrefix(memory.SourceChannelID, "kurohelper:"))
	messageID := strings.TrimSpace(memory.SourceRequestID)
	if channelID == "" || messageID == "" {
		return "來源對話：這條記憶沒有保存 Discord 訊息 ID。"
	}

	source, err := session.ChannelMessage(channelID, messageID)
	if err != nil || source == nil {
		return "來源對話：無法讀取原始 Discord 訊息，可能已被刪除或 Bot 沒有頻道權限。"
	}

	guildID := source.GuildID
	if guildID == "" {
		if channel, channelErr := session.Channel(channelID); channelErr == nil && channel != nil {
			guildID = channel.GuildID
		}
	}
	return formatKuroSourceMessageWithLink(source, guildID, channelID)
}

func formatKuroSourceMessageWithLink(source *discordgo.Message, guildID, channelID string) string {
	if source == nil {
		return "來源對話：無法讀取原始 Discord 訊息。"
	}
	lines := []string{
		"來源使用者訊息（即時讀取 Discord）：",
		formatKuroSourceMessage(source),
	}
	if guildID != "" && channelID != "" && source.ID != "" {
		lines = append(lines, fmt.Sprintf("[跳到來源訊息](https://discord.com/channels/%s/%s/%s)", guildID, channelID, source.ID))
	}
	return strings.Join(lines, "\n")
}

func formatKuroSourceMessage(message *discordgo.Message) string {
	name := "未知使用者"
	if message.Member != nil && strings.TrimSpace(message.Member.Nick) != "" {
		name = strings.TrimSpace(message.Member.Nick)
	} else if message.Author != nil {
		if strings.TrimSpace(message.Author.GlobalName) != "" {
			name = strings.TrimSpace(message.Author.GlobalName)
		} else if strings.TrimSpace(message.Author.Username) != "" {
			name = strings.TrimSpace(message.Author.Username)
		}
	}
	content := strings.TrimSpace(message.Content)
	for _, attachment := range message.Attachments {
		filename := strings.TrimSpace(attachment.Filename)
		if filename == "" {
			filename = "附件"
		}
		if content != "" {
			content += "\n"
		}
		content += "[附件：" + filename + "]"
	}
	if content == "" {
		content = "[無文字內容]"
	}
	content = truncateKuroText(content, 450)
	quoted := "> " + strings.ReplaceAll(content, "\n", "\n> ")
	return fmt.Sprintf("**%s**\n%s", name, quoted)
}

func appendKuroMemorySource(detail, source string) string {
	if strings.TrimSpace(source) == "" {
		return detail
	}
	return truncateKuroText(strings.TrimSpace(detail)+"\n\n"+strings.TrimSpace(source), 1900)
}
