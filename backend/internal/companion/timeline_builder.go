// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
)

func appendTimelineEntry(entries []TimelineEntry, start, end time.Time, state, sourceType string, priority int, reason string) []TimelineEntry {
	if end.Before(start) || end.Equal(start) {
		return entries
	}
	return append(entries, TimelineEntry{
		StartTime: start, EndTime: end,
		State: state, SourceType: sourceType,
		Priority: priority, Reason: reason,
	})
}

func (s *service) loadWorkSchedule(characterID string, date string) (bool, time.Time, time.Time, WorkProfile) {
	var workStart, workEnd time.Time
	var wp WorkProfile
	hasWork := false
	if err := s.db.Where("character_id = ?", characterID).Limit(1).First(&wp); err == nil && wp.Enabled == 1 && wp.WorkDays != "" {
		parts := []string{wp.WorkStartTime, wp.WorkEndTime}
		if len(parts) == 2 {
			workStart = parseTimeStr(parts[0], parseDate(date))
			workEnd = parseTimeStr(parts[1], parseDate(date))
			if workEnd.Before(workStart) || workEnd.Equal(workStart) {
				workEnd = workEnd.Add(12 * time.Hour)
			}
			todayWeekday := int(parseDate(date).Weekday())
			workDays := parseWorkDays(wp.WorkDays)
			if workDays[todayWeekday] {
				hasWork = true
			}
		}
	}
	return hasWork, workStart, workEnd, wp
}

func mergeTimelineEntries(entries []TimelineEntry) []TimelineEntry {
	if len(entries) == 0 {
		return entries
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].StartTime.Before(entries[j].StartTime) })
	merged := make([]TimelineEntry, 0, len(entries))
	for _, e := range entries {
		if len(merged) == 0 {
			merged = append(merged, e)
			continue
		}
		last := &merged[len(merged)-1]
		if e.StartTime.Before(last.EndTime) {
			if e.Priority > last.Priority {
				last.EndTime = e.StartTime
				merged = append(merged, e)
			}
		} else {
			merged = append(merged, e)
		}
	}
	return merged
}

func (s *service) buildTimeline(date string, schedule TodaySchedule, characterID string) []TimelineEntry {
	today := parseDate(date)
	midnight := today
	nextMidnight := today.Add(24 * time.Hour)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var entries []TimelineEntry

	hasWork, workStart, workEnd, _ := s.loadWorkSchedule(characterID, date)
	classes := s.buildClassEntries(date, characterID)

	wake := schedule.WakeTime
	lunch := schedule.LunchTime
	dinner := schedule.DinnerTime
	sleep := schedule.SleepTime
	if sleep.Before(wake) || sleep.Equal(wake) {
		sleep = sleep.Add(24 * time.Hour)
	}

	entries = appendTimelineEntry(entries, midnight, wake, "SLEEPING", "schedule", 100, "\u7761\u7720\u65f6\u95f4")
	wakeEnd := wake.Add(30 * time.Minute)
	entries = appendTimelineEntry(entries, wake, wakeEnd, "WAKING_UP", "schedule", 90, "\u8d77\u5e8a\u6d17\u6f31")
	afterWake := wakeEnd

	if hasWork && !schedule.IsRestDay {
		entries = s.buildWorkDayTimeline(entries, afterWake, workStart, workEnd, lunch, dinner, sleep, schedule, rng)
	} else if schedule.IsRestDay {
		entries = s.buildRestDayTimeline(entries, afterWake, lunch, dinner, sleep, schedule, rng)
	} else {
		entries = s.buildNoWorkTimeline(entries, afterWake, lunch, dinner, sleep, schedule, rng)
	}

	for _, c := range classes {
		entries = append(entries, c)
	}
	entries = appendTimelineEntry(entries, sleep, nextMidnight, "SLEEPING", "schedule", 100, "\u7761\u7720\u65f6\u95f4")
	return mergeTimelineEntries(entries)
}

