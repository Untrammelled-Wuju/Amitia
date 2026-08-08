package domain

import (
	"strings"
)

const (
	maxCapabilityLength = 128
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

var knownCapabilities = map[Capability]struct{}{
	CapabilityRealtimeControl: {},
	CapabilityStateStreaming:  {},
	CapabilityEventStreaming:  {},
	CapabilityBinaryStreaming: {},
	CapabilityCustomRPC:       {},
	CapabilityHostAPI:         {},
	CapabilitySharedControl:   {},
	CapabilityCustomUI:        {},
	CapabilityMultiService:    {},
}

func IsKnownCapability(capability Capability) bool {
	_, ok := knownCapabilities[capability]
	return ok
}

func ValidateCapability(capability Capability) error {
	if capability == "" {
		return NewHostError(ErrInvalidArgument, "capability must not be empty")
	}
	if len(capability) > maxCapabilityLength {
		return NewHostError(ErrInvalidArgument, "capability exceeds maximum length")
	}
	if strings.ContainsAny(string(capability), "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewHostError(ErrInvalidArgument, "capability contains control characters")
	}
	return nil
}

func AllKnownCapabilities() []Capability {
	caps := make([]Capability, 0, len(knownCapabilities))
	for c := range knownCapabilities {
		caps = append(caps, c)
	}
	return caps
}
