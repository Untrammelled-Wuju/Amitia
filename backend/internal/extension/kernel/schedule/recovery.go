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
	}

	states, err := r.store.ListStatesByStatus(ctx, DefinitionStatusEnabled)
	if err != nil {
		return fmt.Errorf("schedule recovery: list enabled states: %w", err)
	}

	batchSize := 50
	if len(states) > batchSize {
		states = states[:batchSize]
	}

	for _, state := range states {
		if state == nil {
			continue
		}
		r.recoverSchedule(ctx, state)
	}

	pausedStates, _ := r.store.ListStatesByStatus(ctx, DefinitionStatusPaused)
	for _, state := range pausedStates {
		if state == nil {
			continue
		}
		r.recoverPausedSchedule(ctx, state)
	}

	triggeringTriggers := make([]*ScheduleTriggerRecord, 0)
	if recoverableStore, ok := r.store.(interface {
		ListTriggersByStatuses(context.Context, []ScheduleRunStatus, int) ([]*ScheduleTriggerRecord, error)
	}); ok {
		triggeringTriggers, _ = recoverableStore.ListTriggersByStatuses(ctx, []ScheduleRunStatus{RunStatusLeased, RunStatusTriggering, RunStatusRunning, RunStatusRecoveryRequired}, 100)
	}
	for _, trigger := range triggeringTriggers {
		if trigger == nil {
			continue
		}
		if trigger.Status == RunStatusLeased || trigger.Status == RunStatusTriggering || trigger.Status == RunStatusRunning {
			r.recoverStaleTrigger(ctx, trigger)
		}
	}

	return nil
}

func (r *RecoveryService) recoverSchedule(ctx context.Context, state *ScheduleState) {
	def, err := r.store.GetDefinition(ctx, state.ScheduleID)
	if err != nil || def == nil {
		return
	}

	if def.EndAt != nil && r.clock.Now().After(*def.EndAt) {
		now := r.clock.Now()
		state.Status = DefinitionStatusExpired
		state.NextScheduledAt = nil
		state.NextEffectiveAt = nil
		state.UpdatedAt = now.UTC()
		_ = r.store.PutState(ctx, state)
		return
	}

	if state.LastScheduledAt != nil {
		detection, _ := r.misfire.DetectMisfire(ctx, def, state)
		if detection != nil && detection.HasMisfire {
			r.misfire.ApplyMisfirePolicy(ctx, def, detection)
		}
	}

	result, err := r.calc.CalculateNext(def, state)
	if err != nil {
		return
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
	_ = r.store.PutState(ctx, state)
}

func (r *RecoveryService) recoverPausedSchedule(ctx context.Context, state *ScheduleState) {
	def, err := r.store.GetDefinition(ctx, state.ScheduleID)
	if err != nil || def == nil {
		return
	}
	if def.EndAt != nil && r.clock.Now().After(*def.EndAt) {
		now := r.clock.Now()
		state.Status = DefinitionStatusExpired
		state.UpdatedAt = now.UTC()
		_ = r.store.PutState(ctx, state)
	}
}

func (r *RecoveryService) recoverStaleTrigger(ctx context.Context, trigger *ScheduleTriggerRecord) {
	_ = r.store.UpdateTriggerStatus(ctx, trigger.TriggerID, RunStatusWaiting, map[string]any{
		"lease_owner":      nil,
		"lease_expires_at": nil,
		"error_code":       nil,
		"error_message":    nil,
	})
}
