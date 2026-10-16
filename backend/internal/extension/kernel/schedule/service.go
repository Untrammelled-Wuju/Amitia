package schedule

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ScheduleService struct {
	store        ScheduleStore
	clock        Clock
	config       ScheduleConfig
	calc         *ScheduleCalculator
	scanner      *ScheduleScanner
	leaseManager *LeaseManager
	planner      *TriggerPlanner
	executor     *ScheduleExecutor
	misfire      *MisfireService
	retry        *RetryService
	circuit      *CircuitService
	recovery     *RecoveryService

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

type ScheduleDeps struct {
	Store             ScheduleStore
	Clock             Clock
	Config            ScheduleConfig
	PermissionChecker PermissionChecker
	ScopeChecker      ScopeChecker
	DependencyChecker DependencyChecker
	ToolExecutor      ToolExecutor
	WorkflowExecutor  WorkflowExecutor
	TaskEnqueueFn     TaskEnqueueFunc
	RuntimeHandlerFn  RuntimeHandlerInvokeFunc
}

func NewScheduleService(deps ScheduleDeps) (*ScheduleService, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("schedule: store is required")
	}
	clock := deps.Clock
	if clock == nil {
		clock = NewRealClock()
	}
	config := deps.Config
	if config.ScanInterval == 0 {
		config = DefaultScheduleConfig()
	}

	calc := NewScheduleCalculator(clock)
	misfire := NewMisfireService(deps.Store, clock)
	retry := NewRetryService(deps.Store, clock, config)
	circuit := NewCircuitService(deps.Store, clock, config)
	leaseManager := NewLeaseManager(deps.Store, clock, config)
	planner := NewTriggerPlanner(deps.Store, clock, config, calc, misfire)
	executor := NewScheduleExecutor(
		deps.Store, clock, config, calc, circuit, retry, leaseManager,
		deps.PermissionChecker, deps.ScopeChecker, deps.DependencyChecker,
	)

	if deps.ToolExecutor != nil {
		executor.RegisterTargetAdapter(NewToolTargetAdapter(deps.ToolExecutor))
	}
	if deps.WorkflowExecutor != nil {
		executor.RegisterTargetAdapter(NewWorkflowTargetAdapter(deps.WorkflowExecutor))
	}
	if deps.TaskEnqueueFn != nil {
		executor.RegisterTargetAdapter(NewTaskTargetAdapter(deps.TaskEnqueueFn))
	}
	if deps.RuntimeHandlerFn != nil {
		executor.RegisterTargetAdapter(NewRuntimeHandlerTargetAdapter(deps.RuntimeHandlerFn))
	}

	recovery := NewRecoveryService(deps.Store, clock, config, calc, misfire, retry, circuit)

	scanner := NewScheduleScanner(deps.Store, clock, config, calc, planner)

	svc := &ScheduleService{
		store:        deps.Store,
		clock:        clock,
		config:       config,
		calc:         calc,
		scanner:      scanner,
		leaseManager: leaseManager,
		planner:      planner,
		executor:     executor,
		misfire:      misfire,
		retry:        retry,
		circuit:      circuit,
		recovery:     recovery,
	}

	return svc, nil
}

func (s *ScheduleService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return fmt.Errorf("schedule service already running")
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true

	if err := s.recovery.Recover(s.ctx); err != nil {
		s.cancel()
		s.running = false
		return fmt.Errorf("schedule recovery: %w", err)
	}

	if err := s.scanner.Start(s.ctx); err != nil {
		return err
	}
	if err := s.leaseManager.Start(s.ctx); err != nil {
		s.scanner.Stop()
		return err
	}
	if err := s.executor.Start(s.ctx); err != nil {
		s.scanner.Stop()
		s.leaseManager.Stop()
		return err
	}
	return nil
}

func (s *ScheduleService) Shutdown(ctx context.Context) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()

	s.executor.Stop()
	s.leaseManager.Stop()
	s.scanner.Stop()
}