func (s *service) buildWorkDayTimeline(entries []TimelineEntry, afterWake, workStart, workEnd, lunch, dinner, sleep time.Time, schedule TodaySchedule, rng *rand.Rand) []TimelineEntry {
	commuteStart := afterWake
	commuteDur := 30 * time.Minute
	entries = appendTimelineEntry(entries, commuteStart, commuteStart.Add(commuteDur), "COMMUTING_TO_WORK", "work", 80, "\u4e0a\u73ed\u901a\u52e4")
	workActualStart := commuteStart.Add(commuteDur)
	if workActualStart.Before(workStart) {
		entries = appendTimelineEntry(entries, workActualStart, workStart, "PREPARING_WORK", "work", 70, "\u51c6\u5907\u4e0a\u73ed")
	}
	morningWorkEnd := lunch.Add(-30 * time.Minute)
	if morningWorkEnd.After(workActualStart) {
		entries = appendTimelineEntry(entries, workActualStart, morningWorkEnd, "WORKING", "work", 75, "\u4e0a\u5348\u5de5\u4f5c")
	}
	entries = appendTimelineEntry(entries, morningWorkEnd, lunch, "LUNCH_BREAK", "schedule", 65, "\u4f11\u606f")
	lunchEnd := lunch.Add(1 * time.Hour)
	entries = appendTimelineEntry(entries, lunch, lunchEnd, "EATING_LUNCH", "schedule", 85, "\u5348\u996d\u65f6\u95f4")
	if schedule.HasNap && schedule.NapStartTime != nil && schedule.NapEndTime != nil {
		ns := *schedule.NapStartTime
		ne := *schedule.NapEndTime
		if ns.After(lunchEnd) {
			entries = appendTimelineEntry(entries, lunchEnd, ns, "IDLE", "schedule", 40, "\u7a7a\u95f2")
		}
		entries = appendTimelineEntry(entries, ns, ne, "NAPPING", "schedule", 85, "\u5348\u7761")
		lunchEnd = ne
	}
	afternoonWorkEnd := dinner.Add(-30 * time.Minute)
	if afternoonWorkEnd.After(lunchEnd) {
		entries = appendTimelineEntry(entries, lunchEnd, afternoonWorkEnd, "WORKING", "work", 75, "\u4e0b\u5348\u5de5\u4f5c")
	}
	commuteHomeStart := afternoonWorkEnd
	entries = appendTimelineEntry(entries, commuteHomeStart, commuteHomeStart.Add(30*time.Minute), "COMMUTING_HOME", "work", 80, "\u4e0b\u73ed\u901a\u52e4")
	afterWork := commuteHomeStart.Add(30 * time.Minute)
	entries = appendTimelineEntry(entries, dinner, dinner.Add(1*time.Hour), "EATING_DINNER", "schedule", 85, "\u665a\u996d\u65f6\u95f4")
	afterDinner := dinner.Add(1 * time.Hour)
	if afterDinner.Before(afterWork) {
		afterDinner = afterWork
	}
	beforeSleep := sleep.Add(-1 * time.Hour)
	if beforeSleep.After(afterDinner) {
		if afterDinner.Before(beforeSleep) {
			gap := beforeSleep.Sub(afterDinner)
			if gap > 2*time.Hour && rng.Intn(3) == 0 {
				studyEnd := afterDinner.Add(time.Duration(30+rng.Intn(61)) * time.Minute)
				if studyEnd.Before(beforeSleep.Add(-30 * time.Minute)) {
					entries = appendTimelineEntry(entries, afterDinner, studyEnd, "STUDYING", "schedule", 55, "\u665a\u95f4\u5b66\u4e60")
					entries = appendTimelineEntry(entries, studyEnd, beforeSleep, "AFTER_WORK", "schedule", 50, "\u665a\u95f4\u653e\u677e")
				} else {
					entries = appendTimelineEntry(entries, afterDinner, beforeSleep, "AFTER_WORK", "schedule", 50, "\u4e0b\u73ed\u540e\u81ea\u7531\u65f6\u95f4")
				}
			} else {
				entries = appendTimelineEntry(entries, afterDinner, beforeSleep, "AFTER_WORK", "schedule", 50, "\u4e0b\u73ed\u540e\u81ea\u7531\u65f6\u95f4")
			}
		}
	}
	entries = appendTimelineEntry(entries, beforeSleep, sleep, "BEFORE_SLEEP", "schedule", 80, "\u7761\u524d\u51c6\u5907")
	return entries
}

