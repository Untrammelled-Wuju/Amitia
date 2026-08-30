package wiring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
		ReturnTo:         cmd.ReturnPolicy,
		PlaybackRate:     1,
		MinimumPlayMs:    cmd.MinimumPlayMS,
		MaximumPlayMs:    cmd.MaximumPlayMS,
		CompletionPolicy: runtimev2.PlayActionCompletionOnStarted,
		DecisionID:       cmd.DecisionID,
		Semantic:         cmd.Semantic,
		ReasonCode:       cmd.ReasonCode,
	}
	if cmd.ExpiresAt != nil {
		payload.ExpiresAt = cmd.ExpiresAt.UTC().Format(time.RFC3339Nano)
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

	expectedRuntimeID := strings.TrimSpace(cmd.RuntimeID)
	expectedPetInstanceID := strings.TrimSpace(cmd.PetInstanceID)
	if expectedRuntimeID != "" && expectedPetInstanceID != "" && expectedRuntimeID != expectedPetInstanceID {
		return rejectedRuntimeReceipt("", behavior.ErrCodeRuntimeCommandFailed, "runtimeId and petInstanceId target different runtime instances"), nil
	}

	var targetConn *runtimev2.Connection
	targetSessionID := ""
	targetGeneration := int64(0)
	for _, conn := range a.facade.ListConnections(cmd.UserID) {
		if conn == nil || conn.GetState() != runtimev2.ConnStateConnected {
			continue
		}
		if string(conn.DeviceID) != cmd.DeviceID {
			continue
		}
		if expectedRuntimeID != "" && string(conn.RuntimeID) != expectedRuntimeID {
			continue
		}
		if expectedPetInstanceID != "" && string(conn.RuntimeID) != expectedPetInstanceID {
			continue
		}
		sessionID, generation := conn.SessionSnapshot()
		if sessionID == "" || generation <= 0 {
			continue
		}
		targetConn = conn
		targetSessionID = sessionID
		targetGeneration = generation
		break
	}
	if targetConn == nil {
		return &behavior.CommandReceipt{
			CommandID:  "",
			Accepted:   false,
			Status:     behavior.CmdOffline,
			Error:      "target desktop pet runtime is offline",
			ReceivedAt: time.Now(),
		}, behavior.NewBehaviorError(behavior.ErrCodeRuntimeOffline, "target desktop pet runtime is offline")
	}

	// Runtime identity is a physical execution fence. Resolve it from the live
	// connection before serializing the payload so an upstream command that only
	// carries user/device affinity cannot produce an empty or stale runtimeId.
	targetRuntimeID := string(targetConn.RuntimeID)
	payload.RuntimeID = targetRuntimeID
	payload.PetInstanceID = targetRuntimeID
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

	v2Cmd, err := a.facade.Commands().CreateEphemeralCommandForSession(
		cmd.UserID, cmd.DeviceID, string(targetConn.RuntimeID), targetSessionID, cmd.InstallationID,
		string(runtimev2.CommandTypePlayAction), idempotencyKey, payloadBytes,
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
			// An in-flight ephemeral command is owned by one exact RuntimeSession.
			// Never treat a duplicate from another session as accepted.
			if v2Cmd.RuntimeSessionID != targetSessionID ||
				(v2Cmd.RuntimeID != "" && v2Cmd.RuntimeID != targetRuntimeID) {
				return &behavior.CommandReceipt{
					CommandID:     v2Cmd.ID,
					Accepted:      false,
					Status:        behavior.CmdOffline,
					PendingReason: behavior.ErrCodeRuntimeOffline,
					Error:         "idempotent command belongs to a previous runtime session",
					ReceivedAt:    time.Now(),
				}, behavior.NewBehaviorError(behavior.ErrCodeRuntimeOffline, "idempotent command belongs to a previous runtime session")
			}
			return acceptedRuntimeReceipt(v2Cmd.ID), nil
		case runtimev2.CommandStatusCreated, runtimev2.CommandStatusQueued, runtimev2.CommandStatusFailedRetryable:
			// A pre-existing unattempted ephemeral row is not safe to retarget across
			// reconnect. It must already be bound to this exact session.
			if v2Cmd.RuntimeSessionID != targetSessionID {
				if markErr := a.facade.Commands().MarkSuperseded(v2Cmd.ID, "stale idempotent ephemeral command on runtime reconnect", time.Now().UTC()); markErr != nil {
					return rejectedRuntimeReceipt(v2Cmd.ID, behavior.ErrCodeRuntimeCommandFailed, "failed to fence stale runtime command"), behavior.NewBehaviorError(behavior.ErrCodeRuntimeCommandFailed, fmt.Sprintf("failed to supersede stale runtime command: %v", markErr))
				}
				return rejectedRuntimeReceipt(v2Cmd.ID, behavior.ErrCodeRuntimeOffline, "idempotent command belongs to a previous runtime session"), nil
			}
		default:
			return rejectedRuntimeReceipt(v2Cmd.ID, behavior.ErrCodeRuntimeCommandFailed, "existing idempotent runtime command has an unknown status"), nil
		}
	}

	// Fence the create/bind race against HandleConnect replacement. If the
	// selected connection changed after command creation, this physical intent
	// belongs to the old session and must never be replayed on the new one.
	currentSessionID, currentGeneration := targetConn.SessionSnapshot()
	if targetConn.GetState() != runtimev2.ConnStateConnected ||
		currentSessionID != targetSessionID || currentGeneration != targetGeneration {
		if markErr := a.facade.Commands().MarkSuperseded(v2Cmd.ID, "runtime session changed during ephemeral command creation", time.Now().UTC()); markErr != nil {
			return rejectedRuntimeReceipt(v2Cmd.ID, behavior.ErrCodeRuntimeCommandFailed, "failed to fence stale runtime command"), behavior.NewBehaviorError(behavior.ErrCodeRuntimeCommandFailed, fmt.Sprintf("failed to supersede runtime command after session race: %v", markErr))
		}
		return &behavior.CommandReceipt{
			CommandID: v2Cmd.ID, Accepted: false, Status: behavior.CmdOffline,
			PendingReason: behavior.ErrCodeRuntimeOffline, Error: "runtime session changed while scheduling action", ReceivedAt: time.Now(),
		}, behavior.NewBehaviorError(behavior.ErrCodeRuntimeOffline, "runtime session changed while scheduling action")
	}
	currentSessionID, currentGeneration = targetConn.SessionSnapshot()
	if targetConn.GetState() != runtimev2.ConnStateConnected ||
		currentSessionID != targetSessionID || currentGeneration != targetGeneration {
		if markErr := a.facade.Commands().MarkSuperseded(v2Cmd.ID, "runtime session changed after ephemeral route bind", time.Now().UTC()); markErr != nil {
			return rejectedRuntimeReceipt(v2Cmd.ID, behavior.ErrCodeRuntimeCommandFailed, "failed to fence stale runtime command"), behavior.NewBehaviorError(behavior.ErrCodeRuntimeCommandFailed, fmt.Sprintf("failed to supersede runtime command after route-bind race: %v", markErr))
		}
		return &behavior.CommandReceipt{
			CommandID: v2Cmd.ID, Accepted: false, Status: behavior.CmdOffline,
			PendingReason: behavior.ErrCodeRuntimeOffline, Error: "runtime session changed after scheduling action", ReceivedAt: time.Now(),
		}, behavior.NewBehaviorError(behavior.ErrCodeRuntimeOffline, "runtime session changed after scheduling action")
	}
	v2Cmd.RuntimeID = string(targetConn.RuntimeID)
	v2Cmd.RuntimeSessionID = targetSessionID
	v2Cmd.InstallationID = cmd.InstallationID

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
