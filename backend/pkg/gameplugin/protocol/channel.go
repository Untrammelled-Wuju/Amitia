package protocol

import (
	"encoding/json"
	"fmt"
)

type ChannelID string
type ChannelKind string
type ChannelDirection string
type FrequencyHint string

const (
	ChannelKindEvent  ChannelKind = "event"
	ChannelKindState  ChannelKind = "state"
	ChannelKindLog    ChannelKind = "log"
	ChannelKindMetric ChannelKind = "metric"
	ChannelKindBinary ChannelKind = "binary"
	ChannelKindCustom ChannelKind = "custom"
)

const (
	// Protocol v1 channels are intentionally one-way: plugin -> host. Host ->
	// plugin delivery uses ordinary RPC until a future protocol major defines
	// a real outbound/bidirectional channel transport.
	ChannelDirectionPluginToHost ChannelDirection = "plugin_to_host"
)

const (
	FrequencyHintLow      FrequencyHint = "low"
	FrequencyHintNormal   FrequencyHint = "normal"
	FrequencyHintHigh     FrequencyHint = "high"
	FrequencyHintRealtime FrequencyHint = "realtime"
)

type ChannelDescriptor struct {
	ID       ChannelID   `json:"id"`
	Kind     ChannelKind `json:"kind"`
	SchemaID string      `json:"schemaId,omitempty"`

	Direction ChannelDirection `json:"direction,omitempty"`

	FrequencyHint *FrequencyHint `json:"frequencyHint,omitempty"`

	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

func ValidateChannelID(id ChannelID) error {
	if id == "" {
		return fmt.Errorf("channel id must not be empty")
	}
	const maxLength = 256
	if len(id) > maxLength {
		return fmt.Errorf("channel id exceeds maximum length of %d", maxLength)
	}
	for _, r := range string(id) {
		if r < 32 || r == 127 {
			return fmt.Errorf("channel id contains control character")
		}
	}
	return nil
}

func ValidateChannelKind(kind ChannelKind) error {
	switch kind {
	case ChannelKindEvent, ChannelKindState, ChannelKindLog,
		ChannelKindMetric, ChannelKindBinary, ChannelKindCustom:
		return nil
	default:
		return fmt.Errorf("invalid channel kind: %s", kind)
	}
}

func ValidateChannelDirection(dir ChannelDirection) error {
	if dir == "" {
		return nil
	}
	if dir == ChannelDirectionPluginToHost {
		return nil
	}
	return fmt.Errorf("invalid channel direction %q: amitia-game-host/1 supports plugin_to_host only", dir)
}

func ValidateFrequencyHint(hint FrequencyHint) error {
	if hint == "" {
		return nil
	}
	switch hint {
	case FrequencyHintLow, FrequencyHintNormal, FrequencyHintHigh, FrequencyHintRealtime:
		return nil
	default:
		return fmt.Errorf("invalid frequency hint: %s", hint)
	}
}

func (c ChannelDescriptor) Validate() error {
	if err := ValidateChannelID(c.ID); err != nil {
		return fmt.Errorf("invalid channel id: %w", err)
	}
	if err := ValidateChannelKind(c.Kind); err != nil {
		return err
	}
	if err := ValidateChannelDirection(c.Direction); err != nil {
		return err
	}
	if c.FrequencyHint != nil {
		if err := ValidateFrequencyHint(*c.FrequencyHint); err != nil {
			return err
		}
	}
	return nil
}

func ValidateChannels(channels []ChannelDescriptor) error {
	seen := make(map[ChannelID]bool)
	for i := range channels {
		if err := channels[i].Validate(); err != nil {
			return fmt.Errorf("channel[%d]: %w", i, err)
		}
		if seen[channels[i].ID] {
			return fmt.Errorf("duplicate channel id '%s'", channels[i].ID)
		}
		seen[channels[i].ID] = true
	}
	return nil
}

func IsChannelIDValid(id ChannelID) bool {
	return ValidateChannelID(id) == nil
}
