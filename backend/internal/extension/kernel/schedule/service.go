package schedule

import (
	"context"
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
	if def.ScheduleID == "" {
		def.ScheduleID = "sched-" + uuid.NewString()
	}
	if def.DefinitionHash == "" {
		def.DefinitionHash = GenerateDefinitionHash(def)
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
		def.ConcurrencyPolicy = DefaultConcurrencyPolicy()
	}
	if def.DSTSpringPolicy == "" {
		def.DSTSpringPolicy = DefaultDSTSpringPolicy()
	}
	if def.DSTFallPolicy == "" {
		def.DSTFallPolicy = DefaultDSTFallPolicy()
	}
	if def.Timezone == "" {
		def.Timezone = s.config.DefaultTimezone
	}

	now := s.clock.Now()
	if err := s.store.PutDefinition(ctx, def); err != nil {
		return err
	}

	state := &ScheduleState{
		ScheduleID: def.ScheduleID,
		Enabled:    def.EnabledByDefault,
		Paused:     false,
		Status:     DefinitionStatusCreated,
		Generation: 1,
		UpdatedAt:  now.UTC(),
	}
	if def.EnabledByDefault {
		state.Status = DefinitionStatusEnabled
		result, err := s.calc.CalculateNext(def, nil)
		if err == nil && result != nil {
			state.NextScheduledAt = result.NextScheduledAt
			state.NextEffectiveAt = result.NextEffectiveAt
		}
	}
	return s.store.PutState(ctx, state)
}

func (s *ScheduleService) Enable(ctx context.Context, scheduleID string) error {
	def, err := s.store.GetDefinition(ctx, scheduleID)
	if err != nil || def == nil {
		return ErrScheduleNotFound
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	if state.Generation != state.Generation {
		return ErrGenerationMismatch
	}
	if !IsValidDefinitionTransition(state.Status, DefinitionStatusEnabled) {
		return fmt.Errorf("%w: %s -> enabled", ErrInvalidStateTransition, state.Status)
	}
	now := s.clock.Now()
	state.Enabled = true
	state.Paused = false
	state.Status = DefinitionStatusEnabled
	result, err := s.calc.CalculateNext(def, state)
	if err == nil && result != nil {
		state.NextScheduledAt = result.NextScheduledAt
		state.NextEffectiveAt = result.NextEffectiveAt
	}
	state.UpdatedAt = now.UTC()
	return s.store.PutState(ctx, state)
}

func (s *ScheduleService) Disable(ctx context.Context, scheduleID string) error {
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
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

func (s *ScheduleService) Pause(ctx context.Context, scheduleID string) error {
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
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

func (s *ScheduleService) Resume(ctx context.Context, scheduleID string) error {
	def, err := s.store.GetDefinition(ctx, scheduleID)
	if err != nil || def == nil {
		return ErrScheduleNotFound
	}
	state, err := s.store.GetState(ctx, scheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	if !IsValidDefinitionTransition(state.Status, DefinitionStatusEnabled) {
		return fmt.Errorf("%w: %s -> enabled", ErrInvalidStateTransition, state.Status)
	}
	now := s.clock.Now()
	state.Paused = false
	state.Status = DefinitionStatusEnabled
	result, err := s.calc.CalculateNext(def, state)
	if err == nil && result != nil {
		detection, _ := s.misfire.DetectMisfire(ctx, def, state)
		if detection != nil && detection.HasMisfire {
			s.misfire.ApplyMisfirePolicy(ctx, def, detection)
		}
		state.NextScheduledAt = result.NextScheduledAt
		state.NextEffectiveAt = result.NextEffectiveAt
	}
	state.UpdatedAt = now.UTC()
	return s.store.PutState(ctx, state)
}

func (s *ScheduleService) RunNow(ctx context.Context, scheduleID string) (*ScheduleTriggerRecord, error) {
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
	if state.Paused {
		return nil, ErrSchedulePaused
	}
	return s.planner.PlanTrigger(def, state, true)
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
		TriggerID:      "trigger-skip-" + uuid.NewString(),
		ScheduleID:     scheduleID,
		ScheduledAt:    skippedAt.UTC(),
		EffectiveAt:    skippedAt.UTC(),
		IdempotencyKey: idempotencyKey,
		Status:         RunStatusSkipped,
		Generation:     state.Generation,
		OverlapDecision: "skipped_by_user",
		CreatedAt:      now.UTC(),
		UpdatedAt:      now.UTC(),
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

func (s *ScheduleService) Update(ctx context.Context, def *ScheduleContributionDefinition) error {
	existing, err := s.store.GetDefinition(ctx, scheduleIDFromDef(def))
	if err != nil || existing == nil {
		return ErrScheduleNotFound
	}
	state, err := s.store.GetState(ctx, def.ScheduleID)
	if err != nil || state == nil {
		return ErrScheduleNotFound
	}
	newHash := GenerateDefinitionHash(def)
	if newHash == existing.DefinitionHash {
		return nil
	}
	def.DefinitionHash = newHash
	now := s.clock.Now()
	if err := s.store.PutDefinition(ctx, def); err != nil {
		return err
	}
	state.Generation++
	state.UpdatedAt = now.UTC()
	if state.Enabled && !state.Paused {
		result, err := s.calc.CalculateNext(def, state)
		if err == nil && result != nil {
			state.NextScheduledAt = result.NextScheduledAt
			state.NextEffectiveAt = result.NextEffectiveAt
		}
	}
	return s.store.PutState(ctx, state)
}

func (s *ScheduleService) Uninstall(ctx context.Context, scheduleID string) error {
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
	return s.store.DeleteAllByExtension(ctx, extensionID)
}

func (s *ScheduleService) GetExecutor() *ScheduleExecutor {
	return s.executor
}

func (s *ScheduleService) GetCalculator() *ScheduleCalculator {
	return s.calc
}

func scheduleIDFromDef(def *ScheduleContributionDefinition) string {
	return def.ScheduleID
}
