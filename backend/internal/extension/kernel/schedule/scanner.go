package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ScheduleScanner struct {
	store   ScheduleStore
	clock   Clock
	config  ScheduleConfig
	calc    *ScheduleCalculator
	planner *TriggerPlanner

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
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
	now := s.clock.Now()
	states, err := s.store.ListDueStates(s.ctx, now, s.config.ScanBatchSize)
	if err != nil {
		return
	}
	for _, state := range states {
		if state == nil {
			continue
		}
		s.processDueState(state)
	}
}

func (s *ScheduleScanner) processDueState(state *ScheduleState) {
	def, err := s.store.GetDefinition(s.ctx, state.ScheduleID)
	if err != nil || def == nil {
		return
	}
	if def.ExecutionOwner != ExecutionOwnerBackend {
		return
	}
	if def.EndAt != nil && s.clock.Now().After(*def.EndAt) {
		s.handleExpired(state)
		return
	}
	result, err := s.calc.CalculateNext(def, state)
	if err != nil {
		return
	}
	if result == nil || result.NextScheduledAt == nil {
		s.handleExpired(state)
		return
	}
	now := s.clock.Now()
	if result.NextScheduledAt.After(now) {
		s.updateNextRun(state, result)
		return
	}
	trigger, err := s.planner.PlanTrigger(def, state, false)
	if err != nil {
		return
	}
	if trigger == nil {
		nextResult, err := s.calc.CalculateNext(def, state)
		if err == nil && nextResult != nil {
			s.updateNextRun(state, nextResult)
		}
		return
	}
}

func (s *ScheduleScanner) handleExpired(state *ScheduleState) {
	now := s.clock.Now()
	state.Status = DefinitionStatusExpired
	state.NextScheduledAt = nil
	state.NextEffectiveAt = nil
	state.UpdatedAt = now
	_ = s.store.PutState(s.ctx, state)
}

func (s *ScheduleScanner) updateNextRun(state *ScheduleState, result *NextRunResult) {
	now := s.clock.Now()
	state.NextScheduledAt = result.NextScheduledAt
	state.NextEffectiveAt = result.NextEffectiveAt
	state.UpdatedAt = now
	_ = s.store.PutState(s.ctx, state)
}

func (s *ScheduleScanner) ScanOnce(ctx context.Context) (ScanResult, error) {
	prevCtx := s.ctx
	s.ctx = ctx
	defer func() { s.ctx = prevCtx }()
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
		s.processDueState(state)
	}
	return result, nil
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
	now := p.clock.Now()

	result, err := p.calc.CalculateNext(def, state)
	if err != nil {
		return nil, err
	}
	if result == nil || result.NextScheduledAt == nil {
		if !manual {
			return nil, nil
		}
	}

	var scheduledAt time.Time
	if manual {
		scheduledAt = now
	} else if result != nil && result.NextScheduledAt != nil {
		scheduledAt = *result.NextScheduledAt
	} else {
		return nil, nil
	}

	if !manual && def.EndAt != nil && scheduledAt.After(*def.EndAt) {
		return nil, nil
	}

	idempotencyKey := GenerateIdempotencyKey(def.ScheduleID, scheduledAt, state.Generation)

	existing, err := p.store.GetTriggerByIdempotencyKey(p.ctx(), idempotencyKey)
	if err == nil && existing != nil {
		return nil, ErrIdempotencyConflict
	}

	overlapDecision, err := p.checkOverlap(def, state)
	if err != nil {
		return nil, err
	}

	var effectiveAt time.Time
	if result != nil && result.NextEffectiveAt != nil && !manual {
		effectiveAt = *result.NextEffectiveAt
	} else {
		effectiveAt = scheduledAt
	}

	trigger := &ScheduleTriggerRecord{
		TriggerID:       "trigger-" + uuid.NewString(),
		ScheduleID:      def.ScheduleID,
		ScheduledAt:     scheduledAt.UTC(),
		EffectiveAt:     effectiveAt.UTC(),
		IdempotencyKey:  idempotencyKey,
		Status:          RunStatusWaiting,
		Attempt:         0,
		Generation:      state.Generation,
		Manual:          manual,
		OverlapDecision: overlapDecision,
		DSTDecision:     result.DSTDecision,
		CreatedAt:       now.UTC(),
		UpdatedAt:       now.UTC(),
	}

	if err := p.store.PutTrigger(p.ctx(), trigger); err != nil {
		return nil, err
	}

	newState := *state
	newState.LastScheduledAt = &scheduledAt
	newState.NextScheduledAt = nil
	newState.NextEffectiveAt = nil
	newState.UpdatedAt = now.UTC()
	_ = p.store.PutState(p.ctx(), &newState)

	return trigger, nil
}

func (p *TriggerPlanner) ctx() context.Context {
	return context.Background()
}

func (p *TriggerPlanner) checkOverlap(def *ScheduleContributionDefinition, state *ScheduleState) (string, error) {
	if def.OverlapPolicy.Policy == OverlapPolicyAllow {
		return "allowed", nil
	}
	activeCount, err := p.store.CountActiveRuns(p.ctx(), def.ScheduleID)
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
		runs, err := p.store.ListRunsBySchedule(p.ctx(), def.ScheduleID, 10)
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
