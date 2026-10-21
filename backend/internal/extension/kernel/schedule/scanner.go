package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ScheduleScanner struct {
	store   ScheduleStore
	clock   Clock
	config  ScheduleConfig
	calc    *ScheduleCalculator
	planner *TriggerPlanner

	mu                 sync.Mutex
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	running            bool
	maintenance        func(context.Context) error
	maintenanceLast    time.Time
	maintenanceRunning bool
}

func NewScheduleScanner(store ScheduleStore, clock Clock, config ScheduleConfig, calc *ScheduleCalculator, planner *TriggerPlanner) *ScheduleScanner {
	if clock == nil {
		clock = NewRealClock()
	}
	return &ScheduleScanner{
		store:   store,
		clock:   clock,
		config:  config,
		calc:    calc,
		planner: planner,
	}
}

func (s *ScheduleScanner) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return fmt.Errorf("scanner already running")
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	s.wg.Add(1)
	go s.loop()
	return nil
}

func (s *ScheduleScanner) Stop() {
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
	s.wg.Wait()
}

func (s *ScheduleScanner) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.config.ScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.scanOnce()
		}
	}
}

func (s *ScheduleScanner) scanOnce() {
	_, _ = s.ScanOnce(s.ctx)
}

func (s *ScheduleScanner) SetMaintenance(maintenance func(context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maintenance = maintenance
	s.maintenanceLast = time.Time{}
}

func (s *ScheduleScanner) runMaintenance(ctx context.Context) error {
	now := s.clock.Now()
	s.mu.Lock()
	if s.maintenance == nil || s.maintenanceRunning || !s.maintenanceLast.IsZero() && now.Before(s.maintenanceLast.Add(time.Hour)) {
		s.mu.Unlock()
		return nil
	}
	maintenance := s.maintenance
	s.maintenanceRunning = true
	s.mu.Unlock()
	err := maintenance(ctx)
	s.mu.Lock()
	s.maintenanceRunning = false
	if err == nil {
		s.maintenanceLast = now
	}
	s.mu.Unlock()
	return err
}

func (s *ScheduleScanner) processDueState(ctx context.Context, state *ScheduleState) {
	if !state.Enabled || state.Paused || state.Status != DefinitionStatusEnabled {
		return
	}
	def, err := s.store.GetDefinition(ctx, state.ScheduleID)
	if err != nil || def == nil {
		return
	}
	if def.ExecutionOwner != ExecutionOwnerBackend {
		return
	}
	if def.EndAt != nil && s.clock.Now().After(*def.EndAt) {
		s.handleExpired(ctx, state)
		return
	}
	now := s.clock.Now()
	if state.NextScheduledAt == nil || state.NextScheduledAt.After(now) {
		return
	}
	if state.NextEffectiveAt != nil && state.NextEffectiveAt.After(now) {
		return
	}
	trigger, err := s.planner.PlanTriggerContext(ctx, def, state, false)
	if err != nil {
		return
	}
	if trigger == nil {
		nextResult, err := s.calc.CalculateNext(def, state)
		if err == nil && nextResult != nil {
			s.updateNextRun(ctx, state, nextResult)
		}
		return
	}
}

func (s *ScheduleScanner) handleExpired(ctx context.Context, state *ScheduleState) {
	now := s.clock.Now()
	state.Status = DefinitionStatusExpired
	state.NextScheduledAt = nil
	state.NextEffectiveAt = nil
	state.UpdatedAt = now
	_ = s.store.PutState(ctx, state)
}

func (s *ScheduleScanner) updateNextRun(ctx context.Context, state *ScheduleState, result *NextRunResult) {
	now := s.clock.Now()
	state.NextScheduledAt = result.NextScheduledAt
	state.NextEffectiveAt = result.NextEffectiveAt
	state.UpdatedAt = now
	_ = s.store.PutState(ctx, state)
}

func (s *ScheduleScanner) ScanOnce(ctx context.Context) (ScanResult, error) {
	now := s.clock.Now()
	states, err := s.store.ListDueStates(ctx, now, s.config.ScanBatchSize)
	if err != nil {
		return ScanResult{}, err
	}
	result := ScanResult{TotalScanned: len(states)}
	for _, state := range states {
		if state == nil {
			continue
		}
		s.processDueState(ctx, state)
	}
	return result, s.runMaintenance(ctx)
}

type TriggerPlanner struct {
	store  ScheduleStore
	clock  Clock
	config ScheduleConfig
	calc   *ScheduleCalculator

	misfireService *MisfireService
}

func NewTriggerPlanner(store ScheduleStore, clock Clock, config ScheduleConfig, calc *ScheduleCalculator, misfire *MisfireService) *TriggerPlanner {
	if clock == nil {
		clock = NewRealClock()
	}
	return &TriggerPlanner{
		store:          store,
		clock:          clock,
		config:         config,
		calc:           calc,
		misfireService: misfire,
	}
}

func (p *TriggerPlanner) PlanTrigger(def *ScheduleContributionDefinition, state *ScheduleState, manual bool) (*ScheduleTriggerRecord, error) {
	return p.PlanTriggerContext(p.ctx(), def, state, manual)
}

func (p *TriggerPlanner) PlanTriggerContext(ctx context.Context, def *ScheduleContributionDefinition, state *ScheduleState, manual bool) (*ScheduleTriggerRecord, error) {
	var trigger *ScheduleTriggerRecord
	err := withScheduleTransaction(ctx, p.store, func(ctx context.Context) error {
		fresh, err := p.store.GetState(ctx, state.ScheduleID)
		if err != nil {
			return err
		}
		if fresh == nil || fresh.Generation != state.Generation {
			return ErrGenerationMismatch
		}
		current, err := p.store.GetDefinition(ctx, def.ScheduleID)
		if err != nil {
			return err
		}
		trigger, err = p.planTrigger(ctx, current, fresh, manual)
		return err
	})
	return trigger, err
}

func (p *TriggerPlanner) planTrigger(ctx context.Context, def *ScheduleContributionDefinition, state *ScheduleState, manual bool) (*ScheduleTriggerRecord, error) {
	now := p.clock.Now()
	if !state.Enabled || state.Paused || state.Status != DefinitionStatusEnabled {
		return nil, ErrScheduleNotEnabled
	}
	occurrences := 0
	if !manual && def.Trigger.MaxOccurrences > 0 {
		var err error
		occurrences, err = countScheduledTriggers(ctx, p.store, def.ScheduleID)
		if err != nil {
			return nil, err
		}
		if occurrences >= def.Trigger.MaxOccurrences {
			return nil, ErrScheduleExpired
		}
	}

	var scheduledAt time.Time
	if manual {
		scheduledAt = now
	} else if state.NextScheduledAt != nil {
		scheduledAt = *state.NextScheduledAt
	} else {
		return nil, nil
	}

	if !manual && def.EndAt != nil && scheduledAt.After(*def.EndAt) {
		return nil, nil
	}

	idempotencyKey := GenerateIdempotencyKey(def.ScheduleID, scheduledAt, state.Generation)

	existing, err := p.store.GetTriggerByIdempotencyKey(ctx, idempotencyKey)
	if err == nil && existing != nil {
		return nil, ErrIdempotencyConflict
	}

	overlapDecision, err := p.checkOverlapContext(ctx, def, state)
	if err != nil {
		return nil, err
	}

	var effectiveAt time.Time
	if state.NextEffectiveAt != nil && !manual {
		effectiveAt = *state.NextEffectiveAt
	} else {
		effectiveAt = scheduledAt
	}

	trigger := &ScheduleTriggerRecord{
		TriggerID:       "trigger-" + idempotencyKey,
		ScheduleID:      def.ScheduleID,
		ScheduledAt:     scheduledAt.UTC(),
		EffectiveAt:     effectiveAt.UTC(),
		IdempotencyKey:  idempotencyKey,
		Status:          RunStatusWaiting,
		Attempt:         0,
		Generation:      state.Generation,
		Manual:          manual,
		OverlapDecision: overlapDecision,
		CreatedAt:       now.UTC(),
		UpdatedAt:       now.UTC(),
	}

	created, err := createTriggerIfAbsent(ctx, p.store, trigger)
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, ErrIdempotencyConflict
	}

	newState := *state
	if !manual {
		newState.LastScheduledAt = &scheduledAt
		newState.NextScheduledAt = nil
		newState.NextEffectiveAt = nil
		if def.Trigger.Type != TriggerTypeOneShot && (def.Trigger.MaxOccurrences == 0 || occurrences+1 < def.Trigger.MaxOccurrences) {
			result, err := p.calc.CalculateNext(def, &newState)
			if err != nil {
				return nil, err
			}
			if result != nil {
				newState.NextScheduledAt = result.NextScheduledAt
				newState.NextEffectiveAt = result.NextEffectiveAt
			}
		}
	}
	newState.UpdatedAt = now.UTC()
	if err := p.store.PutState(ctx, &newState); err != nil {
		return nil, err
	}

	return trigger, nil
}

