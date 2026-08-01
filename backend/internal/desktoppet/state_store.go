// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/taskstate"
	"gorm.io/gorm"
)

type StateStore struct {
	db *gorm.DB
}

func NewStateStore(db *gorm.DB) *StateStore {
	return &StateStore{db: db}
}

func (s *StateStore) WithTx(tx *gorm.DB) *StateStore {
	return &StateStore{db: tx}
}

func tableNameFor(et contracts.EntityType) string {
	switch et {
	case contracts.EntityGenerationTask:
		return "desktop_pet_generation_tasks"
	case contracts.EntityGenerationAction:
		return "desktop_pet_generation_task_actions"
	case contracts.EntityGenerationFrame:
		return "desktop_pet_generation_frames"
	case contracts.EntityProcessingTask:
		return "desktop_pet_processing_tasks"
	case contracts.EntityProcessingAction:
		return "desktop_pet_processing_actions"
	case contracts.EntityProcessedFrame:
		return "desktop_pet_processed_frames"
	case contracts.EntityPackage:
		return "desktop_pet_packages"
	default:
		return ""
	}
}

func columnsForEntity(et contracts.EntityType) map[string]bool {
	cols := map[string]bool{
		"status": true, "current_stage": true, "row_version": true,
		"status_reason": true, "failure_stage": true, "last_transition_at": true,
		"updated_at": true, "error_code": true, "error_message": true,
	}
	switch et {
	case contracts.EntityGenerationTask, contracts.EntityProcessingTask:
		for _, c := range []string{"progress", "started_at", "completed_at", "execution_id",
			"worker_id", "lease_expires_at", "last_heartbeat_at",
			"cancel_requested_at", "submitted_at", "cancelling_at", "cancelled_at"} {
			cols[c] = true
		}
	case contracts.EntityGenerationAction, contracts.EntityProcessingAction:
		for _, c := range []string{"progress", "started_at", "completed_at",
			"execution_id", "worker_id"} {
			cols[c] = true
		}
	case contracts.EntityGenerationFrame, contracts.EntityProcessedFrame:
		for _, c := range []string{"started_at", "completed_at", "execution_id"} {
			cols[c] = true
		}
	}
	return cols
}

