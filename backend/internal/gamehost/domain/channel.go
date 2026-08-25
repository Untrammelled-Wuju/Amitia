package domain

import (
	"strings"
)

const (
	maxSchemaIDLength = 1024
)

type ChannelID string

type ChannelKind string
type ChannelDirection string
type ChannelFrequency string

const (
	ChannelKindEvent  ChannelKind = "event"
	ChannelKindState  ChannelKind = "state"
	ChannelKindLog    ChannelKind = "log"
	ChannelKindMetric ChannelKind = "metric"
	ChannelKindBinary ChannelKind = "binary"
	ChannelKindCustom ChannelKind = "custom"
)

const (
	ChannelDirectionPluginToHost  ChannelDirection = "plugin_to_host"
	ChannelDirectionHostToPlugin  ChannelDirection = "host_to_plugin"
	ChannelDirectionBidirectional ChannelDirection = "bidirectional"
)

const (
	ChannelFrequencyLow      ChannelFrequency = "low"
	ChannelFrequencyNormal   ChannelFrequency = "normal"
	ChannelFrequencyHigh     ChannelFrequency = "high"
	ChannelFrequencyRealtime ChannelFrequency = "realtime"
)

var validChannelDirections = map[ChannelDirection]struct{}{
	ChannelDirectionPluginToHost:  {},
	ChannelDirectionHostToPlugin:  {},
	ChannelDirectionBidirectional: {},
}

var validChannelFrequencies = map[ChannelFrequency]struct{}{
	ChannelFrequencyLow:      {},
	ChannelFrequencyNormal:   {},
	ChannelFrequencyHigh:     {},
	ChannelFrequencyRealtime: {},
}

var validChannelKinds = map[ChannelKind]struct{}{
	ChannelKindEvent:  {},
	ChannelKindState:  {},
	ChannelKindLog:    {},
	ChannelKindMetric: {},
	ChannelKindBinary: {},
	ChannelKindCustom: {},
}

type ChannelDescriptor struct {
	ID        ChannelID
	ServiceID ServiceID
	Kind      ChannelKind
	SchemaID  string
	Direction ChannelDirection
	Frequency ChannelFrequency

	Metadata map[string]string
}

func (c ChannelDescriptor) Validate() error {
	if c.ID == "" {
		return NewHostError(ErrInvalidArgument, "channel id must not be empty")
	}
	if c.ServiceID == "" {
		return NewHostError(ErrInvalidArgument, "channel service id must not be empty")
	}
	if _, ok := validChannelKinds[c.Kind]; !ok {
		return NewHostErrorWithCause(ErrInvalidArgument, "invalid channel kind",
			NewHostError(ErrInvalidArgument, string(c.Kind)))
	}
	if c.Direction != "" {
		if _, ok := validChannelDirections[c.Direction]; !ok {
			return NewHostError(ErrInvalidArgument, "invalid channel direction")
		}
	}
	if c.Frequency != "" {
		if _, ok := validChannelFrequencies[c.Frequency]; !ok {
			return NewHostError(ErrInvalidArgument, "invalid channel frequency")
		}
	}
	if strings.ContainsAny(string(c.ID), "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewHostError(ErrInvalidArgument, "channel id contains control characters")
	}
	if len(c.SchemaID) > maxSchemaIDLength {
		return NewHostError(ErrInvalidArgument, "channel schema id exceeds maximum length")
	}
	if c.Metadata != nil {
		if err := validateMetadata(c.Metadata); err != nil {
			return NewHostErrorWithCause(ErrInvalidArgument, "channel metadata validation failed", err)
		}
	}
	return nil
}

func IsValidChannelKind(kind ChannelKind) bool {
	_, ok := validChannelKinds[kind]
	return ok
}
