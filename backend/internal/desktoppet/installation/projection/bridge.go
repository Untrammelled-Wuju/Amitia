// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package projection

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
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
	Sequence         int64  `gorm:"column:sequence"`
	Delivered        int    `gorm:"column:delivered"`
}

func (RuntimeEventRecord) TableName() string {
	return "desktop_pet_runtime_event_records"
}

type ProjectionBridge struct {
	db        *gorm.DB
	service   *Service
	runtimeID string
	userID    string
	deviceID   string
	stopCh    chan struct{}
	doneCh    chan struct{}
}

func NewProjectionBridge(db *gorm.DB, service *Service, userID, deviceID, runtimeID string) *ProjectionBridge {
	return &ProjectionBridge{
		db:        db,
		service:   service,
		userID:    userID,
		deviceID:  deviceID,
		runtimeID: runtimeID,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

func (b *ProjectionBridge) Start(ctx context.Context) {
	if b == nil || b.db == nil || b.service == nil {
		return
	}
	go b.run(ctx)
}

func (b *ProjectionBridge) Stop() {
	if b == nil {
		return
	}
	close(b.stopCh)
	<-b.doneCh
}

func (b *ProjectionBridge) run(ctx context.Context) {
	defer close(b.doneCh)
	ticker := time.NewTicker(bridgePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.processBatch(ctx)
		}
	}
}

func (b *ProjectionBridge) processBatch(ctx context.Context) {
	var events []RuntimeEventRecord
	err := b.db.Where("delivered = 0").
		Order("sequence ASC").
		Limit(eventBatchSize).
		Find(&events).Error
	if err != nil {
		return
	}

	for _, event := range events {
		b.processEvent(ctx, event)
	}
}

func (b *ProjectionBridge) processEvent(ctx context.Context, event RuntimeEventRecord) {
	switch event.EventType {
	case "runtime.state.snapshot":
		b.handleStateSnapshot(ctx, event)
	case "runtime.command.acknowledged":
		b.handleCommandAcknowledged(ctx, event)
	}

	b.markDelivered(event.ID)
}

func (b *ProjectionBridge) handleStateSnapshot(ctx context.Context, event RuntimeEventRecord) {
	var snapshot StateSnapshotPayload
	if err := json.Unmarshal(event.Payload, &snapshot); err != nil {
		return
	}

	heartbeat := &coordinator.RuntimeHeartbeat{
		AppliedDesiredRevision:  snapshot.AppliedDesiredRevision,
		AppliedSettingsRevision: int64(snapshot.AppliedSettingsRevision),
		ActualReleaseID:         snapshot.ReleaseID,
		ActualVisible:           boolToInt(snapshot.WindowStatus == "visible"),
		ActualActionKey:         snapshot.CurrentActionKey,
		ActualHealth:            b.mapHealthStatus(snapshot.PlaybackStatus),
		Timestamp:               snapshot.CapturedAt,
	}

	_ = b.service.HandleRuntimeHeartbeat(ctx, b.userID, b.deviceID, b.runtimeID, heartbeat)
}

func (b *ProjectionBridge) handleCommandAcknowledged(ctx context.Context, event RuntimeEventRecord) {
	var ack CommandAckPayload
	if err := json.Unmarshal(event.Payload, &ack); err != nil {
		return
	}

	if ack.Status != "completed" && ack.Status != "failed_terminal" {
		return
	}

	result := &coordinator.CommandResult{
		AppliedRevision: 0,
		Timestamp:       time.Now().Format("2006-01-02 15:04:05"),
	}

	if ack.Status == "completed" {
		result.Success = true
		result.AppliedRevision = ack.AppliedRevision
	} else {
		result.Success = false
	}

	_ = b.service.HandleCommandResult(ctx, b.userID, b.deviceID, result)
}

func (b *ProjectionBridge) markDelivered(eventID string) {
	_ = b.db.Model(&RuntimeEventRecord{}).
		Where("id = ?", eventID).
		Update("delivered", 1).Error
}

func (b *ProjectionBridge) mapHealthStatus(playbackStatus string) string {
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

func boolToInt(b bool) int {
	if b {
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
