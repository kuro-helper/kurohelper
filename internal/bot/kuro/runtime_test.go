package kuro

import (
	"testing"
	"time"
)

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

func TestChannelGenerationLocksAllowDifferentChannels(t *testing.T) {
	releaseFirst := LockChannelGeneration("parallel-a")
	defer releaseFirst()

	acquired := make(chan func(), 1)
	go func() { acquired <- LockChannelGeneration("parallel-b") }()

	select {
	case releaseSecond := <-acquired:
		releaseSecond()
	case <-time.After(time.Second):
		t.Fatal("a different channel was blocked by the active channel")
	}
}

func TestChannelGenerationLocksKeepSameChannelFIFO(t *testing.T) {
	releaseFirst := LockChannelGeneration("fifo")
	acquired := make(chan func(), 1)
	go func() { acquired <- LockChannelGeneration("fifo") }()

	select {
	case releaseSecond := <-acquired:
		releaseSecond()
		releaseFirst()
		t.Fatal("the second same-channel request ran before the first completed")
	case <-time.After(50 * time.Millisecond):
	}

	releaseFirst()
	select {
	case releaseSecond := <-acquired:
		releaseSecond()
	case <-time.After(time.Second):
		t.Fatal("the queued same-channel request did not resume")
	}
}

func TestGlobalGenerationGateWaitsForActiveChannel(t *testing.T) {
	releaseChannel := LockChannelGeneration("maintenance")
	acquired := make(chan func(), 1)
	go func() { acquired <- LockAllGenerations() }()

	select {
	case releaseAll := <-acquired:
		releaseAll()
		releaseChannel()
		t.Fatal("global maintenance overlapped an active generation")
	case <-time.After(50 * time.Millisecond):
	}

	releaseChannel()
	select {
	case releaseAll := <-acquired:
		releaseAll()
	case <-time.After(time.Second):
		t.Fatal("global maintenance did not resume")
	}
}
