package runtimegate

import (
	"strings"
	"sync"
)

const (
	EmotionExtensionID         = "com.amitia.builtin.emotion"
	HostRuntimeCharacterPsyche = "host.character.psyche"
)

var (
	gateMu sync.RWMutex
	states = map[string]bool{}
)

var defaultEnabled = map[string]bool{
	EmotionExtensionID: false,
}

var hostRuntimeAllowlist = map[string]struct{}{
	HostRuntimeCharacterPsyche: {},
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
