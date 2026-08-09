package conformance

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
	sdk "github.com/u-ai/backend/pkg/gameplugin/sdk/go"
)

type EnvelopeValidator struct{}

func (v EnvelopeValidator) Name() string {
	return "envelope_validator"
}

func (v EnvelopeValidator) Validate(data []byte) error {
	var envelope protocol.Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("failed to unmarshal envelope: %w", err)
	}
	if err := envelope.Validate(); err != nil {
		return err
	}
	if envelope.Type == protocol.MessageTypeError && envelope.Error != nil {
		if err := envelope.Error.Validate(); err != nil {
			return fmt.Errorf("error validation failed: %w", err)
		}
	}
	return nil
}

type PluginMethodValidator struct{}

func (v PluginMethodValidator) Name() string {
	return "plugin_method_validator"
}

func (v PluginMethodValidator) Validate(data []byte) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}
	methodRaw, ok := envelope["method"]
	if !ok {
		return nil
	}
	var method string
	if err := json.Unmarshal(methodRaw, &method); err != nil {
		return fmt.Errorf("failed to unmarshal method: %w", err)
	}
	return protocol.ValidatePluginMethod(method)
}

type ServiceDescriptorValidator struct{}

func (v ServiceDescriptorValidator) Name() string {
	return "service_descriptor_validator"
}

func (v ServiceDescriptorValidator) Validate(data []byte) error {
	var svc protocol.ServiceDescriptor
	if err := json.Unmarshal(data, &svc); err != nil {
		return fmt.Errorf("failed to unmarshal service: %w", err)
	}
	return svc.Validate()
}

type ChannelDescriptorValidator struct{}

func (v ChannelDescriptorValidator) Name() string {
	return "channel_descriptor_validator"
}

func (v ChannelDescriptorValidator) Validate(data []byte) error {
	var ch protocol.ChannelDescriptor
	if err := json.Unmarshal(data, &ch); err != nil {
		return fmt.Errorf("failed to unmarshal channel: %w", err)
	}
	return ch.Validate()
}

type CapabilityValidator struct{}

func (v CapabilityValidator) Name() string {
	return "capability_validator"
}

func (v CapabilityValidator) Validate(data []byte) error {
	var caps []protocol.Capability
	if err := json.Unmarshal(data, &caps); err != nil {
		var single protocol.Capability
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return fmt.Errorf("failed to unmarshal capability: %w", err)
		}
		return protocol.ValidateCapability(single)
	}
	return protocol.ValidateCapabilities(caps)
}

type ProtocolErrorValidator struct{}

func (v ProtocolErrorValidator) Name() string {
	return "protocol_error_validator"
}

func (v ProtocolErrorValidator) Validate(data []byte) error {
	var pe protocol.ProtocolError
	if err := json.Unmarshal(data, &pe); err != nil {
		return fmt.Errorf("failed to unmarshal protocol error: %w", err)
	}
	return pe.Validate()
}

type PluginSchemaValidator struct{}

func (v PluginSchemaValidator) Name() string {
	return "plugin_schema_validator"
}

func (v PluginSchemaValidator) Validate(data []byte) error {
	var ps protocol.PluginSchema
	if err := json.Unmarshal(data, &ps); err != nil {
		return fmt.Errorf("failed to unmarshal plugin schema: %w", err)
	}
	return ps.Validate()
}

type DescriptorValidator struct{}

func (v DescriptorValidator) Name() string {
	return "descriptor_validator"
}

func (v DescriptorValidator) Validate(data []byte) error {
	var desc sdk.Descriptor
	if err := json.Unmarshal(data, &desc); err != nil {
		return fmt.Errorf("failed to unmarshal descriptor: %w", err)
	}
	desc.ProtocolVersion = protocol.ProtocolVersion
	return desc.Validate()
}
