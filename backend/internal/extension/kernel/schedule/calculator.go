package schedule

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ScheduleCalculator struct {
	clock Clock
}

func NewScheduleCalculator(clock Clock) *ScheduleCalculator {
	if clock == nil {
		clock = NewRealClock()
	}
	return &ScheduleCalculator{clock: clock}
}

func (c *ScheduleCalculator) CalculateNext(def *ScheduleContributionDefinition, state *ScheduleState) (*NextRunResult, error) {
	now := c.clock.Now()

	if def.StartAt != nil && now.Before(*def.StartAt) {
		next := *def.StartAt
		return &NextRunResult{
			NextScheduledAt:   &next,
			NextEffectiveAt:   &next,
			CalculationReason: "before_start_at",
		}, nil
	}

	if def.EndAt != nil && now.After(*def.EndAt) {
		return &NextRunResult{
			NextScheduledAt:   nil,
			NextEffectiveAt:   nil,
			CalculationReason: "after_end_at",
		}, nil
	}

	loc, err := time.LoadLocation(def.Timezone)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTimezone, def.Timezone)
	}

	var lastScheduledAt *time.Time
	if state != nil && state.LastScheduledAt != nil {
		lastScheduledAt = state.LastScheduledAt
	}

	var baseTime time.Time
	if lastScheduledAt != nil {
		baseTime = lastScheduledAt.Add(time.Minute)
	} else if def.StartAt != nil {
		baseTime = *def.StartAt
	} else {
		baseTime = now
	}

	var nextScheduled *time.Time
	var dstDecision string

	switch def.Trigger.Type {
	case TriggerTypeCron:
		nextScheduled, dstDecision, err = c.nextCron(def, loc, baseTime, now)
	case TriggerTypeInterval:
		nextScheduled, dstDecision, err = c.nextInterval(def, loc, baseTime, now)
	case TriggerTypeOneShot:
		nextScheduled, dstDecision, err = c.nextOneShot(def, loc, now)
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidTriggerType, def.Trigger.Type)
	}

	if err != nil {
		return nil, err
	}

	if nextScheduled == nil {
		return &NextRunResult{
			NextScheduledAt:   nil,
			NextEffectiveAt:   nil,
			CalculationReason: "no_next_run",
			DSTDecision:       dstDecision,
		}, nil
	}

	if def.EndAt != nil && nextScheduled.After(*def.EndAt) {
		return &NextRunResult{
			NextScheduledAt:   nil,
			NextEffectiveAt:   nil,
			CalculationReason: "next_exceeds_end_at",
			DSTDecision:       dstDecision,
		}, nil
	}

	nextEffective := c.applyJitter(def, *nextScheduled)

	return &NextRunResult{
		NextScheduledAt:   nextScheduled,
		NextEffectiveAt:   &nextEffective,
		CalculationReason: "calculated",
		DSTDecision:       dstDecision,
	}, nil
}

func (c *ScheduleCalculator) nextCron(def *ScheduleContributionDefinition, loc *time.Location, baseTime, now time.Time) (*time.Time, string, error) {
	if def.Trigger.Cron == nil {
		return nil, "", fmt.Errorf("%w: cron definition missing", ErrInvalidCronExpression)
	}
	if def.Trigger.Cron.Seconds {
		return nil, "", fmt.Errorf("%w: seconds not supported", ErrFrequencyTooHigh)
	}

	sched, err := parseCron5(def.Trigger.Cron.Expression)
	if err != nil {
		return nil, "", err
	}

	if err := validateMinFrequency(sched); err != nil {
		return nil, "", err
	}

	searchStart := baseTime.In(loc)
	if searchStart.Before(now) {
		searchStart = now
	}

	next, dstDecision, err := sched.nextTime(searchStart, loc, def)
	if err != nil {
		return nil, "", err
	}

	utcNext := next.UTC()
	return &utcNext, dstDecision, nil
}

func (c *ScheduleCalculator) nextInterval(def *ScheduleContributionDefinition, loc *time.Location, baseTime, now time.Time) (*time.Time, string, error) {
	if def.Trigger.Interval == nil {
		return nil, "", fmt.Errorf("%w: interval definition missing", ErrInvalidInterval)
	}

	interval := def.Trigger.Interval.Interval
	if interval < time.Minute {
		return nil, "", fmt.Errorf("%w: %v", ErrFrequencyTooHigh, interval)
	}
	if interval > 365*24*time.Hour {
		return nil, "", fmt.Errorf("%w: %v", ErrIntervalTooLarge, interval)
	}

	anchor := def.Trigger.Interval.AnchorAt
	if anchor.IsZero() {
		if def.StartAt != nil {
			anchor = *def.StartAt
		} else {
			anchor = now
		}
	}

	var base time.Time
	if baseTime.After(anchor) {
		base = baseTime
	} else {
		base = anchor
	}
	if base.Before(now) {
		base = now
	}

	elapsed := base.Sub(anchor)
	if elapsed < 0 {
		elapsed = 0
	}
	count := elapsed / interval
	next := anchor.Add((count + 1) * interval)

	for next.Before(now) || next.Before(base) {
		next = next.Add(interval)
		if def.EndAt != nil && next.After(*def.EndAt) {
			return nil, "", nil
		}
	}

	nextLocal := next.In(loc)
	utcNext := nextLocal.UTC()
	return &utcNext, "no_dst_transition", nil
}

