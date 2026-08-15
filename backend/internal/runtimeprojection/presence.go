package runtimeprojection

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type PresenceEvent struct {
	RuntimeID    runtimeidentity.RuntimeID
	SessionID    runtimeidentity.RuntimeSessionID
	Identity     runtimeidentity.Identity
	Placement    RuntimePlacement
	Online       bool
	Health       string
	Capabilities []capability.CapabilityID
	Timestamp    time.Time
}

type RuntimePresencePort interface {
	UpsertPresence(ctx context.Context, event PresenceEvent) error
	RemovePresence(ctx context.Context, runtimeID runtimeidentity.RuntimeID) error
	GetPresence(ctx context.Context, runtimeID runtimeidentity.RuntimeID) (*RuntimeProjection, error)
	ListPlacements(ctx context.Context) ([]RuntimePlacement, error)
}

type InMemoryPresenceStore struct {
	projections map[runtimeidentity.RuntimeID]RuntimeProjection
}

func NewInMemoryPresenceStore() *InMemoryPresenceStore {
	return &InMemoryPresenceStore{
		projections: make(map[runtimeidentity.RuntimeID]RuntimeProjection),
	}
}

func (s *InMemoryPresenceStore) UpsertPresence(ctx context.Context, event PresenceEvent) error {
	proj := RuntimeProjection{
		RuntimeID:         event.RuntimeID,
		SessionID:         event.SessionID,
		Identity:          event.Identity,
		Placement:         event.Placement,
		Online:            event.Online,
		Health:            event.Health,
		Capabilities:      event.Capabilities,
		ConnectionGeneration: 0,
		UpdatedAt:         event.Timestamp,
	}
	s.projections[event.RuntimeID] = proj
	return nil
}

func (s *InMemoryPresenceStore) RemovePresence(ctx context.Context, runtimeID runtimeidentity.RuntimeID) error {
	delete(s.projections, runtimeID)
	return nil
}

func (s *InMemoryPresenceStore) GetPresence(ctx context.Context, runtimeID runtimeidentity.RuntimeID) (*RuntimeProjection, error) {
	if p, ok := s.projections[runtimeID]; ok {
		return &p, nil
	}
	return nil, nil
}

func (s *InMemoryPresenceStore) ListPlacements(ctx context.Context) ([]RuntimePlacement, error) {
	seen := make(map[RuntimePlacement]bool)
	var result []RuntimePlacement
	for _, p := range s.projections {
		if !seen[p.Placement] {
			seen[p.Placement] = true
			result = append(result, p.Placement)
		}
	}
	return result, nil
}
