package update

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RecoveryItem struct {
	OperationID string
	StepType    JournalStepType
	Status      JournalStepStatus
	Action      string
}

type RecoveryAction struct {
	OperationID string
	Strategy    string
	Detail      string
}

type RollbackRepository struct {
	db *sql.DB
}

func NewRollbackRepository(db *sql.DB) *RollbackRepository {
	return &RollbackRepository{db: db}
}

func (r *RollbackRepository) SaveRollbackPlan(ctx context.Context, plan RollbackPlan) error {
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("update: marshal rollback plan: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO extension_rollback_plans
			(rollback_id, operation_id, extension_id, from_generation, to_generation, level, plan_json, status, automatic, requires_user_action, started_at, finished_at, error_code, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rollback_id) DO UPDATE SET
			operation_id = excluded.operation_id,
			extension_id = excluded.extension_id,
			from_generation = excluded.from_generation,
			to_generation = excluded.to_generation,
			level = excluded.level,
			plan_json = excluded.plan_json,
			status = excluded.status,
			automatic = excluded.automatic,
			requires_user_action = excluded.requires_user_action,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			error_code = excluded.error_code,
			error_message = excluded.error_message
	`,
		plan.RollbackID, plan.OperationID, plan.ExtensionID,
		plan.FromGeneration, plan.ToGeneration, string(plan.Level),
		string(planJSON), string(plan.Status),
		boolToInt(plan.Automatic), boolToInt(plan.RequiresUserAction),
		nullableTime(plan.StartedAt), nullableTime(plan.FinishedAt),
		nullableString(plan.ErrorCode), nullableString(plan.ErrorMessage),
	)
	if err != nil {
		return fmt.Errorf("update: save rollback plan: %w", err)
	}
	return nil
}

func (r *RollbackRepository) GetRollbackPlan(ctx context.Context, rollbackID string) (*RollbackPlan, error) {
	var planJSON string
	var status string
	var errorCode, errorMessage sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT plan_json, status, error_code, error_message FROM extension_rollback_plans WHERE rollback_id = ?`, rollbackID,
	).Scan(&planJSON, &status, &errorCode, &errorMessage)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("update: rollback plan %s not found", rollbackID)
		}
		return nil, fmt.Errorf("update: query rollback plan: %w", err)
	}
	var plan RollbackPlan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		return nil, fmt.Errorf("update: unmarshal rollback plan: %w", err)
	}
	plan.Status = RollbackStatus(status)
	plan.ErrorCode = errorCode.String
	plan.ErrorMessage = errorMessage.String
	return &plan, nil
}

func (r *RollbackRepository) UpdateRollbackStatus(ctx context.Context, rollbackID string, status RollbackStatus, errorCode, errorMessage string) error {
	var finishedAt interface{}
	if status == RollbackStatusCompleted || status == RollbackStatusFailed ||
		status == RollbackStatusPartial || status == RollbackStatusManualIntervention {
		finishedAt = time.Now().UTC()
	} else {
		finishedAt = nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_rollback_plans
		SET status = ?, error_code = ?, error_message = ?, finished_at = COALESCE(?, finished_at)
		WHERE rollback_id = ?
	`, string(status), nullableString(errorCode), nullableString(errorMessage), finishedAt, rollbackID)
	if err != nil {
		return fmt.Errorf("update: update rollback status: %w", err)
	}
	return nil
}

func (r *RollbackRepository) ListRollbackPlans(ctx context.Context, extensionID string) ([]RollbackPlan, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT plan_json, status, error_code, error_message FROM extension_rollback_plans WHERE extension_id = ? ORDER BY started_at DESC`,
		extensionID)
	if err != nil {
		return nil, fmt.Errorf("update: list rollback plans: %w", err)
	}
	defer rows.Close()
	var out []RollbackPlan
	for rows.Next() {
		var planJSON string
		var status string
		var errorCode, errorMessage sql.NullString
		if err := rows.Scan(&planJSON, &status, &errorCode, &errorMessage); err != nil {
			return nil, fmt.Errorf("update: scan rollback plan: %w", err)
		}
		var plan RollbackPlan
		if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
			return nil, fmt.Errorf("update: unmarshal rollback plan: %w", err)
		}
		plan.Status = RollbackStatus(status)
		plan.ErrorCode = errorCode.String
		plan.ErrorMessage = errorMessage.String
		out = append(out, plan)
	}
	return out, rows.Err()
}

func (r *RollbackRepository) SaveRollbackStep(ctx context.Context, step RollbackStepRecord) error {
	stepUUID := uuid.New().String()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_rollback_steps
			(id, rollback_id, step_id, step_type, status, started_at, finished_at, error_code, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rollback_id, step_id) DO UPDATE SET
			step_type = excluded.step_type,
			status = excluded.status,
			finished_at = excluded.finished_at,
			error_code = excluded.error_code,
			error_message = excluded.error_message
	`, stepUUID, step.RollbackID, step.StepID, step.StepType, step.Status,
		step.StartedAt, nullableTime(step.FinishedAt),
		nullableString(step.ErrorCode), nullableString(step.ErrorMessage))
	if err != nil {
		return fmt.Errorf("update: save rollback step: %w", err)
	}
	return nil
}

func (r *RollbackRepository) ListRollbackSteps(ctx context.Context, rollbackID string) ([]RollbackStepRecord, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT step_id, rollback_id, step_type, status, started_at, finished_at, error_code, error_message
		 FROM extension_rollback_steps WHERE rollback_id = ? ORDER BY step_id ASC`, rollbackID)
	if err != nil {
		return nil, fmt.Errorf("update: list rollback steps: %w", err)
	}
	defer rows.Close()
	var out []RollbackStepRecord
	for rows.Next() {
		var step RollbackStepRecord
		var finishedAt sql.NullTime
		var errorCode, errorMessage sql.NullString
		if err := rows.Scan(&step.StepID, &step.RollbackID, &step.StepType, &step.Status,
			&step.StartedAt, &finishedAt, &errorCode, &errorMessage); err != nil {
			return nil, fmt.Errorf("update: scan rollback step: %w", err)
		}
		step.FinishedAt = timePtr(finishedAt)
		step.ErrorCode = errorCode.String
		step.ErrorMessage = errorMessage.String
		out = append(out, step)
	}
	return out, rows.Err()
}

