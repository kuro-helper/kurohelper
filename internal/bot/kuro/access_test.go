package kuro

import "testing"

func TestParseIDSet(t *testing.T) {
	ids := ParseIDSet(" 1,2,1, ")
	if len(ids) != 2 {
		t.Fatalf("ids = %#v", ids)
	}
	if !isAllowed(ids, "1") || isAllowed(ids, "3") {
		t.Fatalf("unexpected allow-list behavior: %#v", ids)
	}
}

func TestConversationAllowedAppliesGuildAndChannelOverrides(t *testing.T) {
	Init(nil, Settings{
		ChannelIDs: map[string]struct{}{"100": {}},
		ChannelOverrides: map[string]bool{
			"200": true,
			"100": false,
		},
		GuildOverrides: map[string]bool{"900": false},
	})
	if !ConversationAllowed("800", "200") {
		t.Fatal("enabled channel override should bypass the environment allow-list")
	}
	if ConversationAllowed("800", "100") {
		t.Fatal("disabled channel override should beat the environment allow-list")
	}
	if ConversationAllowed("900", "200") {
		t.Fatal("disabled guild should beat an enabled channel override")
	}
}

func TestAccessOverridesUpdateAtRuntime(t *testing.T) {
	Init(nil, Settings{})
	UpdateAccessOverride("channel", "200", false)
	if ConversationAllowed("800", "200") {
		t.Fatal("runtime channel disable did not take effect")
	}
	DeleteAccessOverride("channel", "200")
	if !ConversationAllowed("800", "200") {
		t.Fatal("removing a runtime override should restore the default behavior")
	}
}
