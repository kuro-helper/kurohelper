package kuro

import (
	"sync"

	servicekuro "kurohelperservice/kuro"
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

var generationMu sync.Mutex

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
	return servicekuro.IsAllowed(settings.ChannelIDs, channelID)
}

func CommandAllowed(userID string) bool {
	settings := GetSettings()
	if len(settings.CommandUserIDs) == 0 {
		return false
	}
	return servicekuro.IsAllowed(settings.CommandUserIDs, userID)
}

// LockGeneration serializes the single SillyTavern character/chat pipeline.
func LockGeneration() {
	generationMu.Lock()
}

func UnlockGeneration() {
	generationMu.Unlock()
}
