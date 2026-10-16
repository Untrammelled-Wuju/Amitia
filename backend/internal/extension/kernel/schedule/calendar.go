package schedule

import (
	"fmt"
	"time"
)

func (c *ScheduleCalculator) nextCalendar(def *ScheduleContributionDefinition, loc *time.Location, baseTime, now time.Time) (*time.Time, string, error) {
	rule := def.Trigger.Calendar
	if rule == nil || rule.AnchorAt.IsZero() {
		return nil, "", fmt.Errorf("%w: calendar anchor missing", ErrInvalidTriggerType)
	}
	if rule.Frequency != "daily" && rule.Frequency != "weekly" && rule.Frequency != "monthly" && rule.Frequency != "weekdays" {
		return nil, "", fmt.Errorf("%w: calendar frequency", ErrInvalidTriggerType)
	}
	weekdays := map[int]bool{}
	for _, weekday := range rule.Weekdays {
		if weekday < 1 || weekday > 7 {
			return nil, "", fmt.Errorf("%w: ISO weekday", ErrInvalidTriggerType)
		}
		weekdays[weekday] = true
	}
	if rule.Frequency == "weekdays" && len(weekdays) == 0 {
		return nil, "", fmt.Errorf("%w: weekdays missing", ErrInvalidTriggerType)
	}
	anchor := rule.AnchorAt.In(loc)
	clockAnchor := anchor
	if rule.WallTime != "" {
		wall, err := time.Parse("15:04:05", rule.WallTime)
		if err != nil {
			return nil, "", fmt.Errorf("%w: calendar wall time", ErrInvalidTriggerType)
		}
		clockAnchor = time.Date(anchor.Year(), anchor.Month(), anchor.Day(), wall.Hour(), wall.Minute(), wall.Second(), 0, time.UTC)
	}
	from := now
	if baseTime.After(from) {
		from = baseTime
	}
	if def.StartAt != nil && def.StartAt.After(from) {
		from = def.StartAt.Add(-time.Nanosecond)
	}
	anchorISO := (int(anchor.Weekday())+6)%7 + 1
	if rule.AnchorAt.After(from) && (rule.Frequency != "weekdays" || weekdays[anchorISO]) {
		first := rule.AnchorAt.UTC()
		return &first, "calendar_first_occurrence", nil
	}
	day := time.Date(from.In(loc).Year(), from.In(loc).Month(), from.In(loc).Day(), 12, 0, 0, 0, loc)
	for i := 0; i < 366*8; i++ {
		candidateDay := day.AddDate(0, 0, i)
		isoDay := (int(candidateDay.Weekday())+6)%7 + 1
		switch rule.Frequency {
		case "weekly":
			if candidateDay.Weekday() != anchor.Weekday() {
				continue
			}
		case "weekdays":
			if !weekdays[isoDay] {
				continue
			}
		case "monthly":
			lastDay := time.Date(candidateDay.Year(), candidateDay.Month()+1, 0, 12, 0, 0, 0, loc).Day()
			wanted := anchor.Day()
			if wanted > lastDay {
				wanted = lastDay
			}
			if candidateDay.Day() != wanted {
				continue
			}
		}
		candidate, decision := calendarWallTime(candidateDay, clockAnchor, loc, def.DSTFallPolicy)
		if decision == "dst_next_valid_time" && def.DSTSpringPolicy == DSTSpringSkip {
			continue
		}
		if candidate.Before(rule.AnchorAt) || !candidate.After(from) {
			continue
		}
		utc := candidate.UTC()
		return &utc, decision, nil
	}
	return nil, "", fmt.Errorf("%w: no calendar occurrence", ErrInvalidTriggerType)
}

func NormalizeCalendarWallTime(wall time.Time, loc *time.Location) (time.Time, string) {
	return calendarWallTime(wall, wall, loc, DSTFallFireOnceFirst)
}

func calendarWallTime(day, anchor time.Time, loc *time.Location, fallPolicy DSTFallPolicy) (time.Time, string) {
	wanted := time.Date(day.Year(), day.Month(), day.Day(), anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), time.UTC)
	naive := time.Date(day.Year(), day.Month(), day.Day(), anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), loc)
	_, before := naive.Add(-24 * time.Hour).Zone()
	_, after := naive.Add(24 * time.Hour).Zone()
	var matches []time.Time
	for _, offset := range []int{before, after} {
		candidate := wanted.Add(-time.Duration(offset) * time.Second).In(loc)
		if candidate.Year() == day.Year() && candidate.Month() == day.Month() && candidate.Day() == day.Day() && candidate.Hour() == anchor.Hour() && candidate.Minute() == anchor.Minute() && candidate.Second() == anchor.Second() {
			matches = append(matches, candidate)
		}
	}
	if len(matches) > 0 {
		chosen := matches[0]
		for _, candidate := range matches[1:] {
			if (fallPolicy == DSTFallFireOnceSecond && candidate.After(chosen)) || (fallPolicy != DSTFallFireOnceSecond && candidate.Before(chosen)) {
				chosen = candidate
			}
		}
		decision := "no_dst_transition"
		if before != after {
			decision = "dst_fire_once_first"
			if fallPolicy == DSTFallFireOnceSecond {
				decision = "dst_fire_once_second"
			}
		}
		return chosen, decision
	}
	start := wanted.Add(-time.Duration(before)*time.Second - 4*time.Hour)
	for i := 0; i < 8*60; i++ {
		candidate := start.Add(time.Duration(i) * time.Minute).In(loc)
		if candidate.Year() != day.Year() || candidate.Month() != day.Month() || candidate.Day() != day.Day() {
			continue
		}
		wall := time.Date(candidate.Year(), candidate.Month(), candidate.Day(), candidate.Hour(), candidate.Minute(), candidate.Second(), candidate.Nanosecond(), time.UTC)
		if !wall.Before(wanted) {
			return candidate, "dst_next_valid_time"
		}
	}
	return naive, "dst_next_valid_time"
}
