package schedule

import (
	"context"
	"sync"
	"testing"
	"time"
)

type memStore struct {
	mu          sync.Mutex
	defs        map[string]*ScheduleContributionDefinition
	states      map[string]*ScheduleState
	triggers    map[string]*ScheduleTriggerRecord
	runs        map[string]*ScheduleRunRecord
	misfires    map[string]*ScheduleMisfireRecord
	circuits    map[string]*ScheduleCircuitRecord
	quarantines map[string]*ScheduleQuarantineRecord
	retries     map[string]*ScheduleRetryRecord
}

func newMemStore() *memStore {
	return &memStore{
		defs:        map[string]*ScheduleContributionDefinition{},
		states:      map[string]*ScheduleState{},
		triggers:    map[string]*ScheduleTriggerRecord{},
		runs:        map[string]*ScheduleRunRecord{},
		misfires:    map[string]*ScheduleMisfireRecord{},
		circuits:    map[string]*ScheduleCircuitRecord{},
		quarantines: map[string]*ScheduleQuarantineRecord{},
		retries:     map[string]*ScheduleRetryRecord{},
	}
}

func (s *memStore) PutDefinition(ctx context.Context, def *ScheduleContributionDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defs[def.ScheduleID] = def
	return nil
}

func (s *memStore) GetDefinition(ctx context.Context, scheduleID string) (*ScheduleContributionDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	def, ok := s.defs[scheduleID]
	if !ok {
		return nil, ErrScheduleNotFound
	}
	return def, nil
}

func (s *memStore) ListDefinitions(ctx context.Context, extensionID string) ([]*ScheduleContributionDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []*ScheduleContributionDefinition{}
	for _, def := range s.defs {
		if def.ExtensionID == extensionID {
			result = append(result, def)
		}
	}
	return result, nil
}

func (s *memStore) ListAllDefinitions(ctx context.Context) ([]*ScheduleContributionDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*ScheduleContributionDefinition, 0, len(s.defs))
	for _, def := range s.defs {
		result = append(result, def)
	}
	return result, nil
}

func (s *memStore) DeleteDefinition(ctx context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.defs, scheduleID)
	return nil
}

func (s *memStore) PutState(ctx context.Context, state *ScheduleState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state.ScheduleID] = state
	return nil
}

func (s *memStore) GetState(ctx context.Context, scheduleID string) (*ScheduleState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.states[scheduleID]
	if !ok {
		return nil, ErrScheduleNotFound
	}
	return state, nil
}

