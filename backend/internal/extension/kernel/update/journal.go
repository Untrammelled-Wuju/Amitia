package update

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type JournalManager struct {
	repo *RollbackRepository
}

func NewJournalManager(repo *RollbackRepository) *JournalManager {
	return &JournalManager{repo: repo}
}

func (m *JournalManager) WriteEntry(ctx context.Context, entry LifecycleJournalEntry) error {
	if entry.EntryID == "" {
		entry.EntryID = uuid.New().String()
	}
	if entry.StartedAt.IsZero() {
		entry.StartedAt = time.Now().UTC()
	}
	if m.repo == nil {
		return nil
	}
	return m.repo.SaveJournalEntry(ctx, entry)
}

func (m *JournalManager) WriteStep(ctx context.Context, operationID, stepID string, stepType JournalStepType, status JournalStepStatus, inputHash, outputHash string, compensation *CompensationDefinition) error {
	now := time.Now().UTC()
	entry := LifecycleJournalEntry{
		EntryID:      uuid.New().String(),
		OperationID:  operationID,
		StepID:       stepID,
		StepType:     stepType,
		Status:       status,
		InputHash:    inputHash,
		OutputHash:   outputHash,
		StartedAt:    now,
		Compensation: compensation,
	}
	if status == JournalStatusCompleted || status == JournalStatusFailed || status == JournalStatusSkipped {
		finished := now
		entry.FinishedAt = &finished
	}
	if m.repo == nil {
		return nil
	}
	return m.repo.SaveJournalEntry(ctx, entry)
}

func (m *JournalManager) ListEntries(ctx context.Context, operationID string) ([]LifecycleJournalEntry, error) {
	if m.repo == nil {
		return nil, nil
	}
	return m.repo.ListJournalEntries(ctx, operationID)
}

func (m *JournalManager) GetLastEntry(ctx context.Context, operationID string) (*LifecycleJournalEntry, error) {
	entries, err := m.ListEntries(ctx, operationID)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	last := entries[len(entries)-1]
	return &last, nil
}

func (m *JournalManager) ScanRecoverable(ctx context.Context) ([]RecoveryItem, error) {
	if m.repo == nil {
		return nil, nil
	}
	opIDs, err := m.repo.ListOperationsWithStartedEntries(ctx)
	if err != nil {
		return nil, fmt.Errorf("update: list operations with started entries: %w", err)
	}
	var items []RecoveryItem
	for _, opID := range opIDs {
		last, err := m.GetLastEntry(ctx, opID)
		if err != nil {
			return nil, fmt.Errorf("update: get last entry for %s: %w", opID, err)
		}
		if last == nil {
			continue
		}
		if last.Status != JournalStatusStarted {
			continue
		}
		state := classifyOperationState(last.StepType)
		action := decideRecoveryAction(state)
		items = append(items, RecoveryItem{
			OperationID: opID,
			StepType:    last.StepType,
			Status:      last.Status,
			Action:      action,
		})
	}
	return items, nil
}

func (m *JournalManager) GetCompensation(ctx context.Context, operationID, stepID string) (*CompensationDefinition, error) {
	entries, err := m.ListEntries(ctx, operationID)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.StepID == stepID && e.Compensation != nil {
			comp := *e.Compensation
			return &comp, nil
		}
	}
	return nil, nil
}

func classifyOperationState(stepType JournalStepType) string {
	switch stepType {
	case JournalStepMigrationPlan, JournalStepMigrationExecute, JournalStepMigrationValidate:
		return "migration_running"
	case JournalStepCanaryStart, JournalStepCanaryStageAdvance:
		return "canary_running"
	case JournalStepGenerationSwitch:
		return "activating"
	case JournalStepRollbackCommit:
		return "committing"
	case JournalStepRollbackExecute:
		return "rollback_running"
	case JournalStepSnapshotCreate, JournalStepDataRestore:
		return "snapshot_restoring"
	default:
		return "unknown"
	}
}

func decideRecoveryAction(state string) string {
	switch state {
	case "migration_running":
		return "compensate"
	case "canary_running":
		return "rollback"
	case "activating":
		return "rollback"
	case "committing":
		return "rollback"
	case "rollback_running":
		return "resume"
	case "snapshot_restoring":
		return "resume"
	default:
		return "manual_intervention"
	}
}
