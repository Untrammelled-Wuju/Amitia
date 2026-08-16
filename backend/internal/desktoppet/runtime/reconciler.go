// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/log"
)

type Reconciler struct {
	config   *DesktopPetRuntimeConfig
	registry *RuntimeRegistry
	pending  *PendingTracker
	store    CommandStore
	state    StateStore
	snapshot *SnapshotBuilder
}

func NewReconciler(
	config *DesktopPetRuntimeConfig,
	registry *RuntimeRegistry,
	pending *PendingTracker,
	store CommandStore,
	state StateStore,
	snapshot *SnapshotBuilder,
) *Reconciler {
	return &Reconciler{
		config:   config,
		registry: registry,
		pending:  pending,
		store:    store,
		state:    state,
		snapshot: snapshot,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, conn *Connection) error {
	if conn == nil {
		return NewRuntimeError(ErrCodeRuntimeProtocolError, "connection is nil", ErrRuntimeProtocolError)
	}

	runtimeID := conn.RuntimeID()
	sessionID := conn.SessionID()
	userID := conn.UserID()

	conn.SetState(SessionStateSyncing)
	log.Logger.Infof("runtime reconciler: starting reconcile runtimeID=%s sessionID=%s userID=%s", runtimeID, sessionID, userID)

	desired, err := r.snapshot.BuildForRuntime(ctx, runtimeID, userID)
	if err != nil {
		conn.SetState(SessionStateDegraded)
		log.Logger.Errorf("runtime reconciler: snapshot build failed runtimeID=%s err=%v", runtimeID, err)
		return NewRuntimeError(ErrCodeRuntimeSnapshotInvalid, "failed to build desired snapshot", err)
	}

	syncPayload := contracts.SyncPayload{
		DesiredRevision: desired.DesiredRevision,
		EnsureAbsent:    desired.EnsureAbsent,
		DesiredPet:      desired.DesiredPet,
	}

	msg, err := buildMessage(
		contracts.KindControl,
		contracts.MsgRuntimeSync,
		runtimeID,
		sessionID,
		syncPayload,
	)
	if err != nil {
		conn.SetState(SessionStateDegraded)
		return NewRuntimeError(ErrCodeRuntimeProtocolError, "failed to build sync message", err)
	}

	deadline := time.Now().Add(r.config.CommandTimeout())
	msg.DeadlineAt = &deadline

	waiterCh, cancel := r.pending.Register("sync_"+sessionID, sessionID, conn.RuntimeID(), r.config.CommandTimeout())
	defer cancel()

	if err := conn.SendBlocking(msg, r.config.CommandTimeout()); err != nil {
		conn.SetState(SessionStateDegraded)
		log.Logger.Errorf("runtime reconciler: send sync failed runtimeID=%s err=%v", runtimeID, err)
		return NewRuntimeError(ErrCodeRuntimeOffline, "failed to send sync command", err)
	}

	log.Logger.Infof("runtime reconciler: sync sent runtimeID=%s desiredRevision=%d ensureAbsent=%v", runtimeID, desired.DesiredRevision, desired.EnsureAbsent)

	select {
	case result := <-waiterCh:
		if result.Err != nil {
			conn.SetState(SessionStateDegraded)
			log.Logger.Errorf("runtime reconciler: sync failed runtimeID=%s status=%s err=%v", runtimeID, result.Status, result.Err)
			return result.Err
		}
		if result.Status == contracts.ResultApplied {
			log.Logger.Infof("runtime reconciler: sync applied runtimeID=%s appliedRev=%d", runtimeID, result.AppliedRev)
			r.updateActualStateFromSync(runtimeID, sessionID, result)
			conn.SetState(SessionStateReady)
			r.cleanupSupersededCommands(ctx, runtimeID, desired.DesiredRevision)
			return nil
		}
		if result.Status == contracts.ResultAccepted {
			log.Logger.Infof("runtime reconciler: sync accepted runtimeID=%s", runtimeID)
			conn.SetState(SessionStateSyncing)
			return nil
		}
		if result.Status == contracts.ResultDuplicate {
			log.Logger.Infof("runtime reconciler: sync duplicate (already at revision) runtimeID=%s", runtimeID)
			conn.SetState(SessionStateReady)
			return nil
		}
		conn.SetState(SessionStateDegraded)
		log.Logger.Warnf("runtime reconciler: sync returned non-applied status runtimeID=%s status=%s", runtimeID, result.Status)
		return NewRuntimeError(ErrCodeRuntimeCommandFailed, "sync returned "+string(result.Status), nil)

	case <-ctx.Done():
		conn.SetState(SessionStateDegraded)
		return ctx.Err()

	case <-time.After(r.config.CommandTimeout()):
		conn.SetState(SessionStateDegraded)
		log.Logger.Errorf("runtime reconciler: sync timeout runtimeID=%s", runtimeID)
		return NewRuntimeError(ErrCodeRuntimeCommandTimeout, "sync command timeout", ErrRuntimeCommandTimeout)
	}
}

func (r *Reconciler) updateActualStateFromSync(runtimeID, sessionID string, result *PendingResult) {
	if result.ActualState != nil {
		state := &RuntimeActualState{
			RuntimeID:        runtimeID,
			InstallationID:   result.ActualState.InstallationID,
			PetInstanceID:    result.ActualState.PetInstanceID,
			SessionID:        sessionID,
			DesiredRevision:  result.AppliedRev,
			Visible:          boolToInt(result.ActualState.Visible),
			CurrentActionKey: result.ActualState.CurrentActionKey,
			PositionX:        result.ActualState.PositionX,
			PositionY:        result.ActualState.PositionY,
			ScreenID:         result.ActualState.ScreenID,
			Scale:            result.ActualState.Scale,
			Health:           "healthy",
			ObservedAt:       time.Now().Format(runtimeTimeFormat),
		}
		if err := r.state.UpsertActualState(state); err != nil {
			log.Logger.Errorf("runtime reconciler: upsert actual state failed runtimeID=%s err=%v", runtimeID, err)
		}
	}
}

func (r *Reconciler) cleanupSupersededCommands(ctx context.Context, runtimeID string, desiredRevision int64) {
	pendingCmds, err := r.store.ListPendingByRuntime(runtimeID)
	if err != nil {
		log.Logger.Errorf("runtime reconciler: list pending commands failed runtimeID=%s err=%v", runtimeID, err)
		return
	}

	superseded := 0
	for _, cmd := range pendingCmds {
		if cmd.CoalesceKey != "" && cmd.DesiredRevision < desiredRevision {
			if err := r.store.MarkSuperseded(runtimeID, cmd.CoalesceKey, desiredRevision); err == nil {
				superseded++
			}
		}
	}

	if superseded > 0 {
		log.Logger.Infof("runtime reconciler: superseded %d stale commands runtimeID=%s desiredRevision=%d", superseded, runtimeID, desiredRevision)
	}
}

func (r *Reconciler) HandleSyncResult(msg *contracts.RuntimeMessage, payload *contracts.SyncResultPayload) {
	if payload == nil {
		return
	}

	runtimeID := msg.RuntimeID
	sessionID := msg.SessionID

	for _, inst := range payload.Instances {
		state := &RuntimeActualState{
			RuntimeID:        runtimeID,
			InstallationID:   inst.InstallationID,
			PetInstanceID:    inst.PetInstanceID,
			SessionID:        sessionID,
			DesiredRevision:  payload.AppliedRevision,
			Visible:          boolToInt(inst.Visible),
			CurrentActionKey: inst.CurrentActionKey,
			PositionX:        inst.PositionX,
			PositionY:        inst.PositionY,
			ScreenID:         inst.ScreenID,
			Scale:            inst.Scale,
			Health:           "healthy",
			ObservedAt:       time.Now().Format(runtimeTimeFormat),
		}
		if err := r.state.UpsertActualState(state); err != nil {
			log.Logger.Errorf("runtime reconciler: upsert actual state failed runtimeID=%s err=%v", runtimeID, err)
		}
	}

	if len(payload.DestroyedStaleIds) > 0 {
		for _, staleID := range payload.DestroyedStaleIds {
			log.Logger.Infof("runtime reconciler: stale instance destroyed runtimeID=%s instanceID=%s", runtimeID, staleID)
		}
	}

	if len(payload.Warnings) > 0 {
		for _, w := range payload.Warnings {
			log.Logger.Warnf("runtime reconciler: sync warning runtimeID=%s: %s", runtimeID, w)
		}
	}

	result := &PendingResult{
		CommandID:  "sync_" + sessionID,
		Status:     contracts.ResultApplied,
		AppliedRev: payload.AppliedRevision,
	}
	if len(payload.Instances) > 0 {
		first := payload.Instances[0]
		result.ActualState = &first
	}
	r.pending.Complete(result)
}

func (r *Reconciler) MarkRuntimeOffline(runtimeID string) {
	if err := r.state.UpdateActualStateHealth(runtimeID, "offline"); err != nil {
		log.Logger.Errorf("runtime reconciler: mark offline failed runtimeID=%s err=%v", runtimeID, err)
	}
}

func (r *Reconciler) HandleHeartbeat(payload *contracts.HeartbeatPayload, runtimeID, sessionID string) {
	if payload == nil {
		return
	}

	for _, inst := range payload.PetInstances {
		state := &RuntimeActualState{
			RuntimeID:        runtimeID,
			InstallationID:   inst.InstallationID,
			PetInstanceID:    inst.PetInstanceID,
			SessionID:        sessionID,
			DesiredRevision:  payload.LastAppliedDesiredRevision,
			Visible:          boolToInt(inst.Visible),
			CurrentActionKey: inst.CurrentActionKey,
			PositionX:        inst.PositionX,
			PositionY:        inst.PositionY,
			ScreenID:         inst.ScreenID,
			Scale:            inst.Scale,
			Health:           "healthy",
			ObservedAt:       time.Now().Format(runtimeTimeFormat),
		}
		if !payload.RendererHealthy {
			state.Health = "degraded"
		}
		if err := r.state.UpsertActualState(state); err != nil {
			log.Logger.Errorf("runtime reconciler: heartbeat upsert failed runtimeID=%s err=%v", runtimeID, err)
		}
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r *Reconciler) RunPeriodic(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.registry.ForEach(func(conn *Connection) {
				if conn.State() != SessionStateReady {
					return
				}
				runtimeID := conn.RuntimeID()
				lastBeat := conn.LastHeartbeat()
				if time.Since(lastBeat) > r.config.HeartbeatTimeout() {
					log.Logger.Warnf("runtime reconciler: heartbeat timeout detected runtimeID=%s lastBeat=%v", runtimeID, lastBeat)
					conn.Close(closeHeartbeatTimeout, "heartbeat timeout")
					r.MarkRuntimeOffline(runtimeID)
				}
			})
			r.pending.CleanupExpired()
		}
	}
}

func init() {
	_ = json.Marshal
}
