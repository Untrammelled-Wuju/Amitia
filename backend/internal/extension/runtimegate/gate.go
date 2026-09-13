package runtimegate

import "sync"

const (
	EmotionExtensionID   = "com.amitia.builtin.emotion"
	LifestyleExtensionID = "com.amitia.builtin.lifestyle"
	ProactiveExtensionID = "com.amitia/proactive"
)

var (
	gateMu sync.RWMutex
	states = map[string]bool{}
)

var defaultEnabled = map[string]bool{
	EmotionExtensionID:   false,
	LifestyleExtensionID: false,
	ProactiveExtensionID: false,
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

func HostRuntimeAllowed(extensionID, runtimeID string) bool {
	switch extensionID {
	case EmotionExtensionID:
		return runtimeID == "host.character.psyche"
	case LifestyleExtensionID:
		return runtimeID == "host.character.lifestyle"
	case ProactiveExtensionID:
		return runtimeID == "host.character.proactive"
	default:
		return false
	}
}
