// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package runtime

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/log"
)

type CommandRequest struct {
	Name            string
	RuntimeID       string
	UserID          string
	InstallationID  string
	PetInstanceID   string
	Payload         interface{}
	Durability      contracts.CommandDurability
	CoalesceKey     string
	IdempotencyKey  string
	DesiredRevision int64
	Deadline        time.Duration
}

type DispatchReceipt struct {
	CommandID        string
	RuntimeID        string
	DeliveryStatus   string
	ActualStateKnown bool
	PendingReason    string
}

type Dispatcher struct {
	config   *DesktopPetRuntimeConfig
	registry *RuntimeRegistry
	pending  *PendingTracker
	store    CommandStore
}

func NewDispatcher(config *DesktopPetRuntimeConfig, registry *RuntimeRegistry, pending *PendingTracker, store CommandStore) *Dispatcher {
	return &Dispatcher{
		config:   config,
		registry: registry,
		pending:  pending,
		store:    store,
	}
}

func (d *Dispatcher) Dispatch(ctx context.Context, req CommandRequest) (DispatchReceipt, error) {
	if req.Name == "" {
		return DispatchReceipt{}, NewRuntimeError(ErrCodeRuntimeProtocolError, "command name is required", ErrRuntimeProtocolError)
	}
	if !contracts.IsValidCommandDurability(req.Durability) {
		return DispatchReceipt{}, NewRuntimeError(ErrCodeRuntimeProtocolError, "invalid durability: "+string(req.Durability), ErrRuntimeProtocolError)
	}

	commandID := "rpcmd_" + uuid.NewString()
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = "idem_" + uuid.NewString()
	}
	deadline := req.Deadline
	if deadline <= 0 {
		deadline = d.config.CommandTimeout()
	}

	if req.Durability == contracts.DurabilityReconcile {
		return DispatchReceipt{CommandID: commandID}, NewRuntimeError(ErrCodeRuntimeCommandRejected, "reconcile commands must be dispatched via Reconciler", ErrRuntimeCommandRejected)
	}

	if req.Durability == contracts.DurabilityEphemeral || req.Durability == contracts.DurabilityEphemeralControl {
		return d.dispatchEphemeral(ctx, req, commandID, idempotencyKey, deadline)
	}

	return d.dispatchDurable(ctx, req, commandID, idempotencyKey, deadline)
}

func (d *Dispatcher) dispatchEphemeral(ctx context.Context, req CommandRequest, commandID, idempotencyKey string, deadline time.Duration) (DispatchReceipt, error) {
	conn, err := d.resolveConnection(req.RuntimeID, req.UserID)
	if err != nil {
		return DispatchReceipt{CommandID: commandID, RuntimeID: req.RuntimeID}, err
	}
	if conn.State() != SessionStateReady {
		return DispatchReceipt{CommandID: commandID, RuntimeID: conn.RuntimeID()}, ErrRuntimeNotReady
	}

	if err := d.checkCapability(conn, req.Name); err != nil {
		return DispatchReceipt{CommandID: commandID, RuntimeID: conn.RuntimeID()}, err
	}

	msg, err := buildMessage(contracts.KindCommand, req.Name, conn.RuntimeID(), conn.SessionID(), req.Payload)
	if err != nil {
		return DispatchReceipt{CommandID: commandID, RuntimeID: conn.RuntimeID()}, err
	}
	msg.CommandID = commandID
	msg.IdempotencyKey = idempotencyKey
	applyMessageMetadata(&msg, req.UserID, req.InstallationID, req.PetInstanceID, deadline)

	ch, cancel := d.pending.Register(commandID, conn.SessionID(), deadline)
	defer cancel()

	if err := conn.Send(msg); err != nil {
		return DispatchReceipt{CommandID: commandID, RuntimeID: conn.RuntimeID()}, err
	}

	result, rerr := d.waitForResult(ctx, ch, deadline)
	if rerr != nil {
		return DispatchReceipt{CommandID: commandID, RuntimeID: conn.RuntimeID()}, rerr
	}

	receipt := DispatchReceipt{CommandID: commandID, RuntimeID: conn.RuntimeID()}
	switch result.Status {
	case contracts.ResultApplied:
		receipt.DeliveryStatus = "applied"
		receipt.ActualStateKnown = result.ActualState != nil
	case contracts.ResultAccepted:
		receipt.DeliveryStatus = "sent"
	case contracts.ResultDuplicate:
		receipt.DeliveryStatus = "applied"
		receipt.ActualStateKnown = result.ActualState != nil
	default:
		return receipt, NewRuntimeError(result.ErrorCode, result.ErrorMsg, result.Err)
	}
	return receipt, nil
}

