package devicemesh

import (
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

const (
	ProtocolName            = "amitia.device-runtime"
	EnvelopeVersion         = 1
	SchemaVersion           = "1.0.0"
	RuntimeContractVersion  = "1.0"
	WebSocketPath           = "/api/device-mesh/v1/runtime/ws"
	BootstrapTicketPrefix   = "amt_mesh_bt_"
	DeviceCredentialPrefix  = "amt_mesh_dc_"
	BootstrapTicketTTL      = 300 // seconds
	DeviceCredentialTTL     = 30 * 24 * 3600 // 30 days in seconds
	HeartbeatInterval       = 20 // seconds
	ReadDeadlineSeconds     = 75
	HelloTimeoutSeconds     = 10
	MaxMessageSizeBytes     = 1 << 20 // 1 MiB
	ProbeTimeoutSeconds     = 5
)

var RuntimeProtocolDescriptor = protocol.Descriptor{
	Name:            ProtocolName,
	EnvelopeVersion: EnvelopeVersion,
	SchemaVersion:   SchemaVersion,
}
