// Package domain provides GameHost domain types.
//
// IMPORTANT: GameHost "Capability" historical terminology has been replaced with "HostFeature".
// A HostFeature describes a GAME PROTOCOL capability (e.g., realtime_control, IPC streaming),
// NOT an AI Capability registered in the kernel Capability Provider Registry.
//
// HostFeatures should NEVER be registered in the shared kernel capability.ProviderRegistry.
// True AI Tools/Capabilities from Game Plugins should be published via Extension Kernel
// providedCapabilities and registered through G11 -> G8.
package domain

import (
	"strings"
)

const (
	maxHostFeatureLength = 128
)

// HostFeature describes a protocol/host capability supported by a GameHost plugin.
// Examples: "realtime_control", "state_streaming", "binary_streaming", "custom_rpc"
//
// This is NOT an AI logical capability and must NOT be registered in the shared kernel
// Capability Provider Registry. It declares what kinds of host interaction the plugin
// protocol supports, not what AI tools it exposes.
type HostFeature string

const (
	HostFeatureRealtimeControl HostFeature = "realtime_control"
	HostFeatureStateStreaming  HostFeature = "state_streaming"
	HostFeatureEventStreaming  HostFeature = "event_streaming"
	HostFeatureBinaryStreaming HostFeature = "binary_streaming"
	HostFeatureCustomRPC       HostFeature = "custom_rpc"
	HostFeatureHostAPI         HostFeature = "host_api"
	HostFeatureSharedControl   HostFeature = "shared_control"
	HostFeatureCustomUI        HostFeature = "custom_ui"
	HostFeatureMultiService    HostFeature = "multi_service"
)

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

// Capability is a backwards-compatible alias for HostFeature.
// New code should use HostFeature directly.
// Deprecated: Use HostFeature instead.
type Capability = HostFeature

var knownHostFeatures = map[HostFeature]struct{}{
	HostFeatureRealtimeControl: {},
	HostFeatureStateStreaming:  {},
	HostFeatureEventStreaming:  {},
	HostFeatureBinaryStreaming: {},
	HostFeatureCustomRPC:       {},
	HostFeatureHostAPI:         {},
	HostFeatureSharedControl:   {},
	HostFeatureCustomUI:        {},
	HostFeatureMultiService:    {},
}

// IsKnownHostFeature reports whether the given HostFeature is recognized.
func IsKnownHostFeature(feature HostFeature) bool {
	_, ok := knownHostFeatures[feature]
	return ok
}

// IsKnownCapability is a backwards-compatible alias for IsKnownHostFeature.
// Deprecated: Use IsKnownHostFeature instead.
func IsKnownCapability(capability Capability) bool {
	return IsKnownHostFeature(HostFeature(capability))
}

// ValidateHostFeature checks whether a HostFeature string is valid.
func ValidateHostFeature(feature HostFeature) error {
	if feature == "" {
		return NewHostError(ErrInvalidArgument, "host feature must not be empty")
	}
	if len(feature) > maxHostFeatureLength {
		return NewHostError(ErrInvalidArgument, "host feature exceeds maximum length")
	}
	if strings.ContainsAny(string(feature), "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewHostError(ErrInvalidArgument, "host feature contains control characters")
	}
	return nil
}

// ValidateCapability is a backwards-compatible alias for ValidateHostFeature.
// Deprecated: Use ValidateHostFeature instead.
func ValidateCapability(capability Capability) error {
	return ValidateHostFeature(HostFeature(capability))
}

// AllKnownHostFeatures returns all registered HostFeature values.
func AllKnownHostFeatures() []HostFeature {
	features := make([]HostFeature, 0, len(knownHostFeatures))
	for f := range knownHostFeatures {
		features = append(features, f)
	}
	return features
}

// AllKnownCapabilities is a backwards-compatible alias for AllKnownHostFeatures.
// Deprecated: Use AllKnownHostFeatures instead.
func AllKnownCapabilities() []Capability {
	features := AllKnownHostFeatures()
	caps := make([]Capability, len(features))
	for i, f := range features {
		caps[i] = Capability(f)
	}
	return caps
}
