package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ScheduleExecutor struct {
	store        ScheduleStore
	clock        Clock
	config       ScheduleConfig
	calc         *ScheduleCalculator
	circuit      *CircuitService
	retry        *RetryService
	leaseManager *LeaseManager

	targetAdapters    map[TargetType]TargetAdapter
	permissionChecker PermissionChecker
	scopeChecker      ScopeChecker
	dependencyChecker DependencyChecker

	leaseOwner string

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

type PermissionChecker interface {
	CheckPermission(ctx context.Context, scheduleID string, requirements []PermissionRequirement, isBackground bool) (bool, string, error)
}

type ScopeChecker interface {
	CheckScope(ctx context.Context, scheduleID string, rule ScopeRule) (bool, string, error)
	CreateSnapshot(ctx context.Context, scheduleID, invocationID string, rule ScopeRule) (string, error)
}

type DependencyChecker interface {
	CheckDependencies(ctx context.Context, requirements []DependencyRequirement) (bool, string, error)
	CreateSnapshot(ctx context.Context, requirements []DependencyRequirement) (string, error)
}

func NewScheduleExecutor(
	store ScheduleStore,
	clock Clock,
	config ScheduleConfig,
	calc *ScheduleCalculator,
	circuit *CircuitService,
	retry *RetryService,
	leaseManager *LeaseManager,
	permissionChecker PermissionChecker,
	scopeChecker ScopeChecker,
	dependencyChecker DependencyChecker,
) *ScheduleExecutor {
	if clock == nil {
		clock = NewRealClock()
	}
	return &ScheduleExecutor{
		store:             store,
		clock:             clock,
		config:            config,
		calc:              calc,
		circuit:           circuit,
		retry:             retry,
		leaseManager:      leaseManager,
		permissionChecker: permissionChecker,
		scopeChecker:      scopeChecker,
		dependencyChecker: dependencyChecker,
		targetAdapters:    map[TargetType]TargetAdapter{},
		leaseOwner:        "amitia-schedule-executor-backend",
	}
}

func (e *ScheduleExecutor) RegisterTargetAdapter(adapter TargetAdapter) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.targetAdapters[adapter.Type()] = adapter
}

func (e *ScheduleExecutor) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return fmt.Errorf("executor already running")
	}
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.running = true
	e.wg.Add(1)
	go e.executionLoop()
	return nil
}

func (e *ScheduleExecutor) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	if e.cancel != nil {
		e.cancel()
	}
	e.mu.Unlock()
	e.wg.Wait()
}

func (e *ScheduleExecutor) executionLoop() {
	defer e.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			_ = e.retry.ProcessDueRetries(e.ctx, e)
			e.processDueTriggers()
		}
	}
}

func (e *ScheduleExecutor) processDueTriggers() {
	now := e.clock.Now()
	triggers, err := e.store.ListDueTriggers(e.ctx, now, e.config.ScanBatchSize)
	if err != nil || len(triggers) == 0 {
		return
	}
	for _, trigger := range triggers {
		if trigger == nil {
			continue
		}
		go e.Execute(e.ctx, trigger)
	}
}

