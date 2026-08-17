// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type operationRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	Kind        string `gorm:"column:kind"`
	Status      string `gorm:"column:status"`
	StartedAt   string `gorm:"column:started_at"`
	UpdatedAt   string `gorm:"column:updated_at"`
	CompletedAt string `gorm:"column:completed_at"`
	Error       string `gorm:"column:error"`
	Metadata    string `gorm:"column:metadata"`
}

func (operationRecord) TableName() string {
	return "desktop_pet_migration_operations"
}

type checkpointRecord struct {
	ID             string `gorm:"column:id;primaryKey"`
	OperationID    string `gorm:"column:operation_id"`
	StepName       string `gorm:"column:step_name"`
	LastPrimaryKey string `gorm:"column:last_primary_key"`
	ProcessedCount int    `gorm:"column:processed_count"`
	InputHash      string `gorm:"column:input_hash"`
	OutputHash     string `gorm:"column:output_hash"`
	ConflictCount  int    `gorm:"column:conflict_count"`
	UpdatedAt      string `gorm:"column:updated_at"`
}

func (checkpointRecord) TableName() string {
	return "desktop_pet_migration_checkpoints"
}

type conflictRecord struct {
	ID             string `gorm:"column:id;primaryKey"`
	OperationID    string `gorm:"column:operation_id"`
	EntityKind     string `gorm:"column:entity_kind"`
	EntityID       string `gorm:"column:entity_id"`
	ConflictReason string `gorm:"column:conflict_reason"`
	DetectedAt     string `gorm:"column:detected_at"`
}

func (conflictRecord) TableName() string {
	return "desktop_pet_migration_conflicts"
}

type readCutoverRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	OperationID string `gorm:"column:operation_id;uniqueIndex:idx_read_cutover_op_step"`
	StepName    string `gorm:"column:step_name;uniqueIndex:idx_read_cutover_op_step"`
	CutoverAt   string `gorm:"column:cutover_at"`
	Verified    int    `gorm:"column:verified"`
}

func (readCutoverRecord) TableName() string {
	return "desktop_pet_read_cutovers"
}

type writeCutoverRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	OperationID string `gorm:"column:operation_id;uniqueIndex:idx_write_cutover_op_step"`
	StepName    string `gorm:"column:step_name;uniqueIndex:idx_write_cutover_op_step"`
	CutoverAt   string `gorm:"column:cutover_at"`
	Verified    int    `gorm:"column:verified"`
}

func (writeCutoverRecord) TableName() string {
	return "desktop_pet_write_cutovers"
}

type metadataJSON struct {
	PlanID         string `json:"planId"`
	SourceVersion  string `json:"sourceVersion"`
	TargetVersion  string `json:"targetVersion"`
	Checkpoint     string `json:"checkpoint"`
	ProcessedCount int64  `json:"processedCount"`
	ConflictCount  int64  `json:"conflictCount"`
	BackupID       string `json:"backupId"`
	Lease          string `json:"lease"`
}

type DBRepository struct {
	db *gorm.DB
}

func NewDBRepository(db *gorm.DB) *DBRepository {
	return &DBRepository{db: db}
}

func (r *DBRepository) DB() *gorm.DB { return r.db }

func (r *DBRepository) encodeMetadata(op *MigrationOperation) string {
	m := metadataJSON{
		PlanID:         op.PlanID,
		SourceVersion:  op.SourceVersion,
		TargetVersion:  op.TargetVersion,
		Checkpoint:     op.Checkpoint,
		ProcessedCount: op.ProcessedCount,
		ConflictCount:  op.ConflictCount,
		BackupID:       op.BackupID,
		Lease:          op.Lease,
	}
	data, _ := json.Marshal(m)
	return string(data)
}

func (r *DBRepository) decodeMetadata(s string) metadataJSON {
	var m metadataJSON
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return metadataJSON{}
	}
	return m
}