func (d *Dispatcher) dispatchDurable(ctx context.Context, req CommandRequest, commandID, idempotencyKey string, deadline time.Duration) (DispatchReceipt, error) {
	payloadJSON := ""
	if req.Payload != nil {
		raw, err := json.Marshal(req.Payload)
		if err != nil {
			return DispatchReceipt{CommandID: commandID}, err
		}
		payloadJSON = string(raw)
	}

	effectiveRuntimeID := req.RuntimeID
	if effectiveRuntimeID == "" && req.UserID != "" {
		effectiveRuntimeID = d.registry.GetUserRuntime(req.UserID)
	}

	now := time.Now()
	nowStr := now.Format(runtimeTimeFormat)
	deadlineAt := now.Add(deadline).Format(runtimeTimeFormat)

	maxAttempts := d.config.MaxRetryAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	cmd := &RuntimeCommand{
		ID:              commandID,
		RuntimeID:       effectiveRuntimeID,
		UserID:          req.UserID,
		InstallationID:  req.InstallationID,
		PetInstanceID:   req.PetInstanceID,
		Name:            req.Name,
		PayloadJSON:     payloadJSON,
		Durability:      string(req.Durability),
		CoalesceKey:     req.CoalesceKey,
		IdempotencyKey:  idempotencyKey,
		DesiredRevision: req.DesiredRevision,
		Status:          string(contracts.CmdPending),
		AttemptCount:    0,
		MaxAttempts:     maxAttempts,
		NextAttemptAt:   nowStr,
		DeadlineAt:      deadlineAt,
		CreatedAt:       nowStr,
		UpdatedAt:       nowStr,
	}

	if req.CoalesceKey != "" && req.DesiredRevision > 0 {
		if err := d.store.MarkSuperseded(effectiveRuntimeID, req.CoalesceKey, req.DesiredRevision); err != nil {
			log.Logger.Warnf("dispatcher: mark superseded failed runtimeID=%s coalesceKey=%s err=%v", effectiveRuntimeID, req.CoalesceKey, err)
		}
	}

	if err := d.store.Create(cmd); err != nil {
		return DispatchReceipt{CommandID: commandID}, NewRuntimeError(ErrCodeRuntimeCommandStoreFailed, "failed to persist command", err)
	}

	conn, err := d.resolveConnection(effectiveRuntimeID, req.UserID)
	if err != nil || conn == nil {
		if req.Durability == contracts.DurabilityDurableImmediate {
			return DispatchReceipt{
				CommandID:      commandID,
				RuntimeID:      effectiveRuntimeID,
				DeliveryStatus: "not_delivered",
				PendingReason:  "runtime_offline",
			}, NewRuntimeError(ErrCodeRuntimeOffline, "runtime offline, immediate command not delivered", ErrRuntimeOffline)
		}
		return DispatchReceipt{
			CommandID:      commandID,
			RuntimeID:      effectiveRuntimeID,
			DeliveryStatus: "pending",
			PendingReason:  "runtime_offline",
		}, nil
	}

	if conn.State() != SessionStateReady {
		if req.Durability == contracts.DurabilityDurableImmediate {
			return DispatchReceipt{
				CommandID:      commandID,
				RuntimeID:      effectiveRuntimeID,
				DeliveryStatus: "not_delivered",
				PendingReason:  "runtime_not_ready",
			}, NewRuntimeError(ErrCodeRuntimeNotReady, "runtime not ready, immediate command not delivered", ErrRuntimeNotReady)
		}
		return DispatchReceipt{
			CommandID:      commandID,
			RuntimeID:      effectiveRuntimeID,
			DeliveryStatus: "pending",
			PendingReason:  "runtime_not_ready",
		}, nil
	}

	dispatchErr := d.dispatchOnline(ctx, conn, cmd)
	if dispatchErr == nil {
		status := "sent"
		if cmd.Status == string(contracts.CmdApplied) {
			status = "applied"
		}
		return DispatchReceipt{
			CommandID:      commandID,
			RuntimeID:      effectiveRuntimeID,
			DeliveryStatus: status,
		}, nil
	}

	if d.retryableError(errorCodeOf(dispatchErr)) {
		return DispatchReceipt{
			CommandID:      commandID,
			RuntimeID:      effectiveRuntimeID,
			DeliveryStatus: "pending",
			PendingReason:  dispatchErr.Error(),
		}, nil
	}

	return DispatchReceipt{
		CommandID:      commandID,
		RuntimeID:      effectiveRuntimeID,
		DeliveryStatus: "pending",
		PendingReason:  dispatchErr.Error(),
	}, dispatchErr
}

