package kuro

import "testing"

func TestCommandAllowedFailsClosedWithoutConfiguredUsers(t *testing.T) {
	Init(nil, Settings{})
	if CommandAllowed("test-admin-a") {
		t.Fatal("CommandAllowed() must reject users when no command allowlist is configured")
	}
}

func TestCommandAllowedUsesConfiguredUserIDs(t *testing.T) {
	Init(nil, Settings{CommandUserIDs: map[string]struct{}{
		"test-admin-a": {},
		"test-admin-b": {},
	}})
	if !CommandAllowed("test-admin-a") {
		t.Fatal("configured user was rejected")
	}
	if CommandAllowed("123") {
		t.Fatal("unconfigured user was allowed")
	}
}