func (s *StateStore) ApplyTransition(ctx context.Context, p taskstate.ApplyTransitionParams) (*taskstate.ApplyTransitionResult, error) {
	tableName := tableNameFor(p.EntityType)
	if tableName == "" {
		return nil, fmt.Errorf("taskstate: unknown entity type %s", p.EntityType)
	}
	if LegacyPackageWritesDisabled && p.EntityType == contracts.EntityPackage {
		return &taskstate.ApplyTransitionResult{
			Applied:        false,
			ConflictReason: ErrCodeLegacyPackageWriteDisabled,
		}, nil
	}
	allowedCols := columnsForEntity(p.EntityType)

	var current struct {
		Status       string `gorm:"column:status"`
		CurrentStage string `gorm:"column:current_stage"`
		RowVersion   int64  `gorm:"column:row_version"`
		ExecutionID  string `gorm:"column:execution_id"`
	}

	selectCols := []string{"status", "current_stage", "row_version"}
	if allowedCols["execution_id"] {
		selectCols = append(selectCols, "execution_id")
	}

	readErr := s.db.Table(tableName).
		Where("id = ?", p.EntityID).
		Select(selectCols).
		First(&current).Error

	if readErr != nil {
		if errors.Is(readErr, gorm.ErrRecordNotFound) {
			return &taskstate.ApplyTransitionResult{Applied: false, ConflictReason: "not_found"}, nil
		}
		return nil, readErr
	}

	if len(p.ExpectedStatuses) > 0 {
		matched := false
		for _, es := range p.ExpectedStatuses {
			if current.Status == string(es) {
				matched = true
				break
			}
		}
		if !matched {
			return &taskstate.ApplyTransitionResult{
				Applied:         false,
				ConflictReason:  "state_conflict",
				PreviousStatus:  contracts.LifecycleStatus(current.Status),
				PreviousStage:   contracts.Stage(current.CurrentStage),
				PreviousVersion: current.RowVersion,
			}, nil
		}
	}

	if p.ExpectedVersion > 0 && current.RowVersion != p.ExpectedVersion {
		return &taskstate.ApplyTransitionResult{
			Applied:         false,
			ConflictReason:  "version_mismatch",
			PreviousStatus:  contracts.LifecycleStatus(current.Status),
			PreviousStage:   contracts.Stage(current.CurrentStage),
			PreviousVersion: current.RowVersion,
		}, nil
	}

	if p.NeedOwnership && p.ExecutionID != "" && current.ExecutionID != p.ExecutionID {
		return &taskstate.ApplyTransitionResult{
			Applied:         false,
			ConflictReason:  "ownership_lost",
			PreviousStatus:  contracts.LifecycleStatus(current.Status),
			PreviousStage:   contracts.Stage(current.CurrentStage),
			PreviousVersion: current.RowVersion,
		}, nil
	}

	updates := buildFieldUpdateMap(p.Updates, allowedCols)

	query := s.db.Table(tableName).Where("id = ?", p.EntityID)
	if len(p.ExpectedStatuses) > 0 {
		statuses := make([]string, len(p.ExpectedStatuses))
		for i, es := range p.ExpectedStatuses {
			statuses[i] = string(es)
		}
		query = query.Where("status IN ?", statuses)
	}
	if p.ExpectedVersion > 0 {
		query = query.Where("row_version = ?", p.ExpectedVersion)
	}
	if p.NeedOwnership && p.ExecutionID != "" && allowedCols["execution_id"] {
		query = query.Where("execution_id = ?", p.ExecutionID)
	}

	result := query.Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		reason := "state_conflict"
		if p.ExpectedVersion > 0 {
			reason = "version_mismatch"
		} else if p.NeedOwnership && p.ExecutionID != "" {
			reason = "ownership_lost"
		}
		return &taskstate.ApplyTransitionResult{
			Applied:         false,
			ConflictReason:  reason,
			PreviousStatus:  contracts.LifecycleStatus(current.Status),
			PreviousStage:   contracts.Stage(current.CurrentStage),
			PreviousVersion: current.RowVersion,
		}, nil
	}

	audit := p.Audit
	audit.FromStatus = contracts.LifecycleStatus(current.Status)
	audit.FromStage = contracts.Stage(current.CurrentStage)
	audit.PreviousVersion = current.RowVersion
	audit.CurrentVersion = current.RowVersion + 1
	if err := s.writeAuditRecord(audit); err != nil {
		// nolint:errcheck
		_ = err
	}

	return &taskstate.ApplyTransitionResult{
		Applied:         true,
		PreviousStatus:  contracts.LifecycleStatus(current.Status),
		PreviousStage:   contracts.Stage(current.CurrentStage),
		PreviousVersion: current.RowVersion,
		CurrentVersion:  current.RowVersion + 1,
	}, nil
}

