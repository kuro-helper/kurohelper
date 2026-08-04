package kuro

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestBuildKuroChannelListShowsGuildAndChannelIdentity(t *testing.T) {
	Init(nil, Settings{})
	state := discordgo.NewState()
	state.User = &discordgo.User{ID: "300000000000000001"}
	guild := &discordgo.Guild{
		ID:   "100000000000000001",
		Name: "測試群組",
		Roles: []*discordgo.Role{{
			ID: "100000000000000001",
			Permissions: discordgo.PermissionViewChannel |
				discordgo.PermissionSendMessages |
				discordgo.PermissionSendMessagesInThreads,
		}},
		Members: []*discordgo.Member{{User: state.User}},
		Channels: []*discordgo.Channel{
			{ID: "200000000000000001", GuildID: "100000000000000001", Name: "聊天", Type: discordgo.ChannelTypeGuildText},
			{ID: "200000000000000002", GuildID: "100000000000000001", Name: "語音", Type: discordgo.ChannelTypeGuildVoice},
		},
	}
	if err := state.GuildAdd(guild); err != nil {
		t.Fatal(err)
	}
	session := &discordgo.Session{State: state}
	messages, err := buildKuroChannelList(session, "")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(messages, "\n")
	for _, expected := range []string{"測試群組", "100000000000000001", "#聊天", "200000000000000001", "設定啟用", "可發言"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("channel list is missing %q: %s", expected, joined)
		}
	}
	if strings.Contains(joined, "語音") {
		t.Fatalf("voice channel should not be listed: %s", joined)
	}
}

func TestEvaluateKuroChannelSendPermissions(t *testing.T) {
	textChannel := &discordgo.Channel{Type: discordgo.ChannelTypeGuildText}
	thread := &discordgo.Channel{Type: discordgo.ChannelTypeGuildPublicThread}
	archivedThread := &discordgo.Channel{
		Type:           discordgo.ChannelTypeGuildPublicThread,
		ThreadMetadata: &discordgo.ThreadMetadata{Archived: true, Locked: true},
	}
	tests := []struct {
		name        string
		channel     *discordgo.Channel
		permissions int64
		allowed     bool
		reason      string
	}{
		{name: "text allowed", channel: textChannel, permissions: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages, allowed: true},
		{name: "missing view", channel: textChannel, permissions: discordgo.PermissionSendMessages, reason: "查看頻道"},
		{name: "missing text send", channel: textChannel, permissions: discordgo.PermissionViewChannel, reason: "傳送訊息"},
		{name: "thread allowed", channel: thread, permissions: discordgo.PermissionViewChannel | discordgo.PermissionSendMessagesInThreads, allowed: true},
		{name: "missing thread send", channel: thread, permissions: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages, reason: "討論串"},
		{name: "administrator", channel: textChannel, permissions: discordgo.PermissionAdministrator, allowed: true},
		{name: "archived thread", channel: archivedThread, permissions: discordgo.PermissionAdministrator, reason: "封存並鎖定"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allowed, reason := evaluateKuroChannelSendPermissions(test.channel, test.permissions)
			if allowed != test.allowed || !strings.Contains(reason, test.reason) {
				t.Fatalf("got (%t, %q), want allowed=%t reason containing %q", allowed, reason, test.allowed, test.reason)
			}
		})
	}
}

func TestValidDiscordID(t *testing.T) {
	if !validDiscordID("123456789012345678") {
		t.Fatal("valid snowflake was rejected")
	}
	for _, invalid := range []string{"", "abc", "-1"} {
		if validDiscordID(invalid) {
			t.Fatalf("invalid Discord ID %q was accepted", invalid)
		}
	}
}
