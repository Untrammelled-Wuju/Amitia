// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package projection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

const (
	bridgePollInterval = 5 * time.Second
	eventBatchSize     = 50
)

type RuntimeEventRecord struct {
	ID               string `gorm:"column:id"`
	EventType        string `gorm:"column:event_type"`
	Payload          []byte `gorm:"column:payload"`
	RuntimeSessionID string `gorm:"column:runtime_session_id"`
	CommandID        string `gorm:"column:command_id"`
	Sequence         int64  `gorm:"column:sequence"`
	Delivered        int    `gorm:"column:delivered"`
}

func (RuntimeEventRecord) TableName() string { return "desktop_pet_runtime_event_records" }

type runtimeSessionIdentity struct {
	ID        string `gorm:"column:id"`
	UserID    string `gorm:"column:user_id"`
	DeviceID  string `gorm:"column:device_id"`
	RuntimeID string `gorm:"column:runtime_id"`
}

func (runtimeSessionIdentity) TableName() string { return "desktop_pet_runtime_sessions" }

type runtimeCommandProjection struct {
	ID              string `gorm:"column:id"`
	DesiredRevision int64  `gorm:"column:desired_revision"`
	IdempotencyKey  string `gorm:"column:idempotency_key"`
}

func (runtimeCommandProjection) TableName() string { return "desktop_pet_runtime_commands_v2" }

type ProjectionBridge struct {
	db      *gorm.DB
	service *Service
	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
}

func NewProjectionBridge(db *gorm.DB, service *Service) *ProjectionBridge {
	return &ProjectionBridge{db: db, service: service}
}

func (b *ProjectionBridge) Start(ctx context.Context) {
	if b == nil || b.db == nil || b.service == nil || ctx == nil {
		return
	}
	b.mu.Lock()
	if b.running.Load() {
		b.mu.Unlock()
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	b.running.Store(true)
	b.wg.Add(1)
	b.mu.Unlock()
	go func() {
		defer b.wg.Done()
		defer b.running.Store(false)
		b.run(workerCtx)
	}()
}

func (b *ProjectionBridge) Stop() {
	if b == nil {
		return
	}
	b.mu.Lock()
	cancel := b.cancel
	b.cancel = nil
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	b.wg.Wait()
}

func (b *ProjectionBridge) IsRunning() bool {
	return b != nil && b.running.Load()
}

func (b *ProjectionBridge) run(ctx context.Context) {
	ticker := time.NewTicker(bridgePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := b.processBatch(ctx); err != nil && ctx.Err() == nil {
				log.Warn("installation projection bridge batch failed: ", err)
			}
		}
	}
}

func (b *ProjectionBridge) processBatch(ctx context.Context) error {
	var events []RuntimeEventRecord
	if err := b.db.WithContext(ctx).Where("delivered = 0").Order("sequence ASC").Limit(eventBatchSize).Find(&events).Error; err != nil {
		return err
	}
	for _, event := range events {
		if err := b.processEvent(ctx, event); err != nil {
			log.Warn("installation projection bridge event ", event.ID, " failed: ", err)
			continue
		}
		if err := b.markDelivered(ctx, event.ID); err != nil {
			return err
		}
	}
	return nil
}

func (b *ProjectionBridge) processEvent(ctx context.Context, event RuntimeEventRecord) error {
	switch event.EventType {
	case "runtime.state.snapshot":
		return b.handleStateSnapshot(ctx, event)
	case "runtime.command.acknowledged", "command.ack":
		return b.handleCommandAcknowledged(ctx, event)
	default:
		return nil
	}
}

func (b *ProjectionBridge) sessionIdentity(ctx context.Context, sessionID string) (*runtimeSessionIdentity, error) {
	if sessionID == "" {
		return nil, errors.New("projection bridge: runtime session id required")
	}
	var identity runtimeSessionIdentity
	if err := b.db.WithContext(ctx).Where("id = ?", sessionID).First(&identity).Error; err != nil {
		return nil, err
	}
	if identity.UserID == "" || identity.DeviceID == "" || identity.RuntimeID == "" {
		return nil, errors.New("projection bridge: incomplete runtime session identity")
	}
	return &identity, nil
}

