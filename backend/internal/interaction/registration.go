package interaction

import (
	"context"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedTurnRecoveryRef struct {
	Scope            coordination.ExecutionScope `json:"scope"`
	RequestID        string                      `json:"requestId"`
	InputFingerprint string                      `json:"inputFingerprint"`
}

func RegisterInteraction(ctx context.Context, tracker InteractionTracker, candidate *InteractionRecord) (*InteractionRecord, bool, error) {
	if tracker == nil || candidate == nil || candidate.Scope.RequestID == "" {
		return nil, false, ErrInteractionNotFound
	}
	existing, found, err := tracker.GetByRequestID(ctx, candidate.Scope.SpaceID, candidate.Scope.RequestID)
	if err != nil {
		return nil, false, err
	}
	if found {
		if existing == nil || existing.Scope.Normalize() != candidate.Scope.Normalize() {
			return nil, false, ErrDuplicateRequest
		}
		return existing, false, nil
	}
	if err := tracker.Create(ctx, candidate); err != nil {
		if !errors.Is(err, ErrDuplicateRequest) {
			return nil, false, err
		}
		existing, found, err = tracker.GetByRequestID(ctx, candidate.Scope.SpaceID, candidate.Scope.RequestID)
		if err != nil {
			return nil, false, err
		}
		if !found || existing == nil || existing.Scope.Normalize() != candidate.Scope.Normalize() {
			return nil, false, ErrDuplicateRequest
		}
		return existing, false, nil
	}
	return candidate, true, nil
}