func (s *memStore) ListDueStates(ctx context.Context, now time.Time, limit int) ([]*ScheduleState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []*ScheduleState{}
	for _, state := range s.states {
		if state.NextScheduledAt != nil && !state.NextScheduledAt.After(now) {
			result = append(result, state)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memStore) ListStatesByStatus(ctx context.Context, status ScheduleDefinitionStatus) ([]*ScheduleState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []*ScheduleState{}
	for _, state := range s.states {
		if state.Status == status {
			result = append(result, state)
		}
	}
	return result, nil
}

func (s *memStore) PutTrigger(ctx context.Context, record *ScheduleTriggerRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.triggers[record.TriggerID] = record
	return nil
}

func (s *memStore) GetTrigger(ctx context.Context, triggerID string) (*ScheduleTriggerRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trigger, ok := s.triggers[triggerID]
	if !ok {
		return nil, ErrTriggerNotFound
	}
	return trigger, nil
}

func (s *memStore) GetTriggerByIdempotencyKey(ctx context.Context, key string) (*ScheduleTriggerRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, trigger := range s.triggers {
		if trigger.IdempotencyKey == key {
			return trigger, nil
		}
	}
	return nil, ErrTriggerNotFound
}

func (s *memStore) ListTriggersBySchedule(ctx context.Context, scheduleID string, limit int) ([]*ScheduleTriggerRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []*ScheduleTriggerRecord{}
	for _, trigger := range s.triggers {
		if trigger.ScheduleID == scheduleID {
			result = append(result, trigger)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memStore) ListDueTriggers(ctx context.Context, now time.Time, limit int) ([]*ScheduleTriggerRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []*ScheduleTriggerRecord{}
	for _, trigger := range s.triggers {
		if !trigger.ScheduledAt.After(now) && trigger.Status == RunStatusWaiting {
			result = append(result, trigger)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memStore) UpdateTriggerStatus(ctx context.Context, triggerID string, status ScheduleRunStatus, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	trigger, ok := s.triggers[triggerID]
	if !ok {
		return ErrTriggerNotFound
	}
	trigger.Status = status
	trigger.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *memStore) AcquireTriggerLease(ctx context.Context, triggerID string, owner string, expiresAt time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trigger, ok := s.triggers[triggerID]
	if !ok {
		return false, ErrTriggerNotFound
	}
	if trigger.LeaseOwner != nil && trigger.LeaseExpiresAt != nil && trigger.LeaseExpiresAt.After(time.Now().UTC()) {
		return false, nil
	}
	trigger.LeaseOwner = &owner
	trigger.LeaseExpiresAt = &expiresAt
	return true, nil
}

func (s *memStore) ReleaseTriggerLease(ctx context.Context, triggerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	trigger, ok := s.triggers[triggerID]
	if !ok {
		return ErrTriggerNotFound
	}
	trigger.LeaseOwner = nil
	trigger.LeaseExpiresAt = nil
	return nil
}

func (s *memStore) ReclaimExpiredLeases(ctx context.Context, now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, trigger := range s.triggers {
		if trigger.LeaseExpiresAt != nil && trigger.LeaseExpiresAt.Before(now) {
			trigger.LeaseOwner = nil
			trigger.LeaseExpiresAt = nil
			count++
		}
	}
	return count, nil
}

func (s *memStore) DeleteTriggersBySchedule(ctx context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, trigger := range s.triggers {
		if trigger.ScheduleID == scheduleID {
			delete(s.triggers, id)
		}
	}
	return nil
}

func (s *memStore) PutRun(ctx context.Context, run *ScheduleRunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.RunID] = run
	return nil
}

func (s *memStore) GetRun(ctx context.Context, runID string) (*ScheduleRunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok {
		return nil, ErrTriggerNotFound
	}
	return run, nil
}

func (s *memStore) ListRunsBySchedule(ctx context.Context, scheduleID string, limit int) ([]*ScheduleRunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []*ScheduleRunRecord{}
	for _, run := range s.runs {
		if run.ScheduleID == scheduleID {
			result = append(result, run)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memStore) ListRunsByTrigger(ctx context.Context, triggerID string) ([]*ScheduleRunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []*ScheduleRunRecord{}
	for _, run := range s.runs {
		if run.TriggerID == triggerID {
			result = append(result, run)
		}
	}
	return result, nil
}

func (s *memStore) UpdateRunStatus(ctx context.Context, runID string, status ScheduleRunStatus, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok {
		return ErrTriggerNotFound
	}
	run.Status = status
	run.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *memStore) CountActiveRuns(ctx context.Context, scheduleID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, run := range s.runs {
		if run.ScheduleID == scheduleID && run.Status.IsActive() {
			count++
		}
	}
	return count, nil
}

func (s *memStore) CountActiveRunsByExtension(ctx context.Context, extensionID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, def := range s.defs {
		if def.ExtensionID != extensionID {
			continue
		}
		for _, run := range s.runs {
			if run.ScheduleID == def.ScheduleID && run.Status.IsActive() {
				count++
			}
		}
	}
	return count, nil
}

func (s *memStore) PutMisfire(ctx context.Context, record *ScheduleMisfireRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.misfires[record.MisfireID] = record
	return nil
}

func (s *memStore) ListMisfiresBySchedule(ctx context.Context, scheduleID string, limit int) ([]*ScheduleMisfireRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []*ScheduleMisfireRecord{}
	for _, record := range s.misfires {
		if record.ScheduleID == scheduleID {
			result = append(result, record)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memStore) PutRetry(ctx context.Context, record *ScheduleRetryRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retries[record.RetryID] = record
	return nil
}

func (s *memStore) ListDueRetries(ctx context.Context, now time.Time, limit int) ([]*ScheduleRetryRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []*ScheduleRetryRecord{}
	for _, record := range s.retries {
		if !record.AvailableAt.After(now) {
			result = append(result, record)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memStore) DeleteRetry(ctx context.Context, retryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.retries, retryID)
	return nil
}

func (s *memStore) GetCircuit(ctx context.Context, scheduleID string) (*ScheduleCircuitRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.circuits[scheduleID]
	if !ok {
		return nil, nil
	}
	return record, nil
}

func (s *memStore) PutCircuit(ctx context.Context, record *ScheduleCircuitRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.circuits[record.ScheduleID] = record
	return nil
}

func (s *memStore) DeleteCircuit(ctx context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.circuits, scheduleID)
	return nil
}

func (s *memStore) PutQuarantine(ctx context.Context, record *ScheduleQuarantineRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quarantines[record.QuarantineID] = record
	return nil
}

func (s *memStore) ListQuarantines(ctx context.Context) ([]*ScheduleQuarantineRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*ScheduleQuarantineRecord, 0, len(s.quarantines))
	for _, record := range s.quarantines {
		result = append(result, record)
	}
	return result, nil
}

func (s *memStore) DeleteAllByExtension(ctx context.Context, extensionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, def := range s.defs {
		if def.ExtensionID == extensionID {
			delete(s.defs, id)
			delete(s.states, id)
		}
	}
	return nil
}

func makeCronDef() *ScheduleContributionDefinition {
	return &ScheduleContributionDefinition{
		ContributionID: "contrib-1",
		ExtensionID:    "com.example",
		ModuleID:       "main",
		ScheduleID:     "sched-1",
		Name:           "test schedule",
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeCron,
			Cron: &CronTriggerDefinition{
				Expression: "*/5 * * * *",
			},
		},
		Target: ScheduleTargetDefinition{
			Type:            TargetTypeTool,
			TargetID:        "tool-1",
			IdempotencyMode: IdempotencyModeIdempotent,
		},
		Timezone: "UTC",
		Version:  "1",
	}
}

func TestFakeClock(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	if !clock.Now().Equal(base) {
		t.Fatalf("expected %v, got %v", base, clock.Now())
	}
	advanced := clock.Advance(5 * time.Minute)
	expected := base.Add(5 * time.Minute)
	if !advanced.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, advanced)
	}
	if !clock.Now().Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, clock.Now())
	}
}

func TestCalculateNextCron(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)
	def := makeCronDef()

	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextScheduledAt == nil {
		t.Fatal("expected next scheduled time")
	}
	expected := base.Add(5 * time.Minute)
	if !result.NextScheduledAt.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, *result.NextScheduledAt)
	}
	if result.CalculationReason != "calculated" {
		t.Fatalf("expected calculated, got %s", result.CalculationReason)
	}
}

