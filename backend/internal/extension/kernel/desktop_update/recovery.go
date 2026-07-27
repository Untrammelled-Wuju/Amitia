package desktop_update

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type RecoveryAction struct {
	OperationID  string
	CurrentState string
	Action       string
	Reason       string
}

const (
	RecoveryActionResume             = "resume"
	RecoveryActionRollback           = "rollback"
	RecoveryActionMarkFailed         = "mark_failed"
	RecoveryActionManualIntervention = "mark_manual_intervention"
)

type RecoveryService struct {
	journal      *UpdateJournal
	stateMachine *StateMachine
	operations   map[string]*UpdateOperation
}

func NewRecoveryService(journal *UpdateJournal, sm *StateMachine) *RecoveryService {
	return &RecoveryService{
		journal:      journal,
		stateMachine: sm,
		operations:   make(map[string]*UpdateOperation),
	}
}

func (r *RecoveryService) SetOperations(ops map[string]*UpdateOperation) {
	r.operations = ops
}

func (r *RecoveryService) ScanAndRecover(ctx context.Context) ([]RecoveryAction, error) {
	var actions []RecoveryAction

	for _, op := range r.operations {
		if !r.stateMachine.IsRecoverable(op.Status) {
			continue
		}

		action := r.determineRecoveryAction(op)
		actions = append(actions, action)

		switch action.Action {
		case RecoveryActionResume:
			r.applyResumeAction(op)
		case RecoveryActionRollback:
			r.applyRollbackAction(op)
		case RecoveryActionMarkFailed:
			r.applyMarkFailedAction(op)
		case RecoveryActionManualIntervention:
			r.applyManualInterventionAction(op)
		}
	}

	sort.Slice(actions, func(i, j int) bool {
		return actions[i].OperationID < actions[j].OperationID
	})

	return actions, nil
}

func (r *RecoveryService) determineRecoveryAction(op *UpdateOperation) RecoveryAction {
	lastEntry := r.journal.GetLastEntry(op.OperationID)
	reason := "unknown"
	if lastEntry != nil {
		reason = fmt.Sprintf("last step: %s (%s)", lastEntry.Step, lastEntry.Status)
	}

	switch op.Status {
	case StateDownloading:
		ds, ok := getDownloadStateSafe(op.OperationID)
		if ok && ds.Status == DownloadStatusCompleted {
			return RecoveryAction{
				OperationID:  op.OperationID,
				CurrentState: string(op.Status),
				Action:       RecoveryActionResume,
				Reason:       fmt.Sprintf("download completed, can resume verification: %s", reason),
			}
		}
		if ok && ds.Status == DownloadStatusFailed {
			return RecoveryAction{
				OperationID:  op.OperationID,
				CurrentState: string(op.Status),
				Action:       RecoveryActionResume,
				Reason:       fmt.Sprintf("download failed, can retry: %s", reason),
			}
		}
		return RecoveryAction{
			OperationID:  op.OperationID,
			CurrentState: string(op.Status),
			Action:       RecoveryActionMarkFailed,
			Reason:       fmt.Sprintf("download interrupted and not resumable: %s", reason),
		}

	case StateVerifying, StateStaging:
		return RecoveryAction{
			OperationID:  op.OperationID,
			CurrentState: string(op.Status),
			Action:       RecoveryActionResume,
			Reason:       fmt.Sprintf("can resume from %s: %s", op.Status, reason),
		}

	case StateDraining:
		return RecoveryAction{
			OperationID:  op.OperationID,
			CurrentState: string(op.Status),
			Action:       RecoveryActionResume,
			Reason:       fmt.Sprintf("can resume drain or proceed to migration: %s", reason),
		}

	case StateMigrating:
		if lastEntry != nil && lastEntry.Status == JournalStatusFailed {
			return RecoveryAction{
				OperationID:  op.OperationID,
				CurrentState: string(op.Status),
				Action:       RecoveryActionRollback,
				Reason:       fmt.Sprintf("migration failed, need rollback: %s", lastEntry.Error),
			}
		}
		return RecoveryAction{
			OperationID:  op.OperationID,
			CurrentState: string(op.Status),
			Action:       RecoveryActionManualIntervention,
			Reason:       fmt.Sprintf("migration interrupted, manual verification needed: %s", reason),
		}

	case StateActivating:
		if lastEntry != nil && lastEntry.Status == JournalStatusFailed {
			return RecoveryAction{
				OperationID:  op.OperationID,
				CurrentState: string(op.Status),
				Action:       RecoveryActionRollback,
				Reason:       fmt.Sprintf("activation failed, need rollback: %s", lastEntry.Error),
			}
		}
		return RecoveryAction{
			OperationID:  op.OperationID,
			CurrentState: string(op.Status),
			Action:       RecoveryActionResume,
			Reason:       fmt.Sprintf("can resume activation: %s", reason),
		}

	case StateVerifyingHealth:
		return RecoveryAction{
			OperationID:  op.OperationID,
			CurrentState: string(op.Status),
			Action:       RecoveryActionResume,
			Reason:       fmt.Sprintf("can resume health verification: %s", reason),
		}

	case StateCommitting:
		if lastEntry != nil && lastEntry.Status == JournalStatusFailed {
			return RecoveryAction{
				OperationID:  op.OperationID,
				CurrentState: string(op.Status),
				Action:       RecoveryActionRollback,
				Reason:       fmt.Sprintf("commit failed, need rollback: %s", lastEntry.Error),
			}
		}
		return RecoveryAction{
			OperationID:  op.OperationID,
			CurrentState: string(op.Status),
			Action:       RecoveryActionResume,
			Reason:       fmt.Sprintf("can resume commit: %s", reason),
		}

	case StateRollingBack:
		return RecoveryAction{
			OperationID:  op.OperationID,
			CurrentState: string(op.Status),
			Action:       RecoveryActionManualIntervention,
			Reason:       fmt.Sprintf("rollback interrupted, manual intervention needed: %s", reason),
		}

	default:
		return RecoveryAction{
			OperationID:  op.OperationID,
			CurrentState: string(op.Status),
			Action:       RecoveryActionMarkFailed,
			Reason:       fmt.Sprintf("unrecoverable state: %s", reason),
		}
	}
}

