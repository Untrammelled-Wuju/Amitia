package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type CandidateRepository struct {
	db *sql.DB
}

func NewCandidateRepository(db *sql.DB) *CandidateRepository {
	return &CandidateRepository{db: db}
}

func (r *CandidateRepository) Save(ctx context.Context, record *CandidateRecord) error {
	if r == nil || r.db == nil {
		return nil
	}
	contribsJSON, err := json.Marshal(record.Contribs)
	if err != nil {
		return fmt.Errorf("candidate-repo: marshal contribs: %w", err)
	}
	instanceIDsJSON, err := json.Marshal(record.InstanceIDs)
	if err != nil {
		return fmt.Errorf("candidate-repo: marshal instance IDs: %w", err)
	}
	scheduleIDsJSON, err := json.Marshal(record.ScheduleIDs)
	if err != nil {
		return fmt.Errorf("candidate-repo: marshal schedule IDs: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO kernel_candidate_contributions
		(candidate_id, extension_id, instance_ids_json, generation_id, candidate_generation,
		 expected_stable_generation,
		 contribs_json, schedule_ids_json, artifact_path, definition_hash, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.CandidateID,
		string(record.ExtensionID),
		string(instanceIDsJSON),
		record.GenerationID,
		record.CandidateGeneration,
		record.ExpectedStableGeneration,
		string(contribsJSON),
		string(scheduleIDsJSON),
		record.ArtifactPath,
		record.DefinitionHash,
		string(record.Status),
		record.CreatedAt.Format(time.RFC3339Nano),
		record.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("candidate-repo: save %s: %w", record.CandidateID, err)
	}
	return nil
}

func (r *CandidateRepository) UpdateStatus(ctx context.Context, candidateID string, status CandidateStatus) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE kernel_candidate_contributions
		SET status = ?, updated_at = ?
		WHERE candidate_id = ?`,
		string(status),
		time.Now().UTC().Format(time.RFC3339Nano),
		candidateID,
	)
	if err != nil {
		return fmt.Errorf("candidate-repo: update status %s: %w", candidateID, err)
	}
	return nil
}

func (r *CandidateRepository) Delete(ctx context.Context, candidateID string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM kernel_candidate_contributions WHERE candidate_id = ?`, candidateID)
	if err != nil {
		return fmt.Errorf("candidate-repo: delete %s: %w", candidateID, err)
	}
	return nil
}

func (r *CandidateRepository) ListAll(ctx context.Context) ([]*CandidateRecord, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT candidate_id, extension_id, instance_ids_json, generation_id, candidate_generation,
		       expected_stable_generation,
		       contribs_json, schedule_ids_json, artifact_path, definition_hash, status, created_at, updated_at
		FROM kernel_candidate_contributions`)
	if err != nil {
		return nil, fmt.Errorf("candidate-repo: list all: %w", err)
	}
	defer rows.Close()

	var records []*CandidateRecord
	for rows.Next() {
		record, err := scanCandidateRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (r *CandidateRepository) Get(ctx context.Context, candidateID string) (*CandidateRecord, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT candidate_id, extension_id, instance_ids_json, generation_id, candidate_generation,
		       expected_stable_generation,
		       contribs_json, schedule_ids_json, artifact_path, definition_hash, status, created_at, updated_at
		FROM kernel_candidate_contributions
		WHERE candidate_id = ?`, candidateID)
	return scanCandidateRow(row)
}

func scanCandidateRecord(rows *sql.Rows) (*CandidateRecord, error) {
	var (
		record          CandidateRecord
		extID           string
		instanceIDsStr  string
		contribsStr     string
		scheduleIDsStr  string
		statusStr       string
		createdAtStr    string
		updatedAtStr    string
	)
	if err := rows.Scan(
		&record.CandidateID,
		&extID,
		&instanceIDsStr,
		&record.GenerationID,
		&record.CandidateGeneration,
		&record.ExpectedStableGeneration,
		&contribsStr,
		&scheduleIDsStr,
		&record.ArtifactPath,
		&record.DefinitionHash,
		&statusStr,
		&createdAtStr,
		&updatedAtStr,
	); err != nil {
		return nil, fmt.Errorf("candidate-repo: scan row: %w", err)
	}
	record.ExtensionID = domain.ExtensionID(extID)
	record.Status = CandidateStatus(statusStr)
	if createdAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
			record.CreatedAt = t
		}
	}
	if updatedAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
			record.UpdatedAt = t
		}
	}
	if instanceIDsStr != "" {
		_ = json.Unmarshal([]byte(instanceIDsStr), &record.InstanceIDs)
	}
	if contribsStr != "" {
		_ = json.Unmarshal([]byte(contribsStr), &record.Contribs)
	}
	if scheduleIDsStr != "" {
		_ = json.Unmarshal([]byte(scheduleIDsStr), &record.ScheduleIDs)
	}
	return &record, nil
}

func scanCandidateRow(row *sql.Row) (*CandidateRecord, error) {
	var (
		record          CandidateRecord
		extID           string
		instanceIDsStr  string
		contribsStr     string
		scheduleIDsStr  string
		statusStr       string
		createdAtStr    string
		updatedAtStr    string
	)
	if err := row.Scan(
		&record.CandidateID,
		&extID,
		&instanceIDsStr,
		&record.GenerationID,
		&record.CandidateGeneration,
		&record.ExpectedStableGeneration,
		&contribsStr,
		&scheduleIDsStr,
		&record.ArtifactPath,
		&record.DefinitionHash,
		&statusStr,
		&createdAtStr,
		&updatedAtStr,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("candidate-repo: scan row: %w", err)
	}
	record.ExtensionID = domain.ExtensionID(extID)
	record.Status = CandidateStatus(statusStr)
	if createdAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
			record.CreatedAt = t
		}
	}
	if updatedAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
			record.UpdatedAt = t
		}
	}
	if instanceIDsStr != "" {
		_ = json.Unmarshal([]byte(instanceIDsStr), &record.InstanceIDs)
	}
	if contribsStr != "" {
		_ = json.Unmarshal([]byte(contribsStr), &record.Contribs)
	}
	if scheduleIDsStr != "" {
		_ = json.Unmarshal([]byte(scheduleIDsStr), &record.ScheduleIDs)
	}
	return &record, nil
}
