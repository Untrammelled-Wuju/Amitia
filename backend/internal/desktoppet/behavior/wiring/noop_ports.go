package wiring

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
)

type NoopActivePetPort struct{}

func (n *NoopActivePetPort) ResolveActivePet(ctx context.Context, userID, characterID string) (*behavior.ActivePetSnapshot, error) {
	return nil, behavior.NewBehaviorError(behavior.ErrCodeNoActiveInstallation, "no active installation")
}

type NoopRuntimeActionPort struct{}

var ErrRuntimeNotConfigured = errors.New("desktop pet runtime is not configured")

func (n *NoopRuntimeActionPort) SubmitBehaviorCommand(ctx context.Context, cmd behavior.BehaviorRuntimeCommand) (*behavior.CommandReceipt, error) {
	return nil, ErrRuntimeNotConfigured
}

func (n *NoopRuntimeActionPort) QueryPlayback(ctx context.Context, petInstanceID string) (*behavior.PlaybackSnapshot, error) {
	return nil, ErrRuntimeNotConfigured
}