func (r *RecoveryService) applyResumeAction(op *UpdateOperation) {
	now := time.Now().UTC()
	r.journal.Record(JournalEntry{
		OperationID: op.OperationID,
		Step:        "recovery_resume",
		Status:      JournalStatusCompleted,
		StartedAt:   now,
		FinishedAt:  &now,
		Compensation: "operation will be resumed by update manager",
	})
}

func (r *RecoveryService) applyRollbackAction(op *UpdateOperation) {
	now := time.Now().UTC()
	if err := r.stateMachine.Transition(op.Status, StateRollbackPending); err == nil {
		op.Status = StateRollbackPending
	} else {
		op.Status = StateRollbackPending
	}
	r.journal.Record(JournalEntry{
		OperationID: op.OperationID,
		Step:        "recovery_rollback",
		Status:      JournalStatusInProgress,
		StartedAt:   now,
		Compensation: "rollback initiated by recovery service",
	})
}

func (r *RecoveryService) applyMarkFailedAction(op *UpdateOperation) {
	now := time.Now().UTC()
	op.Status = StateFailed
	op.FinishedAt = &now
	r.journal.Record(JournalEntry{
		OperationID: op.OperationID,
		Step:        "recovery_mark_failed",
		Status:      JournalStatusCompleted,
		StartedAt:   now,
		FinishedAt:  &now,
		Compensation: "operation marked as failed by recovery service",
	})
}

func (r *RecoveryService) applyManualInterventionAction(op *UpdateOperation) {
	now := time.Now().UTC()
	op.Status = StateManualIntervention
	op.FinishedAt = &now
	r.journal.Record(JournalEntry{
		OperationID: op.OperationID,
		Step:        "recovery_manual_intervention",
		Status:      JournalStatusCompleted,
		StartedAt:   now,
		FinishedAt:  &now,
		Compensation: "operation requires manual intervention",
	})
}

var downloadStateProvider func(operationID string) (*DownloadState, bool)

func getDownloadStateSafe(operationID string) (*DownloadState, bool) {
	if downloadStateProvider == nil {
		return nil, false
	}
	return downloadStateProvider(operationID)
}

func SetDownloadStateProvider(provider func(operationID string) (*DownloadState, bool)) {
	downloadStateProvider = provider
}