func (s *ScheduleService) InstallDefinition(ctx context.Context, def *ScheduleContributionDefinition) error {
	if def != nil && def.Target.Type == TargetTypeHostWorkload {
		return ErrScopeDenied
	}
	return s.installDefinition(ctx, def)
}

func (s *ScheduleService) InstallHostDefinition(ctx context.Context, def *ScheduleContributionDefinition) error {
	if def == nil || def.Target.Type != TargetTypeHostWorkload || def.Target.TargetID == "" {
		return ErrInvalidTargetType
	}
	return s.installDefinition(ctx, def)
}

func (s *ScheduleService) InstallArchivedHostDefinition(ctx context.Context, def *ScheduleContributionDefinition) error {
	if def == nil || def.Target.Type != TargetTypeHostWorkload || def.Target.TargetID == "" || def.Trigger.RestoredOccurrences < 0 {
		return ErrInvalidTargetType
	}
	return withScheduleTransaction(ctx, s.store, func(ctx context.Context) error {
		if _, err := s.store.GetDefinition(ctx, def.ScheduleID); err == nil {
			return ErrIdempotencyConflict
		} else if !errors.Is(err, ErrScheduleNotFound) {
			return err
		}
		def.EnabledByDefault = false
		if err := s.installDefinition(ctx, def); err != nil {
			return err
		}
		state, err := s.store.GetState(ctx, def.ScheduleID)
		if err != nil {
			return err
		}
		state.Enabled = false
		state.Paused = true
		state.Status = DefinitionStatusPaused
		state.NextScheduledAt = nil
		state.NextEffectiveAt = nil
		return s.store.PutState(ctx, state)
	})
}

func (s *ScheduleService) RestoreHostDefinition(ctx context.Context, id string, expectedGeneration int64, def *ScheduleContributionDefinition) error {
	if def == nil || def.Target.Type != TargetTypeHostWorkload || def.Target.TargetID == "" || def.Trigger.RestoredOccurrences < 0 {
		return ErrInvalidTargetType
	}
	return withScheduleTransaction(ctx, s.store, func(ctx context.Context) error {
		existing, err := s.store.GetDefinition(ctx, id)
		if err != nil {
			return err
		}
		if existing.Target.Type != TargetTypeHostWorkload {
			return ErrScopeDenied
		}
		count, err := countScheduledTriggers(ctx, s.store, id)
		if err != nil {
			return err
		}
		actual := count - existing.Trigger.RestoredOccurrences
		archiveCount := def.Trigger.RestoredOccurrences
		if archiveCount < count {
			archiveCount = count
		}
		disabled := false
		if err := s.update(ctx, id, expectedGeneration, def, &disabled); err != nil {
			return err
		}
		def.Trigger.RestoredOccurrences = archiveCount - actual
		def.DefinitionHash = GenerateDefinitionHash(def)
		if err := s.store.PutDefinition(ctx, def); err != nil {
			return err
		}
		state, err := s.store.GetState(ctx, id)
		if err != nil {
			return err
		}
		if state.Generation == expectedGeneration {
			state.Generation++
		}
		state.Enabled = false
		state.Paused = true
		state.Status = DefinitionStatusPaused
		state.NextScheduledAt = nil
		state.NextEffectiveAt = nil
		return s.store.PutState(ctx, state)
	})
}