func (d *Dispatcher) dispatchOnline(ctx context.Context, conn *Connection, cmd *RuntimeCommand) error {
	if err := d.checkCapability(conn, cmd.Name); err != nil {
		return err
	}

	timeout := d.config.CommandTimeout()
	if cmd.DeadlineAt != "" {
		if deadline, err := time.Parse(runtimeTimeFormat, cmd.DeadlineAt); err == nil {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return NewRuntimeError(ErrCodeRuntimeCommandTimeout, "command deadline already exceeded", ErrRuntimeCommandTimeout)
			}
			if remaining < timeout {
				timeout = remaining
			}
		}
	}

	if err := d.store.UpdateStatusCAS(cmd.ID, []string{string(contracts.CmdPending)}, string(contracts.CmdSent), map[string]interface{}{
		"attempt_count":   cmd.AttemptCount + 1,
		"last_session_id": conn.SessionID(),
	}); err != nil {
		return NewRuntimeError(ErrCodeRuntimeCommandStoreFailed, "command no longer pending", err)
	}
	cmd.AttemptCount++
	cmd.LastSessionID = conn.SessionID()

	ch, cancel := d.pending.Register(cmd.ID, conn.SessionID(), timeout)
	defer cancel()

	var payloadArg interface{}
	if cmd.PayloadJSON != "" {
		payloadArg = json.RawMessage(cmd.PayloadJSON)
	}
	msg, err := buildMessage(contracts.KindCommand, cmd.Name, conn.RuntimeID(), conn.SessionID(), payloadArg)
	if err != nil {
		d.revertToPending(cmd, ErrCodeRuntimeProtocolError, err.Error())
		return NewRuntimeError(ErrCodeRuntimeProtocolError, "build message failed", err)
	}
	msg.CommandID = cmd.ID
	msg.IdempotencyKey = cmd.IdempotencyKey
	applyMessageMetadata(&msg, cmd.UserID, cmd.InstallationID, cmd.PetInstanceID, timeout)

	if err := conn.Send(msg); err != nil {
		code := ErrCodeRuntimeDisconnected
		if err == ErrRuntimeBusy {
			code = ErrCodeRuntimeBusy
		}
		d.revertToPending(cmd, code, err.Error())
		return NewRuntimeError(code, err.Error(), err)
	}

	result, rerr := d.waitForResult(ctx, ch, timeout)
	if rerr != nil {
		code := errorCodeOf(rerr)
		if code == "" {
			code = ErrCodeRuntimeCommandTimeout
		}
		d.revertToPending(cmd, code, rerr.Error())
		return rerr
	}

	if result == nil {
		d.revertToPending(cmd, ErrCodeRuntimeCommandFailed, "nil result received")
		return ErrRuntimeCommandFailed
	}

	switch result.Status {
	case contracts.ResultApplied, contracts.ResultDuplicate:
		nowStr := time.Now().Format(runtimeTimeFormat)
		_ = d.store.UpdateStatusCAS(cmd.ID, []string{string(contracts.CmdSent)}, string(contracts.CmdApplied), map[string]interface{}{
			"result_json":  marshalResultJSON(result),
			"completed_at": nowStr,
		})
		cmd.Status = string(contracts.CmdApplied)
		return nil
	case contracts.ResultAccepted:
		_ = d.store.UpdateStatusCAS(cmd.ID, []string{string(contracts.CmdSent)}, string(contracts.CmdSent), map[string]interface{}{
			"result_json": marshalResultJSON(result),
		})
		cmd.Status = string(contracts.CmdSent)
		return nil
	default:
		nowStr := time.Now().Format(runtimeTimeFormat)
		status := string(contracts.CmdFailed)
		if result.Status == contracts.ResultRejected {
			status = string(contracts.CmdRejected)
		} else if result.Status == contracts.ResultExpired {
			status = string(contracts.CmdExpired)
		} else if result.Status == contracts.ResultCancelled {
			status = string(contracts.CmdCancelled)
		}
		_ = d.store.UpdateStatusCAS(cmd.ID, []string{string(contracts.CmdSent)}, status, map[string]interface{}{
			"result_json":        marshalResultJSON(result),
			"last_error_code":    result.ErrorCode,
			"last_error_message": result.ErrorMsg,
			"completed_at":       nowStr,
		})
		cmd.Status = status
		if result.Err != nil {
			return result.Err
		}
		return NewRuntimeError(result.ErrorCode, result.ErrorMsg, nil)
	}
}

