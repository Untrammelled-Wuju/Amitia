package extension

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

type memScheduleStore struct {
	mu             sync.RWMutex
	definitions    map[string]*schedule.ScheduleContributionDefinition
	states         map[string]*schedule.ScheduleState
	triggers       map[string]*schedule.ScheduleTriggerRecord
	triggersByIdem map[string]*schedule.ScheduleTriggerRecord
	runs           map[string]*schedule.ScheduleRunRecord
	misfires       map[string]*schedule.ScheduleMisfireRecord
	retries        map[string]*schedule.ScheduleRetryRecord
	circuits       map[string]*schedule.ScheduleCircuitRecord
	quarantines    map[string]*schedule.ScheduleQuarantineRecord
}

var _ schedule.ScheduleStore = (*memScheduleStore)(nil)

func newMemScheduleStore() *memScheduleStore {
	return &memScheduleStore{
		definitions:    make(map[string]*schedule.ScheduleContributionDefinition),
		states:         make(map[string]*schedule.ScheduleState),
		triggers:       make(map[string]*schedule.ScheduleTriggerRecord),
		triggersByIdem: make(map[string]*schedule.ScheduleTriggerRecord),
		runs:           make(map[string]*schedule.ScheduleRunRecord),
		misfires:       make(map[string]*schedule.ScheduleMisfireRecord),
		retries:        make(map[string]*schedule.ScheduleRetryRecord),
		circuits:       make(map[string]*schedule.ScheduleCircuitRecord),
		quarantines:    make(map[string]*schedule.ScheduleQuarantineRecord),
	}
}

func (s *memScheduleStore) PutDefinition(_ context.Context, def *schedule.ScheduleContributionDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *def
	s.definitions[def.ScheduleID] = &cp
	return nil
}

func (s *memScheduleStore) GetDefinition(_ context.Context, scheduleID string) (*schedule.ScheduleContributionDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	def, ok := s.definitions[scheduleID]
	if !ok {
		return nil, nil
	}
	cp := *def
	return &cp, nil
}

func (s *memScheduleStore) ListDefinitions(_ context.Context, extensionID string) ([]*schedule.ScheduleContributionDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleContributionDefinition, 0)
	for _, def := range s.definitions {
		if def.ExtensionID == extensionID {
			cp := *def
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ScheduleID < result[j].ScheduleID
	})
	return result, nil
}

func (s *memScheduleStore) ListAllDefinitions(_ context.Context) ([]*schedule.ScheduleContributionDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleContributionDefinition, 0, len(s.definitions))
	for _, def := range s.definitions {
		cp := *def
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ScheduleID < result[j].ScheduleID
	})
	return result, nil
}

func (s *memScheduleStore) DeleteDefinition(_ context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.definitions, scheduleID)
	return nil
}

func (s *memScheduleStore) PutState(_ context.Context, state *schedule.ScheduleState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *state
	s.states[state.ScheduleID] = &cp
	return nil
}

func (s *memScheduleStore) GetState(_ context.Context, scheduleID string) (*schedule.ScheduleState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.states[scheduleID]
	if !ok {
		return nil, nil
	}
	cp := *state
	return &cp, nil
}

func (s *memScheduleStore) ListDueStates(_ context.Context, now time.Time, limit int) ([]*schedule.ScheduleState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleState, 0)
	for _, state := range s.states {
		if state.NextScheduledAt != nil && !state.NextScheduledAt.After(now) {
			cp := *state
			result = append(result, &cp)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ScheduleID < result[j].ScheduleID
	})
	return result, nil
}

func (s *memScheduleStore) ListStatesByStatus(_ context.Context, status schedule.ScheduleDefinitionStatus) ([]*schedule.ScheduleState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleState, 0)
	for _, state := range s.states {
		if state.Status == status {
			cp := *state
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ScheduleID < result[j].ScheduleID
	})
	return result, nil
}

func (s *memScheduleStore) PutTrigger(_ context.Context, record *schedule.ScheduleTriggerRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *record
	s.triggers[record.TriggerID] = &cp
	if record.IdempotencyKey != "" {
		s.triggersByIdem[record.IdempotencyKey] = &cp
	}
	return nil
}

func (s *memScheduleStore) GetTrigger(_ context.Context, triggerID string) (*schedule.ScheduleTriggerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	trigger, ok := s.triggers[triggerID]
	if !ok {
		return nil, nil
	}
	cp := *trigger
	return &cp, nil
}

func (s *memScheduleStore) GetTriggerByIdempotencyKey(_ context.Context, key string) (*schedule.ScheduleTriggerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	trigger, ok := s.triggersByIdem[key]
	if !ok {
		return nil, nil
	}
	cp := *trigger
	return &cp, nil
}

