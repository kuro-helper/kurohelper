package kuro

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"

	kurosvc "kurohelperservice/kuro"
)

func handleKuroAccessCommand(session *discordgo.Session, event *discordgo.MessageCreate, command kuroTextCommand) bool {
	switch command.Name {
	case "channel-list":
		if len(command.Args) > 1 || (len(command.Args) == 1 && !validDiscordID(command.Args[0])) {
			sendKuroCommandMessage(session, event.ChannelID, "用法：小黑 /channel-list [群組ID]")
			return true
		}
		guildID := ""
		if len(command.Args) == 1 {
			guildID = command.Args[0]
		}
		messages, err := buildKuroChannelList(session, guildID)
		if err != nil {
			sendKuroCommandMessage(session, event.ChannelID, "讀取 Discord 頻道清單失敗："+err.Error())
			return true
		}
		for _, message := range messages {
			sendKuroCommandMessage(session, event.ChannelID, message)
		}
		return true

	case "channel-add", "channel-remove":
		if len(command.Args) != 1 || !validDiscordID(command.Args[0]) {
			sendKuroCommandMessage(session, event.ChannelID, fmt.Sprintf("用法：小黑 /%s <頻道ID>", command.Name))
			return true
		}
		channel, err := session.Channel(command.Args[0])
		if err != nil || channel == nil || channel.GuildID == "" || !isKuroTextChannel(channel.Type) {
			sendKuroCommandMessage(session, event.ChannelID, "找不到 Bot 可存取的 Discord 文字頻道。")
			return true
		}
		enabled := command.Name == "channel-add"
		if enabled {
			allowed, reason, permissionErr := kuroChannelSendAllowed(session, channel)
			if permissionErr != nil {
				sendKuroCommandMessage(session, event.ChannelID, "無法確認 Bot 在該頻道的有效權限，請稍後再試。")
				return true
			}
			if !allowed {
				sendKuroCommandMessage(session, event.ChannelID, "無法加入該頻道："+reason+"。")
				return true
			}
		}
		if err := kurosvc.SetAccessRule(kurosvc.AccessScopeChannel, channel.ID, enabled); err != nil {
			sendKuroCommandMessage(session, event.ChannelID, "更新頻道存取規則失敗，請稍後再試。")
			return true
		}
		UpdateAccessOverride(kurosvc.AccessScopeChannel, channel.ID, enabled)
		action := "已加入"
		if !enabled {
			action = "已移除"
		}
		sendKuroCommandMessage(session, event.ChannelID, fmt.Sprintf("%s Kuro 對話頻道：#%s（`%s`）。", action, channel.Name, channel.ID))
		return true

	case "guild-disable", "guild-enable":
		if len(command.Args) != 1 || !validDiscordID(command.Args[0]) {
			sendKuroCommandMessage(session, event.ChannelID, fmt.Sprintf("用法：小黑 /%s <群組ID>", command.Name))
			return true
		}
		guild, err := session.Guild(command.Args[0])
		if err != nil || guild == nil {
			sendKuroCommandMessage(session, event.ChannelID, "找不到 Bot 已加入的 Discord 群組。")
			return true
		}
		if command.Name == "guild-disable" {
			if err := kurosvc.SetAccessRule(kurosvc.AccessScopeGuild, guild.ID, false); err != nil {
				sendKuroCommandMessage(session, event.ChannelID, "更新群組存取規則失敗，請稍後再試。")
				return true
			}
			UpdateAccessOverride(kurosvc.AccessScopeGuild, guild.ID, false)
			sendKuroCommandMessage(session, event.ChannelID, fmt.Sprintf("已在群組「%s」（`%s`）停用 Kuro 對話；管理指令仍可使用。", guild.Name, guild.ID))
			return true
		}
		if err := kurosvc.DeleteAccessRule(kurosvc.AccessScopeGuild, guild.ID); err != nil {
			sendKuroCommandMessage(session, event.ChannelID, "更新群組存取規則失敗，請稍後再試。")
			return true
		}
		DeleteAccessOverride(kurosvc.AccessScopeGuild, guild.ID)
		sendKuroCommandMessage(session, event.ChannelID, fmt.Sprintf("已在群組「%s」（`%s`）恢復 Kuro 對話，頻道規則會重新生效。", guild.Name, guild.ID))
		return true
	}
	return false
}

