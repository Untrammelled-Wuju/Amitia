package wiring

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
)

type NoopActivePetPort struct{}

func (n *NoopActivePetPort) ResolveActivePet(ctx context.Context, userID, characterID string) (*behavior.ActivePetSnapshot, error) {
	return nil, behavior.NewBehaviorError(behavior.ErrCodeNoActiveInstallation, "no active installation (noop)")
}

type NoopRuntimeActionPort struct{}

func (n *NoopRuntimeActionPort) SubmitBehaviorCommand(ctx context.Context, cmd behavior.BehaviorRuntimeCommand) (*behavior.CommandReceipt, error) {
	return &behavior.CommandReceipt{
		CommandID:  cmd.CommandID,
		Accepted:   false,
		Status:     behavior.CmdOffline,
		Error:      "runtime offline (noop)",
		ReceivedAt: time.Now(),
	}, nil
}

func (n *NoopRuntimeActionPort) QueryPlayback(ctx context.Context, petInstanceID string) (*behavior.PlaybackSnapshot, error) {
	return &behavior.PlaybackSnapshot{
		PetInstanceID: petInstanceID,
		RuntimeOnline: false,
	}, nil
}