func (s *ScheduleService) installDefinition(ctx context.Context, def *ScheduleContributionDefinition) error {
	if def == nil {
		return ErrScheduleNotFound
	}
	if def.Trigger.RestoredOccurrences < 0 {
		return ErrInvalidTriggerType
	}
	if def.ScheduleID == "" {
		def.ScheduleID = "sched-" + uuid.NewString()
	}
	if def.Version == "" {
		def.Version = "1.0.0"
	}
	if def.MisfirePolicy.Policy == "" {
		def.MisfirePolicy = DefaultMisfirePolicy()
	}
	if def.OverlapPolicy.Policy == "" {
		def.OverlapPolicy = DefaultOverlapPolicy()
	}
	if def.RetryPolicy.MaxAttempts == 0 {
		def.RetryPolicy = DefaultRetryPolicy()
	}
	if def.JitterPolicy.SeedMode == "" && !def.JitterPolicy.Enabled {
		def.JitterPolicy = DefaultJitterPolicy()
	}
	if def.ConcurrencyPolicy.MaxConcurrentRuns == 0 {
		serializationKey := def.ConcurrencyPolicy.SerializationKey
		def.ConcurrencyPolicy = DefaultConcurrencyPolicy()
		def.ConcurrencyPolicy.SerializationKey = serializationKey
	}
	if def.DSTSpringPolicy == "" {
		def.DSTSpringPolicy = DefaultDSTSpringPolicy()
		if def.Trigger.Type == TriggerTypeCalendar {
			def.DSTSpringPolicy = DSTSpringNextValidTime
		}
	}
	if def.DSTFallPolicy == "" {
		def.DSTFallPolicy = DefaultDSTFallPolicy()
	}
	if def.ExecutionOwner == "" {
		def.ExecutionOwner = ExecutionOwnerBackend
	}
	if def.Timezone == "" {
		def.Timezone = s.config.DefaultTimezone
	}

	now := s.clock.Now()
	def.DefinitionHash = GenerateDefinitionHash(def)
	state := &ScheduleState{
		ScheduleID: def.ScheduleID,
		Enabled:    def.EnabledByDefault,
		Paused:     false,
		Status:     DefinitionStatusCreated,
		Generation: 1,
		UpdatedAt:  now.UTC(),
	}
	result, err := s.calc.CalculateNext(def, nil)
	if err != nil {
		return err
	}
	if def.EnabledByDefault {
		state.Status = DefinitionStatusEnabled
		if result == nil || result.NextScheduledAt == nil {
			return ErrInvalidOneShotTime
		}
		state.NextScheduledAt = result.NextScheduledAt
		state.NextEffectiveAt = result.NextEffectiveAt
	}
	return withScheduleTransaction(ctx, s.store, func(ctx context.Context) error {
		if existing, err := s.store.GetDefinition(ctx, def.ScheduleID); err == nil && existing != nil {
			if existing.DefinitionHash == def.DefinitionHash {
				return nil
			}
			return ErrIdempotencyConflict
		} else if err != nil && !errors.Is(err, ErrScheduleNotFound) {
			return err
		}
		if err := s.store.PutDefinition(ctx, def); err != nil {
			return err
		}
		return s.store.PutState(ctx, state)
	})
}

func (s *ScheduleService) Enable(ctx context.Context, scheduleID string, expectedGeneration int64) error {
	return s.changeState(ctx, scheduleID, expectedGeneration, s.enable)
}

