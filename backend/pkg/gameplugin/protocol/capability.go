package protocol

import (
	"fmt"
	"strings"
	"unicode"
)

type Capability string

const (
	CapabilityRealtimeControl Capability = "realtime_control"
	CapabilityStateStreaming  Capability = "state_streaming"
	CapabilityEventStreaming  Capability = "event_streaming"
	CapabilityBinaryStreaming Capability = "binary_streaming"
	CapabilityCustomRPC       Capability = "custom_rpc"
	CapabilityHostAPI         Capability = "host_api"
	CapabilitySharedControl   Capability = "shared_control"
	CapabilityCustomUI        Capability = "custom_ui"
	CapabilityMultiService    Capability = "multi_service"
)

func IsKnownCapability(cap Capability) bool {
	switch cap {
	case CapabilityRealtimeControl, CapabilityStateStreaming,
		CapabilityEventStreaming, CapabilityBinaryStreaming,
		CapabilityCustomRPC, CapabilityHostAPI,
		CapabilitySharedControl, CapabilityCustomUI,
		CapabilityMultiService:
		return true
	default:
		return false
	}
}

func ValidateCapability(cap Capability) error {
	if cap == "" {
		return fmt.Errorf("capability must not be empty")
	}
	const maxLength = 256
	if len(cap) > maxLength {
		return fmt.Errorf("capability exceeds maximum length of %d", maxLength)
	}
	for _, r := range string(cap) {
		if unicode.IsControl(r) {
			return fmt.Errorf("capability contains control character")
		}
	}
	return nil
}

func IsCustomCapability(cap Capability) bool {
	return !IsKnownCapability(cap) && strings.Contains(string(cap), ".")
}

func ValidateCapabilities(caps []Capability) error {
	seen := make(map[Capability]bool)
	for i := range caps {
		if err := ValidateCapability(caps[i]); err != nil {
			return fmt.Errorf("capability[%d]: %w", i, err)
		}
		if seen[caps[i]] {
			return fmt.Errorf("duplicate capability '%s'", caps[i])
		}
		seen[caps[i]] = true
	}
	return nil
}