func buildKuroChannelList(session *discordgo.Session, guildFilter string) ([]string, error) {
	guilds := append([]*discordgo.Guild(nil), session.State.Guilds...)
	if guildFilter != "" {
		guild, err := session.Guild(guildFilter)
		if err != nil {
			return nil, fmt.Errorf("找不到指定群組")
		}
		guilds = []*discordgo.Guild{guild}
	}
	sort.Slice(guilds, func(i, j int) bool {
		if guilds[i].Name == guilds[j].Name {
			return guilds[i].ID < guilds[j].ID
		}
		return guilds[i].Name < guilds[j].Name
	})

	lines := []string{"Bot 可存取的 Discord 文字頻道："}
	for _, guild := range guilds {
		if guild == nil {
			continue
		}
		channels := append([]*discordgo.Channel(nil), guild.Channels...)
		if len(channels) == 0 {
			fetched, err := session.GuildChannels(guild.ID)
			if err != nil {
				continue
			}
			channels = fetched
		}
		sort.Slice(channels, func(i, j int) bool {
			if channels[i].Position == channels[j].Position {
				return channels[i].ID < channels[j].ID
			}
			return channels[i].Position < channels[j].Position
		})
		for _, channel := range channels {
			if channel == nil || !isKuroTextChannel(channel.Type) {
				continue
			}
			status := "設定停用"
			if ConversationAllowed(guild.ID, channel.ID) {
				status = "設定啟用"
			}
			allowed, reason, permissionErr := kuroChannelSendAllowed(session, channel)
			if permissionErr != nil {
				status += "；權限無法確認"
			} else if allowed {
				status += "；可發言"
			} else {
				status += "；無法發言：" + reason
			}
			lines = append(lines, fmt.Sprintf("%s — `%s` — #%s — `%s` — %s", guild.Name, guild.ID, channel.Name, channel.ID, status))
		}
	}
	if len(lines) == 1 {
		lines = append(lines, "目前沒有可列出的文字頻道。")
	}
	return splitKuroText(strings.Join(lines, "\n"), 1900), nil
}

func kuroChannelSendAllowed(session *discordgo.Session, channel *discordgo.Channel) (bool, string, error) {
	if session == nil || session.State == nil || session.State.User == nil || channel == nil {
		return false, "Discord 狀態不完整", fmt.Errorf("Discord state is incomplete")
	}
	permissions, err := session.UserChannelPermissions(session.State.User.ID, channel.ID)
	if err != nil {
		return false, "無法取得頻道權限", err
	}
	allowed, reason := evaluateKuroChannelSendPermissions(channel, permissions)
	return allowed, reason, nil
}

func evaluateKuroChannelSendPermissions(channel *discordgo.Channel, permissions int64) (bool, string) {
	if channel == nil {
		return false, "頻道資料不完整"
	}
	if isKuroThreadChannel(channel.Type) && channel.ThreadMetadata != nil && channel.ThreadMetadata.Archived {
		if channel.ThreadMetadata.Locked {
			return false, "討論串已封存並鎖定"
		}
		return false, "討論串已封存"
	}
	if permissions&discordgo.PermissionAdministrator != 0 {
		return true, ""
	}
	if permissions&discordgo.PermissionViewChannel == 0 {
		return false, "Bot 缺少「查看頻道」權限"
	}
	if isKuroThreadChannel(channel.Type) {
		if permissions&discordgo.PermissionSendMessagesInThreads == 0 {
			return false, "Bot 缺少「在討論串中傳送訊息」權限"
		}
		return true, ""
	}
	if permissions&discordgo.PermissionSendMessages == 0 {
		return false, "Bot 缺少「傳送訊息」權限"
	}
	return true, ""
}

func isKuroThreadChannel(channelType discordgo.ChannelType) bool {
	switch channelType {
	case discordgo.ChannelTypeGuildNewsThread,
		discordgo.ChannelTypeGuildPublicThread,
		discordgo.ChannelTypeGuildPrivateThread:
		return true
	default:
		return false
	}
}

func isKuroTextChannel(channelType discordgo.ChannelType) bool {
	switch channelType {
	case discordgo.ChannelTypeGuildText,
		discordgo.ChannelTypeGuildNews,
		discordgo.ChannelTypeGuildNewsThread,
		discordgo.ChannelTypeGuildPublicThread,
		discordgo.ChannelTypeGuildPrivateThread:
		return true
	default:
		return false
	}
}

func validDiscordID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}
