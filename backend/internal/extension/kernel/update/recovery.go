package update

import (
	"context"
	"fmt"
)

type RecoveryManager struct {
	journal    *JournalManager
	executor   *RollbackExecutorV2
	planner    *RollbackPlanner
	repo       *RollbackRepository
	migrations *MigrationExecutor
}

func NewRecoveryManager(journal *JournalManager, executor *RollbackExecutorV2, planner *RollbackPlanner, repo *RollbackRepository) *RecoveryManager {
	return &RecoveryManager{
		journal:    journal,
		executor:   executor,
		planner:    planner,
		repo:       repo,
		migrations: executor.migrations,
	}
}

func (m *RecoveryManager) ScanOnStartup(ctx context.Context) ([]RecoveryAction, error) {
	items, err := m.journal.ScanRecoverable(ctx)
	if err != nil {
		return nil, fmt.Errorf("update: scan recoverable: %w", err)
	}
	actions := make([]RecoveryAction, 0, len(items))
	for _, item := range items {
		strategy := m.decideStrategy(item)
		actions = append(actions, RecoveryAction{
			OperationID: item.OperationID,
			Strategy:    strategy,
			Detail:      fmt.Sprintf("operation %s in step %s (status: %s), recommended: %s", item.OperationID, string(item.StepType), string(item.Status), strategy),
		})
	}
	return actions, nil
}

func (m *RecoveryManager) decideStrategy(item RecoveryItem) string {
	switch item.Action {
	case "resume":
		return "resume"
	case "compensate":
		return "compensate"
	case "rollback":
		return "rollback"
	case "manual_intervention":
		return "manual_intervention"
	default:
		return "manual_intervention"
	}
}

func (m *RecoveryManager) ExecuteRecovery(ctx context.Context, action RecoveryAction) error {
	switch action.Strategy {
	case "resume":
		return m.executeResume(ctx, action)
	case "compensate":
		return m.executeCompensate(ctx, action)
	case "rollback":
		return m.executeRollback(ctx, action)
	case "manual_intervention":
		return m.executeManualIntervention(ctx, action)
	default:
		return fmt.Errorf("update: unknown recovery strategy: %s", action.Strategy)
	}
}

func (m *RecoveryManager) executeResume(ctx context.Context, action RecoveryAction) error {
	if m.executor != nil {
		if err := m.executor.RecoverRollback(ctx, action.OperationID); err != nil {
			return fmt.Errorf("update: resume recovery failed: %w", err)
		}
	}
	if m.journal != nil {
		last, err := m.journal.GetLastEntry(ctx, action.OperationID)
		if err != nil {
			return fmt.Errorf("update: get last entry for resume: %w", err)
		}
		operationID := action.OperationID
		if last != nil {
			operationID = last.OperationID
		}
		if err := m.journal.WriteStep(ctx, operationID, "recovery_resume", JournalStepRollbackExecute, JournalStatusCompleted, "", "", nil); err != nil {
			return fmt.Errorf("update: write resume recovery step: %w", err)
		}
	}
	return nil
}

func (m *RecoveryManager) executeCompensate(ctx context.Context, action RecoveryAction) error {
	if m.journal == nil {
		return fmt.Errorf("update: journal not available for compensation")
	}
	last, err := m.journal.GetLastEntry(ctx, action.OperationID)
	if err != nil {
		return fmt.Errorf("update: get last entry for compensation: %w", err)
	}
	if last == nil {
		return m.executeRollback(ctx, action)
	}
	comp, err := m.journal.GetCompensation(ctx, action.OperationID, last.StepID)
	if err != nil {
		return fmt.Errorf("update: get compensation definition: %w", err)
	}
	if comp == nil {
		return m.executeRollback(ctx, action)
	}
	if err := m.applyCompensation(ctx, action.OperationID, comp); err != nil {
		return fmt.Errorf("update: apply compensation: %w", err)
	}
	if err := m.journal.WriteStep(ctx, action.OperationID, "recovery_compensate", JournalStepRollbackExecute, JournalStatusCompleted, "", comp.Action, nil); err != nil {
		return fmt.Errorf("update: write compensation recovery step: %w", err)
	}
	return nil
}

func (m *RecoveryManager) applyCompensation(ctx context.Context, operationID string, comp *CompensationDefinition) error {
	if comp == nil {
		return fmt.Errorf("update: compensation definition is nil")
	}
	switch comp.Action {
	case "reverse_migration":
		if m.migrations == nil {
			return fmt.Errorf("update: migration executor not available for reverse_migration")
		}
		runs := m.migrations.ListRuns(ctx, operationID)
		for i := len(runs) - 1; i >= 0; i-- {
			r := runs[i]
			if r.Status != MigrationStatusSucceeded {
				continue
			}
			if err := m.migrations.RollbackMigration(ctx, r.RunID, func(ctx context.Context) error {
				return nil
			}); err != nil {
				return fmt.Errorf("update: reverse migration %s failed: %w", r.RunID, err)
			}
		}
		return nil
	case "restore_snapshot":
		if m.migrations == nil {
			return fmt.Errorf("update: migration executor not available for restore_snapshot")
		}
		if comp.Target == "" {
			return fmt.Errorf("update: snapshot ID not specified in compensation target")
		}
		_, err := m.migrations.RestoreSnapshot(ctx, comp.Target)
		if err != nil {
			return fmt.Errorf("update: restore snapshot %s failed: %w", comp.Target, err)
		}
		return nil
	case "rebuild_index":
		if m.repo == nil {
			return fmt.Errorf("update: repository not available for rebuild_index")
		}
		return nil
	case "call_compensation_endpoint":
		if comp.Target == "" {
			return fmt.Errorf("update: compensation endpoint not specified")
		}
		return nil
	default:
		return fmt.Errorf("update: unsupported compensation type: %s", comp.Action)
	}
}

func (m *RecoveryManager) executeRollback(ctx context.Context, action RecoveryAction) error {
	if m.executor != nil {
		if err := m.executor.RecoverRollback(ctx, action.OperationID); err != nil {
			return fmt.Errorf("update: rollback recovery failed: %w", err)
		}
	}
	if m.journal != nil {
		if err := m.journal.WriteStep(ctx, action.OperationID, "recovery_rollback", JournalStepRollbackExecute, JournalStatusCompleted, "", "", nil); err != nil {
			return fmt.Errorf("update: write rollback recovery step: %w", err)
		}
	}
	return nil
}

func (m *RecoveryManager) executeManualIntervention(ctx context.Context, action RecoveryAction) error {
	if m.journal != nil {
		m.journal.WriteStep(ctx, action.OperationID, "recovery_manual", JournalStepRollbackExecute, JournalStatusSkipped, "", action.Detail, nil)
	}
	return fmt.Errorf("update: operation %s requires manual intervention: %s", action.OperationID, action.Detail)
}

func (m *RecoveryManager) IsRecoveryRequired(ctx context.Context) (bool, error) {
	items, err := m.journal.ScanRecoverable(ctx)
	if err != nil {
		return false, err
	}
	return len(items) > 0, nil
}
