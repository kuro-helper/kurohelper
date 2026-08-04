package kuro

import (
	"sync"

	servicekuro "kurohelperservice/airuntime"
)

type Settings struct {
	TriggerPrefix      string
	ChannelIDs         map[string]struct{}
	CommandUserIDs     map[string]struct{}
	RecentMessageLimit int
	RecentContextChars int
}

var state struct {
	sync.RWMutex
	client   *servicekuro.Client
	settings Settings
}

type channelGenerationLock struct {
	refs int
	tail chan struct{}
}

var generationGate sync.RWMutex
var channelGenerationLocks struct {
	sync.Mutex
	locks map[string]*channelGenerationLock
}

func Init(client *servicekuro.Client, settings Settings) {
	state.Lock()
	state.client = client
	state.settings = settings
	state.Unlock()
}

func Client() *servicekuro.Client {
	state.RLock()
	defer state.RUnlock()
	return state.client
}

func GetSettings() Settings {
	state.RLock()
	defer state.RUnlock()
	return state.settings
}

func ChannelAllowed(channelID string) bool {
	settings := GetSettings()
	return isAllowed(settings.ChannelIDs, channelID)
}

func CommandAllowed(userID string) bool {
	settings := GetSettings()
	if len(settings.CommandUserIDs) == 0 {
		return false
	}
	return isAllowed(settings.CommandUserIDs, userID)
}

// LockChannelGeneration keeps messages in one Discord channel FIFO while the
// runtime's independent SillyTavern workers serve other channels in parallel.
func LockChannelGeneration(channelID string) func() {
	channelGenerationLocks.Lock()
	if channelGenerationLocks.locks == nil {
		channelGenerationLocks.locks = make(map[string]*channelGenerationLock)
	}
	entry := channelGenerationLocks.locks[channelID]
	if entry == nil {
		ready := make(chan struct{})
		close(ready)
		entry = &channelGenerationLock{tail: ready}
		channelGenerationLocks.locks[channelID] = entry
	}
	entry.refs++
	previous := entry.tail
	done := make(chan struct{})
	entry.tail = done
	channelGenerationLocks.Unlock()

	<-previous
	generationGate.RLock()
	return func() {
		generationGate.RUnlock()
		close(done)
		channelGenerationLocks.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(channelGenerationLocks.locks, channelID)
		}
		channelGenerationLocks.Unlock()
	}
}

// LockAllGenerations is reserved for maintenance that must not overlap any
// memory recall or generation, such as replacing the complete memory store.
func LockAllGenerations() func() {
	generationGate.Lock()
	return generationGate.Unlock
}