func (s *service) buildRestDayTimeline(entries []TimelineEntry, afterWake, lunch, dinner, sleep time.Time, schedule TodaySchedule, rng *rand.Rand) []TimelineEntry {
	entries = appendTimelineEntry(entries, afterWake, lunch, "IDLE", "schedule", 50, "\u4f11\u606f\u65e5\u81ea\u7531\u65f6\u95f4")
	entries = appendTimelineEntry(entries, lunch, lunch.Add(1*time.Hour), "EATING_LUNCH", "schedule", 85, "\u5348\u996d\u65f6\u95f4")
	lunchEnd := lunch.Add(1 * time.Hour)
	if schedule.HasNap && schedule.NapStartTime != nil && schedule.NapEndTime != nil {
		ns := *schedule.NapStartTime
		ne := *schedule.NapEndTime
		if ns.After(lunchEnd) {
			entries = appendTimelineEntry(entries, lunchEnd, ns, "IDLE", "schedule", 40, "\u7a7a\u95f2")
		}
		entries = appendTimelineEntry(entries, ns, ne, "NAPPING", "schedule", 85, "\u5348\u7761")
		lunchEnd = ne
	}
	entries = appendTimelineEntry(entries, lunchEnd, dinner, "IDLE", "schedule", 45, "\u4f11\u606f\u65e5\u4e0b\u5348")
	entries = appendTimelineEntry(entries, dinner, dinner.Add(1*time.Hour), "EATING_DINNER", "schedule", 85, "\u665a\u996d\u65f6\u95f4")
	afterDinner := dinner.Add(1 * time.Hour)
	beforeSleep := sleep.Add(-1 * time.Hour)
	if beforeSleep.After(afterDinner) {
		entries = appendTimelineEntry(entries, afterDinner, beforeSleep, "IDLE", "schedule", 40, "\u665a\u95f4\u4f11\u606f")
	}
	entries = appendTimelineEntry(entries, beforeSleep, sleep, "BEFORE_SLEEP", "schedule", 80, "\u7761\u524d\u51c6\u5907")
	return entries
}

func (s *service) buildNoWorkTimeline(entries []TimelineEntry, afterWake, lunch, dinner, sleep time.Time, schedule TodaySchedule, rng *rand.Rand) []TimelineEntry {
	lunchEnd := lunch.Add(time.Duration(40+rng.Intn(41)) * time.Minute)
	dinnerEnd := dinner.Add(time.Duration(40+rng.Intn(41)) * time.Minute)
	if schedule.HasNap && rng.Intn(10) < 7 && schedule.NapStartTime != nil && schedule.NapEndTime != nil {
		ns := *schedule.NapStartTime
		ne := *schedule.NapEndTime
		entries = appendTimelineEntry(entries, afterWake, lunch, "IDLE", "schedule", 50, "\u81ea\u7531\u65f6\u95f4")
		entries = appendTimelineEntry(entries, lunch, lunchEnd, "EATING_LUNCH", "schedule", 85, "\u5348\u996d\u65f6\u95f4")
		if ns.After(lunchEnd) {
			entries = appendTimelineEntry(entries, lunchEnd, ns, "IDLE", "schedule", 40, "\u7a7a\u95f2")
		}
		entries = appendTimelineEntry(entries, ns, ne, "NAPPING", "schedule", 85, "\u5348\u7761")
		afterLunchEnd := ne
		if afterLunchEnd.Before(lunchEnd) {
			afterLunchEnd = lunchEnd
		}
		entries = appendTimelineEntry(entries, afterLunchEnd, dinner, "IDLE", "schedule", 45, "\u5348\u540e\u65f6\u95f4")
	} else {
		entries = appendTimelineEntry(entries, afterWake, lunch, "IDLE", "schedule", 50, "\u81ea\u7531\u65f6\u95f4")
		entries = appendTimelineEntry(entries, lunch, lunchEnd, "EATING_LUNCH", "schedule", 85, "\u5348\u996d\u65f6\u95f4")
		if rng.Intn(4) == 0 {
			studyEnd := lunchEnd.Add(time.Duration(30+rng.Intn(61)) * time.Minute)
			if studyEnd.Before(dinner.Add(-30 * time.Minute)) {
				entries = appendTimelineEntry(entries, lunchEnd, studyEnd, "STUDYING", "schedule", 55, "\u5348\u540e\u5b66\u4e60")
				entries = appendTimelineEntry(entries, studyEnd, dinner, "IDLE", "schedule", 45, "\u5348\u540e\u65f6\u95f4")
			} else {
				entries = appendTimelineEntry(entries, lunchEnd, dinner, "IDLE", "schedule", 45, "\u5348\u540e\u65f6\u95f4")
			}
		} else {
			entries = appendTimelineEntry(entries, lunchEnd, dinner, "IDLE", "schedule", 45, "\u5348\u540e\u65f6\u95f4")
		}
	}
	entries = appendTimelineEntry(entries, dinner, dinnerEnd, "EATING_DINNER", "schedule", 85, "\u665a\u996d\u65f6\u95f4")
	beforeSleep := sleep.Add(-1 * time.Hour)
	if beforeSleep.After(dinnerEnd) {
		gap := beforeSleep.Sub(dinnerEnd)
		if gap > 2*time.Hour && rng.Intn(3) == 0 {
			readEnd := dinnerEnd.Add(time.Duration(30+rng.Intn(61)) * time.Minute)
			if readEnd.Before(beforeSleep.Add(-30 * time.Minute)) {
				entries = appendTimelineEntry(entries, dinnerEnd, readEnd, "STUDYING", "schedule", 55, "\u665a\u95f4\u9605\u8bfb")
				entries = appendTimelineEntry(entries, readEnd, beforeSleep, "IDLE", "schedule", 40, "\u665a\u95f4\u653e\u677e")
			} else {
				entries = appendTimelineEntry(entries, dinnerEnd, beforeSleep, "IDLE", "schedule", 40, "\u665a\u95f4\u81ea\u7531\u65f6\u95f4")
			}
		} else {
			entries = appendTimelineEntry(entries, dinnerEnd, beforeSleep, "IDLE", "schedule", 40, "\u665a\u95f4\u81ea\u7531\u65f6\u95f4")
		}
	}
	entries = appendTimelineEntry(entries, beforeSleep, sleep, "BEFORE_SLEEP", "schedule", 80, "\u7761\u524d\u51c6\u5907")
	return entries
}

