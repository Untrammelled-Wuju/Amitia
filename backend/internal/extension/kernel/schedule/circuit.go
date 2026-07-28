package schedule

import (
	"context"
)

type CircuitService struct {
	store  ScheduleStore
	clock  Clock
	config ScheduleConfig
}

func NewCircuitService(store ScheduleStore, clock Clock, config ScheduleConfig) *CircuitService {
	if clock == nil {
		clock = NewRealClock()
	}
	return &CircuitService{store: store, clock: clock, config: config}
}

func (c *CircuitService) RecordSuccess(ctx context.Context, scheduleID string) error {
	record, err := c.store.GetCircuit(ctx, scheduleID)
	if err != nil {
		return err
	}
	now := c.clock.Now()
	if record == nil {
		record = &ScheduleCircuitRecord{
			ScheduleID:       scheduleID,
			State:            CircuitStateClosed,
			ConsecutiveFails: 0,
			TotalSuccess:     1,
			UpdatedAt:        now.UTC(),
		}
		return c.store.PutCircuit(ctx, record)
	}
	record.ConsecutiveFails = 0
	record.TotalSuccess++
	if record.State == CircuitStateHalfOpen {
		record.State = CircuitStateClosed
		record.OpenedAt = nil
	}
	record.UpdatedAt = now.UTC()
	return c.store.PutCircuit(ctx, record)
}

func (c *CircuitService) RecordFailure(ctx context.Context, scheduleID string, errorCode string) error {
	record, err := c.store.GetCircuit(ctx, scheduleID)
	if err != nil {
		return err
	}
	now := c.clock.Now()
	if record == nil {
		record = &ScheduleCircuitRecord{
			ScheduleID:       scheduleID,
			State:            CircuitStateClosed,
			ConsecutiveFails: 1,
			TotalFails:       1,
			LastFailCode:     &errorCode,
			LastFailTime:     &now,
			UpdatedAt:        now.UTC(),
		}
		return c.store.PutCircuit(ctx, record)
	}

	record.ConsecutiveFails++
	record.TotalFails++
	record.LastFailCode = &errorCode
	lastTime := now.UTC()
	record.LastFailTime = &lastTime

	if record.State == CircuitStateHalfOpen {
		record.State = CircuitStateOpen
		openedAt := now.UTC()
		record.OpenedAt = &openedAt
	} else if record.State == CircuitStateClosed && record.ConsecutiveFails >= c.config.CircuitFailThreshold {
		record.State = CircuitStateOpen
		openedAt := now.UTC()
		record.OpenedAt = &openedAt
	}

	record.UpdatedAt = now.UTC()
	return c.store.PutCircuit(ctx, record)
}

func (c *CircuitService) GetState(ctx context.Context, scheduleID string) (CircuitState, error) {
	record, err := c.store.GetCircuit(ctx, scheduleID)
	if err != nil {
		return CircuitStateClosed, err
	}
	if record == nil {
		return CircuitStateClosed, nil
	}
	if record.State == CircuitStateOpen && record.OpenedAt != nil {
		if c.clock.Now().Sub(*record.OpenedAt) >= c.config.CircuitRecoveryAfter {
			record.State = CircuitStateHalfOpen
			record.UpdatedAt = c.clock.Now().UTC()
			_ = c.store.PutCircuit(ctx, record)
		}
	}
	return record.State, nil
}

func (c *CircuitService) Reset(ctx context.Context, scheduleID string) error {
	now := c.clock.Now()
	record := &ScheduleCircuitRecord{
		ScheduleID:       scheduleID,
		State:            CircuitStateClosed,
		ConsecutiveFails: 0,
		UpdatedAt:        now.UTC(),
	}
	return c.store.PutCircuit(ctx, record)
}

func (c *CircuitService) IsOpen(ctx context.Context, scheduleID string) (bool, error) {
	state, err := c.GetState(ctx, scheduleID)
	if err != nil {
		return false, err
	}
	return state == CircuitStateOpen, nil
}

var circuitFailureCodes = map[string]bool{
	ErrCodeTargetNotFound:        true,
	ErrCodeRuntimeHandlerMissing: true,
	ErrCodeTargetExecutionFailed: true,
	ErrCodePermissionDenied:      true,
	ErrCodeScopeDenied:           true,
	ErrCodeDependencyMissing:     true,
}

func IsCircuitBreakingError(errorCode string) bool {
	return circuitFailureCodes[errorCode]
}

func (c *CircuitService) HandleExecutionResult(ctx context.Context, scheduleID string, success bool, errorCode string) error {
	if success {
		return c.RecordSuccess(ctx, scheduleID)
	}
	if IsCircuitBreakingError(errorCode) {
		return c.RecordFailure(ctx, scheduleID, errorCode)
	}
	return nil
}