func TestCalculateNextCronStartAtFuture(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)
	def := makeCronDef()
	startAt := base.Add(1 * time.Hour)
	def.StartAt = &startAt

	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextScheduledAt == nil {
		t.Fatal("expected next scheduled time")
	}
	if !result.NextScheduledAt.Equal(startAt) {
		t.Fatalf("expected %v, got %v", startAt, *result.NextScheduledAt)
	}
	if result.CalculationReason != "before_start_at" {
		t.Fatalf("expected before_start_at, got %s", result.CalculationReason)
	}
}

func TestCalculateNextCronEndAtPast(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)
	def := makeCronDef()
	endAt := base.Add(-1 * time.Hour)
	def.EndAt = &endAt

	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextScheduledAt != nil {
		t.Fatalf("expected nil, got %v", *result.NextScheduledAt)
	}
	if result.CalculationReason != "after_end_at" {
		t.Fatalf("expected after_end_at, got %s", result.CalculationReason)
	}
}

func TestCalculateNextInterval(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)
	anchor := base.Add(-1 * time.Hour)
	def := &ScheduleContributionDefinition{
		ScheduleID: "sched-interval",
		Name:       "interval test",
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeInterval,
			Interval: &IntervalTriggerDefinition{
				Interval: time.Minute,
				AnchorAt: anchor,
			},
		},
		Target: ScheduleTargetDefinition{
			Type:            TargetTypeTool,
			TargetID:        "tool-1",
			IdempotencyMode: IdempotencyModeIdempotent,
		},
		Timezone: "UTC",
		Version:  "1",
	}

	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextScheduledAt == nil {
		t.Fatal("expected next scheduled time")
	}
	expected := base.Add(1 * time.Minute)
	if !result.NextScheduledAt.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, *result.NextScheduledAt)
	}
}

func TestCalculateNextOneShotFuture(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)
	runAt := base.Add(1 * time.Hour)
	def := &ScheduleContributionDefinition{
		ScheduleID: "sched-oneshot",
		Name:       "oneshot test",
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeOneShot,
			OneShot: &OneShotTriggerDefinition{
				RunAt: runAt,
			},
		},
		Target: ScheduleTargetDefinition{
			Type:            TargetTypeTool,
			TargetID:        "tool-1",
			IdempotencyMode: IdempotencyModeIdempotent,
		},
		Timezone: "UTC",
		Version:  "1",
	}

	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextScheduledAt == nil {
		t.Fatal("expected next scheduled time")
	}
	if !result.NextScheduledAt.Equal(runAt) {
		t.Fatalf("expected %v, got %v", runAt, *result.NextScheduledAt)
	}
}