func (b *ProjectionBridge) handleStateSnapshot(ctx context.Context, event RuntimeEventRecord) error {
	var snapshot StateSnapshotPayload
	if err := json.Unmarshal(event.Payload, &snapshot); err != nil {
		return err
	}
	identity, err := b.sessionIdentity(ctx, event.RuntimeSessionID)
	if err != nil {
		return err
	}
	heartbeat := &coordinator.RuntimeHeartbeat{
		InstallationID:          snapshot.InstallationID,
		PetID:                   snapshot.PetID,
		AppliedDesiredRevision:  snapshot.AppliedDesiredRevision,
		AppliedSettingsRevision: int64(snapshot.AppliedSettingsRevision),
		ActualReleaseID:         snapshot.ReleaseID,
		ActualVisible:           boolToInt(snapshot.WindowStatus == "visible"),
		ActualActionKey:         snapshot.CurrentActionKey,
		ActualHealth:            mapHealthStatus(snapshot.PlaybackStatus),
		Timestamp:               snapshot.CapturedAt,
	}
	return b.service.HandleRuntimeHeartbeat(ctx, identity.UserID, identity.DeviceID, identity.RuntimeID, heartbeat)
}

func (b *ProjectionBridge) handleCommandAcknowledged(ctx context.Context, event RuntimeEventRecord) error {
	var ack CommandAckPayload
	if err := json.Unmarshal(event.Payload, &ack); err != nil {
		return err
	}
	if ack.Status != "completed" && ack.Status != "failed_terminal" {
		return nil
	}
	identity, err := b.sessionIdentity(ctx, event.RuntimeSessionID)
	if err != nil {
		return err
	}
	appliedRevision := ack.AppliedRevision
	commandID := ack.CommandID
	if commandID == "" {
		commandID = event.CommandID
	}
	var command runtimeCommandProjection
	if commandID != "" {
		if err := b.db.WithContext(ctx).Where("id = ?", commandID).First(&command).Error; err != nil {
			return fmt.Errorf("projection bridge: load acknowledged command %s: %w", commandID, err)
		}
		if appliedRevision == 0 {
			appliedRevision = command.DesiredRevision
		}
	}
	result := &coordinator.CommandResult{
		Success:         ack.Status == "completed",
		AppliedRevision: appliedRevision,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}
	if err := b.service.HandleCommandResult(ctx, identity.UserID, identity.DeviceID, result); err != nil {
		return err
	}
	if err := b.completeRecenterOperationFromACK(ctx, command.IdempotencyKey, ack.Status); err != nil {
		return err
	}
	return b.completeDesiredStateOperationFromACK(ctx, identity.UserID, identity.DeviceID, command.DesiredRevision, ack.Status)
}

func (b *ProjectionBridge) completeDesiredStateOperationFromACK(ctx context.Context, userID, deviceID string, desiredRevision int64, ackStatus string) error {
	if desiredRevision <= 0 {
		return nil
	}
	var state struct {
		OperationID string `gorm:"column:operation_id"`
	}
	err := b.db.WithContext(ctx).Table("desktop_pet_runtime_desired_states").
		Select("operation_id").
		Where("user_id = ? AND device_id = ? AND desired_revision = ?", userID, deviceID, desiredRevision).
		Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if state.OperationID == "" {
		return nil
	}
	var op operation.InstallationOperation
	if err := b.db.WithContext(ctx).Where("id = ?", state.OperationID).First(&op).Error; err != nil {
		return err
	}
	switch op.OperationType {
	case operation.TypeEnable, operation.TypeDisable, operation.TypeSettings, operation.TypeDefaultAction:
	default:
		return nil
	}
	if op.IsTerminal() {
		return nil
	}
	if op.Status != operation.OpStatusWaitingRuntimeACK || op.Stage != operation.OpStageWaitingRuntimeACK {
		return fmt.Errorf("projection bridge: desired-state operation %s is not ready for terminal ACK: status=%s stage=%s", op.ID, op.Status, op.Stage)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	operationUpdates := map[string]interface{}{"updated_at": now}
	installationSyncState := ""
	switch ackStatus {
	case "completed":
		operationUpdates["status"] = operation.OpStatusCompleted
		operationUpdates["stage"] = operation.OpStageCompleted
		operationUpdates["completed_at"] = now
		installationSyncState = "confirmed"
	case "failed_terminal":
		operationUpdates["status"] = operation.OpStatusFailedTerminal
		operationUpdates["error_code"] = "RUNTIME_COMMAND_FAILED"
		installationSyncState = "failed"
	default:
		return nil
	}
	return b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&operation.InstallationOperation{}).
			Where("id = ? AND status = ? AND stage = ?", op.ID, operation.OpStatusWaitingRuntimeACK, operation.OpStageWaitingRuntimeACK).
			Updates(operationUpdates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errors.New("projection bridge: desired-state terminal operation CAS failed")
		}
		if op.InstallationID == "" {
			return nil
		}
		installRes := tx.Table("desktop_pet_installations").
			Where("id = ? AND user_id = ? AND device_id = ?", op.InstallationID, op.UserID, op.DeviceID).
			Updates(map[string]interface{}{"runtime_sync_state": installationSyncState, "updated_at": now})
		if installRes.Error != nil {
			return installRes.Error
		}
		if installRes.RowsAffected != 1 {
			return errors.New("projection bridge: desired-state installation sync-state update failed")
		}
		return nil
	})
}