func (e *ScheduleExecutor) Execute(ctx context.Context, trigger *ScheduleTriggerRecord) (*ExecuteResult, error) {
	def, err := e.store.GetDefinition(ctx, trigger.ScheduleID)
	if err != nil || def == nil {
		return e.failTrigger(ctx, trigger, ErrCodeTargetNotFound, "schedule definition not found")
	}

	if def.ExecutionOwner != ExecutionOwnerBackend {
		return e.blockTrigger(ctx, trigger, ErrCodeExecutionOwnerDenied, "execution owner is not backend")
	}

	isOpen, err := e.circuit.IsOpen(ctx, def.ScheduleID)
	if err == nil && isOpen {
		return e.blockTrigger(ctx, trigger, ErrCodeCircuitOpen, "circuit breaker open")
	}

	state, err := e.store.GetState(ctx, trigger.ScheduleID)
	if err != nil || state == nil {
		return e.failTrigger(ctx, trigger, ErrCodeTargetNotFound, "schedule state not found")
	}
	if !state.Enabled || state.Paused {
		return e.blockTrigger(ctx, trigger, "SCHEDULE_NOT_ACTIVE", "schedule not active")
	}

	if trigger.Generation != state.Generation {
		return e.blockTrigger(ctx, trigger, ErrCodeGenerationMismatch, "generation mismatch")
	}

	leaseOwner := fmt.Sprintf("amitia-schedule-executor-%s", string(def.ExecutionOwner))
	if trigger.LeaseOwner != nil && *trigger.LeaseOwner != leaseOwner &&
		trigger.LeaseExpiresAt != nil && trigger.LeaseExpiresAt.After(e.clock.Now().UTC()) {
		return e.quarantineTrigger(ctx, trigger, def, QuarantineDualScheduler,
			fmt.Sprintf("trigger leased by %s", *trigger.LeaseOwner))
	}

	acquired, err := e.leaseManager.AcquireLease(ctx, trigger.TriggerID, leaseOwner)
	if err != nil || !acquired {
		freshTrigger, getErr := e.store.GetTrigger(ctx, trigger.TriggerID)
		if getErr == nil && freshTrigger != nil && freshTrigger.LeaseOwner != nil &&
			*freshTrigger.LeaseOwner != leaseOwner &&
			freshTrigger.LeaseExpiresAt != nil && freshTrigger.LeaseExpiresAt.After(e.clock.Now().UTC()) {
			return e.quarantineTrigger(ctx, trigger, def, QuarantineDualScheduler,
				fmt.Sprintf("trigger leased by %s", *freshTrigger.LeaseOwner))
		}
		return &ExecuteResult{
			TriggerID:    trigger.TriggerID,
			Status:       trigger.Status,
			ErrorCode:    ErrCodeLeaseFailed,
			ErrorMessage: "lease acquisition failed",
		}, nil
	}
	defer func() {
		_ = e.leaseManager.ReleaseLease(ctx, trigger.TriggerID)
	}()

	if len(def.PermissionRequirements) > 0 && e.permissionChecker == nil {
		return e.failTrigger(ctx, trigger, ErrCodePermissionDenied, "permission checker not configured")
	}
	if e.permissionChecker != nil && len(def.PermissionRequirements) > 0 {
		allowed, reason, err := e.permissionChecker.CheckPermission(ctx, def.ScheduleID, def.PermissionRequirements, true)
		if err != nil || !allowed {
			return e.failTrigger(ctx, trigger, ErrCodePermissionDenied, reason)
		}
		if trigger.PermissionSnapshotID == "" {
			trigger.PermissionSnapshotID = fmt.Sprintf("perm-snap-%s-%d", trigger.TriggerID, trigger.Generation)
		}
	}

	var scopeSnapshotID string
	if e.scopeChecker != nil {
		allowed, reason, err := e.scopeChecker.CheckScope(ctx, def.ScheduleID, def.ScopeRule)
		if err != nil || !allowed {
			return e.failTrigger(ctx, trigger, ErrCodeScopeDenied, reason)
		}
		snapshotID, err := e.scopeChecker.CreateSnapshot(ctx, def.ScheduleID, trigger.TriggerID, def.ScopeRule)
		if err != nil {
			return e.failTrigger(ctx, trigger, ErrCodeScopeDenied, err.Error())
		}
		scopeSnapshotID = snapshotID
	}
	if e.scopeChecker == nil && def.ScopeRule.ScopeType != "" && def.ScopeRule.ScopeType != "global" {
		return e.failTrigger(ctx, trigger, ErrCodeScopeDenied, "scope checker not configured")
	}

	var depSnapshotID string
	if e.dependencyChecker != nil && len(def.DependencyRequirements) > 0 {
		ok, reason, err := e.dependencyChecker.CheckDependencies(ctx, def.DependencyRequirements)
		if err != nil || !ok {
			return e.blockTrigger(ctx, trigger, ErrCodeDependencyMissing, reason)
		}
		snapshotID, err := e.dependencyChecker.CreateSnapshot(ctx, def.DependencyRequirements)
		if err == nil {
			depSnapshotID = snapshotID
		}
	}

	e.mu.Lock()
	adapter := e.targetAdapters[def.Target.Type]
	e.mu.Unlock()
	if adapter == nil {
		return e.failTrigger(ctx, trigger, ErrCodeTargetNotFound, fmt.Sprintf("no adapter for target type: %s", def.Target.Type))
	}

	now := e.clock.Now()
	trigger.Status = RunStatusTriggering
	trigger.TriggeredAt = &now
	trigger.ScopeSnapshotID = scopeSnapshotID
	trigger.DependencySnapshotID = depSnapshotID
	trigger.UpdatedAt = now.UTC()
	_ = e.store.PutTrigger(ctx, trigger)

	runID := "run-" + uuid.NewString()
	run := &ScheduleRunRecord{
		RunID:      runID,
		TriggerID:  trigger.TriggerID,
		ScheduleID: def.ScheduleID,
		Status:     RunStatusRunning,
		Attempt:    trigger.Attempt,
		StartedAt:  now.UTC(),
		TargetType: def.Target.Type,
		TargetID:   def.Target.TargetID,
		Generation: trigger.Generation,
		CreatedAt:  now.UTC(),
		UpdatedAt:  now.UTC(),
	}
	if err := e.store.PutRun(ctx, run); err != nil {
		return e.failTrigger(ctx, trigger, ErrCodeTargetExecutionFailed, "failed to create run record")
	}

	targetResult, err := adapter.Execute(ctx, def, trigger)

	finishTime := e.clock.Now()
	run.FinishedAt = &finishTime
	run.UpdatedAt = finishTime.UTC()

	if err != nil {
		run.Status = RunStatusFailed
		run.ErrorCode = strPtr(ErrCodeTargetExecutionFailed)
		run.ErrorMessage = strPtr(err.Error())
		_ = e.store.PutRun(ctx, run)
		_ = e.circuit.HandleExecutionResult(ctx, def.ScheduleID, false, ErrCodeTargetExecutionFailed)
		e.handleFailure(ctx, def, trigger, ErrCodeTargetExecutionFailed, err.Error())
		return &ExecuteResult{
			TriggerID:    trigger.TriggerID,
			Status:       RunStatusFailed,
			ErrorCode:    ErrCodeTargetExecutionFailed,
			ErrorMessage: err.Error(),
		}, nil
	}

	if targetResult.Success {
		run.Status = RunStatusCompleted
		if targetResult.OperationID != "" {
			run.OperationID = targetResult.OperationID
		}
		if targetResult.InvocationID != "" {
			run.InvocationID = targetResult.InvocationID
		}
		if len(targetResult.ResultJSON) > 0 {
			run.ResultJSON = targetResult.ResultJSON
		}
		_ = e.store.PutRun(ctx, run)
		_ = e.circuit.HandleExecutionResult(ctx, def.ScheduleID, true, "")

		trigger.Status = RunStatusCompleted
		trigger.OperationID = strPtr(targetResult.OperationID)
		if targetResult.InvocationID != "" {
			trigger.InvocationID = strPtr(targetResult.InvocationID)
		}
		trigger.UpdatedAt = finishTime.UTC()
		_ = e.store.PutTrigger(ctx, trigger)

		e.updateScheduleStateOnSuccess(ctx, def, state, trigger, finishTime)

		return &ExecuteResult{
			TriggerID:    trigger.TriggerID,
			Status:       RunStatusCompleted,
			OperationID:  targetResult.OperationID,
			InvocationID: targetResult.InvocationID,
		}, nil
	}

	run.Status = RunStatusFailed
	run.ErrorCode = strPtr(targetResult.ErrorCode)
	run.ErrorMessage = strPtr(targetResult.ErrorMessage)
	_ = e.store.PutRun(ctx, run)
	_ = e.circuit.HandleExecutionResult(ctx, def.ScheduleID, false, targetResult.ErrorCode)
	e.handleFailure(ctx, def, trigger, targetResult.ErrorCode, targetResult.ErrorMessage)

	return &ExecuteResult{
		TriggerID:    trigger.TriggerID,
		Status:       RunStatusFailed,
		ErrorCode:    targetResult.ErrorCode,
		ErrorMessage: targetResult.ErrorMessage,
	}, nil
}

