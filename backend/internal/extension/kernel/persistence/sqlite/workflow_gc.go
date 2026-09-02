package sqlite

import (
	"context"
	"fmt"
	"time"
)

// WorkflowRetentionPolicy controls durable workflow history cleanup. A zero
// duration disables that category. Defaults intentionally retain side-effect
// and compensation evidence longer than ordinary execution details.
type WorkflowRetentionPolicy struct {
	RunRetention          time.Duration
	CheckpointRetention   time.Duration
	AttemptRetention      time.Duration
	SideEffectRetention   time.Duration
	CompensationRetention time.Duration
	LeaseGrace            time.Duration
	HeartbeatRetention    time.Duration
	SyncInboxRetention    time.Duration
	AckedOutboxRetention  time.Duration
}

func DefaultWorkflowRetentionPolicy() WorkflowRetentionPolicy {
	return WorkflowRetentionPolicy{
		RunRetention:          180 * 24 * time.Hour,
		CheckpointRetention:   90 * 24 * time.Hour,
		AttemptRetention:      90 * 24 * time.Hour,
		SideEffectRetention:   180 * 24 * time.Hour,
		CompensationRetention: 180 * 24 * time.Hour,
		LeaseGrace:            24 * time.Hour,
		HeartbeatRetention:    7 * 24 * time.Hour,
		SyncInboxRetention:    30 * 24 * time.Hour,
		AckedOutboxRetention:  14 * 24 * time.Hour,
	}
}

type WorkflowGCResult struct {
	Runs          int64 `json:"runs"`
	Checkpoints   int64 `json:"checkpoints"`
	Attempts      int64 `json:"attempts"`
	StepRuns      int64 `json:"stepRuns"`
	SideEffects   int64 `json:"sideEffects"`
	Compensations int64 `json:"compensations"`
	Leases        int64 `json:"leases"`
	Heartbeats    int64 `json:"heartbeats"`
	SyncInbox     int64 `json:"syncInbox"`
	SyncOutbox    int64 `json:"syncOutbox"`
	Orphans       int64 `json:"orphans"`
}

func (r *WorkflowExecutionRepository) CollectGarbageDefault(ctx context.Context, now time.Time) (WorkflowGCResult, error) {
	return r.CollectGarbage(ctx, now, DefaultWorkflowRetentionPolicy())
}

