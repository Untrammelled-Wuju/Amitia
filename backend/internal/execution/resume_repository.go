package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type ResumeRepository interface {
	Save(ctx context.Context, resume ResumeContext) error
	Get(ctx context.Context, resumeID string) (*ResumeContext, error)
	GetByAcquisitionTransactionID(ctx context.Context, acquisitionTxnID string) (*ResumeContext, error)
	UpdateState(ctx context.Context, resumeID string, state ResumeState, reason string) error
	ListPending(ctx context.Context) ([]ResumeContext, error)
	ListByRootExecution(ctx context.Context, rootExecutionID string) ([]ResumeContext, error)
	Delete(ctx context.Context, resumeID string) error
}

type SQLiteResumeRepository struct {
	db *sql.DB
}

func NewSQLiteResumeRepository(db *sql.DB) *SQLiteResumeRepository {
	return &SQLiteResumeRepository{db: db}
}

func (r *SQLiteResumeRepository) InitTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS execution_resumes (
			resume_id TEXT PRIMARY KEY,
			root_execution_id TEXT,
			parent_execution_id TEXT,
			resume_type TEXT NOT NULL,
			resume_state TEXT NOT NULL,
			checkpoint_ref TEXT,
			required_capability_id TEXT,
			acquisition_transaction_id TEXT,
			task_id TEXT,
			payload_ref TEXT,
			reason TEXT,
			metadata TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_execution_resumes_state ON execution_resumes(resume_state);
		CREATE INDEX IF NOT EXISTS idx_execution_resumes_root ON execution_resumes(root_execution_id);
		CREATE INDEX IF NOT EXISTS idx_execution_resumes_acq_txn ON execution_resumes(acquisition_transaction_id);
	`)
	return err
}

func (r *SQLiteResumeRepository) Save(ctx context.Context, resume ResumeContext) error {
	metadataJSON, err := json.Marshal(resume.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO execution_resumes (
			resume_id, root_execution_id, parent_execution_id,
			resume_type, resume_state, checkpoint_ref,
			required_capability_id, acquisition_transaction_id,
			task_id, payload_ref, reason, metadata,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(resume_id) DO UPDATE SET
			resume_state = excluded.resume_state,
			checkpoint_ref = excluded.checkpoint_ref,
			reason = excluded.reason,
			metadata = excluded.metadata,
			updated_at = excluded.updated_at
	`,
		resume.ResumeID, resume.RootExecutionID, resume.ParentExecutionID,
		string(resume.Type), string(resume.State), resume.CheckpointRef,
		resume.RequiredCapabilityID, resume.AcquisitionTransactionID,
		resume.TaskID, resume.PayloadRef, resume.Reason, string(metadataJSON),
		resume.CreatedAt.UTC(), resume.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("execution_resumes: save %s: %w", resume.ResumeID, err)
	}
	return nil
}

func (r *SQLiteResumeRepository) Get(ctx context.Context, resumeID string) (*ResumeContext, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT resume_id, root_execution_id, parent_execution_id,
		       resume_type, resume_state, checkpoint_ref,
		       required_capability_id, acquisition_transaction_id,
		       task_id, payload_ref, reason, metadata,
		       created_at, updated_at
		FROM execution_resumes WHERE resume_id = ?
	`, resumeID)
	return scanResumeContext(row)
}

func (r *SQLiteResumeRepository) GetByAcquisitionTransactionID(ctx context.Context, acquisitionTxnID string) (*ResumeContext, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT resume_id, root_execution_id, parent_execution_id,
		       resume_type, resume_state, checkpoint_ref,
		       required_capability_id, acquisition_transaction_id,
		       task_id, payload_ref, reason, metadata,
		       created_at, updated_at
		FROM execution_resumes WHERE acquisition_transaction_id = ?
	`, acquisitionTxnID)
	return scanResumeContext(row)
}

func (r *SQLiteResumeRepository) UpdateState(ctx context.Context, resumeID string, state ResumeState, reason string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE execution_resumes SET resume_state = ?, reason = ?, updated_at = ?
		WHERE resume_id = ?
	`, string(state), reason, now, resumeID)
	if err != nil {
		return fmt.Errorf("execution_resumes: update state %s: %w", resumeID, err)
	}
	return nil
}

func (r *SQLiteResumeRepository) ListPending(ctx context.Context) ([]ResumeContext, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT resume_id, root_execution_id, parent_execution_id,
		       resume_type, resume_state, checkpoint_ref,
		       required_capability_id, acquisition_transaction_id,
		       task_id, payload_ref, reason, metadata,
		       created_at, updated_at
		FROM execution_resumes
		WHERE resume_state IN (?, ?)
		ORDER BY created_at ASC
	`, string(ResumeStatePending), string(ResumeStateInProgress))
	if err != nil {
		return nil, fmt.Errorf("execution_resumes: list pending: %w", err)
	}
	defer rows.Close()

	var results []ResumeContext
	for rows.Next() {
		rc, err := scanResumeContext(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *rc)
	}
	return results, rows.Err()
}

func (r *SQLiteResumeRepository) ListByRootExecution(ctx context.Context, rootExecutionID string) ([]ResumeContext, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT resume_id, root_execution_id, parent_execution_id,
		       resume_type, resume_state, checkpoint_ref,
		       required_capability_id, acquisition_transaction_id,
		      	task_id, payload_ref, reason, metadata,
		       created_at, updated_at
		FROM execution_resumes WHERE root_execution_id = ?
		ORDER BY created_at ASC
	`, rootExecutionID)
	if err != nil {
		return nil, fmt.Errorf("execution_resumes: list by root: %w", err)
	}
	defer rows.Close()

	var results []ResumeContext
	for rows.Next() {
		rc, err := scanResumeContext(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *rc)
	}
	return results, rows.Err()
}

func (r *SQLiteResumeRepository) Delete(ctx context.Context, resumeID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM execution_resumes WHERE resume_id = ?`, resumeID)
	if err != nil {
		return fmt.Errorf("execution_resumes: delete %s: %w", resumeID, err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanResumeContext(s scanner) (*ResumeContext, error) {
	var resume ResumeContext
	var resumeType, resumeState string
	var metadataRaw sql.NullString
	var createdAt, updatedAt time.Time

	err := s.Scan(
		&resume.ResumeID, &resume.RootExecutionID, &resume.ParentExecutionID,
		&resumeType, &resumeState, &resume.CheckpointRef,
		&resume.RequiredCapabilityID, &resume.AcquisitionTransactionID,
		&resume.TaskID, &resume.PayloadRef, &resume.Reason, &metadataRaw,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("execution_resumes: scan: %w", err)
	}

	resume.Type = ResumeType(resumeType)
	resume.State = ResumeState(resumeState)
	resume.CreatedAt = createdAt.UTC()
	resume.UpdatedAt = updatedAt.UTC()

	if metadataRaw.Valid && metadataRaw.String != "" {
		meta := make(map[string]any)
		if err := json.Unmarshal([]byte(metadataRaw.String), &meta); err == nil {
			resume.Metadata = meta
		}
	}

	return &resume, nil
}
