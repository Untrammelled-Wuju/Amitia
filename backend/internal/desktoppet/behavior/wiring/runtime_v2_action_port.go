package wiring

import (
	"context"
	"encoding/json"
	"errors"
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

	queuePolicy := runtimev2.PlayActionQueueEnqueue
	if cmd.InterruptPolicy == "force" {
		queuePolicy = runtimev2.PlayActionQueueReplaceCurrent
	}
	returnTo := cmd.ReturnPolicy
	if returnTo == "" {
		returnTo = "default"
	}
	payload := runtimev2.PlayActionPayload{
		RuntimeID:        cmd.RuntimeID,
		ActionKey:        cmd.ActionKey,
		CharacterID:      cmd.CharacterID,
		PetInstanceID:    cmd.PetInstanceID,
		InstallationID:   cmd.InstallationID,
		PlaybackMode:     "once",
		Priority:         cmd.Priority,
		QueuePolicy:      queuePolicy,
		Interruptible:    cmd.InterruptPolicy != "uninterruptible",
		ReturnTo:         returnTo,
		PlaybackRate:     1,
		MinimumPlayMs:    cmd.MinimumPlayMS,
		MaximumPlayMs:    cmd.MaximumPlayMS,
		CompletionPolicy: runtimev2.PlayActionCompletionOnStarted,
		DecisionID:       cmd.DecisionID,
		Semantic:         cmd.Semantic,
		ReasonCode:       cmd.ReasonCode,
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

	if cmd.UserID == "" || cmd.DeviceID == "" {
		return &behavior.CommandReceipt{
			CommandID:  "",
			Accepted:   false,
			Status:     behavior.CmdOffline,
			Error:      "runtime route identity is incomplete",
			ReceivedAt: time.Now(),
		}, behavior.NewBehaviorError(behavior.ErrCodeRuntimeOffline, "runtime route identity is incomplete")
	}

	targetOnline := false
	for _, conn := range a.facade.ListConnections(cmd.UserID) {
		if conn == nil || conn.GetState() != runtimev2.ConnStateConnected {
			continue
		}
		if string(conn.DeviceID) != cmd.DeviceID {
			continue
		}
		if cmd.RuntimeID != "" && string(conn.RuntimeID) != cmd.RuntimeID {
			continue
		}
		targetOnline = true
		break
	}
	if !targetOnline {
		return &behavior.CommandReceipt{
			CommandID:  "",
			Accepted:   false,
			Status:     behavior.CmdOffline,
			Error:      "target desktop pet runtime is offline",
			ReceivedAt: time.Now(),
		}, behavior.NewBehaviorError(behavior.ErrCodeRuntimeOffline, "target desktop pet runtime is offline")
	}

	v2Cmd, err := a.facade.Commands().CreateEphemeralCommand(
		cmd.UserID, cmd.DeviceID, string(runtimev2.CommandTypePlayAction), idempotencyKey, payloadBytes,
	)
	duplicate := errors.Is(err, runtimev2.ErrCommandDuplication)
	if err != nil && !duplicate {
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
	if v2Cmd == nil {
		return &behavior.CommandReceipt{
			CommandID:  "",
			Accepted:   false,
			Status:     behavior.CmdRejected,
			Error:      "runtime command service returned nil command",
			ReceivedAt: time.Now(),
		}, nil
	}

	if duplicate {
		status := runtimev2.CommandStatus(v2Cmd.Status)
		switch status {
		case runtimev2.CommandStatusCompleted:
			// The exact idempotent command already completed. Treat this as a
			// successful replay and never mutate a terminal Runtime V2 row.
			return acceptedRuntimeReceipt(v2Cmd.ID), nil
		case runtimev2.CommandStatusFailedTerminal, runtimev2.CommandStatusExpired,
			runtimev2.CommandStatusCancelRequested, runtimev2.CommandStatusCancelled,
			runtimev2.CommandStatusSuperseded:
			reason := v2Cmd.ErrorCode
			if reason == "" {
				reason = "runtime_command_" + v2Cmd.Status
			}
			message := v2Cmd.ErrorMessage
			if message == "" {
				message = "existing idempotent runtime command is terminal"
			}
			return rejectedRuntimeReceipt(v2Cmd.ID, reason, message), nil
		case runtimev2.CommandStatusDispatching, runtimev2.CommandStatusTransportDispatched,
			runtimev2.CommandStatusRuntimeReceived, runtimev2.CommandStatusRuntimeAccepted,
			runtimev2.CommandStatusRendererAccepted, runtimev2.CommandStatusPlaybackStarted:
			// An in-flight ephemeral command must never be retargeted to a new
			// runtime incarnation: that could execute the same decision twice.
			if cmd.RuntimeID != "" && v2Cmd.RuntimeID != "" && v2Cmd.RuntimeID != cmd.RuntimeID {
				return &behavior.CommandReceipt{
					CommandID:     v2Cmd.ID,
					Accepted:      false,
					Status:        behavior.CmdOffline,
					PendingReason: behavior.ErrCodeRuntimeOffline,
					Error:         "idempotent command is in flight on a previous runtime incarnation",
					ReceivedAt:    time.Now(),
				}, behavior.NewBehaviorError(behavior.ErrCodeRuntimeOffline, "idempotent command is in flight on a previous runtime incarnation")
			}
			return acceptedRuntimeReceipt(v2Cmd.ID), nil
		case runtimev2.CommandStatusCreated, runtimev2.CommandStatusQueued, runtimev2.CommandStatusFailedRetryable:
			// Dispatchable rows may safely be pinned to the currently selected
			// runtime before their next dispatch attempt.
		default:
			return rejectedRuntimeReceipt(v2Cmd.ID, behavior.ErrCodeRuntimeCommandFailed, "existing idempotent runtime command has an unknown status"), nil
		}
	}

	updates := map[string]interface{}{}
	if cmd.RuntimeID != "" {
		updates["runtime_id"] = cmd.RuntimeID
	}
	if cmd.InstallationID != "" {
		updates["installation_id"] = cmd.InstallationID
	}
	if len(updates) > 0 {
		if err := a.facade.Commands().DB().Model(&runtimev2.RuntimeCommand{}).Where("id = ?", v2Cmd.ID).Updates(updates).Error; err != nil {
			log.Logger.Warnf("wiring/runtime_v2_action_port: pin command route failed commandId=%s runtimeId=%s err=%v", v2Cmd.ID, cmd.RuntimeID, err)
			if markErr := a.facade.Commands().MarkFailed(v2Cmd.ID, "runtime_route_persist_failed", err.Error(), time.Now()); markErr != nil {
				log.Logger.Errorf("wiring/runtime_v2_action_port: persist route failure status failed commandId=%s err=%v", v2Cmd.ID, markErr)
			}
			return rejectedRuntimeReceipt(v2Cmd.ID, "runtime_route_persist_failed", "persist runtime route failed"), nil
		}
		v2Cmd.RuntimeID = cmd.RuntimeID
		v2Cmd.InstallationID = cmd.InstallationID
	}

	return acceptedRuntimeReceipt(v2Cmd.ID), nil
}

func acceptedRuntimeReceipt(commandID string) *behavior.CommandReceipt {
	return &behavior.CommandReceipt{
		CommandID:  commandID,
		Accepted:   true,
		Status:     behavior.CmdAccepted,
		ReceivedAt: time.Now(),
	}
}

func rejectedRuntimeReceipt(commandID, reason, message string) *behavior.CommandReceipt {
	return &behavior.CommandReceipt{
		CommandID:     commandID,
		Accepted:      false,
		Status:        behavior.CmdRejected,
		PendingReason: reason,
		Error:         message,
		ReceivedAt:    time.Now(),
	}
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
		return snapshot, err
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
