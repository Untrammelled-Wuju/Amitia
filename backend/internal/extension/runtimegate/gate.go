package runtimegate

import (
	"strings"
	"sync"
)

const (
	EmotionExtensionID             = "com.amitia.builtin.emotion"
	ActiveTaskExtensionID          = "com.amitia.builtin.active-task"
	HostRuntimeCharacterPsyche     = "host.character.psyche"
	HostRuntimeCharacterActiveTask = "host.character.active-task"
	HostRuntimeCharacterDetail     = "host.character.detail"
	HostRuntimeCharacterTeams      = "host.character.teams"
	HostRuntimeCharacterWorldBook  = "host.character.worldbook"
	HostRuntimeCreativeCards       = "host.creative.character-cards"
	HostRuntimeCreativePetInstall  = "host.creative.pet-installations"
	HostRuntimeGamePlugin          = "host.game.plugin"
	HostRuntimeMemoryOverview      = "host.memory.overview"
	HostRuntimeMemoryTimeline      = "host.memory.timeline"
	HostRuntimeMemoryEpisodic      = "host.memory.episodic"
	HostRuntimeMemoryGraph         = "host.memory.graph"
	HostRuntimeMemoryKnowledge     = "host.memory.knowledge"
	HostRuntimeProfileSummary      = "host.profile.summary"
)

var (
	gateMu sync.RWMutex
	states = map[string]bool{}
)

var defaultEnabled = map[string]bool{
	EmotionExtensionID: false,
}

var hostRuntimeAllowlist = map[string]struct{}{
	HostRuntimeCharacterPsyche:     {},
	HostRuntimeCharacterActiveTask: {},
	HostRuntimeCharacterDetail:     {},
	HostRuntimeCharacterTeams:      {},
	HostRuntimeCharacterWorldBook:  {},
	HostRuntimeCreativeCards:       {},
	HostRuntimeCreativePetInstall:  {},
	HostRuntimeGamePlugin:          {},
	HostRuntimeMemoryOverview:      {},
	HostRuntimeMemoryTimeline:      {},
	HostRuntimeMemoryEpisodic:      {},
	HostRuntimeMemoryGraph:         {},
	HostRuntimeMemoryKnowledge:     {},
	HostRuntimeProfileSummary:      {},
}

func Set(extensionID string, enabled bool) {
	gateMu.Lock()
	states[extensionID] = enabled
	gateMu.Unlock()
}

func IsEnabled(extensionID string) bool {
	gateMu.RLock()
	value, ok := states[extensionID]
	gateMu.RUnlock()
	if ok {
		return value
	}
	return defaultEnabled[extensionID]
}

func HostRuntimeAllowed(runtimeID string) bool {
	_, ok := hostRuntimeAllowlist[strings.TrimSpace(runtimeID)]
	return ok
}