func (d *Dispatcher) ScanAndDispatch(ctx context.Context) int {
	conns := d.registry.ListByState(SessionStateReady)
	dispatched := 0
	for _, conn := range conns {
		if ctx.Err() != nil {
			break
		}
		cmds, err := d.store.ListDispatchable(conn.RuntimeID(), 50)
		if err != nil {
			log.Logger.Warnf("dispatcher: list dispatchable failed runtimeID=%s err=%v", conn.RuntimeID(), err)
			continue
		}
		for _, cmd := range cmds {
			if ctx.Err() != nil {
				break
			}
			if err := d.dispatchOnline(ctx, conn, cmd); err != nil {
				log.Logger.Warnf("dispatcher: dispatch failed commandID=%s runtimeID=%s err=%v", cmd.ID, conn.RuntimeID(), err)
			} else {
				dispatched++
			}
		}
	}
	return dispatched
}

func (d *Dispatcher) checkCapability(conn *Connection, commandName string) error {
	var requiredCap string
	switch commandName {
	case contracts.MsgPetSpawn, contracts.MsgPetDestroy, contracts.MsgPetShow, contracts.MsgPetHide:
		requiredCap = contracts.CapPetWindow
	case contracts.MsgPetPlayAction:
		requiredCap = contracts.CapPetAnimationFrameSequence
	case contracts.MsgPetUpdateSettings:
		requiredCap = contracts.CapPetSettings
	case contracts.MsgPetRecenter:
		requiredCap = contracts.CapPetRecenter
	case contracts.MsgRuntimeSync, contracts.MsgRuntimeStateProbe:
		return nil
	default:
		return nil
	}
	if !conn.HasCapability(requiredCap) {
		return NewRuntimeError(ErrCodeRuntimeCapabilityMissing, "runtime missing capability: "+requiredCap, ErrRuntimeCapabilityMissing)
	}
	return nil
}

func (d *Dispatcher) retryDelay(attempt int) time.Duration {
	base := d.config.RetryBaseDelay()
	maxDelay := d.config.RetryMaxDelay()
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}
	if attempt < 1 {
		attempt = 1
	}
	delay := base
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= maxDelay {
			delay = maxDelay
			break
		}
	}
	if delay < maxDelay {
		jitter := time.Duration(rand.Int63n(int64(base) + 1))
		delay += jitter
		if delay > maxDelay {
			delay = maxDelay
		}
	}
	return delay
}