func (s *service) buildClassEntries(date string, characterID string) []TimelineEntry {
	var entries []TimelineEntry
	today := parseDate(date)

	classes := s.GetEffectiveClasses(date, characterID)
	for _, c := range classes {
		slots, _ := c["slots"].([]map[string]interface{})
		for _, slot := range slots {
			name, _ := slot["className"].(string)
			if name == "" {
				name, _ = slot["name"].(string)
			}
			startStr, _ := slot["startTime"].(string)
			endStr, _ := slot["endTime"].(string)
			if startStr == "" || endStr == "" {
				continue
			}

			start := parseTimeStr(startStr, today)
			end := parseTimeStr(endStr, today)
			if end.Before(start) {
				continue
			}

			reason := fmt.Sprintf("课程: %s", name)
			entries = append(entries, TimelineEntry{
				StartTime: start, EndTime: end,
				State: "IN_CLASS", SourceType: "class",
				Priority: 80, Reason: reason,
			})
			if start.After(today.Add(30 * time.Minute)) {
				prepStart := start.Add(-15 * time.Minute)
				entries = append(entries, TimelineEntry{
					StartTime: prepStart, EndTime: start,
					State: "PREPARING_CLASS", SourceType: "class",
					Priority: 60, Reason: fmt.Sprintf("准备课程: %s", name),
				})
			}
			afterStart := end
			afterEnd := end.Add(15 * time.Minute)
			entries = append(entries, TimelineEntry{
				StartTime: afterStart, EndTime: afterEnd,
				State: "AFTER_CLASS", SourceType: "class",
				Priority: 50, Reason: fmt.Sprintf("课程结束: %s", name),
			})
		}
	}

	var fixedEvents []FixedEvent
	s.db.Where("enabled = 1").Find(&fixedEvents)
	for _, e := range fixedEvents {
		if e.EventType == "study" || e.EventType == "course" {
			start := parseTimeStr(e.StartTime, today)
			end := parseTimeStr(e.EndTime, today)
			if end.Before(start) {
				continue
			}
			entries = append(entries, TimelineEntry{
				StartTime: start, EndTime: end,
				State: "STUDYING", SourceType: "fixed_event",
				Priority: 70, Reason: fmt.Sprintf("学习: %s", e.Title),
			})
		}
	}

	return entries
}

func (s *service) GetTimelineToday(characterID string) map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	schedule := s.buildTodaySchedule(today, characterID)
	entries := s.buildTimeline(today, schedule, characterID)
	result := make([]map[string]interface{}, len(entries))
	for i, e := range entries {
		result[i] = map[string]interface{}{
			"startTime":  e.StartTime.Format("2006-01-02T15:04:05"),
			"endTime":    e.EndTime.Format("2006-01-02T15:04:05"),
			"state":      e.State,
			"sourceType": e.SourceType,
			"priority":   e.Priority,
			"reason":     e.Reason,
		}
	}
	if result == nil {
		result = []map[string]interface{}{}
	}
	return map[string]interface{}{"date": today, "events": result, "schedule": scheduleToMap(schedule)}
}

func scheduleToMap(s TodaySchedule) map[string]interface{} {
	result := map[string]interface{}{
		"wakeTime":   s.WakeTime.Format("2006-01-02T15:04:05"),
		"lunchTime":  s.LunchTime.Format("2006-01-02T15:04:05"),
		"dinnerTime": s.DinnerTime.Format("2006-01-02T15:04:05"),
		"hasNap":     s.HasNap,
		"sleepTime":  s.SleepTime.Format("2006-01-02T15:04:05"),
		"isRestDay":  s.IsRestDay,
	}
	if s.NapStartTime != nil {
		result["napStartTime"] = s.NapStartTime.Format("2006-01-02T15:04:05")
	}
	if s.NapEndTime != nil {
		result["napEndTime"] = s.NapEndTime.Format("2006-01-02T15:04:05")
	}
	return result
}