func (b *ProjectionBridge) completeRecenterOperationFromACK(ctx context.Context, idempotencyKey, ackStatus string) error {
	const prefix = "recenter:"
	if !strings.HasPrefix(idempotencyKey, prefix) {
		return nil
	}
	recenterIdentity := strings.TrimPrefix(idempotencyKey, prefix)
	if recenterIdentity == "" {
		return errors.New("projection bridge: recenter command missing operation id")
	}
	opID := strings.SplitN(recenterIdentity, ":", 2)[0]
	if opID == "" {
		return errors.New("projection bridge: recenter command missing operation id")
	}
	var op operation.InstallationOperation
	if err := b.db.WithContext(ctx).Where("id = ? AND operation_type = ?", opID, operation.TypeRecenter).First(&op).Error; err != nil {
		return fmt.Errorf("projection bridge: load recenter operation %s: %w", opID, err)
	}
	if op.IsTerminal() {
		return nil
	}
	if op.Status != operation.OpStatusWaitingRuntimeACK || op.Stage != operation.OpStageWaitingRuntimeACK {
		return fmt.Errorf("projection bridge: recenter operation %s is not ready for terminal ACK: status=%s stage=%s", opID, op.Status, op.Stage)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]interface{}{"updated_at": now}
	switch ackStatus {
	case "completed":
		updates["status"] = operation.OpStatusCompleted
		updates["stage"] = operation.OpStageCompleted
		updates["completed_at"] = now
	case "failed_terminal":
		updates["status"] = operation.OpStatusFailedTerminal
		updates["error_code"] = "RUNTIME_COMMAND_FAILED"
	default:
		return nil
	}
	res := b.db.WithContext(ctx).Model(&operation.InstallationOperation{}).
		Where("id = ? AND status = ? AND stage = ?", opID, operation.OpStatusWaitingRuntimeACK, operation.OpStageWaitingRuntimeACK).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("projection bridge: recenter terminal operation CAS failed")
	}
	return nil
}

func (b *ProjectionBridge) markDelivered(ctx context.Context, eventID string) error {
	result := b.db.WithContext(ctx).Model(&RuntimeEventRecord{}).Where("id = ? AND delivered = 0").Updates(map[string]interface{}{
		"delivered":    1,
		"delivered_at": time.Now().UTC().Format(time.RFC3339),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("projection bridge: event delivery CAS failed")
	}
	return nil
}

func mapHealthStatus(playbackStatus string) string {
	switch playbackStatus {
	case "playing", "holding":
		return "healthy"
	case "failed":
		return "failed"
	case "idle", "stopped":
		return "online_no_pet"
	default:
		return "syncing"
	}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

type StateSnapshotPayload struct {
	ConnectionGeneration         int64  `json:"connectionGeneration"`
	EventSequence                int64  `json:"eventSequence"`
	ActualStateHash              string `json:"actualStateHash"`
	InstanceStatus               string `json:"instanceStatus"`
	WindowStatus                 string `json:"windowStatus"`
	RendererStatus               string `json:"rendererStatus"`
	PlaybackStatus               string `json:"playbackStatus"`
	AppliedDesiredRevision       int64  `json:"appliedDesiredRevision"`
	AppliedDesiredHash           string `json:"appliedDesiredHash,omitempty"`
	AppliedSettingsRevision      int    `json:"appliedSettingsRevision"`
	InstallationID               string `json:"installationId"`
	PetID                        string `json:"petId"`
	ReleaseID                    string `json:"releaseId"`
	StableActionKey              string `json:"stableActionKey"`
	CurrentActionKey             string `json:"currentActionKey"`
	PlaybackInstanceID           string `json:"playbackInstanceId,omitempty"`
	CurrentCommandID             string `json:"currentCommandId,omitempty"`
	LastProcessedCommandSequence int64  `json:"lastProcessedCommandSequence"`
	CapturedAt                   string `json:"capturedAt"`
}

type CommandAckPayload struct {
	CommandID        string `json:"commandId"`
	Status           string `json:"status"`
	AppliedRevision  int64  `json:"appliedRevision,omitempty"`
	RejectErrorCode  string `json:"rejectErrorCode,omitempty"`
	RejectReason     string `json:"rejectReason,omitempty"`
	RuntimeSessionID string `json:"runtimeSessionId,omitempty"`
	Sequence         int64  `json:"sequence,omitempty"`
}