func buildFieldUpdateMap(u taskstate.FieldUpdates, allowedCols map[string]bool) map[string]interface{} {
	m := make(map[string]interface{})

	if allowedCols["status"] {
		m["status"] = string(u.Status)
	}
	if allowedCols["current_stage"] && u.Stage != "" {
		m["current_stage"] = string(u.Stage)
	}
	if allowedCols["status_reason"] {
		m["status_reason"] = u.StatusReason
	}
	if allowedCols["failure_stage"] {
		m["failure_stage"] = u.FailureStage
	}
	if allowedCols["error_code"] {
		if u.ClearError {
			m["error_code"] = ""
		} else if u.ErrorCode != "" {
			m["error_code"] = u.ErrorCode
		}
	}
	if allowedCols["error_message"] {
		if u.ClearError {
			m["error_message"] = ""
		} else if u.ErrorMessage != "" {
			m["error_message"] = u.ErrorMessage
		}
	}
	if u.Progress != nil && allowedCols["progress"] {
		m["progress"] = *u.Progress
	}
	if allowedCols["started_at"] {
		if u.SetStartedIfEmpty {
			m["started_at"] = gorm.Expr("CASE WHEN started_at = '' OR started_at IS NULL THEN ? ELSE started_at END", u.StartedAt)
		} else if u.StartedAt != "" {
			m["started_at"] = u.StartedAt
		}
	}
	if allowedCols["completed_at"] {
		if u.ClearCompletedAt {
			m["completed_at"] = ""
		} else if u.CompletedAt != "" {
			m["completed_at"] = u.CompletedAt
		}
	}
	if allowedCols["submitted_at"] && u.SubmittedAt != "" {
		m["submitted_at"] = u.SubmittedAt
	}
	if allowedCols["cancelling_at"] {
		if u.CancellingAt != "" {
			m["cancelling_at"] = u.CancellingAt
		}
	}
	if allowedCols["cancelled_at"] {
		if u.ClearCancelledAt {
			m["cancelled_at"] = ""
		} else if u.CancelledAt != "" {
			m["cancelled_at"] = u.CancelledAt
		}
	}
	if allowedCols["cancel_requested_at"] && u.CancelRequestedAt != "" {
		m["cancel_requested_at"] = u.CancelRequestedAt
	}
	if allowedCols["last_transition_at"] && u.LastTransitionAt != "" {
		m["last_transition_at"] = u.LastTransitionAt
	}
	if allowedCols["updated_at"] && u.UpdatedAt != "" {
		m["updated_at"] = u.UpdatedAt
	}
	if allowedCols["execution_id"] {
		if u.ClearExecution {
			m["execution_id"] = ""
		} else if u.ExecutionID != "" {
			m["execution_id"] = u.ExecutionID
		}
	}
	if allowedCols["worker_id"] {
		if u.ClearWorker {
			m["worker_id"] = ""
		} else if u.WorkerID != "" {
			m["worker_id"] = u.WorkerID
		}
	}
	if allowedCols["lease_expires_at"] {
		if u.ClearLease {
			m["lease_expires_at"] = ""
		} else if u.LeaseExpiresAt != "" {
			m["lease_expires_at"] = u.LeaseExpiresAt
		}
	}
	if allowedCols["last_heartbeat_at"] {
		if u.ClearHeartbeat {
			m["last_heartbeat_at"] = ""
		} else if u.LastHeartbeatAt != "" {
			m["last_heartbeat_at"] = u.LastHeartbeatAt
		}
	}
	if u.BumpVersion && allowedCols["row_version"] {
		m["row_version"] = gorm.Expr("row_version + 1")
	}
	return m
}

func (s *StateStore) GetSnapshot(ctx context.Context, et contracts.EntityType, id string) (*taskstate.EntitySnapshot, error) {
	tableName := tableNameFor(et)
	if tableName == "" {
		return nil, fmt.Errorf("taskstate: unknown entity type %s", et)
	}

	var row struct {
		Status           string `gorm:"column:status"`
		CurrentStage     string `gorm:"column:current_stage"`
		RowVersion       int64  `gorm:"column:row_version"`
		ExecutionID      string `gorm:"column:execution_id"`
		WorkerID         string `gorm:"column:worker_id"`
		CancelRequestedAt string `gorm:"column:cancel_requested_at"`
		LeaseExpiresAt   string `gorm:"column:lease_expires_at"`
	}

	allowedCols := columnsForEntity(et)
	selectCols := []string{"status", "current_stage", "row_version"}
	if allowedCols["execution_id"] {
		selectCols = append(selectCols, "execution_id")
	}
	if allowedCols["worker_id"] {
		selectCols = append(selectCols, "worker_id")
	}
	if allowedCols["cancel_requested_at"] {
		selectCols = append(selectCols, "cancel_requested_at")
	}
	if allowedCols["lease_expires_at"] {
		selectCols = append(selectCols, "lease_expires_at")
	}

	err := s.db.Table(tableName).Where("id = ?", id).Select(selectCols).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	snap := &taskstate.EntitySnapshot{
		Status:          contracts.LifecycleStatus(row.Status),
		Stage:           contracts.Stage(row.CurrentStage),
		RowVersion:      row.RowVersion,
		ExecutionID:     row.ExecutionID,
		WorkerID:        row.WorkerID,
		CancelRequested: row.CancelRequestedAt != "",
		CancelRequestedAt: row.CancelRequestedAt,
		LeaseExpiresAt:  row.LeaseExpiresAt,
	}
	return snap, nil
}

