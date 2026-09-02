package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MisfireService struct {
	store ScheduleStore
	clock Clock
}

func NewMisfireService(store ScheduleStore, clock Clock) *MisfireService {
	if clock == nil {
		clock = NewRealClock()
	}
	return &MisfireService{store: store, clock: clock}
}

type MisfireDetection struct {
	HasMisfire     bool
	MissedCount    int
	MissedTimes    []time.Time
	EarliestMissed *time.Time
	LatestMissed   *time.Time
	Generation     int64
}

func (m *MisfireService) DetectMisfire(ctx context.Context, def *ScheduleContributionDefinition, state *ScheduleState) (*MisfireDetection, error) {
	now := m.clock.Now()
	if def == nil || state == nil {
		return &MisfireDetection{HasMisfire: false}, nil
	}

	missed := []time.Time{}
	var current time.Time
	// next_scheduled_at is the durable cursor for an occurrence that was due
	// while the scheduler was stopped/paused. This also covers the very first
	// one-shot occurrence, where LastScheduledAt is intentionally nil.
	if state.NextScheduledAt != nil && state.NextScheduledAt.Before(now) {
		current = state.NextScheduledAt.UTC()
		missed = append(missed, current)
		if def.Trigger.Type == TriggerTypeOneShot {
			return &MisfireDetection{
				HasMisfire: true, MissedCount: 1, MissedTimes: missed,
				EarliestMissed: &missed[0], LatestMissed: &missed[0], Generation: state.Generation,
			}, nil
		}
	} else if state.LastScheduledAt != nil {
		lastScheduled := state.LastScheduledAt.UTC()
		if !lastScheduled.Before(now) {
			return &MisfireDetection{HasMisfire: false, Generation: state.Generation}, nil
		}
		current = lastScheduled
	} else {
		return &MisfireDetection{HasMisfire: false, Generation: state.Generation}, nil
	}

	iterClock := NewFakeClock(current)
	calc := NewScheduleCalculator(iterClock)
	for i := 0; i < 1000; i++ {
		iterClock.Set(current)
		result, err := calc.CalculateNext(def, &ScheduleState{
			LastScheduledAt: &current,
			Generation:      state.Generation,
		})
		if err != nil || result == nil || result.NextScheduledAt == nil {
			break
		}
		next := result.NextScheduledAt.UTC()
		if !next.After(current) || !next.Before(now) {
			break
		}
		if def.EndAt != nil && next.After(*def.EndAt) {
			break
		}
		missed = append(missed, next)
		current = next
	}

	if len(missed) == 0 {
		return &MisfireDetection{HasMisfire: false, Generation: state.Generation}, nil
	}

	return &MisfireDetection{
		HasMisfire:     true,
		MissedCount:    len(missed),
		MissedTimes:    missed,
		EarliestMissed: &missed[0],
		LatestMissed:   &missed[len(missed)-1],
		Generation:     state.Generation,
	}, nil
}

type MisfireActionResult struct {
	Action       string
	FireCount    int
	FireTimes    []time.Time
	SkippedCount int
	SkipAll      bool
	Reschedule   bool
}

