package wiring

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/runtime"
	"github.com/u-ai/backend/log"
)

type RuntimeActionAdapter struct {
	svc *runtime.Service
}

func NewRuntimeActionAdapter(svc *runtime.Service) *RuntimeActionAdapter {
	return &RuntimeActionAdapter{svc: svc}
}

func (a *RuntimeActionAdapter) SubmitBehaviorCommand(ctx context.Context, cmd behavior.BehaviorRuntimeCommand) (*behavior.CommandReceipt, error) {
	if a.svc == nil {
		return nil, behavior.NewBehaviorError(behavior.ErrCodeRuntimeOffline, "runtime service unavailable")
	}

	expiresAt := time.Now().Add(30 * time.Second)
	if cmd.ExpiresAt != nil {
		expiresAt = *cmd.ExpiresAt
	}

	payload := contracts.PlayActionPayload{
		ActionKey: cmd.ActionKey,
		ExpiresAt: expiresAt,
	}

	idempotencyKey := cmd.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = "behavior_" + cmd.DecisionID + "_" + uuid.NewString()
	}

	req := runtime.CommandRequest{
		Name:            contracts.MsgPetPlayAction,
		RuntimeID:       cmd.RuntimeID,
		InstallationID:  cmd.InstallationID,
		PetInstanceID:   cmd.PetInstanceID,
		Payload:         payload,
		Durability:      contracts.DurabilityDurableImmediate,
		IdempotencyKey:  idempotencyKey,
		DesiredRevision: cmd.InstallationRevision,
		Deadline:        15 * time.Second,
	}

	receipt, err := a.svc.Dispatch(ctx, req)
	if err != nil {
		log.Logger.Warnf("wiring/runtime_action_port: dispatch failed decisionId=%s actionKey=%s err=%v",
			cmd.DecisionID, cmd.ActionKey, err)
		return &behavior.CommandReceipt{
			CommandID:  receipt.CommandID,
			Accepted:   false,
			Status:     behavior.CmdRejected,
			Error:      err.Error(),
			ReceivedAt: time.Now(),
		}, nil
	}

	status := behavior.CmdAccepted
	accepted := false
	switch receipt.DeliveryStatus {
	case "applied", "sent":
		accepted = true
		status = behavior.CmdAccepted
	case "pending":
		status = behavior.CmdOffline
	default:
		status = behavior.CmdRejected
	}

	return &behavior.CommandReceipt{
		CommandID:  receipt.CommandID,
		Accepted:   accepted,
		Status:     status,
		Error:      receipt.PendingReason,
		ReceivedAt: time.Now(),
	}, nil
}

func (a *RuntimeActionAdapter) QueryPlayback(ctx context.Context, petInstanceID string) (*behavior.PlaybackSnapshot, error) {
	if a.svc == nil {
		return &behavior.PlaybackSnapshot{
			PetInstanceID: petInstanceID,
			RuntimeOnline: false,
		}, nil
	}

	conn := a.svc.Registry().GetByRuntime(petInstanceID)
	runtimeOnline := conn != nil && conn.State() == runtime.SessionStateReady

	snapshot := &behavior.PlaybackSnapshot{
		PetInstanceID: petInstanceID,
		RuntimeOnline: runtimeOnline,
	}

	if conn != nil {
		states, err := a.svc.State().ListActualStatesByRuntime(petInstanceID)
		if err == nil {
			for _, state := range states {
				if state.CurrentActionKey != "" {
					snapshot.CurrentActionKey = state.CurrentActionKey
					snapshot.IsPlaying = true
					break
				}
			}
		}
	}

	return snapshot, nil
}

var _ behavior.RuntimeActionPort = (*RuntimeActionAdapter)(nil)
