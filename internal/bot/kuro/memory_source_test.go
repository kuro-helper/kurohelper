package kuro

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestFormatKuroSourceMessageIncludesUserTextAndJumpLink(t *testing.T) {
	source := &discordgo.Message{
		ID: "message-1", Content: "原始問題",
		Author: &discordgo.User{Username: "Tommy"},
	}
	formatted := formatKuroSourceMessageWithLink(source, "guild-1", "channel-1")
	for _, expected := range []string{"來源使用者訊息", "Tommy", "原始問題", "https://discord.com/channels/guild-1/channel-1/message-1"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("source message is missing %q: %s", expected, formatted)
		}
	}
	if strings.Contains(formatted, "Kuro") || strings.Contains(formatted, "推測配對") {
		t.Fatalf("source output should not infer a bot reply: %s", formatted)
	}
}
