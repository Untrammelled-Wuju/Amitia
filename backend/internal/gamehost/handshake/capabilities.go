package handshake

import "github.com/u-ai/backend/internal/gamehost/domain"

func ProductionHostCapabilities() []domain.Capability {
	return domain.AllKnownCapabilities()
}