func (e *ScheduleExecutor) handleFailure(ctx context.Context, def *ScheduleContributionDefinition, trigger *ScheduleTriggerRecord, errorCode, errorMessage string) {
	if def.Target.IdempotencyMode == IdempotencyModeNonIdempotent && trigger.Attempt >= 0 {
		e.markManualIntervention(ctx, trigger, errorCode, errorMessage)
		return
	}

	retryRecord, err := e.retry.ScheduleRetry(ctx, def, trigger, errorCode, errorMessage)
	if err != nil || retryRecord == nil {
		e.markFailed(ctx, trigger, errorCode, errorMessage)
	}
}

func (e *ScheduleExecutor) markManualIntervention(ctx context.Context, trigger *ScheduleTriggerRecord, errorCode, errorMessage string) (*ExecuteResult, error) {
	now := e.clock.Now()
	trigger.Status = RunStatusQuarantined
	trigger.ErrorCode = strPtr(errorCode)
	trigger.ErrorMessage = strPtr(errorMessage)
	trigger.UpdatedAt = now.UTC()
	_ = e.store.UpdateTriggerStatus(ctx, trigger.TriggerID, RunStatusQuarantined, map[string]any{
		"error_code":    errorCode,
		"error_message": errorMessage,
	})
	return &ExecuteResult{
		TriggerID:    trigger.TriggerID,
		Status:       RunStatusQuarantined,
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
	}, nil
}

