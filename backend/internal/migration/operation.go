package migration

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type migrationOperationRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	Kind        string `gorm:"column:kind"`
	Status      string `gorm:"column:status"`
	StartedAt   string `gorm:"column:started_at"`
	UpdatedAt   string `gorm:"column:updated_at"`
	CompletedAt string `gorm:"column:completed_at"`
	Error       string `gorm:"column:error"`
	Metadata    string `gorm:"column:metadata"`
}

func (migrationOperationRecord) TableName() string {
	return "desktop_pet_migration_operations"
}

type migrationCheckpointRecord struct {
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

func (migrationCheckpointRecord) TableName() string {
	return "desktop_pet_migration_checkpoints"
}

func ensureOperationTables(db *gorm.DB) {
	db.Exec(`CREATE TABLE IF NOT EXISTS desktop_pet_migration_operations (
		id TEXT PRIMARY KEY,
		kind TEXT NOT NULL,
		status TEXT NOT NULL,
		started_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		completed_at TEXT NOT NULL DEFAULT '',
		error TEXT NOT NULL DEFAULT '',
		metadata TEXT NOT NULL DEFAULT '{}'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS desktop_pet_migration_checkpoints (
		id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		step_name TEXT NOT NULL,
		last_primary_key TEXT NOT NULL DEFAULT '',
		processed_count INTEGER NOT NULL DEFAULT 0,
		input_hash TEXT NOT NULL DEFAULT '',
		output_hash TEXT NOT NULL DEFAULT '',
		conflict_count INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL
	)`)
}

func (r Runner) startMigrationOperation(hasPending bool) (string, error) {
	ensureOperationTables(r.DB)
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	kind := "schema"
	if !hasPending {
		kind = "no_op"
	}
	record := migrationOperationRecord{
		ID:        id,
		Kind:      kind,
		Status:    "running",
		StartedAt: now,
		UpdatedAt: now,
		Metadata:  "{}",
	}
	if err := r.DB.Create(&record).Error; err != nil {
		return id, err
	}
	return id, nil
}

func (r Runner) recordMigrationCheckpoint(operationID, version string) error {
	ensureOperationTables(r.DB)
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	record := migrationCheckpointRecord{
		ID:             id,
		OperationID:    operationID,
		StepName:       version,
		LastPrimaryKey: version,
		ProcessedCount: 1,
		UpdatedAt:      now,
	}
	return r.DB.Create(&record).Error
}

func (r Runner) completeMigrationOperation(operationID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.DB.Model(&migrationOperationRecord{}).
		Where("id = ?", operationID).
		Updates(map[string]interface{}{
			"status":       "completed",
			"completed_at": now,
			"updated_at":   now,
		}).Error
}

func (r Runner) failMigrationOperation(operationID, errorMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.DB.Model(&migrationOperationRecord{}).
		Where("id = ?", operationID).
		Updates(map[string]interface{}{
			"status":       "failed",
			"error":        errorMsg,
			"completed_at": now,
			"updated_at":   now,
		}).Error
}
