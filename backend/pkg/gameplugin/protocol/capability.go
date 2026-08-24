package protocol

import "fmt"

// Capability is retained as a source-compatible name for HostFeature in the
// transport protocol. It MUST NOT be used for AI/tool capabilities, extension
// permissions, or runtime-engine capabilities.
// Deprecated: use HostFeature.
type Capability = HostFeature

const (
	CapabilityRealtimeControl = HostFeatureRealtimeControl
	CapabilityStateStreaming  = HostFeatureStateStreaming
	CapabilityEventStreaming  = HostFeatureEventStreaming
	CapabilityCustomRPC       = HostFeatureCustomRPC
	CapabilityHostAPI         = HostFeatureHostAPI
	CapabilitySharedControl   = HostFeatureSharedControl
	CapabilityCustomUI        = HostFeatureCustomUI
	CapabilityMultiService    = HostFeatureMultiService
)

func IsKnownCapability(cap Capability) bool { return IsKnownHostFeature(HostFeature(cap)) }

// Custom capability strings are deliberately not valid GameHost features.
// Game-specific AI/tool capabilities belong in the Extension Kernel provider
// registry and are opaque to GameHost.
func IsCustomCapability(cap Capability) bool { return false }

func ValidateCapability(cap Capability) error {
	if !IsKnownHostFeature(HostFeature(cap)) {
		return fmt.Errorf("unknown host feature %q", cap)
	}
	return nil
}

func ValidateCapabilities(caps []Capability) error {
	seen := make(map[Capability]bool, len(caps))
	for i := range caps {
		if err := ValidateCapability(caps[i]); err != nil {
			return fmt.Errorf("hostFeatures[%d]: %w", i, err)
		}
		if seen[caps[i]] {
			return fmt.Errorf("duplicate host feature %q", caps[i])
		}
		seen[caps[i]] = true
	}
	return nil
}
