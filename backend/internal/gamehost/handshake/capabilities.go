package handshake

import "github.com/u-ai/backend/internal/gamehost/domain"

// ProductionHostCapabilities is deliberately explicit. A feature is advertised
// only when amitia-game-host/1 has a production execution path for it. Custom UI
// belongs to the Extension Kernel UI contribution system, not the GameHost wire
// protocol, so it must not be negotiated here.
func ProductionHostCapabilities() []domain.Capability {
	return []domain.Capability{
		domain.CapabilityRealtimeControl,
		domain.CapabilityStateStreaming,
		domain.CapabilityEventStreaming,
		domain.CapabilityCustomRPC,
		domain.CapabilityHostAPI,
		domain.CapabilitySharedControl,
		domain.CapabilityMultiService,
		domain.CapabilityBinaryStreaming,
	}
}