func (c *ScheduleCalculator) nextOneShot(def *ScheduleContributionDefinition, loc *time.Location, now time.Time) (*time.Time, string, error) {
	if def.Trigger.OneShot == nil {
		return nil, "", fmt.Errorf("%w: one-shot definition missing", ErrInvalidOneShotTime)
	}

	runAt := def.Trigger.OneShot.RunAt
	if def.EndAt != nil && runAt.After(*def.EndAt) {
		return nil, "", nil
	}

	if runAt.Before(now) {
		return nil, "", nil
	}

	utcRunAt := runAt.UTC()
	return &utcRunAt, "no_dst_transition", nil
}

func (c *ScheduleCalculator) applyJitter(def *ScheduleContributionDefinition, scheduledAt time.Time) time.Time {
	if !def.JitterPolicy.Enabled || def.JitterPolicy.MaxDelay <= 0 {
		return scheduledAt
	}

	seed := scheduleSeed(def.ScheduleID, scheduledAt)
	delay := jitterDelay(seed, def.JitterPolicy.MaxDelay)
	return scheduledAt.Add(delay)
}

func scheduleSeed(scheduleID string, scheduledAt time.Time) int64 {
	h := sha256.New()
	h.Write([]byte(scheduleID))
	h.Write([]byte(scheduledAt.UTC().Format(time.RFC3339Nano)))
	hash := h.Sum(nil)
	var seed int64
	for i := 0; i < 8; i++ {
		seed = (seed << 8) | int64(hash[i])
	}
	if seed < 0 {
		seed = -seed
	}
	return seed
}

func jitterDelay(seed int64, maxDelay time.Duration) time.Duration {
	if maxDelay <= 0 {
		return 0
	}
	normalized := seed % int64(maxDelay)
	return time.Duration(normalized)
}

