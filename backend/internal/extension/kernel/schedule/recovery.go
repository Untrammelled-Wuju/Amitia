package schedule

import (
	"context"
	"fmt"
)

type RecoveryService struct {
	store   ScheduleStore
	clock   Clock
	config  ScheduleConfig
	calc    *ScheduleCalculator
	misfire *MisfireService
	retry   *RetryService
	circuit *CircuitService
}

func NewRecoveryService(
	store ScheduleStore,
	clock Clock,
	config ScheduleConfig,
	calc *ScheduleCalculator,
	misfire *MisfireService,
	retry *RetryService,
	circuit *CircuitService,
) *RecoveryService {
	if clock == nil {
		clock = NewRealClock()
	}
	return &RecoveryService{
		store:   store,
		clock:   clock,
		config:  config,
		calc:    calc,
		misfire: misfire,
		retry:   retry,
		circuit: circuit,
	}
}

func (r *RecoveryService) Recover(ctx context.Context) error {
	now := r.clock.Now()

	if _, err := r.store.ReclaimExpiredLeases(ctx, now); err != nil {
		return err
	}

	states, err := r.store.ListStatesByStatus(ctx, DefinitionStatusEnabled)
	if err != nil {
		return fmt.Errorf("schedule recovery: list enabled states: %w", err)
	}

	for _, state := range states {
		if state == nil {
			continue
		}
		if err := r.recoverSchedule(ctx, state); err != nil {
			return err
		}
	}

	pausedStates, err := r.store.ListStatesByStatus(ctx, DefinitionStatusPaused)
	if err != nil {
		return err
	}
	for _, state := range pausedStates {
		if state == nil {
			continue
		}
		if err := r.recoverPausedSchedule(ctx, state); err != nil {
			return err
		}
	}

	triggeringTriggers := make([]*ScheduleTriggerRecord, 0)
	if recoverableStore, ok := r.store.(interface {
		ListTriggersByStatuses(context.Context, []ScheduleRunStatus, int) ([]*ScheduleTriggerRecord, error)
	}); ok {
		triggeringTriggers, err = recoverableStore.ListTriggersByStatuses(ctx, []ScheduleRunStatus{RunStatusLeased, RunStatusTriggering, RunStatusRunning, RunStatusRecoveryRequired}, -1)
		if err != nil {
			return err
		}
	}
	for _, trigger := range triggeringTriggers {
		if trigger == nil {
			continue
		}
		if trigger.Status == RunStatusLeased || trigger.Status == RunStatusTriggering || trigger.Status == RunStatusRunning {
			if trigger.LeaseExpiresAt != nil && trigger.LeaseExpiresAt.After(now) {
				continue
			}
			if err := r.recoverStaleTrigger(ctx, trigger); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *RecoveryService) recoverSchedule(ctx context.Context, state *ScheduleState) error {
	return withScheduleTransaction(ctx, r.store, func(ctx context.Context) error {
		fresh, err := r.store.GetState(ctx, state.ScheduleID)
		if err != nil {
			return err
		}
		if fresh == nil || fresh.Generation != state.Generation || fresh.Status != DefinitionStatusEnabled {
			return nil
		}
		return r.recoverEnabledState(ctx, fresh)
	})
}

func (r *RecoveryService) recoverEnabledState(ctx context.Context, state *ScheduleState) error {
	def, err := r.store.GetDefinition(ctx, state.ScheduleID)
	if err != nil {
		return err
	}
	if def == nil {
		return ErrScheduleNotFound
	}

	if def.EndAt != nil && r.clock.Now().After(*def.EndAt) {
		now := r.clock.Now()
		state.Status = DefinitionStatusExpired
		state.Enabled = false
		state.NextScheduledAt = nil
		state.NextEffectiveAt = nil
		state.UpdatedAt = now.UTC()
		return r.store.PutState(ctx, state)
	}

	if state.LastScheduledAt != nil || state.NextScheduledAt != nil {
		detection, err := r.misfire.DetectMisfire(ctx, def, state)
		if err != nil {
			return err
		}
		if detection != nil && detection.HasMisfire {
			action, applyErr := r.misfire.ApplyMisfirePolicy(ctx, def, detection)
			if applyErr != nil {
				return applyErr
			}
			if action != nil && action.Reschedule {
				rescheduled := r.clock.Now().UTC()
				state.LastScheduledAt = &rescheduled
			} else if detection.LatestMissed != nil {
				accounted := detection.LatestMissed.UTC()
				state.LastScheduledAt = &accounted
			}
		}
	}

	result, err := r.calc.CalculateNext(def, state)
	if err != nil {
		return err
	}
	now := r.clock.Now()
	if result != nil {
		state.NextScheduledAt = result.NextScheduledAt
		state.NextEffectiveAt = result.NextEffectiveAt
	} else {
		state.Status = DefinitionStatusExpired
		state.NextScheduledAt = nil
		state.NextEffectiveAt = nil
	}
	state.UpdatedAt = now.UTC()
	return r.store.PutState(ctx, state)
}

func (r *RecoveryService) recoverPausedSchedule(ctx context.Context, state *ScheduleState) error {
	return withScheduleTransaction(ctx, r.store, func(ctx context.Context) error {
		fresh, err := r.store.GetState(ctx, state.ScheduleID)
		if err != nil {
			return err
		}
		if fresh == nil || fresh.Generation != state.Generation || fresh.Status != DefinitionStatusPaused {
			return nil
		}
		return r.recoverPausedState(ctx, fresh)
	})
}

func (r *RecoveryService) recoverPausedState(ctx context.Context, state *ScheduleState) error {
	def, err := r.store.GetDefinition(ctx, state.ScheduleID)
	if err != nil {
		return err
	}
	if def == nil {
		return ErrScheduleNotFound
	}
	if def.EndAt != nil && r.clock.Now().After(*def.EndAt) {
		now := r.clock.Now()
		state.Status = DefinitionStatusExpired
		state.Enabled = false
		state.UpdatedAt = now.UTC()
		return r.store.PutState(ctx, state)
	}
	return nil
}

func (r *RecoveryService) recoverStaleTrigger(ctx context.Context, trigger *ScheduleTriggerRecord) error {
	def, err := r.store.GetDefinition(ctx, trigger.ScheduleID)
	if err != nil {
		return err
	}
	status := RunStatusWaiting
	if trigger.Status != RunStatusLeased && (def == nil || def.Target.IdempotencyMode == IdempotencyModeNonIdempotent) {
		status = RunStatusQuarantined
	}
	return r.store.UpdateTriggerStatus(ctx, trigger.TriggerID, status, map[string]any{
		"lease_owner":      nil,
		"lease_expires_at": nil,
		"error_code":       nil,
		"error_message":    nil,
	})
}
