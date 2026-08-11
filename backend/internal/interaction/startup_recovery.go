package interaction

import (
	"context"
	"errors"
	"time"
)

type StartupRecoveryResult struct {
	Scanned   int
	Recovered int
	Skipped   int
	Failed    int
}

func (o *Orchestrator) RecoverStaleInteractions(ctx context.Context, cutoff time.Time) (StartupRecoveryResult, error) {
	return RecoverStaleInteractions(ctx, o.tracker, cutoff)
}

func (e *UnifiedEntry) RecoverStaleInteractions(ctx context.Context, cutoff time.Time) (StartupRecoveryResult, error) {
	return e.orchestrator.RecoverStaleInteractions(ctx, cutoff)
}

func RecoverStaleInteractions(ctx context.Context, tracker InteractionTracker, cutoff time.Time) (StartupRecoveryResult, error) {
	var result StartupRecoveryResult
	err := tracker.Range(ctx, func(record *InteractionRecord) bool {
		result.Scanned++
		if !shouldStartupRecover(record, cutoff) {
			result.Skipped++
			return true
		}
		if record.Status == InteractionStatusCommitted {
			if record.CommitID != "" {
				result.Recovered++
				return true
			}
		}
		if record.Status == InteractionStatusDeliveryPending {
			_, err := tracker.Fail(ctx, record.ID, record.StatusVersion, "delivery_timeout", "delivery pending timed out after restart")
			if err == nil {
				result.Recovered++
				return true
			}
			if errors.Is(err, ErrVersionConflict) || errors.Is(err, ErrAlreadyTerminal) {
				result.Skipped++
				return true
			}
			result.Failed++
			return true
		}
		if descriptorRecoveryPath(record) {
			result.Skipped++
			return true
		}
		_, err := tracker.Fail(ctx, record.ID, record.StatusVersion, "startup_recovered", "interaction was interrupted by process restart")
		if err == nil {
			result.Recovered++
			return true
		}
		if errors.Is(err, ErrVersionConflict) || errors.Is(err, ErrAlreadyTerminal) || errors.Is(err, ErrInvalidTransition) {
			result.Skipped++
			return true
		}
		result.Failed++
		return true
	})
	if err != nil {
		return result, err
	}
	if result.Failed > 0 {
		return result, errors.New("interaction startup recovery failed")
	}
	return result, nil
}

func shouldStartupRecover(record *InteractionRecord, cutoff time.Time) bool {
	if record == nil {
		return false
	}
	if isPausedStatus(record.Status) {
		return false
	}
	if !cutoff.IsZero() && !record.UpdatedAt.Before(cutoff) {
		return false
	}
	switch record.Status {
	case InteractionStatusProcessing,
		InteractionStatusContextReady,
		InteractionStatusDecided,
		InteractionStatusGenerated:
		return true
	case InteractionStatusCommitted:
		return true
	case InteractionStatusDeliveryPending:
		return record.HeartbeatAt.IsZero() || time.Since(record.HeartbeatAt) > 5*time.Minute
	default:
		return false
	}
}

func descriptorRecoveryPath(record *InteractionRecord) bool {
	if record == nil {
		return false
	}
	desc := record.RecoveryDescriptor
	if desc == nil {
		return false
	}
	switch desc.State {
	case RecoveryDescriptorRecoveryRequired,
		RecoveryDescriptorManualIntervention:
		return true
	case RecoveryDescriptorActive,
		RecoveryDescriptorPauseReady:
		return true
	}
	return false
}
