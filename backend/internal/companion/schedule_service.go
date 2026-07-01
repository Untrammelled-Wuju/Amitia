// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"fmt"
	"log"
	"time"
)

func (s *service) GetSchedule(date string, characterID string) map[string]interface{} {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	return scheduleToMap(s.buildTodaySchedule(date, characterID))
}

func (s *service) GetScheduleConflicts(date string, characterID string) []map[string]interface{} {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	schedule := s.buildTodaySchedule(date, characterID)
	timeline := s.buildTimeline(date, schedule, characterID)

	type conflict struct {
		Type      string `json:"type"`
		Level     string `json:"level"`
		Message   string `json:"message"`
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
		SourceA   string `json:"sourceA"`
		SourceB   string `json:"sourceB"`
	}

	var conflicts []conflict
	add := func(c conflict) { conflicts = append(conflicts, c) }
	for i := 0; i < len(timeline); i++ {
		for j := i + 1; j < len(timeline); j++ {
			a, b := timeline[i], timeline[j]
			if a.EndTime.Before(b.StartTime) || a.EndTime.Equal(b.StartTime) {
				continue
			}
			if b.EndTime.Before(a.StartTime) || b.EndTime.Equal(a.StartTime) {
				continue
			}
			level := "warning"
			msg := fmt.Sprintf("%s 与 %s 时间重叠", a.Reason, b.Reason)
			if a.State == "SLEEPING" && (b.State == "IN_EXAM" || b.State == "IN_CLASS") {
				level = "error"
				msg = fmt.Sprintf("睡眠时间与%s冲突", b.Reason)
			}
			add(conflict{
				Type: "time_overlap", Level: level, Message: msg,
				StartTime: a.StartTime.Format("2006-01-02T15:04:05"),
				EndTime:   a.EndTime.Format("2006-01-02T15:04:05"),
				SourceA:   a.State, SourceB: b.State,
			})
		}
	}

	if schedule.HasNap && schedule.NapStartTime != nil && schedule.NapEndTime != nil {
		for _, e := range timeline {
			if e.State == "SLEEPING" {
				continue
			}
			ns := *schedule.NapStartTime
			ne := *schedule.NapEndTime
			if e.StartTime.Before(ne) && e.EndTime.After(ns) {
				add(conflict{
					Type: "time_overlap", Level: "warning",
					Message:   fmt.Sprintf("午睡时间与%s重叠", e.Reason),
					StartTime: ns.Format("2006-01-02T15:04:05"),
					EndTime:   ne.Format("2006-01-02T15:04:05"),
					SourceA:   "nap", SourceB: e.State,
				})
			}
		}
	}

	result := make([]map[string]interface{}, len(conflicts))
	for i, c := range conflicts {
		result[i] = map[string]interface{}{
			"type": c.Type, "level": c.Level, "message": c.Message,
			"startTime": c.StartTime, "endTime": c.EndTime,
			"sourceA": c.SourceA, "sourceB": c.SourceB,
		}
	}
	if result == nil {
		result = []map[string]interface{}{}
	}
	return result
}

func (s *service) GetScheduleToday(characterID string) map[string]interface{} {
	return s.GetSchedule(time.Now().Format("2006-01-02"), characterID)
}

func (s *service) scheduleChanged() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Companion] scheduleChanged panic recovered: %v", r)
		}
	}()
	var charIDs []string
	s.db.Table("characters").Pluck("id", &charIDs)
	for _, cid := range charIDs {
		s.ScheduleBasedGenerator(time.Now().Format("2006-01-02"), cid)
	}
}
