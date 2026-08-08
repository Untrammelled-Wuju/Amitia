package wiring

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	runtimev2 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v2"
	"github.com/u-ai/backend/log"
)

type V2RuntimeActionAdapter struct {
	facade *runtimev2.RuntimeFacade
}

func NewV2RuntimeActionAdapter(facade *runtimev2.RuntimeFacade) *V2RuntimeActionAdapter {
	return &V2RuntimeActionAdapter{facade: facade}
}

func (a *V2RuntimeActionAdapter) SubmitBehaviorCommand(ctx context.Context, cmd behavior.BehaviorRuntimeCommand) (*behavior.CommandReceipt, error) {
	if a.facade == nil {
		return &behavior.CommandReceipt{
			CommandID:  "",
			Accepted:   false,
			Status:     behavior.CmdOffline,
			Error:      "v2 runtime facade unavailable",
			ReceivedAt: time.Now(),
		}, behavior.NewBehaviorError(behavior.ErrCodeRuntimeOffline, "v2 runtime facade unavailable")
	}

	payload := runtimev2.PlayActionPayload{
		ActionKey: cmd.ActionKey,
		Priority:  cmd.Priority,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return &behavior.CommandReceipt{
			CommandID:  "",
			Accepted:   false,
			Status:     behavior.CmdRejected,
			Error:      "marshal payload failed",
			ReceivedAt: time.Now(),
		}, nil
	}

	idempotencyKey := cmd.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = "behavior_" + cmd.DecisionID
	}

	v2Cmd, err := a.facade.Commands().CreateEphemeralCommand(
		"", "", string(runtimev2.CommandTypePlayAction), idempotencyKey, payloadBytes,
	)
	if err != nil {
		log.Logger.Warnf("wiring/runtime_v2_action_port: create command failed decisionId=%s actionKey=%s err=%v",
			cmd.DecisionID, cmd.ActionKey, err)
		return &behavior.CommandReceipt{
			CommandID:  "",
			Accepted:   false,
			Status:     behavior.CmdRejected,
			Error:      err.Error(),
			ReceivedAt: time.Now(),
		}, nil
	}

	return &behavior.CommandReceipt{
		CommandID:  v2Cmd.ID,
		Accepted:   true,
		Status:     behavior.CmdAccepted,
		ReceivedAt: time.Now(),
	}, nil
}

func (a *V2RuntimeActionAdapter) QueryPlayback(ctx context.Context, petInstanceID string) (*behavior.PlaybackSnapshot, error) {
	if a.facade == nil {
		return &behavior.PlaybackSnapshot{
			PetInstanceID: petInstanceID,
			RuntimeOnline: false,
		}, nil
	}

	snapshot := &behavior.PlaybackSnapshot{
		PetInstanceID: petInstanceID,
		RuntimeOnline: false,
	}

	states, err := a.facade.StateService().ListByRuntime(petInstanceID)
	if err != nil {
		return snapshot, nil
	}

	for _, state := range states {
		snapshot.RuntimeOnline = true
		if state.CurrentActionKey != "" {
			snapshot.CurrentActionKey = state.CurrentActionKey
			snapshot.IsPlaying = true
			break
		}
	}

	return snapshot, nil
}

var _ behavior.RuntimeActionPort = (*V2RuntimeActionAdapter)(nil)