func (s *memScheduleStore) ListTriggersBySchedule(_ context.Context, scheduleID string, limit int) ([]*schedule.ScheduleTriggerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleTriggerRecord, 0)
	for _, trigger := range s.triggers {
		if trigger.ScheduleID == scheduleID {
			cp := *trigger
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *memScheduleStore) ListDueTriggers(_ context.Context, now time.Time, limit int) ([]*schedule.ScheduleTriggerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleTriggerRecord, 0)
	for _, trigger := range s.triggers {
		if trigger.Status == schedule.RunStatusWaiting && !trigger.ScheduledAt.After(now) {
			cp := *trigger
			result = append(result, &cp)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memScheduleStore) UpdateTriggerStatus(_ context.Context, triggerID string, status schedule.ScheduleRunStatus, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	trigger, ok := s.triggers[triggerID]
	if !ok {
		return schedule.ErrTriggerNotFound
	}
	trigger.Status = status
	trigger.UpdatedAt = time.Now().UTC()
	for k, v := range updates {
		switch k {
		case "triggeredAt":
			if t, ok := v.(time.Time); ok {
				trigger.TriggeredAt = &t
			}
		case "leaseOwner":
			if o, ok := v.(string); ok {
				trigger.LeaseOwner = &o
			}
		case "leaseExpiresAt":
			if t, ok := v.(time.Time); ok {
				trigger.LeaseExpiresAt = &t
			}
		case "errorCode":
			if c, ok := v.(string); ok {
				trigger.ErrorCode = &c
			}
		case "errorMessage":
			if m, ok := v.(string); ok {
				trigger.ErrorMessage = &m
			}
		case "attempt":
			if a, ok := v.(int); ok {
				trigger.Attempt = a
			}
		}
	}
	return nil
}

func (s *memScheduleStore) AcquireTriggerLease(_ context.Context, triggerID string, owner string, expiresAt time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trigger, ok := s.triggers[triggerID]
	if !ok {
		return false, schedule.ErrTriggerNotFound
	}
	if trigger.LeaseOwner != nil && trigger.LeaseExpiresAt != nil && trigger.LeaseExpiresAt.After(time.Now()) {
		return false, nil
	}
	ownerCopy := owner
	trigger.LeaseOwner = &ownerCopy
	expCopy := expiresAt
	trigger.LeaseExpiresAt = &expCopy
	trigger.Status = schedule.RunStatusLeased
	return true, nil
}

func (s *memScheduleStore) ReleaseTriggerLease(_ context.Context, triggerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	trigger, ok := s.triggers[triggerID]
	if !ok {
		return schedule.ErrTriggerNotFound
	}
	trigger.LeaseOwner = nil
	trigger.LeaseExpiresAt = nil
	return nil
}

func (s *memScheduleStore) ReclaimExpiredLeases(_ context.Context, now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, trigger := range s.triggers {
		if trigger.LeaseExpiresAt != nil && trigger.LeaseExpiresAt.Before(now) {
			trigger.LeaseOwner = nil
			trigger.LeaseExpiresAt = nil
			trigger.Status = schedule.RunStatusWaiting
			count++
		}
	}
	return count, nil
}

func (s *memScheduleStore) DeleteTriggersBySchedule(_ context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, trigger := range s.triggers {
		if trigger.ScheduleID == scheduleID {
			delete(s.triggersByIdem, trigger.IdempotencyKey)
			delete(s.triggers, id)
		}
	}
	return nil
}

func (s *memScheduleStore) PutRun(_ context.Context, run *schedule.ScheduleRunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *run
	s.runs[run.RunID] = &cp
	return nil
}

func (s *memScheduleStore) GetRun(_ context.Context, runID string) (*schedule.ScheduleRunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[runID]
	if !ok {
		return nil, nil
	}
	cp := *run
	return &cp, nil
}

func (s *memScheduleStore) ListRunsBySchedule(_ context.Context, scheduleID string, limit int) ([]*schedule.ScheduleRunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleRunRecord, 0)
	for _, run := range s.runs {
		if run.ScheduleID == scheduleID {
			cp := *run
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *memScheduleStore) ListRunsByTrigger(_ context.Context, triggerID string) ([]*schedule.ScheduleRunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleRunRecord, 0)
	for _, run := range s.runs {
		if run.TriggerID == triggerID {
			cp := *run
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *memScheduleStore) UpdateRunStatus(_ context.Context, runID string, status schedule.ScheduleRunStatus, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok {
		return schedule.ErrTriggerNotFound
	}
	run.Status = status
	run.UpdatedAt = time.Now().UTC()
	for k, v := range updates {
		switch k {
		case "finishedAt":
			if t, ok := v.(time.Time); ok {
				run.FinishedAt = &t
			}
		case "errorCode":
			if c, ok := v.(string); ok {
				run.ErrorCode = &c
			}
		case "errorMessage":
			if m, ok := v.(string); ok {
				run.ErrorMessage = &m
			}
		case "attempt":
			if a, ok := v.(int); ok {
				run.Attempt = a
			}
		}
	}
	return nil
}

func (s *memScheduleStore) CountActiveRuns(_ context.Context, scheduleID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, run := range s.runs {
		if run.ScheduleID == scheduleID && run.Status.IsActive() {
			count++
		}
	}
	return count, nil
}

func (s *memScheduleStore) CountActiveRunsByExtension(_ context.Context, extensionID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	scheduleIDs := make(map[string]bool)
	for _, def := range s.definitions {
		if def.ExtensionID == extensionID {
			scheduleIDs[def.ScheduleID] = true
		}
	}
	count := 0
	for _, run := range s.runs {
		if scheduleIDs[run.ScheduleID] && run.Status.IsActive() {
			count++
		}
	}
	return count, nil
}

func (s *memScheduleStore) PutMisfire(_ context.Context, record *schedule.ScheduleMisfireRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *record
	s.misfires[record.MisfireID] = &cp
	return nil
}

func (s *memScheduleStore) ListMisfiresBySchedule(_ context.Context, scheduleID string, limit int) ([]*schedule.ScheduleMisfireRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleMisfireRecord, 0)
	for _, rec := range s.misfires {
		if rec.ScheduleID == scheduleID {
			cp := *rec
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].DetectedAt.After(result[j].DetectedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *memScheduleStore) PutRetry(_ context.Context, record *schedule.ScheduleRetryRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *record
	s.retries[record.RetryID] = &cp
	return nil
}

func (s *memScheduleStore) ListDueRetries(_ context.Context, now time.Time, limit int) ([]*schedule.ScheduleRetryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleRetryRecord, 0)
	for _, rec := range s.retries {
		if !rec.AvailableAt.After(now) {
			cp := *rec
			result = append(result, &cp)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *memScheduleStore) DeleteRetry(_ context.Context, retryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.retries, retryID)
	return nil
}

func (s *memScheduleStore) GetCircuit(_ context.Context, scheduleID string) (*schedule.ScheduleCircuitRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.circuits[scheduleID]
	if !ok {
		return nil, nil
	}
	cp := *rec
	return &cp, nil
}

func (s *memScheduleStore) PutCircuit(_ context.Context, record *schedule.ScheduleCircuitRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *record
	s.circuits[record.ScheduleID] = &cp
	return nil
}

func (s *memScheduleStore) DeleteCircuit(_ context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.circuits, scheduleID)
	return nil
}

func (s *memScheduleStore) PutQuarantine(_ context.Context, record *schedule.ScheduleQuarantineRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *record
	s.quarantines[record.QuarantineID] = &cp
	return nil
}

func (s *memScheduleStore) ListQuarantines(_ context.Context) ([]*schedule.ScheduleQuarantineRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*schedule.ScheduleQuarantineRecord, 0, len(s.quarantines))
	for _, rec := range s.quarantines {
		cp := *rec
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].QuarantinedAt.After(result[j].QuarantinedAt)
	})
	return result, nil
}

func (s *memScheduleStore) DeleteAllByExtension(_ context.Context, extensionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	scheduleIDs := make(map[string]bool)
	for id, def := range s.definitions {
		if def.ExtensionID == extensionID {
			scheduleIDs[id] = true
			delete(s.definitions, id)
		}
	}
	for id := range scheduleIDs {
		delete(s.states, id)
		delete(s.circuits, id)
	}
	for trigID, trigger := range s.triggers {
		if scheduleIDs[trigger.ScheduleID] {
			delete(s.triggersByIdem, trigger.IdempotencyKey)
			delete(s.triggers, trigID)
		}
	}
	for runID, run := range s.runs {
		if scheduleIDs[run.ScheduleID] {
			delete(s.runs, runID)
		}
	}
	for mfID, mf := range s.misfires {
		if scheduleIDs[mf.ScheduleID] {
			delete(s.misfires, mfID)
		}
	}
	for rID, r := range s.retries {
		if scheduleIDs[r.ScheduleID] {
			delete(s.retries, rID)
		}
	}
	return nil
}