func (s *StateStore) writeAuditRecord(record taskstate.AuditRecord) error {
	metadataJSON := "{}"
	if record.Metadata != nil {
		if b, err := json.Marshal(record.Metadata); err == nil {
			metadataJSON = string(b)
		}
	}

	auditRow := map[string]interface{}{
		"id":               record.ID,
		"entity_type":      string(record.EntityType),
		"entity_id":        record.EntityID,
		"parent_task_id":   record.ParentTaskID,
		"execution_id":     record.ExecutionID,
		"attempt_id":       record.AttemptID,
		"from_status":      string(record.FromStatus),
		"to_status":        string(record.ToStatus),
		"from_stage":       string(record.FromStage),
		"to_stage":         string(record.ToStage),
		"reason_code":      string(record.ReasonCode),
		"error_code":       record.ErrorCode,
		"error_message":    record.ErrorMessage,
		"failure_stage":    string(record.FailureStage),
		"actor_type":       string(record.ActorType),
		"actor_id":         record.ActorID,
		"previous_version": record.PreviousVersion,
		"current_version":  record.CurrentVersion,
		"metadata_json":    metadataJSON,
		"created_at":       record.CreatedAt,
	}

	return s.db.Table("desktop_pet_state_transitions").Create(auditRow).Error
}

func (s *StateStore) WriteAudit(ctx context.Context, record taskstate.AuditRecord) error {
	return s.writeAuditRecord(record)
}

type stateTransitionRow struct {
	ID              string `gorm:"column:id"`
	EntityType      string `gorm:"column:entity_type"`
	EntityID        string `gorm:"column:entity_id"`
	ParentTaskID    string `gorm:"column:parent_task_id"`
	ExecutionID     string `gorm:"column:execution_id"`
	AttemptID       string `gorm:"column:attempt_id"`
	FromStatus      string `gorm:"column:from_status"`
	ToStatus        string `gorm:"column:to_status"`
	FromStage       string `gorm:"column:from_stage"`
	ToStage         string `gorm:"column:to_stage"`
	ReasonCode      string `gorm:"column:reason_code"`
	ErrorCode       string `gorm:"column:error_code"`
	ErrorMessage    string `gorm:"column:error_message"`
	FailureStage    string `gorm:"column:failure_stage"`
	ActorType       string `gorm:"column:actor_type"`
	ActorID         string `gorm:"column:actor_id"`
	PreviousVersion int64  `gorm:"column:previous_version"`
	CurrentVersion  int64  `gorm:"column:current_version"`
	MetadataJSON    string `gorm:"column:metadata_json"`
	CreatedAt       string `gorm:"column:created_at"`
}

func (r stateTransitionRow) toRecord() taskstate.AuditRecord {
	var metadata map[string]any
	if r.MetadataJSON != "" && r.MetadataJSON != "{}" {
		_ = json.Unmarshal([]byte(r.MetadataJSON), &metadata)
	}
	return taskstate.AuditRecord{
		ID:              r.ID,
		EntityType:      contracts.EntityType(r.EntityType),
		EntityID:        r.EntityID,
		ParentTaskID:    r.ParentTaskID,
		ExecutionID:     r.ExecutionID,
		AttemptID:       r.AttemptID,
		FromStatus:      contracts.LifecycleStatus(r.FromStatus),
		ToStatus:        contracts.LifecycleStatus(r.ToStatus),
		FromStage:       contracts.Stage(r.FromStage),
		ToStage:         contracts.Stage(r.ToStage),
		ReasonCode:      contracts.TransitionReason(r.ReasonCode),
		ErrorCode:       r.ErrorCode,
		ErrorMessage:    r.ErrorMessage,
		FailureStage:    contracts.Stage(r.FailureStage),
		ActorType:       contracts.ActorType(r.ActorType),
		ActorID:         r.ActorID,
		PreviousVersion: r.PreviousVersion,
		CurrentVersion:  r.CurrentVersion,
		Metadata:        metadata,
		CreatedAt:       r.CreatedAt,
	}
}

func (s *StateStore) ListAuditsByEntity(ctx context.Context, et contracts.EntityType, entityID string, limit int) ([]taskstate.AuditRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []stateTransitionRow
	err := s.db.Table("desktop_pet_state_transitions").
		Where("entity_type = ? AND entity_id = ?", string(et), entityID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	records := make([]taskstate.AuditRecord, len(rows))
	for i, r := range rows {
		records[i] = r.toRecord()
	}
	return records, nil
}

func (s *StateStore) ListAuditsByParent(ctx context.Context, parentTaskID string, limit int) ([]taskstate.AuditRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []stateTransitionRow
	err := s.db.Table("desktop_pet_state_transitions").
		Where("parent_task_id = ?", parentTaskID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	records := make([]taskstate.AuditRecord, len(rows))
	for i, r := range rows {
		records[i] = r.toRecord()
	}
	return records, nil
}
