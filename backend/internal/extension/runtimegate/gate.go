package runtimegate

import (
	"strings"
	"sync"
)

const (
	EmotionExtensionID             = "com.amitia.builtin.emotion"
	ActiveTaskExtensionID          = "com.amitia/active-task"
	HostRuntimeCharacterPsyche     = "host.character.psyche"
	HostRuntimeCharacterActiveTask = "host.character.active-task"
	HostRuntimeCharacterDetail     = "host.character.detail"
	HostRuntimeCharacterTeams      = "host.character.teams"
	HostRuntimeCharacterWorldBook  = "host.character.worldbook"
	HostRuntimeCreativeCards       = "host.creative.character-cards"
	HostRuntimeCreativePetInstall  = "host.creative.pet-installations"
	HostRuntimeGamePlugin          = "host.game.plugin"
	HostRuntimeChannelsHub         = "host.channels.hub"
	HostRuntimeUserSpace           = "host.user-space"
	HostRuntimeMobileNativePage    = "host.mobile.native-page"
	HostRuntimeContinuityThreads   = "host.tasks.continuity"
	HostRuntimeExtensionPackages   = "host.extensions.packages"
	HostRuntimeExtensionMCP        = "host.extensions.mcp"
	HostRuntimeExtensionSkills     = "host.extensions.agent-skills"
	HostRuntimeExtensionWorkflows  = "host.extensions.workflows"
	HostRuntimeKernelWasm          = "host.kernel.wasm"
	HostRuntimeKernelHooks         = "host.kernel.hooks"
	HostRuntimeKernelTrusted       = "host.kernel.trusted-services"
	HostRuntimeKernelTasks         = "host.kernel.tasks"
	HostRuntimeKernelEvents        = "host.kernel.events"
	HostRuntimeKernelSchedules     = "host.kernel.schedules"
	HostRuntimeKernelDesktop       = "host.kernel.desktop"
	HostRuntimeKernelUpdates       = "host.kernel.updates"
	HostRuntimeKernelDevConsole    = "host.kernel.dev-console"
	HostRuntimeKernelDevMode       = "host.kernel.dev-mode"
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
	HostRuntimeChannelsHub:         {},
	HostRuntimeUserSpace:           {},
	HostRuntimeMobileNativePage:    {},
	HostRuntimeContinuityThreads:   {},
	HostRuntimeExtensionPackages:   {},
	HostRuntimeExtensionMCP:        {},
	HostRuntimeExtensionSkills:     {},
	HostRuntimeExtensionWorkflows:  {},
	HostRuntimeKernelWasm:          {},
	HostRuntimeKernelHooks:         {},
	HostRuntimeKernelTrusted:       {},
	HostRuntimeKernelTasks:         {},
	HostRuntimeKernelEvents:        {},
	HostRuntimeKernelSchedules:     {},
	HostRuntimeKernelDesktop:       {},
	HostRuntimeKernelUpdates:       {},
	HostRuntimeKernelDevConsole:    {},
	HostRuntimeKernelDevMode:       {},
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