func (d *Dispatcher) retryableError(errCode string) bool {
	switch errCode {
	case ErrCodeRuntimeCommandRejected, ErrCodeRuntimeCapabilityMissing,
		ErrCodeRuntimeProtocolIncompatible, ErrCodeRuntimeUnauthorized,
		ErrCodeRuntimeForbiddenOrigin:
		return false
	case ErrCodeRuntimeDisconnected, ErrCodeRuntimeHeartbeatTimeout,
		ErrCodeRuntimeSessionReplaced, ErrCodeRuntimeCommandTimeout,
		ErrCodeRuntimeBusy:
		return true
	}
	return true
}

func (d *Dispatcher) resolveConnection(runtimeID, userID string) (*Connection, error) {
	if runtimeID != "" {
		conn := d.registry.GetByRuntime(runtimeID)
		if conn != nil {
			return conn, nil
		}
		return nil, ErrRuntimeOffline
	}
	return d.registry.SelectRuntime(userID)
}

func (d *Dispatcher) waitForResult(ctx context.Context, ch <-chan *PendingResult, timeout time.Duration) (*PendingResult, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, NewRuntimeError(ErrCodeRuntimeCommandTimeout, "command timeout waiting for applied", ErrRuntimeCommandTimeout)
	}
}

func (d *Dispatcher) revertToPending(cmd *RuntimeCommand, errorCode, errorMsg string) {
	nextAttempt := time.Now().Add(d.retryDelay(cmd.AttemptCount)).Format(runtimeTimeFormat)
	_ = d.store.UpdateStatusCAS(cmd.ID, []string{string(contracts.CmdSent)}, string(contracts.CmdPending), map[string]interface{}{
		"next_attempt_at":    nextAttempt,
		"last_error_code":    errorCode,
		"last_error_message": errorMsg,
	})
	cmd.Status = string(contracts.CmdPending)
}

func applyMessageMetadata(msg *contracts.RuntimeMessage, userID, installationID, petInstanceID string, deadline time.Duration) {
	if userID != "" {
		msg.UserID = userID
	}
	if installationID != "" {
		msg.InstallationID = installationID
	}
	if petInstanceID != "" {
		msg.PetInstanceID = petInstanceID
	}
	if deadline > 0 {
		dl := time.Now().Add(deadline)
		msg.DeadlineAt = &dl
	}
}

func errorCodeOf(err error) string {
	if err == nil {
		return ""
	}
	if re, ok := err.(*RuntimeError); ok && re.Code != "" {
		return re.Code
	}
	switch err {
	case ErrRuntimeDisconnected:
		return ErrCodeRuntimeDisconnected
	case ErrRuntimeHeartbeatTimeout:
		return ErrCodeRuntimeHeartbeatTimeout
	case ErrRuntimeSessionReplaced:
		return ErrCodeRuntimeSessionReplaced
	case ErrRuntimeCommandTimeout:
		return ErrCodeRuntimeCommandTimeout
	case ErrRuntimeBusy:
		return ErrCodeRuntimeBusy
	case ErrRuntimeCapabilityMissing:
		return ErrCodeRuntimeCapabilityMissing
	case ErrRuntimeCommandRejected:
		return ErrCodeRuntimeCommandRejected
	case ErrRuntimeProtocolIncompatible:
		return ErrCodeRuntimeProtocolIncompatible
	case ErrRuntimeUnauthorized:
		return ErrCodeRuntimeUnauthorized
	case ErrRuntimeForbiddenOrigin:
		return ErrCodeRuntimeForbiddenOrigin
	}
	return ""
}

func marshalResultJSON(result *PendingResult) string {
	if result == nil {
		return ""
	}
	data, err := json.Marshal(map[string]interface{}{
		"status":          string(result.Status),
		"errorCode":       result.ErrorCode,
		"errorMessage":    result.ErrorMsg,
		"appliedRevision": result.AppliedRev,
		"acceptedAction":  result.AcceptedAction,
		"playbackReqId":   result.PlaybackReqID,
	})
	if err != nil {
		return ""
	}
	return string(data)
}
