package protocol

import (
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

const (
	ProtocolName           = "amitia.device-runtime"
	EnvelopeVersion        = 1
	SchemaVersion          = "1.0.0"
	RuntimeContractVersion = "1.0"
	WebSocketPath          = "/api/device-mesh/v1/runtime/ws"
	HelloTimeoutSeconds    = 10
	MaxMessageSizeBytes    = 1 << 20
	ReadDeadlineSeconds    = 75
	HeartbeatInterval      = 20
	BootstrapTicketTTL     = 300
	DeviceCredentialTTL    = 30 * 24 * 3600
	ProbeTimeoutSeconds    = 5
)

var RuntimeProtocolDescriptor = protocol.Descriptor{
	Name:            ProtocolName,
	EnvelopeVersion: EnvelopeVersion,
	SchemaVersion:   SchemaVersion,
}
