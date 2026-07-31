package kernel

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const packageRestoreJournalDDL = `CREATE TABLE IF NOT EXISTS extension_package_restore_journal (
	journal_id TEXT NOT NULL,
	operation_id TEXT NOT NULL,
	extension_id TEXT NOT NULL,
	rollback_point_id TEXT NOT NULL,
	state TEXT NOT NULL DEFAULT 'prepared',
	checkpoint_data TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	error_detail TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (operation_id)
)`

type RestoreState string

const (
	RestoreStatePrepared            RestoreState = "prepared"
	RestoreStateRestoringUserData   RestoreState = "restoring_user_data"
	RestoreStateRestoringResources  RestoreState = "restoring_resources"
	RestoreStateRestoringConfig     RestoreState = "restoring_config"
	RestoreStateRestoringMigration  RestoreState = "restoring_migration_state"
	RestoreStateSwitchingGeneration RestoreState = "switching_generation"
	RestoreStateVerifying           RestoreState = "verifying"
	RestoreStateCompleted           RestoreState = "completed"
	RestoreStateFailed              RestoreState = "failed"
)

var restoreStateOrder = []RestoreState{
	RestoreStatePrepared,
	RestoreStateRestoringUserData,
	RestoreStateRestoringResources,
	RestoreStateRestoringConfig,
	RestoreStateRestoringMigration,
	RestoreStateSwitchingGeneration,
	RestoreStateVerifying,
	RestoreStateCompleted,
}

type PackageRestoreJournal struct {
	JournalID       string
	OperationID     string
	ExtensionID     string
	RollbackPointID string
	State           RestoreState
	CheckpointData  string
	StartedAt       string
	UpdatedAt       string
	ErrorDetail     string
}

type PackageSnapshotRepository struct {
	db *sql.DB
}

func NewPackageSnapshotRepository(db *sql.DB) *PackageSnapshotRepository {
	return &PackageSnapshotRepository{db: db}
}

func (r *PackageSnapshotRepository) EnsureSchema(ctx context.Context) error {
	if r.db == nil {
		return fmt.Errorf("kernel: snapshot repository database unavailable")
	}
	_, err := r.db.ExecContext(ctx, packageRestoreJournalDDL)
	return err
}

