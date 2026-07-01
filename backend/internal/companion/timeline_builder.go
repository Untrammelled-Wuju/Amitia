// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
)

func (s *service) buildTimeline(date string, schedule TodaySchedule, characterID string) []TimelineEntry {
	today := parseDate(date)
	midnight := today
	nextMidnight := today.Add(24 * time.Hour)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var entries []TimelineEntry

	addEntry := func(start, end time.Time, state, sourceType string, priority int, reason string) {
		if end.Before(start) || end.Equal(start) {
			return
		}
		entries = append(entries, TimelineEntry{
			StartTime: start, EndTime: end,
			State: state, SourceType: sourceType,
			Priority: priority, Reason: reason,
		})
	}

	hasWork := false
	var workStart, workEnd time.Time
	var wp WorkProfile
	if err := s.db.Where("character_id = ?", characterID).Limit(1).First(&wp); err == nil && wp.Enabled == 1 && wp.WorkDays != "" {
		parts := []string{wp.WorkStartTime, wp.WorkEndTime}
		if len(parts) == 2 {
			workStart = parseTimeStr(parts[0], today)
			workEnd = parseTimeStr(parts[1], today)
			if workEnd.Before(workStart) || workEnd.Equal(workStart) {
				workEnd = workEnd.Add(12 * time.Hour)
			}
			todayWeekday := int(today.Weekday())
			if wp.WorkDays != "" {
				workDays := parseWorkDays(wp.WorkDays)
				if workDays[todayWeekday] {
					hasWork = true
				}
			} else {
				hasWork = todayWeekday >= 1 && todayWeekday <= 5
			}
		}
	}

	classes := s.buildClassEntries(date, characterID)

	wake := schedule.WakeTime
	lunch := schedule.LunchTime
	dinner := schedule.DinnerTime
	sleep := schedule.SleepTime

	if sleep.Before(wake) || sleep.Equal(wake) {
		sleep = sleep.Add(24 * time.Hour)
	}

	addEntry(midnight, wake, "SLEEPING", "schedule", 100, "睡眠时间")

	wakeEnd := wake.Add(30 * time.Minute)
	addEntry(wake, wakeEnd, "WAKING_UP", "schedule", 90, "起床洗漱")

	afterWake := wakeEnd

	if hasWork && !schedule.IsRestDay {
		commuteStart := afterWake
		commuteDur := 30 * time.Minute
		addEntry(commuteStart, commuteStart.Add(commuteDur), "COMMUTING_TO_WORK", "work", 80, "上班通勤")
		workActualStart := commuteStart.Add(commuteDur)
		if workActualStart.Before(workStart) {
			addEntry(workActualStart, workStart, "PREPARING_WORK", "work", 70, "准备上班")
		}
		morningWorkEnd := lunch.Add(-30 * time.Minute)
		if morningWorkEnd.After(workActualStart) {
			addEntry(workActualStart, morningWorkEnd, "WORKING", "work", 75, "上午工作")
		}
		addEntry(morningWorkEnd, lunch, "LUNCH_BREAK", "schedule", 65, "午休")

		lunchEnd := lunch.Add(1 * time.Hour)
		addEntry(lunch, lunchEnd, "EATING_LUNCH", "schedule", 85, "午饭时间")

		if schedule.HasNap && schedule.NapStartTime != nil && schedule.NapEndTime != nil {
			ns := *schedule.NapStartTime
			ne := *schedule.NapEndTime
			if ns.After(lunchEnd) {
				addEntry(lunchEnd, ns, "IDLE", "schedule", 40, "空闲")
			}
			addEntry(ns, ne, "NAPPING", "schedule", 85, "午睡")
			lunchEnd = ne
		}

		afternoonWorkEnd := dinner.Add(-30 * time.Minute)
		if afternoonWorkEnd.After(lunchEnd) {
			addEntry(lunchEnd, afternoonWorkEnd, "WORKING", "work", 75, "下午工作")
		}

		commuteHomeStart := afternoonWorkEnd
		addEntry(commuteHomeStart, commuteHomeStart.Add(30*time.Minute), "COMMUTING_HOME", "work", 80, "下班通勤")
		afterWork := commuteHomeStart.Add(30 * time.Minute)

		addEntry(dinner, dinner.Add(1*time.Hour), "EATING_DINNER", "schedule", 85, "晚饭时间")
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
						addEntry(afterDinner, studyEnd, "STUDYING", "schedule", 55, "晚间学习")
						addEntry(studyEnd, beforeSleep, "AFTER_WORK", "schedule", 50, "晚间放松")
					} else {
						addEntry(afterDinner, beforeSleep, "AFTER_WORK", "schedule", 50, "下班后自由时间")
					}
				} else {
					addEntry(afterDinner, beforeSleep, "AFTER_WORK", "schedule", 50, "下班后自由时间")
				}
			}
		}
		addEntry(beforeSleep, sleep, "BEFORE_SLEEP", "schedule", 80, "睡前准备")

	} else if schedule.IsRestDay {
		addEntry(afterWake, lunch, "IDLE", "schedule", 50, "休息日自由时间")
		addEntry(lunch, lunch.Add(1*time.Hour), "EATING_LUNCH", "schedule", 85, "午饭时间")
		lunchEnd := lunch.Add(1 * time.Hour)
		if schedule.HasNap && schedule.NapStartTime != nil && schedule.NapEndTime != nil {
			ns := *schedule.NapStartTime
			ne := *schedule.NapEndTime
			if ns.After(lunchEnd) {
				addEntry(lunchEnd, ns, "IDLE", "schedule", 40, "空闲")
			}
			addEntry(ns, ne, "NAPPING", "schedule", 85, "午睡")
			lunchEnd = ne
		}
		addEntry(lunchEnd, dinner, "IDLE", "schedule", 45, "休息日下午")
		addEntry(dinner, dinner.Add(1*time.Hour), "EATING_DINNER", "schedule", 85, "晚饭时间")
		afterDinner := dinner.Add(1 * time.Hour)
		beforeSleep := sleep.Add(-1 * time.Hour)
		if beforeSleep.After(afterDinner) {
			addEntry(afterDinner, beforeSleep, "IDLE", "schedule", 40, "晚间休息")
		}
		addEntry(beforeSleep, sleep, "BEFORE_SLEEP", "schedule", 80, "睡前准备")
	} else {
		lunchEnd := lunch.Add(time.Duration(40+rng.Intn(41)) * time.Minute)
		dinnerEnd := dinner.Add(time.Duration(40+rng.Intn(41)) * time.Minute)
		if schedule.HasNap && rng.Intn(10) < 7 && schedule.NapStartTime != nil && schedule.NapEndTime != nil {
			ns := *schedule.NapStartTime
			ne := *schedule.NapEndTime
			addEntry(afterWake, lunch, "IDLE", "schedule", 50, "自由时间")
			addEntry(lunch, lunchEnd, "EATING_LUNCH", "schedule", 85, "午饭时间")
			if ns.After(lunchEnd) {
				addEntry(lunchEnd, ns, "IDLE", "schedule", 40, "空闲")
			}
			addEntry(ns, ne, "NAPPING", "schedule", 85, "午睡")
			afterLunchEnd := ne
			if afterLunchEnd.Before(lunchEnd) {
				afterLunchEnd = lunchEnd
			}
			addEntry(afterLunchEnd, dinner, "IDLE", "schedule", 45, "午后时间")
		} else {
			addEntry(afterWake, lunch, "IDLE", "schedule", 50, "自由时间")
			addEntry(lunch, lunchEnd, "EATING_LUNCH", "schedule", 85, "午饭时间")
			if rng.Intn(4) == 0 {
				studyEnd := lunchEnd.Add(time.Duration(30+rng.Intn(61)) * time.Minute)
				if studyEnd.Before(dinner.Add(-30 * time.Minute)) {
					addEntry(lunchEnd, studyEnd, "STUDYING", "schedule", 55, "午后学习")
					addEntry(studyEnd, dinner, "IDLE", "schedule", 45, "午后时间")
				} else {
					addEntry(lunchEnd, dinner, "IDLE", "schedule", 45, "午后时间")
				}
			} else {
				addEntry(lunchEnd, dinner, "IDLE", "schedule", 45, "午后时间")
			}
		}
		addEntry(dinner, dinnerEnd, "EATING_DINNER", "schedule", 85, "晚饭时间")
		beforeSleep := sleep.Add(-1 * time.Hour)
		if beforeSleep.After(dinnerEnd) {
			gap := beforeSleep.Sub(dinnerEnd)
			if gap > 2*time.Hour && rng.Intn(3) == 0 {
				readEnd := dinnerEnd.Add(time.Duration(30+rng.Intn(61)) * time.Minute)
				if readEnd.Before(beforeSleep.Add(-30 * time.Minute)) {
					addEntry(dinnerEnd, readEnd, "STUDYING", "schedule", 55, "晚间阅读")
					addEntry(readEnd, beforeSleep, "IDLE", "schedule", 40, "晚间放松")
				} else {
					addEntry(dinnerEnd, beforeSleep, "IDLE", "schedule", 40, "晚间自由时间")
				}
			} else {
				addEntry(dinnerEnd, beforeSleep, "IDLE", "schedule", 40, "晚间自由时间")
			}
		}
		addEntry(beforeSleep, sleep, "BEFORE_SLEEP", "schedule", 80, "睡前准备")
	}

	for _, c := range classes {
		entries = append(entries, c)
	}

	addEntry(sleep, nextMidnight, "SLEEPING", "schedule", 100, "睡眠时间")

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