func (m *MisfireService) ApplyMisfirePolicy(ctx context.Context, def *ScheduleContributionDefinition, detection *MisfireDetection) (*MisfireActionResult, error) {
	if detection == nil || !detection.HasMisfire {
		return &MisfireActionResult{Action: "no_misfire"}, nil
	}

	policy := def.MisfirePolicy
	if policy.Policy == "" {
		policy = DefaultMisfirePolicy()
	}

	maxCatchUp := policy.MaxCatchUp
	if maxCatchUp <= 0 {
		maxCatchUp = 3
	}

	result := &MisfireActionResult{}

	switch policy.Policy {
	case MisfirePolicySkip:
		result.Action = "skip_all"
		result.SkippedCount = detection.MissedCount
		result.SkipAll = true
		m.recordMisfire(ctx, def, detection, "skip", "skip_all", detection.MissedCount)

	case MisfirePolicyFireOnce:
		if len(detection.MissedTimes) > 0 {
			result.Action = "fire_once"
			result.FireCount = 1
			result.FireTimes = []time.Time{detection.LatestMissed.UTC()}
			m.recordMisfire(ctx, def, detection, "fire_once", "fire_once", detection.MissedCount-1)
		}

	case MisfirePolicyCatchUpLimited:
		count := detection.MissedCount
		if count > maxCatchUp {
			count = maxCatchUp
		}
		result.Action = "catch_up_limited"
		result.FireCount = count
		start := len(detection.MissedTimes) - count
		if start < 0 {
			start = 0
		}
		result.FireTimes = detection.MissedTimes[start:]
		result.SkippedCount = detection.MissedCount - count
		m.recordMisfire(ctx, def, detection, "catch_up_limited", fmt.Sprintf("catch_up_%d_of_%d", count, detection.MissedCount), detection.MissedCount-count)

	case MisfirePolicyRescheduleFromNow:
		result.Action = "reschedule_from_now"
		result.Reschedule = true
		result.SkippedCount = detection.MissedCount
		m.recordMisfire(ctx, def, detection, "reschedule_from_now", "reschedule", detection.MissedCount)

	default:
		result.Action = "fire_once"
		result.FireCount = 1
		if detection.LatestMissed != nil {
			result.FireTimes = []time.Time{detection.LatestMissed.UTC()}
		}
		m.recordMisfire(ctx, def, detection, "fire_once", "default_fire_once", detection.MissedCount-1)
	}

	if len(result.FireTimes) > 0 {
		if err := m.materializeCatchUpTriggers(ctx, def, detection, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (m *MisfireService) materializeCatchUpTriggers(ctx context.Context, def *ScheduleContributionDefinition, detection *MisfireDetection, result *MisfireActionResult) error {
	if m == nil || m.store == nil || def == nil || detection == nil || result == nil {
		return nil
	}
	now := m.clock.Now().UTC()
	for _, fireAt := range result.FireTimes {
		scheduledAt := fireAt.UTC()
		idempotencyKey := GenerateIdempotencyKey(def.ScheduleID, scheduledAt, detection.Generation)
		// TriggerID is deterministic as a second line of defence against two
		// recovery workers materializing the same historical occurrence. The
		// workflow target receives the same idempotency key as well.
		trigger := &ScheduleTriggerRecord{
			TriggerID:       "trigger-misfire-" + idempotencyKey,
			ScheduleID:      def.ScheduleID,
			ScheduledAt:     scheduledAt,
			EffectiveAt:     now,
			IdempotencyKey:  idempotencyKey,
			Status:          RunStatusWaiting,
			Generation:      detection.Generation,
			MisfireDecision: result.Action,
			OverlapDecision: "misfire_recovery",
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if _, err := createTriggerIfAbsent(ctx, m.store, trigger); err != nil {
			return fmt.Errorf("materialize schedule misfire trigger %s: %w", scheduledAt.Format(time.RFC3339Nano), err)
		}
	}
	return nil
}

func (m *MisfireService) recordMisfire(ctx context.Context, def *ScheduleContributionDefinition, detection *MisfireDetection, policy, action string, skippedCount int) {
	now := m.clock.Now()
	var scheduledAt time.Time
	if detection.EarliestMissed != nil {
		scheduledAt = *detection.EarliestMissed
	}
	record := &ScheduleMisfireRecord{
		MisfireID:    "misfire-" + uuid.NewString(),
		ScheduleID:   def.ScheduleID,
		ScheduledAt:  scheduledAt.UTC(),
		DetectedAt:   now.UTC(),
		Policy:       MisfirePolicy(policy),
		Action:       action,
		SkippedCount: skippedCount,
		Detail:       fmt.Sprintf("missed %d executions", detection.MissedCount),
	}
	_ = m.store.PutMisfire(ctx, record)
}