func (r *DBRepository) CreateOperation(ctx context.Context, op *MigrationOperation) error {
	record := operationRecord{
		ID:          op.ID,
		Kind:        op.PlanID,
		Status:      string(op.Stage),
		StartedAt:   op.StartedAt,
		UpdatedAt:   op.UpdatedAt,
		CompletedAt: op.CompletedAt,
		Error:       op.Error,
		Metadata:    r.encodeMetadata(op),
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *DBRepository) GetOperation(ctx context.Context, id string) (*MigrationOperation, error) {
	if id == "" {
		return nil, errors.New("migration: operation id is empty")
	}
	var record operationRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("migration: get operation: %w", err)
	}
	meta := r.decodeMetadata(record.Metadata)
	op := &MigrationOperation{
		ID:             record.ID,
		PlanID:         meta.PlanID,
		SourceVersion:  meta.SourceVersion,
		TargetVersion:  meta.TargetVersion,
		Stage:          MigrationStage(record.Status),
		Checkpoint:     meta.Checkpoint,
		ProcessedCount: meta.ProcessedCount,
		ConflictCount:  meta.ConflictCount,
		BackupID:       meta.BackupID,
		Lease:          meta.Lease,
		Error:          record.Error,
		StartedAt:      record.StartedAt,
		UpdatedAt:      record.UpdatedAt,
		CompletedAt:    record.CompletedAt,
	}
	op.VerifiedReadCutover, _ = r.HasVerifiedReadCutover(ctx, record.ID)
	op.VerifiedWriteCutover, _ = r.HasVerifiedWriteCutover(ctx, record.ID)
	if op.PlanID == "" {
		op.PlanID = record.Kind
	}
	return op, nil
}

func (r *DBRepository) UpdateOperationStageCAS(ctx context.Context, operationID string, expected, next MigrationStage, mutate func(*MigrationOperation)) (bool, error) {
	if operationID == "" {
		return false, errors.New("migration: operation id is empty")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record operationRecord
		if err := tx.Where("id = ?", operationID).Take(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: operation not found", ErrMigrationStage)
			}
			return err
		}
		if MigrationStage(record.Status) != expected {
			return fmt.Errorf("%w: expected stage %s, got %s", ErrMigrationStage, expected, record.Status)
		}
		meta := r.decodeMetadata(record.Metadata)
		op := &MigrationOperation{
			ID:             record.ID,
			PlanID:         meta.PlanID,
			SourceVersion:  meta.SourceVersion,
			TargetVersion:  meta.TargetVersion,
			Stage:          MigrationStage(record.Status),
			Checkpoint:     meta.Checkpoint,
			ProcessedCount: meta.ProcessedCount,
			ConflictCount:  meta.ConflictCount,
			BackupID:       meta.BackupID,
			Lease:          meta.Lease,
			Error:          record.Error,
			StartedAt:      record.StartedAt,
			UpdatedAt:      record.UpdatedAt,
			CompletedAt:    record.CompletedAt,
		}
		mutate(op)
		updates := map[string]interface{}{
			"status":     string(next),
			"updated_at": now,
			"metadata":   r.encodeMetadata(op),
		}
		if op.Error != "" {
			updates["error"] = op.Error
		}
		if next == StageCompleted || next == StageFailedRetryable || next == StageFailedTerminal || next == StageManualReview {
			updates["completed_at"] = now
		}
		return tx.Model(&operationRecord{}).Where("id = ? AND status = ?", operationID, string(expected)).Updates(updates).Error
	})
	if err != nil {
		if errors.Is(err, ErrMigrationStage) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *DBRepository) SaveCheckpoint(ctx context.Context, operationID, stepName, lastPrimaryKey string, processedCount int, inputHash, outputHash string, conflictCount int) error {
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	record := checkpointRecord{
		ID:             id,
		OperationID:    operationID,
		StepName:       stepName,
		LastPrimaryKey: lastPrimaryKey,
		ProcessedCount: processedCount,
		InputHash:      inputHash,
		OutputHash:     outputHash,
		ConflictCount:  conflictCount,
		UpdatedAt:      now,
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *DBRepository) LoadCheckpoint(ctx context.Context, operationID string) (stepName, lastPrimaryKey string, processedCount int, conflictCount int, err error) {
	if operationID == "" {
		return "", "", 0, 0, errors.New("migration: operation id is empty")
	}
	var record checkpointRecord
	if err = r.db.WithContext(ctx).Where("operation_id = ?", operationID).Order("processed_count desc").Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", 0, 0, nil
		}
		return "", "", 0, 0, fmt.Errorf("migration: load checkpoint: %w", err)
	}
	return record.StepName, record.LastPrimaryKey, record.ProcessedCount, record.ConflictCount, nil
}

