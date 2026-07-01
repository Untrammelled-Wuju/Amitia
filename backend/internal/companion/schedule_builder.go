// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"fmt"
	"time"
)

func (s *service) buildTodaySchedule(date string, characterID string) TodaySchedule {
	today := parseDate(date)

	wakeTime := parseTimeStr("08:00", today)
	bedTime := parseTimeStr("23:00", today)

	var bed, wake string
	var sleepEnabled int
	err := s.db.Table("sleep_settings").Select("bed_time, wake_time, enabled").Where("character_id = ?", characterID).Limit(1).Row().Scan(&bed, &wake, &sleepEnabled)
	if err == nil {
		if wake != "" {
			wakeTime = parseTimeStr(wake, today)
		}
		if bed != "" {
			bedTime = parseTimeStr(bed, today)
		}
		if bedTime.Before(wakeTime) || bedTime.Equal(wakeTime) {
			bedTime = bedTime.Add(24 * time.Hour)
		}
	}

	lunchTime := parseTimeStr("12:00", today)
	dinnerTime := parseTimeStr("18:30", today)
	hasNap := false
	var napStart, napEnd *time.Time

	var events []FixedEvent
	s.db.Where("enabled = 1").Find(&events)
	for _, e := range events {
		switch e.EventType {
		case "meal_lunch":
			if e.StartTime != "" {
				lunchTime = parseTimeStr(e.StartTime, today)
			}
		case "meal_dinner":
			if e.StartTime != "" {
				dinnerTime = parseTimeStr(e.StartTime, today)
			}
		case "nap":
			if e.StartTime != "" && e.EndTime != "" {
				ns := parseTimeStr(e.StartTime, today)
				ne := parseTimeStr(e.EndTime, today)
				napStart = &ns
				napEnd = &ne
				hasNap = true
			}
		}
	}

	var lt LifestyleTendency
	if err := s.db.Where("character_id = ?", characterID).Limit(1).First(&lt); err == nil {
		if lt.ActivityEnergy < 30 {
			if wakeTime.Hour() < 7 {
				wakeTime = wakeTime.Add(30 * time.Minute)
			}
		} else if lt.ActivityEnergy > 70 {
			if wakeTime.Hour() > 6 {
				wakeTime = wakeTime.Add(-15 * time.Minute)
			}
		}
	}

	isRestDay := false
	var specials []SpecialEvent
	s.db.Where("enabled = 1 AND start_date = ? AND character_id = ?", date, characterID).Find(&specials)
	for _, sp := range specials {
		if sp.EventType == "rest_day" || sp.StartTime == "" || (sp.StartTime == "00:00" && sp.EndTime == "23:59") {
			isRestDay = true
			break
		}
	}

	return TodaySchedule{
		WakeTime:     wakeTime,
		LunchTime:    lunchTime,
		DinnerTime:   dinnerTime,
		HasNap:       hasNap,
		NapStartTime: napStart,
		NapEndTime:   napEnd,
		SleepTime:    bedTime,
		IsRestDay:    isRestDay,
	}
}

func parseDayOfWeek(date string) int {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return int(time.Now().Weekday())
	}
	return int(t.Weekday())
}

func parseDate(date string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return time.Now()
	}
	return t
}

func parseTimeStr(t string, date time.Time) time.Time {
	parts := splitTimeRange(t)
	if len(parts) < 2 {
		parts = []string{"08", "00"}
	}
	h := 0
	m := 0
	fmt.Sscanf(parts[0], "%d", &h)
	fmt.Sscanf(parts[1], "%d", &m)
	return time.Date(date.Year(), date.Month(), date.Day(), h, m, 0, 0, time.Local)
}

func splitTimeRange(s string) []string {
	for _, sep := range []string{":", "-"} {
		if idx := indexOf(s, sep); idx >= 0 {
			if sep == ":" {
				parts := []string{}
				for _, p := range []string{s[:idx], s[idx+1:]} {
					p2 := ""
					for _, sep2 := range []string{"-"} {
						if idx2 := indexOf(p, sep2); idx2 >= 0 {
							parts = append(parts, p[:idx2], p[idx2+1:])
							p2 = ""
							break
						} else {
							p2 = p
						}
					}
					if p2 != "" {
						parts = append(parts, p2)
					}
				}
				if len(parts) >= 2 {
					return parts
				}
			}
			return []string{s[:idx], s[idx+1:]}
		}
	}
	return []string{s}
}
