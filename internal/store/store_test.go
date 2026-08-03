package store

import "testing"

func TestAllowListsRemainSeparated(t *testing.T) {
	allowListMu.Lock()
	guildDiscordAllowList = map[string]struct{}{"guild": {}}
	dmDiscordAllowList = map[string]struct{}{"user": {}}
	allowListMu.Unlock()

	if !GuildAllowed("guild") || GuildAllowed("user") {
		t.Fatal("guild allow-list was not isolated")
	}
	if !DMAllowed("user") || DMAllowed("guild") {
		t.Fatal("DM allow-list was not isolated")
	}
}

func TestUserStoreAccessors(t *testing.T) {
	userStoreMu.Lock()
	userStore = make(map[string]struct{})
	userStoreMu.Unlock()

	if HasUser("user") {
		t.Fatal("unexpected user before AddUser")
	}
	AddUser("user")
	if !HasUser("user") {
		t.Fatal("user was not added")
	}
}