func (r *DBRepository) CreateConflict(ctx context.Context, operationID, entityKind, entityID, reasonCode string) error {
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	record := conflictRecord{
		ID:             id,
		OperationID:    operationID,
		EntityKind:     entityKind,
		EntityID:       entityID,
		ConflictReason: reasonCode,
		DetectedAt:     now,
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *DBRepository) CountOpenConflicts(ctx context.Context, operationID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&conflictRecord{}).Where("operation_id = ?", operationID).Count(&count).Error
	return count, err
}

func (r *DBRepository) RecordReadCutover(ctx context.Context, operationID, stepName string) error {
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	record := readCutoverRecord{
		ID:          id,
		OperationID: operationID,
		StepName:    stepName,
		CutoverAt:   now,
		Verified:    0,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "operation_id"}, {Name: "step_name"}},
			DoUpdates: clause.AssignmentColumns([]string{"cutover_at"}),
		}).
		Create(&record).Error
}

func (r *DBRepository) RecordWriteCutover(ctx context.Context, operationID, stepName string) error {
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	record := writeCutoverRecord{
		ID:          id,
		OperationID: operationID,
		StepName:    stepName,
		CutoverAt:   now,
		Verified:    0,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "operation_id"}, {Name: "step_name"}},
			DoUpdates: clause.AssignmentColumns([]string{"cutover_at"}),
		}).
		Create(&record).Error
}

func (r *DBRepository) MarkReadCutoverVerified(ctx context.Context, operationID, stepName string) error {
	result := r.db.WithContext(ctx).Model(&readCutoverRecord{}).
		Where("operation_id = ? AND step_name = ?", operationID, stepName).
		Update("verified", 1)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("migration: mark read cutover verified expected 1 row affected, got %d", result.RowsAffected)
	}
	return nil
}

func (r *DBRepository) MarkWriteCutoverVerified(ctx context.Context, operationID, stepName string) error {
	result := r.db.WithContext(ctx).Model(&writeCutoverRecord{}).
		Where("operation_id = ? AND step_name = ?", operationID, stepName).
		Update("verified", 1)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("migration: mark write cutover verified expected 1 row affected, got %d", result.RowsAffected)
	}
	return nil
}

func (r *DBRepository) HasVerifiedReadCutover(ctx context.Context, operationID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&readCutoverRecord{}).Where("operation_id = ? AND verified = 1", operationID).Count(&count).Error
	return count > 0, err
}

func (r *DBRepository) HasVerifiedWriteCutover(ctx context.Context, operationID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&writeCutoverRecord{}).Where("operation_id = ? AND verified = 1", operationID).Count(&count).Error
	return count > 0, err
}

func (r *DBRepository) LegacyInstallationWriteDisabled() bool {
	var count int64
	r.db.Model(&writeCutoverRecord{}).Where("step_name = ? AND verified = 1", "installation").Count(&count)
	return count > 0
}

func (r *DBRepository) LegacyEditingWriteDisabled() bool {
	var count int64
	r.db.Model(&writeCutoverRecord{}).Where("step_name = ? AND verified = 1", "editing").Count(&count)
	return count > 0
}

func (r *DBRepository) UpdateOperationCheckpoint(ctx context.Context, op *MigrationOperation) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.WithContext(ctx).Model(&operationRecord{}).
		Where("id = ?", op.ID).
		Updates(map[string]interface{}{
			"updated_at": now,
			"metadata":   r.encodeMetadata(op),
		}).Error
}

func ComputeChecksum(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}