func (s *ScheduleService) enable(ctx context.Context, scheduleID string, expectedGeneration int64) error {
	def, err := s.store.GetDefinition(ctx, scheduleID)
	if err != nil || def == nil {
		return ErrScheduleNotFound
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	if state.Generation != expectedGeneration {
		return ErrGenerationMismatch
	}
	if def.Trigger.MaxOccurrences > 0 {
		count, err := countScheduledTriggers(ctx, s.store, scheduleID)
		if err != nil {
			return err
		}
		if count >= def.Trigger.MaxOccurrences {
			return ErrScheduleExpired
		}
	}
	if !IsValidDefinitionTransition(state.Status, DefinitionStatusEnabled) {
		return fmt.Errorf("%w: %s -> enabled", ErrInvalidStateTransition, state.Status)
	}
	now := s.clock.Now()
	state.Enabled = true
	state.Paused = false
	state.Status = DefinitionStatusEnabled
	result, err := s.calc.CalculateNext(def, state)
	if err != nil {
		return err
	}
	if result == nil || result.NextScheduledAt == nil {
		return ErrScheduleExpired
	}
	if result != nil {
		state.NextScheduledAt = result.NextScheduledAt
		state.NextEffectiveAt = result.NextEffectiveAt
	}
	state.UpdatedAt = now.UTC()
	return s.store.PutState(ctx, state)
}

func (s *ScheduleService) Disable(ctx context.Context, scheduleID string, expectedGeneration int64) error {
	return s.changeState(ctx, scheduleID, expectedGeneration, s.disable)
}

func (s *ScheduleService) disable(ctx context.Context, scheduleID string, expectedGeneration int64) error {
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	if state.Generation != expectedGeneration {
		return ErrGenerationMismatch
	}
	if !IsValidDefinitionTransition(state.Status, DefinitionStatusDisabled) {
		return fmt.Errorf("%w: %s -> disabled", ErrInvalidStateTransition, state.Status)
	}
	now := s.clock.Now()
	state.Enabled = false
	state.Status = DefinitionStatusDisabled
	state.NextScheduledAt = nil
	state.NextEffectiveAt = nil
	state.UpdatedAt = now.UTC()
	return s.store.PutState(ctx, state)
}

func (s *ScheduleService) Pause(ctx context.Context, scheduleID string, expectedGeneration int64) error {
	return s.changeState(ctx, scheduleID, expectedGeneration, s.pause)
}

func (s *ScheduleService) pause(ctx context.Context, scheduleID string, expectedGeneration int64) error {
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	if state.Generation != expectedGeneration {
		return ErrGenerationMismatch
	}
	if !IsValidDefinitionTransition(state.Status, DefinitionStatusPaused) {
		return fmt.Errorf("%w: %s -> paused", ErrInvalidStateTransition, state.Status)
	}
	now := s.clock.Now()
	state.Paused = true
	state.Status = DefinitionStatusPaused
	state.NextScheduledAt = nil
	state.NextEffectiveAt = nil
	state.UpdatedAt = now.UTC()
	return s.store.PutState(ctx, state)
}

func (s *ScheduleService) Resume(ctx context.Context, scheduleID string, expectedGeneration int64) error {
	return s.changeState(ctx, scheduleID, expectedGeneration, s.resume)
}

func (s *ScheduleService) resume(ctx context.Context, scheduleID string, expectedGeneration int64) error {
	def, err := s.store.GetDefinition(ctx, scheduleID)
	if err != nil || def == nil {
		return ErrScheduleNotFound
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	if state.Generation != expectedGeneration {
		return ErrGenerationMismatch
	}
	if def.Trigger.MaxOccurrences > 0 {
		count, err := countScheduledTriggers(ctx, s.store, scheduleID)
		if err != nil {
			return err
		}
		if count >= def.Trigger.MaxOccurrences {
			return ErrScheduleExpired
		}
	}
	if !IsValidDefinitionTransition(state.Status, DefinitionStatusEnabled) {
		return fmt.Errorf("%w: %s -> enabled", ErrInvalidStateTransition, state.Status)
	}
	now := s.clock.Now()
	state.Paused = false
	state.Enabled = true
	state.Status = DefinitionStatusEnabled
	result, err := s.calc.CalculateNext(def, state)
	if err != nil {
		return err
	}
	if result == nil || result.NextScheduledAt == nil {
		return ErrScheduleExpired
	}
	if result != nil {
		state.NextScheduledAt = result.NextScheduledAt
		state.NextEffectiveAt = result.NextEffectiveAt
	}
	state.UpdatedAt = now.UTC()
	return s.store.PutState(ctx, state)
}

func (s *ScheduleService) changeState(ctx context.Context, scheduleID string, expectedGeneration int64, change func(context.Context, string, int64) error) error {
	return withScheduleTransaction(ctx, s.store, func(ctx context.Context) error {
		if err := change(ctx, scheduleID, expectedGeneration); err != nil {
			return err
		}
		state, err := s.store.GetState(ctx, scheduleID)
		if err != nil {
			return err
		}
		state.Generation++
		return s.store.PutState(ctx, state)
	})
}

func (s *ScheduleService) RunNow(ctx context.Context, scheduleID string) (*ScheduleTriggerRecord, error) {
	return s.RunNowVersion(ctx, scheduleID, 0)
}

func (s *ScheduleService) RunNowVersion(ctx context.Context, scheduleID string, expectedGeneration int64) (*ScheduleTriggerRecord, error) {
	var trigger *ScheduleTriggerRecord
	err := withScheduleTransaction(ctx, s.store, func(ctx context.Context) error {
		var err error
		trigger, err = s.runNow(ctx, scheduleID, expectedGeneration)
		return err
	})
	return trigger, err
}

func (s *ScheduleService) runNow(ctx context.Context, scheduleID string, expectedGeneration int64) (*ScheduleTriggerRecord, error) {
	def, err := s.store.GetDefinition(ctx, scheduleID)
	if err != nil || def == nil {
		return nil, ErrScheduleNotFound
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return nil, ErrScheduleNotFound
	}
	if !state.Enabled {
		return nil, ErrScheduleNotEnabled
	}
	if expectedGeneration > 0 && state.Generation != expectedGeneration {
		return nil, ErrGenerationMismatch
	}
	if state.Paused {
		return nil, ErrSchedulePaused
	}
	return s.planner.PlanTriggerContext(ctx, def, state, true)
}

func (s *ScheduleService) SkipNext(ctx context.Context, scheduleID string) error {
	def, err := s.store.GetDefinition(ctx, scheduleID)
	if err != nil || def == nil {
		return ErrScheduleNotFound
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	now := s.clock.Now()
	var skippedAt time.Time
	if state.NextScheduledAt != nil {
		skippedAt = *state.NextScheduledAt
	} else {
		skippedAt = now
	}
	idempotencyKey := GenerateIdempotencyKey(scheduleID, skippedAt, state.Generation)
	trigger := &ScheduleTriggerRecord{
		TriggerID:       "trigger-skip-" + uuid.NewString(),
		ScheduleID:      scheduleID,
		ScheduledAt:     skippedAt.UTC(),
		EffectiveAt:     skippedAt.UTC(),
		IdempotencyKey:  idempotencyKey,
		Status:          RunStatusSkipped,
		Generation:      state.Generation,
		OverlapDecision: "skipped_by_user",
		CreatedAt:       now.UTC(),
		UpdatedAt:       now.UTC(),
	}
	if err := s.store.PutTrigger(ctx, trigger); err != nil {
		return err
	}
	state.LastScheduledAt = &skippedAt
	result, err := s.calc.CalculateNext(def, state)
	if err == nil && result != nil {
		state.NextScheduledAt = result.NextScheduledAt
		state.NextEffectiveAt = result.NextEffectiveAt
	}
	state.UpdatedAt = now.UTC()
	return s.store.PutState(ctx, state)
}

func (s *ScheduleService) Recalculate(ctx context.Context, scheduleID string) error {
	def, err := s.store.GetDefinition(ctx, scheduleID)
	if err != nil || def == nil {
		return ErrScheduleNotFound
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	result, err := s.calc.CalculateNext(def, state)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	if result != nil {
		state.NextScheduledAt = result.NextScheduledAt
		state.NextEffectiveAt = result.NextEffectiveAt
	} else {
		state.Status = DefinitionStatusExpired
		state.NextScheduledAt = nil
		state.NextEffectiveAt = nil
	}
	state.UpdatedAt = now.UTC()
	return s.store.PutState(ctx, state)
}

func (s *ScheduleService) Update(ctx context.Context, scheduleID string, expectedGeneration int64, def *ScheduleContributionDefinition) error {
	if def != nil && def.Target.Type == TargetTypeHostWorkload {
		return ErrScopeDenied
	}
	return s.UpdateHostDefinition(ctx, scheduleID, expectedGeneration, def)
}

func (s *ScheduleService) UpdateHostDefinition(ctx context.Context, scheduleID string, expectedGeneration int64, def *ScheduleContributionDefinition) error {
	return s.UpdateHostDefinitionState(ctx, scheduleID, expectedGeneration, def, nil)
}

func (s *ScheduleService) UpdateHostDefinitionState(ctx context.Context, scheduleID string, expectedGeneration int64, def *ScheduleContributionDefinition, enabled *bool) error {
	return withScheduleTransaction(ctx, s.store, func(ctx context.Context) error {
		return s.update(ctx, scheduleID, expectedGeneration, def, enabled)
	})
}

func (s *ScheduleService) CountScheduledTriggers(ctx context.Context, id string) (int, error) {
	return countScheduledTriggers(ctx, s.store, id)
}

func (s *ScheduleService) PruneHostArchiveHistory(ctx context.Context, id string, expectedGeneration int64, removeIDs map[string]bool) error {
	return withScheduleTransaction(ctx, s.store, func(ctx context.Context) error {
		def, state, err := s.GetSchedule(ctx, id)
		if err != nil {
			return err
		}
		if def.Target.Type != TargetTypeHostWorkload {
			return ErrScopeDenied
		}
		if state.Generation != expectedGeneration {
			return ErrGenerationMismatch
		}
		var input map[string]json.RawMessage
		if err := json.Unmarshal(def.Target.InputTemplate, &input); err != nil {
			return err
		}
		var entries []json.RawMessage
		if len(input["archiveHistory"]) == 0 {
			return nil
		}
		if err := json.Unmarshal(input["archiveHistory"], &entries); err != nil {
			return err
		}
		kept := make([]json.RawMessage, 0, len(entries))
		for _, entry := range entries {
			var receipt struct {
				TriggerID string `json:"triggerId"`
			}
			if err := json.Unmarshal(entry, &receipt); err != nil {
				return err
			}
			if !removeIDs[receipt.TriggerID] {
				kept = append(kept, entry)
			}
		}
		if len(kept) == len(entries) {
			return nil
		}
		input["archiveHistory"], err = json.Marshal(kept)
		if err != nil {
			return err
		}
		def.Target.InputTemplate, err = json.Marshal(input)
		if err != nil {
			return err
		}
		def.DefinitionHash = GenerateDefinitionHash(def)
		return s.store.PutDefinition(ctx, def)
	})
}

func (s *ScheduleService) update(ctx context.Context, scheduleID string, expectedGeneration int64, def *ScheduleContributionDefinition, enabled *bool) error {
	if def == nil {
		return ErrScheduleNotFound
	}
	def.ScheduleID = scheduleID
	existing, err := s.store.GetDefinition(ctx, scheduleID)
	if err != nil || existing == nil {
		return ErrScheduleNotFound
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	if state.Generation != expectedGeneration {
		return ErrGenerationMismatch
	}
	if state.Status == DefinitionStatusUninstalled {
		return ErrScheduleNotFound
	}
	if def.ExtensionID != existing.ExtensionID || def.ModuleID != existing.ModuleID {
		return ErrScopeDenied
	}
	if existing.Target.Type == TargetTypeHostWorkload {
		def.Trigger.RestoredOccurrences = existing.Trigger.RestoredOccurrences
	}
	newHash := GenerateDefinitionHash(def)
	if newHash == existing.DefinitionHash && (enabled == nil || *enabled == state.Enabled) {
		return nil
	}
	def.DefinitionHash = newHash
	if _, err := s.calc.CalculateNext(def, nil); err != nil {
		return err
	}
	now := s.clock.Now()
	state.Generation++
	state.UpdatedAt = now.UTC()
	if enabled != nil {
		if *enabled && state.Status == DefinitionStatusExpired {
			oldTrigger, _ := json.Marshal(existing.Trigger)
			newTrigger, _ := json.Marshal(def.Trigger)
			if bytes.Equal(oldTrigger, newTrigger) && equalTimePointers(existing.EndAt, def.EndAt) {
				return ErrScheduleExpired
			}
		}
		state.Enabled = *enabled
		if !*enabled {
			state.Paused = false
			state.Status = DefinitionStatusDisabled
		}
	}
	state.NextScheduledAt = nil
	state.NextEffectiveAt = nil
	if state.Enabled && !state.Paused {
		if def.Trigger.MaxOccurrences > 0 {
			count, err := countScheduledTriggers(ctx, s.store, scheduleID)
			if err != nil {
				return err
			}
			if count >= def.Trigger.MaxOccurrences {
				return ErrScheduleExpired
			}
		}
		result, err := s.calc.CalculateNext(def, state)
		if err != nil {
			return err
		}
		if result == nil || result.NextScheduledAt == nil {
			return ErrScheduleExpired
		}
		state.NextScheduledAt = result.NextScheduledAt
		state.NextEffectiveAt = result.NextEffectiveAt
		state.Status = DefinitionStatusEnabled
	}
	if err := s.store.PutDefinition(ctx, def); err != nil {
		return err
	}
	return s.store.PutState(ctx, state)
}

func equalTimePointers(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func (s *ScheduleService) Uninstall(ctx context.Context, scheduleID string) error {
	def, err := s.store.GetDefinition(ctx, scheduleID)
	if err != nil {
		return err
	}
	if def == nil {
		return ErrScheduleNotFound
	}
	if def.Target.Type == TargetTypeHostWorkload {
		return ErrScopeDenied
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	now := s.clock.Now()
	state.Enabled = false
	state.Paused = false
	state.Status = DefinitionStatusUninstalled
	state.NextScheduledAt = nil
	state.NextEffectiveAt = nil
	state.UpdatedAt = now.UTC()
	if err := s.store.PutState(ctx, state); err != nil {
		return err
	}
	_ = s.store.DeleteTriggersBySchedule(ctx, scheduleID)
	_ = s.store.DeleteCircuit(ctx, scheduleID)
	_ = s.store.DeleteDefinition(ctx, scheduleID)
	return nil
}

func (s *ScheduleService) GetSchedule(ctx context.Context, scheduleID string) (*ScheduleContributionDefinition, *ScheduleState, error) {
	def, err := s.store.GetDefinition(ctx, scheduleID)
	if err != nil {
		return nil, nil, err
	}
	if def == nil {
		return nil, nil, ErrScheduleNotFound
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil {
		return nil, nil, err
	}
	return def, state, nil
}

func (s *ScheduleService) GetScheduleState(ctx context.Context, scheduleID string) (*ScheduleState, error) {
	return s.store.GetState(ctx, scheduleID)
}

func (s *ScheduleService) ListSchedules(ctx context.Context, extensionID string) ([]*ScheduleContributionDefinition, error) {
	return s.store.ListDefinitions(ctx, extensionID)
}

func (s *ScheduleService) ListAllSchedules(ctx context.Context) ([]*ScheduleContributionDefinition, error) {
	return s.store.ListAllDefinitions(ctx)
}

func (s *ScheduleService) GetRuns(ctx context.Context, scheduleID string, limit int) ([]*ScheduleRunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.store.ListRunsBySchedule(ctx, scheduleID, limit)
}

func (s *ScheduleService) GetTriggers(ctx context.Context, scheduleID string, limit int) ([]*ScheduleTriggerRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.store.ListTriggersBySchedule(ctx, scheduleID, limit)
}

func (s *ScheduleService) GetAllRuns(ctx context.Context, scheduleID string) ([]*ScheduleRunRecord, error) {
	return s.store.ListRunsBySchedule(ctx, scheduleID, -1)
}

func (s *ScheduleService) GetAllTriggers(ctx context.Context, scheduleID string) ([]*ScheduleTriggerRecord, error) {
	return s.store.ListTriggersBySchedule(ctx, scheduleID, -1)
}

func (s *ScheduleService) CleanTriggerRunHistory(ctx context.Context, triggerID string) error {
	store, ok := s.store.(TriggerHistoryCleanupStore)
	if !ok {
		return errors.New("schedule history cleanup is unavailable")
	}
	return store.CleanTriggerRunHistory(ctx, triggerID)
}

func (s *ScheduleService) GuardHostExecution(ctx context.Context, scheduleID string, generation int64, commit func(context.Context) error) error {
	if commit == nil {
		return ErrInvalidTargetType
	}
	return withScheduleTransaction(ctx, s.store, func(ctx context.Context) error {
		def, err := s.store.GetDefinition(ctx, scheduleID)
		if err != nil {
			return err
		}
		if def == nil || def.Target.Type != TargetTypeHostWorkload {
			return ErrScopeDenied
		}
		state, err := s.store.GetState(ctx, scheduleID)
		if err != nil {
			return err
		}
		if state == nil || state.Generation != generation {
			return ErrGenerationMismatch
		}
		if !state.Enabled || state.Status != DefinitionStatusEnabled {
			return ErrScheduleNotEnabled
		}
		if state.Paused {
			return ErrSchedulePaused
		}
		if err := s.store.PutState(ctx, state); err != nil {
			return err
		}
		return commit(ctx)
	})
}

func (s *ScheduleService) SetMaintenance(maintenance func(context.Context) error) {
	s.scanner.SetMaintenance(maintenance)
}

func (s *ScheduleService) WithArchiveTransaction(ctx context.Context, restore func(context.Context) error) error {
	if restore == nil {
		return ErrInvalidTargetType
	}
	return withScheduleTransaction(ctx, s.store, restore)
}

func (s *ScheduleService) GetMisfires(ctx context.Context, scheduleID string, limit int) ([]*ScheduleMisfireRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.store.ListMisfiresBySchedule(ctx, scheduleID, limit)
}

func (s *ScheduleService) GetCircuit(ctx context.Context, scheduleID string) (*ScheduleCircuitRecord, error) {
	return s.store.GetCircuit(ctx, scheduleID)
}

func (s *ScheduleService) ResetCircuit(ctx context.Context, scheduleID string) error {
	return s.circuit.Reset(ctx, scheduleID)
}

func (s *ScheduleService) GetQuarantines(ctx context.Context) ([]*ScheduleQuarantineRecord, error) {
	return s.store.ListQuarantines(ctx)
}

func (s *ScheduleService) Quarantine(ctx context.Context, scheduleID string, reason QuarantineReason, detail string) error {
	now := s.clock.Now()
	record := &ScheduleQuarantineRecord{
		QuarantineID:  "quarantine-" + uuid.NewString(),
		ScheduleID:    scheduleID,
		Reason:        reason,
		Detail:        detail,
		QuarantinedAt: now.UTC(),
	}
	if err := s.store.PutQuarantine(ctx, record); err != nil {
		return err
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err == nil && state != nil {
		state.Status = DefinitionStatusCreated
		state.UpdatedAt = now.UTC()
		_ = s.store.PutState(ctx, state)
	}
	return nil
}

func (s *ScheduleService) DeleteAllByExtension(ctx context.Context, extensionID string) error {
	defs, err := s.store.ListDefinitions(ctx, extensionID)
	if err != nil {
		return err
	}
	for _, def := range defs {
		if def.Target.Type == TargetTypeHostWorkload {
			state, err := s.store.GetState(ctx, def.ScheduleID)
			if err != nil {
				return err
			}
			if state.Status != DefinitionStatusUninstalled {
				if err := s.CancelHostDefinition(ctx, def.ScheduleID, state.Generation); err != nil {
					return err
				}
			}
		} else {
			if err := s.Uninstall(ctx, def.ScheduleID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ScheduleService) GetExecutor() *ScheduleExecutor {
	return s.executor
}

func (s *ScheduleService) CancelHostDefinition(ctx context.Context, id string, generation int64) error {
	return withScheduleTransaction(ctx, s.store, func(ctx context.Context) error {
		def, err := s.store.GetDefinition(ctx, id)
		if err != nil {
			return err
		}
		if def.Target.Type != TargetTypeHostWorkload {
			return ErrScopeDenied
		}
		state, err := s.store.GetState(ctx, id)
		if err != nil {
			return err
		}
		if state.Generation != generation {
			return ErrGenerationMismatch
		}
		state.Enabled = false
		state.Paused = false
		state.Status = DefinitionStatusUninstalled
		state.Generation++
		state.NextScheduledAt = nil
		state.NextEffectiveAt = nil
		state.UpdatedAt = s.clock.Now().UTC()
		return s.store.PutState(ctx, state)
	})
}

func (s *ScheduleService) GetCalculator() *ScheduleCalculator {
	return s.calc
}

func (s *ScheduleService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func scheduleIDFromDef(def *ScheduleContributionDefinition) string {
	return def.ScheduleID
}
