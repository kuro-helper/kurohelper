package kuro

import "testing"

func TestCommandAllowedFailsClosedWithoutConfiguredUsers(t *testing.T) {
	Init(nil, Settings{})
	if CommandAllowed("594197917477109773") {
		t.Fatal("CommandAllowed() must reject users when no command allowlist is configured")
	}
}

func TestCommandAllowedUsesConfiguredUserIDs(t *testing.T) {
	Init(nil, Settings{CommandUserIDs: map[string]struct{}{
		"594197917477109773": {},
		"566279354343096323": {},
	}})
	if !CommandAllowed("594197917477109773") {
		t.Fatal("configured user was rejected")
	}
	if CommandAllowed("123") {
		t.Fatal("unconfigured user was allowed")
	}
}