func (p *TriggerPlanner) ctx() context.Context {
	return context.Background()
}

func (p *TriggerPlanner) checkOverlap(def *ScheduleContributionDefinition, state *ScheduleState) (string, error) {
	return p.checkOverlapContext(p.ctx(), def, state)
}

func (p *TriggerPlanner) checkOverlapContext(ctx context.Context, def *ScheduleContributionDefinition, state *ScheduleState) (string, error) {
	if def.OverlapPolicy.Policy == OverlapPolicyAllow {
		return "allowed", nil
	}
	activeCount, err := p.store.CountActiveRuns(ctx, def.ScheduleID)
	if err != nil {
		return "", err
	}
	if activeCount == 0 {
		return "no_active_run", nil
	}
	switch def.OverlapPolicy.Policy {
	case OverlapPolicyForbid:
		return "blocked_overlap", ErrOverlapForbidden
	case OverlapPolicySkipIfRunning:
		return "skipped_overlap", nil
	case OverlapPolicyQueueOne:
		return "queued_one", nil
	case OverlapPolicyReplace:
		runs, err := p.store.ListRunsBySchedule(ctx, def.ScheduleID, 10)
		if err != nil {
			return "", err
		}
		for _, run := range runs {
			if run.Status == RunStatusRunning || run.Status == RunStatusTriggering {
				if run.TargetType == TargetTypeTask {
					return "replace_cancellable", nil
				}
				return "blocked_uncancellable", ErrOverlapForbidden
			}
		}
		return "replace_no_active", nil
	default:
		return "blocked_overlap", ErrOverlapForbidden
	}
}
