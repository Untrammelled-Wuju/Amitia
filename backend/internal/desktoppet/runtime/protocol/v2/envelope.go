package v2

import (
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

var Descriptor = protocol.Descriptor{
	Name:            "amitia.desktop-pet.runtime",
	EnvelopeVersion: 2,
	SchemaVersion:   contracts.RuntimeContractVersion,
}

const (
	EnvelopeVersion       = 2
	ProtocolName          = "amitia.desktop-pet.runtime"
	CurrentSchemaVersion  = contracts.RuntimeContractVersion
	CurrentRuntimeVersion = contracts.RuntimeVersion

	CapabilitySyncDesiredV2 = "runtime.sync_desired_v2"
	CapabilityPlayActionV2  = "runtime.play_action_v2"
	CapabilityRendererAckV2 = "runtime.renderer_ack_v2"
	CapabilityExpiryRFC3339 = "runtime.expiry_rfc3339_v1"
)

func mandatoryRuntimeCapabilities() []string {
	return []string{
		CapabilitySyncDesiredV2,
		CapabilityPlayActionV2,
		CapabilityRendererAckV2,
		CapabilityExpiryRFC3339,
	}
}

type MessageType = protocol.MessageType

const (
	MessageTypeHello         = protocol.MessageTypeHello
	MessageTypeHelloAck      = protocol.MessageTypeHelloAck
	MessageTypeCommand       = protocol.MessageTypeCommand
	MessageTypeCommandAck    = protocol.MessageTypeCommandAck
	MessageTypeEventAck      = protocol.MessageTypeEventAck
	MessageTypeRuntimeEvent  = protocol.MessageTypeRuntimeEvent
	MessageTypeStateSnapshot = protocol.MessageTypeStateSnapshot
	MessageTypeError         = protocol.MessageTypeError
	MessageTypePing          = protocol.MessageTypePing
	MessageTypePong          = protocol.MessageTypePong
)

func IsValidMessageType(t string) bool {
	return protocol.MessageType(t).IsValid()
}

type Envelope protocol.Envelope

func (e *Envelope) core() *protocol.Envelope {
	return (*protocol.Envelope)(e)
}

func (e *Envelope) Validate() error {
	return e.core().Validate(Descriptor)
}

func (e *Envelope) ValidateBase() error {
	return e.core().ValidateBase(Descriptor)
}

func (e *Envelope) ValidateEstablishedSession() error {
	return e.core().ValidateEstablishedSession(Descriptor)
}

func (e *Envelope) VerifyPayloadHash() bool {
	return e.core().VerifyPayloadHash()
}

func ComputePayloadHash(payload []byte) string {
	return protocol.ComputePayloadHash(payload)
}

func CanonicalJSON(data []byte) string {
	return protocol.CanonicalJSON(data)
}