func TestCalculateNextOneShotPast(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)
	runAt := base.Add(-1 * time.Hour)
	def := &ScheduleContributionDefinition{
		ScheduleID: "sched-oneshot-past",
		Name:       "oneshot past test",
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeOneShot,
			OneShot: &OneShotTriggerDefinition{
				RunAt: runAt,
			},
		},
		Target: ScheduleTargetDefinition{
			Type:            TargetTypeTool,
			TargetID:        "tool-1",
			IdempotencyMode: IdempotencyModeIdempotent,
		},
		Timezone: "UTC",
		Version:  "1",
	}

	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextScheduledAt != nil {
		t.Fatalf("expected nil, got %v", *result.NextScheduledAt)
	}
	if result.CalculationReason != "no_next_run" {
		t.Fatalf("expected no_next_run, got %s", result.CalculationReason)
	}
}

func TestIsValidDefinitionTransition(t *testing.T) {
	tests := []struct {
		name string
		from ScheduleDefinitionStatus
		to   ScheduleDefinitionStatus
		want bool
	}{
		{"created to enabled", DefinitionStatusCreated, DefinitionStatusEnabled, true},
		{"enabled to paused", DefinitionStatusEnabled, DefinitionStatusPaused, true},
		{"paused to enabled", DefinitionStatusPaused, DefinitionStatusEnabled, true},
		{"enabled to disabled", DefinitionStatusEnabled, DefinitionStatusDisabled, true},
		{"disabled to enabled", DefinitionStatusDisabled, DefinitionStatusEnabled, true},
		{"created to expired", DefinitionStatusCreated, DefinitionStatusExpired, false},
		{"expired to enabled", DefinitionStatusExpired, DefinitionStatusEnabled, false},
		{"uninstalled to enabled", DefinitionStatusUninstalled, DefinitionStatusEnabled, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidDefinitionTransition(tt.from, tt.to)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestDetectMisfireNilLastScheduled(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	store := newMemStore()
	svc := NewMisfireService(store, clock)
	def := makeCronDef()
	state := &ScheduleState{
		ScheduleID:      "sched-1",
		LastScheduledAt: nil,
	}

	detection, err := svc.DetectMisfire(context.Background(), def, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detection.HasMisfire {
		t.Fatal("expected no misfire when last scheduled is nil")
	}
}

func TestDetectMisfireFutureLastScheduled(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	store := newMemStore()
	svc := NewMisfireService(store, clock)
	def := makeCronDef()
	future := base.Add(1 * time.Hour)
	state := &ScheduleState{
		ScheduleID:      "sched-1",
		LastScheduledAt: &future,
	}

	detection, err := svc.DetectMisfire(context.Background(), def, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detection.HasMisfire {
		t.Fatal("expected no misfire when last scheduled is in the future")
	}
}

func TestDetectMisfirePastLastScheduled(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	store := newMemStore()
	svc := NewMisfireService(store, clock)
	def := makeCronDef()
	past := base.Add(-1 * time.Hour)
	state := &ScheduleState{
		ScheduleID:      "sched-1",
		LastScheduledAt: &past,
		Generation:      1,
	}

	detection, err := svc.DetectMisfire(context.Background(), def, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detection.HasMisfire {
		t.Fatalf("expected misfire detected, got %+v", detection)
	}
	if detection.MissedCount <= 0 {
		t.Fatalf("expected missed count > 0, got %d", detection.MissedCount)
	}
	if detection.EarliestMissed == nil {
		t.Fatal("expected earliest missed time")
	}
	if detection.LatestMissed == nil {
		t.Fatal("expected latest missed time")
	}
}

func TestGenerateDefinitionHash(t *testing.T) {
	def1 := makeCronDef()
	def2 := makeCronDef()
	hash1 := GenerateDefinitionHash(def1)
	hash2 := GenerateDefinitionHash(def2)
	if hash1 != hash2 {
		t.Fatalf("expected same hash for identical definitions, got %s and %s", hash1, hash2)
	}
	if hash1 == "" {
		t.Fatal("expected non-empty hash")
	}

	def2.Name = "different name"
	hash3 := GenerateDefinitionHash(def2)
	if hash1 == hash3 {
		t.Fatal("expected different hash for different definitions")
	}
}