func GenerateIdempotencyKey(scheduleID string, scheduledAt time.Time, generation int64) string {
	h := sha256.New()
	h.Write([]byte(scheduleID))
	h.Write([]byte(scheduledAt.UTC().Format(time.RFC3339Nano)))
	h.Write([]byte(fmt.Sprintf("%d", generation)))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func GenerateDefinitionHash(def *ScheduleContributionDefinition) string {
	h := sha256.New()
	h.Write([]byte(def.ScheduleID))
	h.Write([]byte(def.Name))
	h.Write([]byte(def.Trigger.Type))
	if def.Trigger.Cron != nil {
		h.Write([]byte(def.Trigger.Cron.Expression))
	}
	if def.Trigger.Interval != nil {
		h.Write([]byte(fmt.Sprintf("%d", def.Trigger.Interval.Interval)))
		h.Write([]byte(def.Trigger.Interval.AnchorAt.Format(time.RFC3339Nano)))
	}
	if def.Trigger.OneShot != nil {
		h.Write([]byte(def.Trigger.OneShot.RunAt.Format(time.RFC3339Nano)))
	}
	h.Write([]byte(def.Target.Type))
	h.Write([]byte(def.Target.TargetID))
	h.Write([]byte(def.Timezone))
	h.Write([]byte(def.Version))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

type cronSchedule struct {
	minutes    map[int]bool
	hours      map[int]bool
	daysOfMonth map[int]bool
	months     map[int]bool
	daysOfWeek map[int]bool
}

func parseCron5(expr string) (*cronSchedule, error) {
	expr = strings.TrimSpace(expr)
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("%w: expected 5 fields, got %d", ErrInvalidCronExpression, len(parts))
	}

	sched := &cronSchedule{
		minutes:     map[int]bool{},
		hours:       map[int]bool{},
		daysOfMonth: map[int]bool{},
		months:      map[int]bool{},
		daysOfWeek:  map[int]bool{},
	}

	var err error
	sched.minutes, err = parseField(parts[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	sched.hours, err = parseField(parts[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	sched.daysOfMonth, err = parseField(parts[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day-of-month field: %w", err)
	}
	sched.months, err = parseField(parts[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	sched.daysOfWeek, err = parseField(parts[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("day-of-week field: %w", err)
	}

	return sched, nil
}

func parseField(field string, min, max int) (map[int]bool, error) {
	result := map[int]bool{}
	if field == "*" {
		for i := min; i <= max; i++ {
			result[i] = true
		}
		return result, nil
	}

	for _, part := range strings.Split(field, ",") {
		step := 1
		if idx := strings.Index(part, "/"); idx >= 0 {
			s, err := strconv.Atoi(part[idx+1:])
			if err != nil || s <= 0 {
				return nil, fmt.Errorf("invalid step: %s", part)
			}
			step = s
			part = part[:idx]
		}

		lo, hi := min, max
		if part != "*" {
			if idx := strings.Index(part, "-"); idx >= 0 {
				lo, err := strconv.Atoi(part[:idx])
				if err != nil || lo < min {
					return nil, fmt.Errorf("invalid range start: %s", part)
				}
				hi, err = strconv.Atoi(part[idx+1:])
				if err != nil || hi > max {
					return nil, fmt.Errorf("invalid range end: %s", part)
				}
			} else {
				v, err := strconv.Atoi(part)
				if err != nil {
					return nil, fmt.Errorf("invalid value: %s", part)
				}
				lo = v
				hi = v
			}
		}

		for i := lo; i <= hi; i += step {
			if i >= min && i <= max {
				result[i] = true
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("empty field: %s", field)
	}
	return result, nil
}

func validateMinFrequency(sched *cronSchedule) error {
	if len(sched.minutes) <= 1 && len(sched.hours) <= 1 {
		enabled := 0
		for _, v := range sched.minutes {
			if v {
				enabled++
			}
		}
		if enabled <= 1 {
			enabled = 0
			for _, v := range sched.hours {
				if v {
					enabled++
				}
			}
			if enabled <= 1 {
				return nil
			}
		}
	}
	return nil
}

func (s *cronSchedule) nextTime(from time.Time, loc *time.Location, def *ScheduleContributionDefinition) (time.Time, string, error) {
	dstDecision := "no_dst_transition"

	t := from.Truncate(time.Minute).Add(time.Minute)

	for i := 0; i < 366*24*60; i++ {
		localT := t.In(loc)

		_, offsetBefore := localT.Add(-time.Minute).Zone()
		_, offsetAfter := localT.Zone()

		if offsetBefore != offsetAfter {
			if offsetAfter < offsetBefore {
				dstDecision = "dst_spring_forward"
				springPolicy := def.DSTSpringPolicy
				if springPolicy == "" {
					springPolicy = DefaultDSTSpringPolicy()
				}
				switch springPolicy {
				case DSTSpringSkip:
					t = t.Add(time.Minute)
					continue
				case DSTSpringFireOnceAfterGap:
					break
				case DSTSpringNextValidTime:
					break
				}
			} else {
				dstDecision = "dst_fall_back"
				fallPolicy := def.DSTFallPolicy
				if fallPolicy == "" {
					fallPolicy = DefaultDSTFallPolicy()
				}
				switch fallPolicy {
				case DSTFallFireOnceFirst:
					if s.matches(localT) {
						return localT, dstDecision, nil
					}
				case DSTFallFireOnceSecond:
					localTAdjusted := localT.Add(-time.Hour)
					if s.matches(localTAdjusted) {
						t = t.Add(time.Minute)
						continue
					}
				case DSTFallFireTwice:
					break
				}
			}
		}

		if s.matches(localT) {
			return localT, dstDecision, nil
		}

		t = t.Add(time.Minute)
	}

	return time.Time{}, dstDecision, fmt.Errorf("%w: no matching time found within search window", ErrInvalidCronExpression)
}

func (s *cronSchedule) matches(t time.Time) bool {
	if !s.minutes[t.Minute()] {
		return false
	}
	if !s.hours[t.Hour()] {
		return false
	}
	if !s.months[int(t.Month())] {
		return false
	}

	dayOfMonthMatch := s.daysOfMonth[t.Day()]
	dayOfWeekMatch := s.daysOfWeek[int(t.Weekday())]

	hasDOMWildcard := s.isWildcard(s.daysOfMonth, 1, 31)
	hasDOWWildcard := s.isWildcard(s.daysOfWeek, 0, 6)

	if hasDOMWildcard && hasDOWWildcard {
		return true
	}
	if hasDOMWildcard {
		return dayOfWeekMatch
	}
	if hasDOWWildcard {
		return dayOfMonthMatch
	}
	return dayOfMonthMatch || dayOfWeekMatch
}

func (s *cronSchedule) isWildcard(m map[int]bool, min, max int) bool {
	if len(m) != (max - min + 1) {
		return false
	}
	for i := min; i <= max; i++ {
		if !m[i] {
			return false
		}
	}
	return true
}

func SortedKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
