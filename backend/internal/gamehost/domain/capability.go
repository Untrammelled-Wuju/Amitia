package domain

import (
	"strings"
)

const (
	maxHostFeatureLength = 128
)

type HostFeature string

const (
	HostFeatureRealtimeControl HostFeature = "realtime_control"
	HostFeatureStateStreaming  HostFeature = "state_streaming"
	HostFeatureEventStreaming  HostFeature = "event_streaming"
	HostFeatureCustomRPC       HostFeature = "custom_rpc"
	HostFeatureHostAPI         HostFeature = "host_api"
	HostFeatureSharedControl   HostFeature = "shared_control"
	HostFeatureMultiService    HostFeature = "multi_service"
	HostFeatureBinaryStreaming HostFeature = "binary_streaming"
)

var knownHostFeatures = map[HostFeature]struct{}{
	HostFeatureRealtimeControl: {},
	HostFeatureStateStreaming:  {},
	HostFeatureEventStreaming:  {},
	HostFeatureCustomRPC:       {},
	HostFeatureHostAPI:         {},
	HostFeatureSharedControl:   {},
	HostFeatureMultiService:    {},
	HostFeatureBinaryStreaming: {},
}

func IsKnownHostFeature(feature HostFeature) bool {
	_, ok := knownHostFeatures[feature]
	return ok
}

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
	if !IsKnownHostFeature(feature) {
		return NewHostError(ErrInvalidArgument, "unknown host feature: "+string(feature))
	}
	return nil
}

func AllKnownHostFeatures() []HostFeature {
	features := make([]HostFeature, 0, len(knownHostFeatures))
	for f := range knownHostFeatures {
		features = append(features, f)
	}
	return features
}