func (r *RollbackRepository) SaveJournalEntry(ctx context.Context, entry LifecycleJournalEntry) error {
	var compJSON interface{}
	if entry.Compensation != nil {
		b, err := json.Marshal(entry.Compensation)
		if err != nil {
			return fmt.Errorf("update: marshal compensation: %w", err)
		}
		compJSON = string(b)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_lifecycle_journal
			(entry_id, operation_id, step_id, step_type, status, input_hash, output_hash, started_at, finished_at, compensation_json, error_code, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entry_id) DO UPDATE SET
			status = excluded.status,
			output_hash = excluded.output_hash,
			finished_at = excluded.finished_at,
			compensation_json = excluded.compensation_json,
			error_code = excluded.error_code,
			error_message = excluded.error_message
	`, entry.EntryID, entry.OperationID, entry.StepID, string(entry.StepType), string(entry.Status),
		nullableString(entry.InputHash), nullableString(entry.OutputHash),
		entry.StartedAt, nullableTime(entry.FinishedAt), compJSON,
		nullableString(entry.ErrorCode), nullableString(entry.ErrorMessage))
	if err != nil {
		return fmt.Errorf("update: save journal entry: %w", err)
	}
	return nil
}

func (r *RollbackRepository) ListJournalEntries(ctx context.Context, operationID string) ([]LifecycleJournalEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT entry_id, operation_id, step_id, step_type, status, input_hash, output_hash, started_at, finished_at, compensation_json, error_code, error_message
		 FROM extension_lifecycle_journal WHERE operation_id = ? ORDER BY started_at ASC`, operationID)
	if err != nil {
		return nil, fmt.Errorf("update: list journal entries: %w", err)
	}
	defer rows.Close()
	var out []LifecycleJournalEntry
	for rows.Next() {
		var entry LifecycleJournalEntry
		var finishedAt sql.NullTime
		var inputHash, outputHash, compJSON, errorCode, errorMessage sql.NullString
		if err := rows.Scan(&entry.EntryID, &entry.OperationID, &entry.StepID, &entry.StepType, &entry.Status,
			&inputHash, &outputHash, &entry.StartedAt, &finishedAt, &compJSON, &errorCode, &errorMessage); err != nil {
			return nil, fmt.Errorf("update: scan journal entry: %w", err)
		}
		entry.InputHash = inputHash.String
		entry.OutputHash = outputHash.String
		entry.FinishedAt = timePtr(finishedAt)
		if compJSON.Valid && compJSON.String != "" {
			var comp CompensationDefinition
			if err := json.Unmarshal([]byte(compJSON.String), &comp); err == nil {
				entry.Compensation = &comp
			}
		}
		entry.ErrorCode = errorCode.String
		entry.ErrorMessage = errorMessage.String
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (r *RollbackRepository) ListOperationsWithStartedEntries(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT operation_id FROM extension_lifecycle_journal WHERE status = 'started'`)
	if err != nil {
		return nil, fmt.Errorf("update: list operations with started entries: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var opID string
		if err := rows.Scan(&opID); err != nil {
			return nil, fmt.Errorf("update: scan operation id: %w", err)
		}
		out = append(out, opID)
	}
	return out, rows.Err()
}

func (r *RollbackRepository) SaveSideEffectAssessment(ctx context.Context, assessment SideEffectAssessment, rollbackID, extensionID string) error {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_side_effect_assessments
			(id, rollback_id, extension_id, contribution_id, side_effect_class, reversibility, can_compensate, compensation_action, evidence, assessed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, nullableString(rollbackID), extensionID, nullableString(assessment.ContributionID),
		assessment.SideEffectClass, assessment.Reversibility, boolToInt(assessment.CanCompensate),
		nullableString(assessment.CompensationAction), nullableString(assessment.Evidence), now)
	if err != nil {
		return fmt.Errorf("update: save side effect assessment: %w", err)
	}
	return nil
}

func (r *RollbackRepository) ListSideEffectAssessments(ctx context.Context, rollbackID string) ([]SideEffectAssessment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT contribution_id, side_effect_class, reversibility, can_compensate, compensation_action, evidence
		 FROM extension_side_effect_assessments WHERE rollback_id = ?`, rollbackID)
	if err != nil {
		return nil, fmt.Errorf("update: list side effect assessments: %w", err)
	}
	defer rows.Close()
	var out []SideEffectAssessment
	for rows.Next() {
		var a SideEffectAssessment
		var contributionID sql.NullString
		var compensationAction, evidence sql.NullString
		var canCompensate int
		if err := rows.Scan(&contributionID, &a.SideEffectClass, &a.Reversibility,
			&canCompensate, &compensationAction, &evidence); err != nil {
			return nil, fmt.Errorf("update: scan side effect assessment: %w", err)
		}
		a.ContributionID = contributionID.String
		a.CanCompensate = canCompensate != 0
		a.CompensationAction = compensationAction.String
		a.Evidence = evidence.String
		out = append(out, a)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

func timePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	t := nt.Time.UTC()
	return &t
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