func (r *PackageSnapshotRepository) GetOrCreateRestoreJournal(ctx context.Context, operationID, extensionID, rollbackPointID string) (*PackageRestoreJournal, error) {
	if r.db == nil {
		return nil, fmt.Errorf("kernel: snapshot repository database unavailable")
	}
	if err := r.EnsureSchema(ctx); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal := &PackageRestoreJournal{
		JournalID:       "restore-journal-" + operationID,
		OperationID:     operationID,
		ExtensionID:     extensionID,
		RollbackPointID: rollbackPointID,
		State:           RestoreStatePrepared,
		StartedAt:       now,
		UpdatedAt:       now,
	}
	var state, checkpointData, startedAt, updatedAt, errorDetail string
	err := r.db.QueryRowContext(ctx,
		`SELECT state, checkpoint_data, started_at, updated_at, error_detail
		 FROM extension_package_restore_journal
		 WHERE operation_id=?`, operationID,
	).Scan(&state, &checkpointData, &startedAt, &updatedAt, &errorDetail)
	if err == nil {
		journal.State = RestoreState(state)
		journal.CheckpointData = checkpointData
		journal.StartedAt = startedAt
		journal.UpdatedAt = updatedAt
		journal.ErrorDetail = errorDetail
		return journal, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("query restore journal: %w", err)
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO extension_package_restore_journal
		 (journal_id, operation_id, extension_id, rollback_point_id, state, checkpoint_data, started_at, updated_at, error_detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		journal.JournalID, operationID, extensionID, rollbackPointID,
		string(RestoreStatePrepared), "", now, now, "")
	if err != nil {
		return nil, fmt.Errorf("create restore journal: %w", err)
	}
	return journal, nil
}

func (r *PackageSnapshotRepository) AdvanceRestoreState(ctx context.Context, operationID string, targetState RestoreState) error {
	if r.db == nil {
		return fmt.Errorf("kernel: snapshot repository database unavailable")
	}
	current, err := r.GetRestoreState(ctx, operationID)
	if err != nil {
		return err
	}
	if !isValidRestoreTransition(current, targetState) {
		return fmt.Errorf("kernel: invalid restore state transition %s -> %s", current, targetState)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = r.db.ExecContext(ctx,
		`UPDATE extension_package_restore_journal
		 SET state=?, error_detail='', updated_at=?
		 WHERE operation_id=?`,
		string(targetState), now, operationID)
	return err
}

func (r *PackageSnapshotRepository) FailRestoreState(ctx context.Context, operationID string, errorDetail string) error {
	if r.db == nil {
		return fmt.Errorf("kernel: snapshot repository database unavailable")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx,
		`UPDATE extension_package_restore_journal
		 SET state=?, error_detail=?, updated_at=?
		 WHERE operation_id=?`,
		string(RestoreStateFailed), errorDetail, now, operationID)
	return err
}

func (r *PackageSnapshotRepository) SaveCheckpoint(ctx context.Context, operationID string, checkpointData string) error {
	if r.db == nil {
		return fmt.Errorf("kernel: snapshot repository database unavailable")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx,
		`UPDATE extension_package_restore_journal
		 SET checkpoint_data=?, updated_at=?
		 WHERE operation_id=?`,
		checkpointData, now, operationID)
	return err
}

func (r *PackageSnapshotRepository) GetRestoreState(ctx context.Context, operationID string) (RestoreState, error) {
	if r.db == nil {
		return RestoreStateFailed, fmt.Errorf("kernel: snapshot repository database unavailable")
	}
	var state string
	err := r.db.QueryRowContext(ctx,
		`SELECT state FROM extension_package_restore_journal WHERE operation_id=?`,
		operationID,
	).Scan(&state)
	if err != nil {
		if err == sql.ErrNoRows {
			return RestoreStatePrepared, nil
		}
		return RestoreStateFailed, fmt.Errorf("query restore state: %w", err)
	}
	return RestoreState(state), nil
}

func (r *PackageSnapshotRepository) GetRestoreJournal(ctx context.Context, operationID string) (*PackageRestoreJournal, error) {
	if r.db == nil {
		return nil, fmt.Errorf("kernel: snapshot repository database unavailable")
	}
	var journal PackageRestoreJournal
	err := r.db.QueryRowContext(ctx,
		`SELECT journal_id, operation_id, extension_id, rollback_point_id, state, checkpoint_data, started_at, updated_at, error_detail
		 FROM extension_package_restore_journal WHERE operation_id=?`,
		operationID,
	).Scan(&journal.JournalID, &journal.OperationID, &journal.ExtensionID,
		&journal.RollbackPointID, &journal.State, &journal.CheckpointData,
		&journal.StartedAt, &journal.UpdatedAt, &journal.ErrorDetail)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query restore journal: %w", err)
	}
	return &journal, nil
}

func (r *PackageSnapshotRepository) IsRestoreComplete(ctx context.Context, operationID string) (bool, error) {
	state, err := r.GetRestoreState(ctx, operationID)
	if err != nil {
		return false, err
	}
	return state == RestoreStateCompleted, nil
}

func (r *PackageSnapshotRepository) CanResumeFromState(state RestoreState) (RestoreState, error) {
	if state == RestoreStateCompleted {
		return RestoreStateCompleted, nil
	}
	if state == RestoreStateFailed {
		return RestoreStatePrepared, fmt.Errorf("restore previously failed, restart from prepared")
	}
	return state, nil
}

func (r *PackageSnapshotRepository) ClearRestoreJournal(ctx context.Context, operationID string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM extension_package_restore_journal WHERE operation_id=?`,
		operationID)
	return err
}

func (r *PackageSnapshotRepository) NextRestoreState(current RestoreState) RestoreState {
	for i, state := range restoreStateOrder {
		if state == current && i+1 < len(restoreStateOrder) {
			return restoreStateOrder[i+1]
		}
	}
	return RestoreStateCompleted
}

func (r *PackageSnapshotRepository) ShouldSkipState(current, target RestoreState) bool {
	currentIdx := restoreStateIndex(current)
	targetIdx := restoreStateIndex(target)
	if currentIdx < 0 || targetIdx < 0 {
		return false
	}
	return currentIdx > targetIdx
}

func isValidRestoreTransition(from, to RestoreState) bool {
	if to == RestoreStateFailed {
		return true
	}
	if from == to {
		return true
	}
	fromIdx := restoreStateIndex(from)
	toIdx := restoreStateIndex(to)
	if fromIdx < 0 || toIdx < 0 {
		return false
	}
	if from == RestoreStateFailed {
		return toIdx == 0
	}
	return toIdx == fromIdx+1
}

func restoreStateIndex(state RestoreState) int {
	for i, s := range restoreStateOrder {
		if s == state {
			return i
		}
	}
	return -1
}