func (r *WorkflowExecutionRepository) CollectGarbage(ctx context.Context, now time.Time, policy WorkflowRetentionPolicy) (WorkflowGCResult, error) {
	var out WorkflowGCResult
	if r == nil || r.db == nil {
		return out, fmt.Errorf("workflow gc: repository unavailable")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("workflow gc begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	terminal := `('succeeded','failed','cancelled','cancel_timeout','cancel_failed','dropped','compensated','compensation_failed','manual_intervention_required')`

	deleteExec := func(query string, args ...any) (int64, error) {
		res, execErr := tx.ExecContext(ctx, query, args...)
		if execErr != nil {
			return 0, execErr
		}
		return res.RowsAffected()
	}
	deleteExpiredChildren := func(table, executionColumn string, retention time.Duration) (int64, error) {
		if retention <= 0 {
			return 0, nil
		}
		query := fmt.Sprintf(`DELETE FROM %s WHERE %s IN (
			SELECT execution_id FROM extension_workflow_executions
			WHERE status IN %s AND updated_at < ?
		)`, table, executionColumn, terminal)
		return deleteExec(query, now.Add(-retention))
	}

	if out.Checkpoints, err = deleteExpiredChildren("extension_workflow_checkpoints", "execution_id", policy.CheckpointRetention); err != nil {
		return out, fmt.Errorf("workflow gc checkpoints: %w", err)
	}
	if out.Attempts, err = deleteExpiredChildren("extension_workflow_step_attempts", "execution_id", policy.AttemptRetention); err != nil {
		return out, fmt.Errorf("workflow gc attempts: %w", err)
	}
	// StepRun is the compact final per-node history. Keep it with RunRetention so
	// run details stay useful for the full run-retention window.
	if out.StepRuns, err = deleteExpiredChildren("extension_workflow_step_runs", "execution_id", policy.RunRetention); err != nil {
		return out, fmt.Errorf("workflow gc step runs: %w", err)
	}
	if out.SideEffects, err = deleteExpiredChildren("extension_workflow_side_effect_journal", "execution_id", policy.SideEffectRetention); err != nil {
		return out, fmt.Errorf("workflow gc side effects: %w", err)
	}
	if out.Compensations, err = deleteExpiredChildren("extension_workflow_compensations", "execution_id", policy.CompensationRetention); err != nil {
		return out, fmt.Errorf("workflow gc compensations: %w", err)
	}

	if policy.LeaseGrace > 0 {
		out.Leases, err = deleteExec(`DELETE FROM extension_workflow_execution_leases
			WHERE lease_expires_at < ?
			  AND (NOT EXISTS (SELECT 1 FROM extension_workflow_executions e WHERE e.execution_id = extension_workflow_execution_leases.execution_id)
			       OR EXISTS (SELECT 1 FROM extension_workflow_executions e WHERE e.execution_id = extension_workflow_execution_leases.execution_id AND e.status IN `+terminal+`))`, now.Add(-policy.LeaseGrace))
		if err != nil {
			return out, fmt.Errorf("workflow gc leases: %w", err)
		}
	}
	if policy.HeartbeatRetention > 0 {
		out.Heartbeats, err = deleteExec(`DELETE FROM extension_workflow_run_heartbeats
			WHERE heartbeat_at < ?
			  AND (NOT EXISTS (SELECT 1 FROM extension_workflow_executions e WHERE e.execution_id = extension_workflow_run_heartbeats.execution_id)
			       OR EXISTS (SELECT 1 FROM extension_workflow_executions e WHERE e.execution_id = extension_workflow_run_heartbeats.execution_id AND e.status IN `+terminal+`))`, now.Add(-policy.HeartbeatRetention))
		if err != nil {
			return out, fmt.Errorf("workflow gc heartbeats: %w", err)
		}
	}
	if policy.SyncInboxRetention > 0 {
		out.SyncInbox, err = deleteExec(`DELETE FROM extension_workflow_sync_inbox WHERE received_at < ?`, now.Add(-policy.SyncInboxRetention))
		if err != nil {
			return out, fmt.Errorf("workflow gc sync inbox: %w", err)
		}
	}
	if policy.AckedOutboxRetention > 0 {
		out.SyncOutbox, err = deleteExec(`DELETE FROM extension_workflow_sync_outbox WHERE acked_at IS NOT NULL AND acked_at < ?`, now.Add(-policy.AckedOutboxRetention))
		if err != nil {
			return out, fmt.Errorf("workflow gc sync outbox: %w", err)
		}
	}

	// Orphan cleanup is recovery-oriented, not age-oriented. These rows cannot be
	// attached to any execution anymore, so retaining them indefinitely provides
	// no audit or recovery value.
	orphanTables := []string{
		"extension_workflow_checkpoints",
		"extension_workflow_step_attempts",
		"extension_workflow_step_runs",
		"extension_workflow_side_effect_journal",
		"extension_workflow_compensations",
		"extension_workflow_execution_leases",
		"extension_workflow_run_heartbeats",
	}
	for _, table := range orphanTables {
		column := "execution_id"
		count, deleteErr := deleteExec(fmt.Sprintf(`DELETE FROM %s WHERE NOT EXISTS (
			SELECT 1 FROM extension_workflow_executions e WHERE e.execution_id = %s.%s
		)`, table, table, column))
		if deleteErr != nil {
			return out, fmt.Errorf("workflow gc orphan %s: %w", table, deleteErr)
		}
		out.Orphans += count
	}

	if policy.RunRetention > 0 {
		// Never delete a run while longer-lived audit children still reference it.
		// The NOT EXISTS guards also make custom retention policies safe when a
		// caller chooses child retention longer than run retention.
		out.Runs, err = deleteExec(`DELETE FROM extension_workflow_executions
			WHERE status IN `+terminal+` AND updated_at < ?
			  AND NOT EXISTS (SELECT 1 FROM extension_workflow_checkpoints c WHERE c.execution_id = extension_workflow_executions.execution_id)
			  AND NOT EXISTS (SELECT 1 FROM extension_workflow_step_attempts a WHERE a.execution_id = extension_workflow_executions.execution_id)
			  AND NOT EXISTS (SELECT 1 FROM extension_workflow_step_runs s WHERE s.execution_id = extension_workflow_executions.execution_id)
			  AND NOT EXISTS (SELECT 1 FROM extension_workflow_side_effect_journal j WHERE j.execution_id = extension_workflow_executions.execution_id)
			  AND NOT EXISTS (SELECT 1 FROM extension_workflow_compensations c WHERE c.execution_id = extension_workflow_executions.execution_id)`, now.Add(-policy.RunRetention))
		if err != nil {
			return out, fmt.Errorf("workflow gc runs: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return out, fmt.Errorf("workflow gc commit: %w", err)
	}
	return out, nil
}
