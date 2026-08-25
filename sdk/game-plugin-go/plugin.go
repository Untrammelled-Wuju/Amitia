package sdk

import (
	"encoding/json"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

type Plugin interface {
	Descriptor() Descriptor
}

type Descriptor struct {
	ID              string `json:"id"`
	Name            string `json:"name,omitempty"`
	Version         string `json:"version,omitempty"`
	ProtocolVersion string `json:"protocolVersion"`

	Services     []protocol.ServiceDescriptor `json:"services,omitempty"`
	Channels     []protocol.ChannelDescriptor `json:"channels,omitempty"`
	Capabilities []protocol.HostFeature       `json:"capabilities,omitempty"`

	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (d Descriptor) Validate() error {
	if d.ID == "" {
		return NewValidationError("descriptor id must not be empty")
	}
	if d.ProtocolVersion != protocol.ProtocolVersion {
		return NewValidationError("invalid protocol version: %s", d.ProtocolVersion)
	}
	if err := protocol.ValidateServices(d.Services); err != nil {
		return NewValidationError("services validation failed: %v", err)
	}
	if err := protocol.ValidateChannels(d.Channels); err != nil {
		return NewValidationError("channels validation failed: %v", err)
	}
	if err := protocol.ValidateCapabilities(d.Capabilities); err != nil {
		return NewValidationError("capabilities validation failed: %v", err)
	}
	return nil
}