func (e *ScheduleExecutor) quarantineTrigger(ctx context.Context, trigger *ScheduleTriggerRecord, def *ScheduleContributionDefinition, reason QuarantineReason, detail string) (*ExecuteResult, error) {
	now := e.clock.Now()
	trigger.Status = RunStatusQuarantined
	trigger.ErrorCode = strPtr(ErrCodeDualScheduler)
	trigger.ErrorMessage = strPtr(detail)
	trigger.UpdatedAt = now.UTC()
	_ = e.store.UpdateTriggerStatus(ctx, trigger.TriggerID, RunStatusQuarantined, map[string]any{
		"error_code":    ErrCodeDualScheduler,
		"error_message": detail,
	})
	_ = e.store.PutQuarantine(ctx, &ScheduleQuarantineRecord{
		ScheduleID:    def.ScheduleID,
		Reason:        reason,
		Detail:        detail,
		QuarantinedAt: now.UTC(),
	})
	return &ExecuteResult{
		TriggerID:    trigger.TriggerID,
		Status:       RunStatusQuarantined,
		ErrorCode:    ErrCodeDualScheduler,
		ErrorMessage: detail,
	}, nil
}

func (e *ScheduleExecutor) markFailed(ctx context.Context, trigger *ScheduleTriggerRecord, errorCode, errorMessage string) (*ExecuteResult, error) {
	now := e.clock.Now()
	trigger.Status = RunStatusFailed
	trigger.ErrorCode = strPtr(errorCode)
	trigger.ErrorMessage = strPtr(errorMessage)
	trigger.UpdatedAt = now.UTC()
	_ = e.store.UpdateTriggerStatus(ctx, trigger.TriggerID, RunStatusFailed, map[string]any{
		"error_code":    errorCode,
		"error_message": errorMessage,
	})
	return &ExecuteResult{
		TriggerID:    trigger.TriggerID,
		Status:       RunStatusFailed,
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
	}, nil
}

func (e *ScheduleExecutor) failTrigger(ctx context.Context, trigger *ScheduleTriggerRecord, errorCode, errorMessage string) (*ExecuteResult, error) {
	return e.markFailed(ctx, trigger, errorCode, errorMessage)
}

func (e *ScheduleExecutor) blockTrigger(ctx context.Context, trigger *ScheduleTriggerRecord, errorCode, errorMessage string) (*ExecuteResult, error) {
	now := e.clock.Now()
	trigger.Status = RunStatusBlocked
	trigger.ErrorCode = strPtr(errorCode)
	trigger.ErrorMessage = strPtr(errorMessage)
	trigger.UpdatedAt = now.UTC()
	_ = e.store.UpdateTriggerStatus(ctx, trigger.TriggerID, RunStatusBlocked, map[string]any{
		"error_code":    errorCode,
		"error_message": errorMessage,
	})
	return &ExecuteResult{
		TriggerID:    trigger.TriggerID,
		Status:       RunStatusBlocked,
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
	}, nil
}

func (e *ScheduleExecutor) updateScheduleStateOnSuccess(ctx context.Context, def *ScheduleContributionDefinition, state *ScheduleState, trigger *ScheduleTriggerRecord, finishTime time.Time) {
	now := e.clock.Now()
	state.LastTriggeredAt = &trigger.ScheduledAt
	state.LastFinishedAt = &finishTime
	state.LastResult = "success"
	state.FailureCount = 0

	if def.Trigger.Type == TriggerTypeOneShot {
		state.Status = DefinitionStatusExpired
		state.NextScheduledAt = nil
		state.NextEffectiveAt = nil
	} else {
		result, err := e.calc.CalculateNext(def, state)
		if err == nil && result != nil {
			state.NextScheduledAt = result.NextScheduledAt
			state.NextEffectiveAt = result.NextEffectiveAt
		}
	}
	state.UpdatedAt = now.UTC()
	_ = e.store.PutState(ctx, state)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
