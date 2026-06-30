// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/embedding"
	"github.com/u-ai/backend/pkg/app"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
)

type Service interface {
	GetSleepSetting(characterID string) map[string]interface{}
	UpdateSleepSetting(body map[string]interface{}, characterID string) map[string]interface{}
	GetSchedule(date string, characterID string) map[string]interface{}
	GetScheduleConflicts(date string, characterID string) []map[string]interface{}
	GetScheduleToday(characterID string) map[string]interface{}
	GetStateLife(characterID string) map[string]interface{}
	GetState(characterID string) map[string]interface{}
	GetTimelineToday(characterID string) map[string]interface{}
	ListFixedEvents(date string, characterID string) []map[string]interface{}
	GetFixedEvent(id int) map[string]interface{}
	CreateFixedEvent(body map[string]interface{}, characterID string) map[string]interface{}
	UpdateFixedEvent(id int, body map[string]interface{}, characterID string) map[string]interface{}
	DeleteFixedEvent(id int, characterID string) bool
	ToggleFixedEventEnabled(id int) map[string]interface{}
	ListSpecialEvents(characterID string) []map[string]interface{}
	CreateSpecialEvent(body map[string]interface{}, characterID string) map[string]interface{}
	UpdateSpecialEvent(id int, body map[string]interface{}, characterID string) map[string]interface{}
	DeleteSpecialEvent(id int, characterID string) bool
	ToggleSpecialEventEnabled(id int) map[string]interface{}
	ListClassAdjustments(characterID string) []map[string]interface{}
	CreateClassAdjustment(body map[string]interface{}, characterID string) map[string]interface{}
	UpdateClassAdjustment(id int, body map[string]interface{}, characterID string) map[string]interface{}
	DeleteClassAdjustment(id int, characterID string) bool
	GetEffectiveClasses(date string, characterID string) []map[string]interface{}
	GetLifestyleTendency(characterID string) map[string]interface{}
	UpdateLifestyleTendency(body map[string]interface{}, characterID string) map[string]interface{}
	ResetLifestyleTendency(characterID string) map[string]interface{}
	GetWorkProfile(characterID string) map[string]interface{}
	UpdateWorkProfile(body map[string]interface{}, characterID string) map[string]interface{}
	GetActiveMessageSetting(characterID string) map[string]interface{}
	UpdateActiveMessageSetting(body map[string]interface{}, characterID string) map[string]interface{}
	GetActiveMessageTasksToday(characterID string) []map[string]interface{}
	RegenerateActiveMessageTasks(characterID string) map[string]interface{}
	RunActiveMessageTask(id int, characterID string) map[string]interface{}
	CancelActiveMessageTask(id int, characterID string) map[string]interface{}
	ListDelayedReplies(characterID string) []map[string]interface{}
	CancelDelayedReply(id int, characterID string) map[string]interface{}
	ProcessDelayedReplies(characterID string) map[string]interface{}
	ProcessDueActiveMessageTasks(characterID string) map[string]interface{}
	GetDebugOverview(characterID string) map[string]interface{}
	RegenerateAllDebug(characterID string) map[string]interface{}
	ProcessActiveMessagesDebug(characterID string) map[string]interface{}
	ProcessDelayedRepliesDebug(characterID string) map[string]interface{}
	GetRuleLogs(characterID string) []map[string]interface{}
	RegenerateSchedule(characterID string) map[string]interface{}
	RegenerateTimeline(characterID string) map[string]interface{}
	ScheduleBasedGenerator(date string, characterID string) map[string]interface{}
	GenerateSharePrompt(taskType string, schedule TodaySchedule, mood string, energy int) string
	GetShareHistory() ShareHistory
	TriggerDailyRegeneration(characterID string) map[string]interface{}
	RandomBurstTrigger(characterID string) map[string]interface{}
}

type service struct {
	db              *gorm.DB
	embeddingSvc    *embedding.Service
	lastBurstAt     time.Time
	todayBurstCount int
}

func NewService(ctx *app.AppContext) Service {
	return &service{db: ctx.DB, embeddingSvc: embedding.NewService(ctx.DB)}
}

func (s *service) getSetting(key string) string {
	var v string
	s.db.Table("app_settings").Select("value").Where("key = ?", key).Row().Scan(&v)
	return v
}

func (s *service) setSetting(key, value string) {
	s.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, datetime('now', 'localtime')) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at", key, value)
}

func (s *service) GetSleepSetting(characterID string) map[string]interface{} {
	var bed, wake string
	var enabled, sleepReplyEnabled int
	var sleepReplyMode string
	err := s.db.Table("sleep_settings").Select("bed_time, wake_time, enabled, COALESCE(sleep_reply_enabled, 0) as sleep_reply_enabled, COALESCE(sleep_reply_mode, 'NO_REPLY') as sleep_reply_mode").Where("character_id = ?", characterID).Limit(1).Row().Scan(&bed, &wake, &enabled, &sleepReplyEnabled, &sleepReplyMode)
	if err != nil {
		return map[string]interface{}{"bedTime": "23:00", "wakeTime": "07:00", "enabled": true, "sleepReplyEnabled": false, "sleepReplyMode": "NO_REPLY"}
	}
	return map[string]interface{}{"bedTime": bed, "wakeTime": wake, "enabled": enabled == 1, "sleepReplyEnabled": sleepReplyEnabled == 1, "sleepReplyMode": sleepReplyMode}
}

func (s *service) UpdateSleepSetting(body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["bedTime"].(string); ok {
		updates["bed_time"] = v
	}
	if v, ok := body["wakeTime"].(string); ok {
		updates["wake_time"] = v
	}
	if v, ok := body["enabled"].(bool); ok {
		if v {
			updates["enabled"] = 1
		} else {
			updates["enabled"] = 0
		}
	}
	if v, ok := body["sleepReplyEnabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["sleep_reply_enabled"] = 1
			} else {
				updates["sleep_reply_enabled"] = 0
			}
		} else if f, ok2 := v.(float64); ok2 {
			updates["sleep_reply_enabled"] = int(f)
		}
	}
	if v, ok := body["sleepReplyMode"].(string); ok {
		updates["sleep_reply_mode"] = v
	}
	if len(updates) > 0 {
		var c64 int64
		s.db.Table("sleep_settings").Where("character_id = ?", characterID).Count(&c64)
		if c64 == 0 {
			s.db.Exec("INSERT INTO sleep_settings (character_id, bed_time, wake_time, enabled) VALUES (?, '23:00', '07:00', 1)", characterID)
		}
		s.db.Table("sleep_settings").Where("character_id = ?", characterID).Updates(updates)
		go s.scheduleChanged()
	}
	return s.GetSleepSetting(characterID)
}

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

func (s *service) getIdleDuration() time.Duration {
	var lastAt string
	err := s.db.Table("messages").Select("created_at").Where("role = 'user'").Order("created_at DESC").Limit(1).Row().Scan(&lastAt)
	if err != nil || lastAt == "" {
		return 0
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", lastAt, time.Local)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", lastAt)
	}
	if err != nil {
		return 24 * time.Hour
	}
	return time.Since(t)
}

func (s *service) GetStateLife(characterID string) map[string]interface{} {
	stateResult := s.GetState(characterID)
	currentState, _ := stateResult["currentState"].(string)
	if currentState == "" {
		currentState = "IDLE"
	}
	sleeping, _ := stateResult["sleeping"].(bool)
	busy, _ := stateResult["busy"].(bool)
	available, _ := stateResult["available"].(bool)
	stateStartedAt, _ := stateResult["stateStartedAt"].(string)
	stateEndsAt, _ := stateResult["stateEndsAt"].(string)

	mood := "neutral"
	var moods []map[string]interface{}
	s.db.Table("moods").Order("created_at DESC").Limit(1).Find(&moods)
	if len(moods) > 0 {
		if m, ok := moods[0]["mood"].(string); ok && m != "" {
			mood = m
		}
		if m, ok := moods[0]["mood_value"].(string); ok && m != "" {
			mood = m
		}
	}
	idleDuration := s.getIdleDuration()
	if idleDuration > 48*time.Hour {
		mood = "depressed"
	} else if idleDuration > 24*time.Hour {
		mood = "sad"
	} else if idleDuration > 12*time.Hour {
		mood = "ignored"
	} else if idleDuration > 6*time.Hour {
		mood = "lonely"
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	schedule := s.buildTodaySchedule(today, characterID)
	energy := calculateEnergy(now, schedule, currentState)

	var currentActivity string
	if currentState == "SLEEPING" {
		currentActivity = "正在睡觉"
	}
	if currentState == "WAKING_UP" {
		currentActivity = "刚睡醒"
	}
	if currentState == "EATING_LUNCH" || currentState == "EATING_DINNER" {
		currentActivity = "正在吃饭"
	}
	if currentState == "NAPPING" {
		currentActivity = "正在午睡"
	}
	if currentState == "WORKING" {
		currentActivity = "正在工作"
	}
	if currentState == "IN_CLASS" {
		currentActivity = "正在上课"
	}
	if currentState == "STUDYING" {
		currentActivity = "正在学习"
	}
	if currentState == "COMMUTING_TO_WORK" {
		currentActivity = "上班路上"
	}
	if currentState == "COMMUTING_HOME" {
		currentActivity = "下班路上"
	}
	if currentState == "BEFORE_SLEEP" {
		currentActivity = "准备睡觉"
	}
	if currentState == "IDLE" {
		currentActivity = "空闲中"
	}
	if currentState == "AFTER_WORK" {
		currentActivity = "下班放松"
	}
	if currentActivity == "" {
		currentActivity = currentState
	}

	sleep := s.GetSleepSetting(characterID)
	result := map[string]interface{}{
		"currentState":    currentState,
		"currentActivity": currentActivity,
		"mood":            mood,
		"energy":          energy,
		"idleDuration":    idleDuration.Seconds(),
		"sleeping":        sleeping,
		"busy":            busy,
		"available":       available,
		"sleepSetting":    sleep,
	}
	if stateStartedAt != "" {
		result["stateStartedAt"] = stateStartedAt
	}
	if stateEndsAt != "" {
		result["stateEndsAt"] = stateEndsAt
	}
	return result
}

func (s *service) GetState(characterID string) map[string]interface{} {
	now := time.Now()
	today := now.Format("2006-01-02")

	timelineRes := s.GetTimelineToday(characterID)
	entries, _ := timelineRes["events"].([]map[string]interface{})

	var matchedEntry map[string]interface{}
	for _, e := range entries {
		startStr, _ := e["startTime"].(string)
		endStr, _ := e["endTime"].(string)
		if startStr == "" || endStr == "" {
			continue
		}
		start, err1 := time.ParseInLocation("2006-01-02T15:04:05", startStr, time.Local)
		end, err2 := time.ParseInLocation("2006-01-02T15:04:05", endStr, time.Local)
		if err1 != nil || err2 != nil {
			continue
		}
		if (now.After(start) || now.Equal(start)) && now.Before(end) {
			matchedEntry = e
			break
		}
	}

	if matchedEntry != nil {
		state, _ := matchedEntry["state"].(string)
		reason, _ := matchedEntry["reason"].(string)
		startStr, _ := matchedEntry["startTime"].(string)
		endStr, _ := matchedEntry["endTime"].(string)
		return buildStateResult(state, reason, startStr, endStr)
	}

	schedule := s.buildTodaySchedule(today, characterID)
	wake := schedule.WakeTime
	sleep := schedule.SleepTime
	if sleep.Before(wake) || sleep.Equal(wake) {
		sleep = sleep.Add(24 * time.Hour)
	}

	if now.Before(wake) || (now.After(sleep) || now.Equal(sleep)) {
		return buildStateResult("SLEEPING", "睡眠时间",
			sleep.Format("2006-01-02T15:04:05"),
			wake.Format("2006-01-02T15:04:05"))
	}
	beforeSleep := sleep.Add(-1 * time.Hour)
	if now.After(beforeSleep) || now.Equal(beforeSleep) {
		return buildStateResult("BEFORE_SLEEP", "睡前准备",
			beforeSleep.Format("2006-01-02T15:04:05"),
			sleep.Format("2006-01-02T15:04:05"))
	}
	return buildStateResult("IDLE", "空闲时间",
		wake.Format("2006-01-02T15:04:05"),
		sleep.Format("2006-01-02T15:04:05"))
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

func (s *service) ListFixedEvents(date string, characterID string) []map[string]interface{} {
	var events []FixedEvent
	q := s.db.Where("character_id = ?", characterID)
	if date != "" {
		dayOfWeek := parseDayOfWeek(date)
		q = q.Where("(week_day = ? OR week_day = -1)", dayOfWeek)
	}
	q.Order("start_time").Find(&events)
	result := make([]map[string]interface{}, len(events))
	for i, e := range events {
		result[i] = map[string]interface{}{"id": e.ID, "title": e.Title, "description": e.Description, "weekDay": e.WeekDay, "startTime": e.StartTime, "endTime": e.EndTime, "eventType": e.EventType, "repeatDays": e.RepeatDays, "prepareMinMinutes": e.PrepareMinMinutes, "prepareMaxMinutes": e.PrepareMaxMinutes, "replyMode": e.ReplyMode, "enabled": e.Enabled == 1}
	}
	return result
}

func (s *service) GetFixedEvent(id int) map[string]interface{} {
	var e FixedEvent
	s.db.First(&e, id)
	return map[string]interface{}{"id": e.ID, "title": e.Title, "description": e.Description, "weekDay": e.WeekDay, "startTime": e.StartTime, "endTime": e.EndTime, "eventType": e.EventType, "repeatDays": e.RepeatDays, "prepareMinMinutes": e.PrepareMinMinutes, "prepareMaxMinutes": e.PrepareMaxMinutes, "replyMode": e.ReplyMode, "enabled": e.Enabled == 1}
}

func (s *service) CreateFixedEvent(body map[string]interface{}, characterID string) map[string]interface{} {
	title := ""
	if v, ok := body["title"].(string); ok {
		title = v
	} else {
		title = "新事件"
	}
	e := FixedEvent{Title: title, CharacterID: characterID, EventType: "CUSTOM_BUSY", Enabled: 1, ReplyMode: "SHORT_REPLY"}
	if v, ok := body["description"].(string); ok {
		e.Description = v
	}
	if v, ok := body["weekDay"].(float64); ok {
		e.WeekDay = int(v)
	}
	if v, ok := body["startTime"].(string); ok {
		e.StartTime = v
	}
	if v, ok := body["endTime"].(string); ok {
		e.EndTime = v
	}
	if v, ok := body["eventType"].(string); ok {
		e.EventType = v
	}
	if v, ok := body["repeatType"].(string); ok {
		e.RepeatType = v
	}
	if v, ok := body["repeatDays"].(string); ok {
		e.RepeatDays = v
	}
	if v, ok := body["prepareMinMinutes"].(float64); ok {
		e.PrepareMinMinutes = int(v)
	}
	if v, ok := body["prepareMaxMinutes"].(float64); ok {
		e.PrepareMaxMinutes = int(v)
	}
	if v, ok := body["replyMode"].(string); ok {
		e.ReplyMode = v
	}
	s.db.Create(&e)
	go s.scheduleChanged()
	return s.GetFixedEvent(e.ID)
}

func (s *service) UpdateFixedEvent(id int, body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["title"].(string); ok {
		updates["title"] = v
	}
	if v, ok := body["description"].(string); ok {
		updates["description"] = v
	}
	if v, ok := body["weekDay"].(float64); ok {
		updates["week_day"] = int(v)
	}
	if v, ok := body["startTime"].(string); ok {
		updates["start_time"] = v
	}
	if v, ok := body["endTime"].(string); ok {
		updates["end_time"] = v
	}
	if v, ok := body["eventType"].(string); ok {
		updates["event_type"] = v
	}
	if v, ok := body["repeatDays"].(string); ok {
		updates["repeat_days"] = v
	}
	if v, ok := body["prepareMinMinutes"].(float64); ok {
		updates["prepare_min_minutes"] = int(v)
	}
	if v, ok := body["prepareMaxMinutes"].(float64); ok {
		updates["prepare_max_minutes"] = int(v)
	}
	if v, ok := body["replyMode"].(string); ok {
		updates["reply_mode"] = v
	}
	if v, ok := body["enabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["enabled"] = 1
			} else {
				updates["enabled"] = 0
			}
		} else if f, ok2 := v.(float64); ok2 {
			updates["enabled"] = int(f)
		}
	}
	if len(updates) > 0 {
		s.db.Model(&FixedEvent{}).Where("id = ? AND character_id = ?", id, characterID).Updates(updates)
		go s.scheduleChanged()
	}
	return s.GetFixedEvent(id)
}

func (s *service) DeleteFixedEvent(id int, characterID string) bool {
	ok := s.db.Where("id = ? AND character_id = ?", id, characterID).Delete(&FixedEvent{}).RowsAffected > 0
	if ok {
		go s.scheduleChanged()
	}
	return ok
}

func (s *service) ToggleFixedEventEnabled(id int) map[string]interface{} {
	s.db.Model(&FixedEvent{}).Where("id = ?", id).Update("enabled", gorm.Expr("CASE WHEN enabled = 1 THEN 0 ELSE 1 END"))
	return s.GetFixedEvent(id)
}

func (s *service) ListSpecialEvents(characterID string) []map[string]interface{} {
	var events []SpecialEvent
	s.db.Where("character_id = ?", characterID).Order("start_date, start_time").Find(&events)
	result := make([]map[string]interface{}, len(events))
	for i, e := range events {
		result[i] = map[string]interface{}{"id": e.ID, "title": e.Title, "description": e.Description, "startDate": e.StartDate, "endDate": e.EndDate, "startTime": e.StartTime, "endTime": e.EndTime, "eventType": e.EventType, "enabled": e.Enabled == 1, "priority": e.Priority, "activeMessageAllowed": e.ActiveMessageAllowed == 1, "replyMode": e.ReplyMode, "affectSchedule": e.AffectSchedule == 1, "affectSleep": e.AffectSleep == 1, "affectMeal": e.AffectMeal == 1, "affectEnergy": e.AffectEnergy == 1}
	}
	return result
}

func (s *service) CreateSpecialEvent(body map[string]interface{}, characterID string) map[string]interface{} {
	title := ""
	if v, ok := body["title"].(string); ok {
		title = v
	} else {
		title = "特殊事件"
	}
	e := SpecialEvent{Title: title, CharacterID: characterID, EventType: "CUSTOM", Enabled: 1, ReplyMode: "SHORT_REPLY", ActiveMessageAllowed: 1}
	if v, ok := body["description"].(string); ok {
		e.Description = v
	}
	if v, ok := body["startDate"].(string); ok {
		e.StartDate = v
	}
	if v, ok := body["endDate"].(string); ok {
		e.EndDate = v
	}
	if v, ok := body["startTime"].(string); ok {
		e.StartTime = v
	}
	if v, ok := body["endTime"].(string); ok {
		e.EndTime = v
	}
	if v, ok := body["eventType"].(string); ok {
		e.EventType = v
	}
	if v, ok := body["replyMode"].(string); ok {
		e.ReplyMode = v
	}
	if v, ok := body["affectSleep"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				e.AffectSleep = 1
			}
		}
	}
	if v, ok := body["affectSchedule"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				e.AffectSchedule = 1
			}
		}
	}
	if v, ok := body["affectMeal"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				e.AffectMeal = 1
			}
		}
	}
	if v, ok := body["affectEnergy"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				e.AffectEnergy = 1
			}
		}
	}
	if v, ok := body["priority"].(float64); ok {
		e.Priority = int(v)
	}
	s.db.Create(&e)
	go s.scheduleChanged()
	return map[string]interface{}{"id": e.ID, "title": e.Title, "startDate": e.StartDate, "endDate": e.EndDate}
}

func (s *service) UpdateSpecialEvent(id int, body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["title"].(string); ok {
		updates["title"] = v
	}
	if v, ok := body["description"].(string); ok {
		updates["description"] = v
	}
	if v, ok := body["startDate"].(string); ok {
		updates["start_date"] = v
	}
	if v, ok := body["endDate"].(string); ok {
		updates["end_date"] = v
	}
	if v, ok := body["startTime"].(string); ok {
		updates["start_time"] = v
	}
	if v, ok := body["endTime"].(string); ok {
		updates["end_time"] = v
	}
	if v, ok := body["eventType"].(string); ok {
		updates["event_type"] = v
	}
	if v, ok := body["replyMode"].(string); ok {
		updates["reply_mode"] = v
	}
	if v, ok := body["affectSleep"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["affect_sleep"] = 1
			} else {
				updates["affect_sleep"] = 0
			}
		}
	}
	if v, ok := body["affectSchedule"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["affect_schedule"] = 1
			} else {
				updates["affect_schedule"] = 0
			}
		}
	}
	if v, ok := body["affectMeal"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["affect_meal"] = 1
			} else {
				updates["affect_meal"] = 0
			}
		}
	}
	if v, ok := body["affectEnergy"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["affect_energy"] = 1
			} else {
				updates["affect_energy"] = 0
			}
		}
	}
	if v, ok := body["priority"].(float64); ok {
		updates["priority"] = int(v)
	}
	if v, ok := body["enabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["enabled"] = 1
			} else {
				updates["enabled"] = 0
			}
		} else if f, ok2 := v.(float64); ok2 {
			updates["enabled"] = int(f)
		}
	}
	if len(updates) > 0 {
		s.db.Model(&SpecialEvent{}).Where("id = ? AND character_id = ?", id, characterID).Updates(updates)
		go s.scheduleChanged()
	}
	return map[string]interface{}{"id": id, "updated": true}
}

func (s *service) DeleteSpecialEvent(id int, characterID string) bool {
	ok := s.db.Where("id = ? AND character_id = ?", id, characterID).Delete(&SpecialEvent{}).RowsAffected > 0
	if ok {
		go s.scheduleChanged()
	}
	return ok
}

func (s *service) ToggleSpecialEventEnabled(id int) map[string]interface{} {
	s.db.Model(&SpecialEvent{}).Where("id = ?", id).Update("enabled", gorm.Expr("CASE WHEN enabled = 1 THEN 0 ELSE 1 END"))
	return map[string]interface{}{"id": id, "toggled": true}
}

func (s *service) ListClassAdjustments(characterID string) []map[string]interface{} {
	var items []ClassAdjustment
	s.db.Where("character_id = ?", characterID).Order("date, slot_index").Find(&items)
	result := make([]map[string]interface{}, len(items))
	for i, a := range items {
		result[i] = map[string]interface{}{"id": a.ID, "date": a.Date, "slotIndex": a.SlotIndex, "className": a.ClassName, "adjustType": a.AdjustType, "description": a.Description}
	}
	return result
}

func (s *service) CreateClassAdjustment(body map[string]interface{}, characterID string) map[string]interface{} {
	a := ClassAdjustment{AdjustType: "swap"}
	if v, ok := body["date"].(string); ok {
		a.Date = v
	}
	if v, ok := body["slotIndex"].(float64); ok {
		a.SlotIndex = int(v)
	}
	if v, ok := body["className"].(string); ok {
		a.ClassName = v
	}
	if v, ok := body["adjustType"].(string); ok {
		a.AdjustType = v
	}
	if v, ok := body["description"].(string); ok {
		a.Description = v
	}
	s.db.Create(&a)
	go s.scheduleChanged()
	return map[string]interface{}{"id": a.ID, "className": a.ClassName}
}

func (s *service) UpdateClassAdjustment(id int, body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["date"].(string); ok {
		updates["date"] = v
	}
	if v, ok := body["slotIndex"].(float64); ok {
		updates["slot_index"] = int(v)
	}
	if v, ok := body["className"].(string); ok {
		updates["class_name"] = v
	}
	if v, ok := body["adjustType"].(string); ok {
		updates["adjust_type"] = v
	}
	if v, ok := body["description"].(string); ok {
		updates["description"] = v
	}
	if len(updates) > 0 {
		s.db.Model(&ClassAdjustment{}).Where("id = ? AND character_id = ?", id, characterID).Updates(updates)
		go s.scheduleChanged()
	}
	return map[string]interface{}{"id": id, "updated": true}
}

func (s *service) DeleteClassAdjustment(id int, characterID string) bool {
	ok := s.db.Where("id = ? AND character_id = ?", id, characterID).Delete(&ClassAdjustment{}).RowsAffected > 0
	if ok {
		go s.scheduleChanged()
	}
	return ok
}

func (s *service) GetEffectiveClasses(date string, characterID string) []map[string]interface{} {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	type classSlot struct {
		Title          string `json:"title"`
		StartTime      string `json:"startTime"`
		EndTime        string `json:"endTime"`
		Location       string `json:"location"`
		SourceType     string `json:"sourceType"`
		AdjustmentType string `json:"adjustmentType"`
	}

	var adjustments []ClassAdjustment
	s.db.Where("date = ? AND character_id = ?", date, characterID).Order("slot_index ASC").Find(&adjustments)

	var slots []classSlot
	for _, adj := range adjustments {
		startHour := 8 + adj.SlotIndex
		startTime := fmt.Sprintf("%02d:00", startHour)
		endTime := fmt.Sprintf("%02d:50", startHour)
		slot := classSlot{
			Title:          adj.ClassName,
			StartTime:      startTime,
			EndTime:        endTime,
			Location:       "教室",
			SourceType:     "class_adjustment",
			AdjustmentType: adj.AdjustType,
		}
		if adj.AdjustType == "canceled" {
			continue
		}
		slots = append(slots, slot)
	}

	var specials []SpecialEvent
	s.db.Where("enabled = 1 AND start_date = ? AND character_id = ?", date, characterID).Find(&specials)
	for _, sp := range specials {
		if sp.EventType == "EXAM" || sp.EventType == "EXAM_WEEK" || sp.EventType == "LIBRARY_STUDY" {
			slots = append(slots, classSlot{
				Title: sp.Title, StartTime: sp.StartTime, EndTime: sp.EndTime,
				Location: "", SourceType: "special_event",
				AdjustmentType: sp.EventType,
			})
		}
	}

	sort.Slice(slots, func(i, j int) bool { return slots[i].StartTime < slots[j].StartTime })

	result := make([]map[string]interface{}, len(slots))
	for i, s := range slots {
		result[i] = map[string]interface{}{
			"title": s.Title, "startTime": s.StartTime, "endTime": s.EndTime,
			"location": s.Location, "sourceType": s.SourceType,
			"adjustmentType": s.AdjustmentType,
		}
	}
	if result == nil {
		result = []map[string]interface{}{}
	}
	return []map[string]interface{}{{"date": date, "dayOfWeek": parseDayOfWeek(date), "slots": result}}
}

func (s *service) GetLifestyleTendency(characterID string) map[string]interface{} {
	var t LifestyleTendency
	s.db.Table("lifestyle_tendencies").Where("character_id = ?", characterID).Limit(1).Find(&t)
	if t.ID == 0 {
		return map[string]interface{}{"punctualityTendency": 50, "earlyPrepareTendency": 50, "selfDisciplineTendency": 50, "sleepinessTendency": 50, "randomnessTendency": 50, "activityEnergy": 50, "socialEnergy": 50, "careTendency": 50, "dailyShareTendency": 50, "manuallyConfigured": false}
	}
	return map[string]interface{}{"id": t.ID, "punctualityTendency": t.PunctualityTendency, "earlyPrepareTendency": t.EarlyPrepareTendency, "selfDisciplineTendency": t.SelfDisciplineTendency, "sleepinessTendency": t.SleepinessTendency, "randomnessTendency": t.RandomnessTendency, "activityEnergy": t.ActivityEnergy, "socialEnergy": t.SocialEnergy, "careTendency": t.CareTendency, "dailyShareTendency": t.DailyShareTendency, "manuallyConfigured": t.ManuallyConfigured == 1}
}

func (s *service) UpdateLifestyleTendency(body map[string]interface{}, characterID string) map[string]interface{} {
	var count int64
	s.db.Model(&LifestyleTendency{}).Where("character_id = ?", characterID).Count(&count)
	if count == 0 {
		s.db.Create(&LifestyleTendency{CharacterID: characterID})
	}
	updates := make(map[string]interface{})
	result := map[string]interface{}{"punctualityTendency": 50, "earlyPrepareTendency": 50, "selfDisciplineTendency": 50, "sleepinessTendency": 50, "randomnessTendency": 50, "activityEnergy": 50, "socialEnergy": 50, "careTendency": 50, "dailyShareTendency": 50, "manuallyConfigured": false}
	if v, ok := body["punctualityTendency"].(float64); ok {
		updates["punctuality_tendency"] = int(v)
		result["punctualityTendency"] = int(v)
	}
	if v, ok := body["earlyPrepareTendency"].(float64); ok {
		updates["early_prepare_tendency"] = int(v)
		result["earlyPrepareTendency"] = int(v)
	}
	if v, ok := body["selfDisciplineTendency"].(float64); ok {
		updates["self_discipline_tendency"] = int(v)
		result["selfDisciplineTendency"] = int(v)
	}
	if v, ok := body["sleepinessTendency"].(float64); ok {
		updates["sleepiness_tendency"] = int(v)
		result["sleepinessTendency"] = int(v)
	}
	if v, ok := body["randomnessTendency"].(float64); ok {
		updates["randomness_tendency"] = int(v)
		result["randomnessTendency"] = int(v)
	}
	if v, ok := body["activityEnergy"].(float64); ok {
		updates["activity_energy"] = int(v)
		result["activityEnergy"] = int(v)
	}
	if v, ok := body["socialEnergy"].(float64); ok {
		updates["social_energy"] = int(v)
		result["socialEnergy"] = int(v)
	}
	if v, ok := body["careTendency"].(float64); ok {
		updates["care_tendency"] = int(v)
		result["careTendency"] = int(v)
	}
	if v, ok := body["dailyShareTendency"].(float64); ok {
		updates["daily_share_tendency"] = int(v)
		result["dailyShareTendency"] = int(v)
	}
	if v, ok := body["manuallyConfigured"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["manually_configured"] = 1
				result["manuallyConfigured"] = true
			} else {
				updates["manually_configured"] = 0
				result["manuallyConfigured"] = false
			}
		}
	}
	if len(updates) > 0 {
		s.db.Model(&LifestyleTendency{}).Where("character_id = ?", characterID).Updates(updates)
		go s.scheduleChanged()
	}
	return result
}

func (s *service) ResetLifestyleTendency(characterID string) map[string]interface{} {
	s.db.Where("character_id = ?", characterID).Delete(&LifestyleTendency{})
	return s.GetLifestyleTendency(characterID)
}

func (s *service) GetWorkProfile(characterID string) map[string]interface{} {
	var w WorkProfile
	s.db.Table("work_profiles").Where("character_id = ?", characterID).Limit(1).Find(&w)
	if w.ID == 0 {
		return map[string]interface{}{"enabled": false, "workDays": "MON,TUE,WED,THU,FRI", "workStartTime": "09:00", "workEndTime": "18:00", "lunchBreakStartTime": "12:00", "lunchBreakEndTime": "13:30", "commuteMinMinutes": 15, "commuteMaxMinutes": 45, "prepareMinMinutes": 20, "prepareMaxMinutes": 60, "replyMode": "SHORT_REPLY", "allowOvertime": false, "overtimeProbability": 10, "overtimeMinMinutes": 30, "overtimeMaxMinutes": 180, "overtimeReplyMode": "SHORT_REPLY", "delayedReplyEnabled": false, "commuteHomeShareEnabled": true, "commuteHomeShareProbability": 60}
	}
	return map[string]interface{}{"id": w.ID, "enabled": w.Enabled == 1, "workDays": w.WorkDays, "workStartTime": w.WorkStartTime, "workEndTime": w.WorkEndTime, "lunchBreakStartTime": w.LunchBreakStartTime, "lunchBreakEndTime": w.LunchBreakEndTime, "commuteMinMinutes": w.CommuteMinMinutes, "commuteMaxMinutes": w.CommuteMaxMinutes, "prepareMinMinutes": w.PrepareMinMinutes, "prepareMaxMinutes": w.PrepareMaxMinutes, "replyMode": w.ReplyMode, "allowOvertime": w.AllowOvertime == 1, "overtimeProbability": w.OvertimeProbability, "overtimeMinMinutes": w.OvertimeMinMinutes, "overtimeMaxMinutes": w.OvertimeMaxMinutes, "overtimeReplyMode": w.OvertimeReplyMode, "delayedReplyEnabled": w.DelayedReplyEnabled == 1, "commuteHomeShareEnabled": w.CommuteHomeShareEnabled == 1, "commuteHomeShareProbability": w.CommuteHomeShareProbability}
}

func (s *service) UpdateWorkProfile(body map[string]interface{}, characterID string) map[string]interface{} {
	var count int64
	s.db.Model(&WorkProfile{}).Where("character_id = ?", characterID).Count(&count)
	if count == 0 {
		s.db.Create(&WorkProfile{CharacterID: characterID})
	}
	updates := make(map[string]interface{})
	if v, ok := body["enabled"].(bool); ok {
		if v {
			updates["enabled"] = 1
		} else {
			updates["enabled"] = 0
		}
	}
	if v, ok := body["workDays"].(string); ok {
		updates["work_days"] = v
	}
	if v, ok := body["workStartTime"].(string); ok {
		updates["work_start_time"] = v
	}
	if v, ok := body["workEndTime"].(string); ok {
		updates["work_end_time"] = v
	}
	if v, ok := body["lunchBreakStartTime"].(string); ok {
		updates["lunch_break_start_time"] = v
	}
	if v, ok := body["lunchBreakEndTime"].(string); ok {
		updates["lunch_break_end_time"] = v
	}
	if v, ok := body["commuteMinMinutes"].(float64); ok {
		updates["commute_min_minutes"] = int(v)
	}
	if v, ok := body["commuteMaxMinutes"].(float64); ok {
		updates["commute_max_minutes"] = int(v)
	}
	if v, ok := body["prepareMinMinutes"].(float64); ok {
		updates["prepare_min_minutes"] = int(v)
	}
	if v, ok := body["prepareMaxMinutes"].(float64); ok {
		updates["prepare_max_minutes"] = int(v)
	}
	if v, ok := body["replyMode"].(string); ok {
		updates["reply_mode"] = v
	}
	if v, ok := body["allowOvertime"].(bool); ok {
		if v {
			updates["allow_overtime"] = 1
		} else {
			updates["allow_overtime"] = 0
		}
	}
	if v, ok := body["overtimeProbability"].(float64); ok {
		updates["overtime_probability"] = int(v)
	}
	if v, ok := body["overtimeMinMinutes"].(float64); ok {
		updates["overtime_min_minutes"] = int(v)
	}
	if v, ok := body["overtimeMaxMinutes"].(float64); ok {
		updates["overtime_max_minutes"] = int(v)
	}
	if v, ok := body["overtimeReplyMode"].(string); ok {
		updates["overtime_reply_mode"] = v
	}
	if v, ok := body["delayedReplyEnabled"].(bool); ok {
		if v {
			updates["delayed_reply_enabled"] = 1
		} else {
			updates["delayed_reply_enabled"] = 0
		}
	}
	if v, ok := body["commuteHomeShareEnabled"].(bool); ok {
		if v {
			updates["commute_home_share_enabled"] = 1
		} else {
			updates["commute_home_share_enabled"] = 0
		}
	}
	if v, ok := body["commuteHomeShareProbability"].(float64); ok {
		updates["commute_home_share_probability"] = int(v)
	}
	if len(updates) > 0 {
		s.db.Model(&WorkProfile{}).Where("character_id = ?", characterID).Updates(updates)
		go s.scheduleChanged()
	}
	result := map[string]interface{}{"id": 0, "enabled": false, "workDays": "MON,TUE,WED,THU,FRI", "workStartTime": "09:00", "workEndTime": "18:00", "lunchBreakStartTime": "12:00", "lunchBreakEndTime": "13:30", "commuteMinMinutes": 15, "commuteMaxMinutes": 45, "prepareMinMinutes": 20, "prepareMaxMinutes": 60, "replyMode": "SHORT_REPLY", "allowOvertime": false, "overtimeProbability": 10, "overtimeMinMinutes": 30, "overtimeMaxMinutes": 180, "overtimeReplyMode": "SHORT_REPLY", "delayedReplyEnabled": false, "commuteHomeShareEnabled": true, "commuteHomeShareProbability": 60}
	if v, ok := body["enabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			result["enabled"] = b
		}
	}
	if v, ok := body["workDays"].(string); ok {
		result["workDays"] = v
	}
	if v, ok := body["workStartTime"].(string); ok {
		result["workStartTime"] = v
	}
	if v, ok := body["workEndTime"].(string); ok {
		result["workEndTime"] = v
	}
	if v, ok := body["lunchBreakStartTime"].(string); ok {
		result["lunchBreakStartTime"] = v
	}
	if v, ok := body["lunchBreakEndTime"].(string); ok {
		result["lunchBreakEndTime"] = v
	}
	if v, ok := body["commuteMinMinutes"].(float64); ok {
		result["commuteMinMinutes"] = int(v)
	}
	if v, ok := body["commuteMaxMinutes"].(float64); ok {
		result["commuteMaxMinutes"] = int(v)
	}
	if v, ok := body["prepareMinMinutes"].(float64); ok {
		result["prepareMinMinutes"] = int(v)
	}
	if v, ok := body["prepareMaxMinutes"].(float64); ok {
		result["prepareMaxMinutes"] = int(v)
	}
	if v, ok := body["replyMode"].(string); ok {
		result["replyMode"] = v
	}
	if v, ok := body["allowOvertime"]; ok {
		if b, ok2 := v.(bool); ok2 {
			result["allowOvertime"] = b
		}
	}
	if v, ok := body["overtimeProbability"].(float64); ok {
		result["overtimeProbability"] = int(v)
	}
	if v, ok := body["overtimeMinMinutes"].(float64); ok {
		result["overtimeMinMinutes"] = int(v)
	}
	if v, ok := body["overtimeMaxMinutes"].(float64); ok {
		result["overtimeMaxMinutes"] = int(v)
	}
	if v, ok := body["overtimeReplyMode"].(string); ok {
		result["overtimeReplyMode"] = v
	}
	if v, ok := body["delayedReplyEnabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			result["delayedReplyEnabled"] = b
		}
	}
	if v, ok := body["commuteHomeShareEnabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			result["commuteHomeShareEnabled"] = b
		}
	}
	if v, ok := body["commuteHomeShareProbability"].(float64); ok {
		result["commuteHomeShareProbability"] = int(v)
	}
	return result
}

func (s *service) GetActiveMessageSetting(characterID string) map[string]interface{} {
	var enabled, activeLevel, minInterval, maxPerDay, maxDailyCalls int
	var channel, quietStart, quietEnd string
	err := s.db.Table("active_message_settings").Select("enabled, COALESCE(active_level, 40) as active_level, min_interval, COALESCE(quiet_start, '23:00') as quiet_start, COALESCE(quiet_end, '07:00') as quiet_end, max_per_day, COALESCE(max_daily_calls, 10) as max_daily_calls, channel").Where("character_id = ?", characterID).Limit(1).Row().Scan(&enabled, &activeLevel, &minInterval, &quietStart, &quietEnd, &maxPerDay, &maxDailyCalls, &channel)
	if err != nil {
		return map[string]interface{}{"enabled": true, "activeLevel": 40, "quietStart": "23:00", "quietEnd": "07:00", "minInterval": 60, "maxPerDay": 6, "maxDailyCalls": 10, "channel": "all"}
	}
	if quietStart == "" {
		quietStart = "23:00"
	}
	if quietEnd == "" {
		quietEnd = "07:00"
	}
	if activeLevel == 0 {
		activeLevel = 40
	}
	return map[string]interface{}{"enabled": enabled == 1, "activeLevel": activeLevel, "quietStart": quietStart, "quietEnd": quietEnd, "minInterval": minInterval, "maxPerDay": maxPerDay, "maxDailyCalls": maxDailyCalls, "channel": channel}
}
func (s *service) UpdateActiveMessageSetting(body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["enabled"].(bool); ok {
		if v {
			updates["enabled"] = 1
		} else {
			updates["enabled"] = 0
		}
	}
	if v, ok := body["activeLevel"].(float64); ok {
		vv := int(v)
		if vv < 1 {
			vv = 1
		}
		if vv > 100 {
			vv = 100
		}
		updates["active_level"] = vv
	}
	if v, ok := body["minInterval"].(float64); ok {
		updates["min_interval"] = int(v)
	}
	if v, ok := body["quietStart"].(string); ok {
		updates["quiet_start"] = v
	}
	if v, ok := body["quietEnd"].(string); ok {
		updates["quiet_end"] = v
	}
	if v, ok := body["maxPerDay"].(float64); ok {
		updates["max_per_day"] = int(v)
	}
	if v, ok := body["maxDailyCalls"].(float64); ok {
		vv := int(v)
		if vv < 1 {
			vv = 1
		}
		if vv > 50 {
			vv = 50
		}
		updates["max_daily_calls"] = vv
	}
	if v, ok := body["channel"].(string); ok {
		updates["channel"] = v
	}
	if len(updates) > 0 {
		var count int64
		s.db.Table("active_message_settings").Where("character_id = ?", characterID).Count(&count)
		if count == 0 {
			s.db.Exec("INSERT INTO active_message_settings (character_id, enabled, active_level, min_interval, quiet_start, quiet_end, max_per_day, max_daily_calls, channel) VALUES (?, 1, 40, 60, '23:00', '07:00', 6, 10, 'all')", characterID)
		}
		s.db.Table("active_message_settings").Where("character_id = ?", characterID).Updates(updates)
	}
	return s.GetActiveMessageSetting(characterID)
}
func (s *service) GetActiveMessageTasksToday(characterID string) []map[string]interface{} {
	var raw []map[string]interface{}
	s.db.Table("active_message_task").Where("date(due_time) = date('now', 'localtime') AND character_id = ?", characterID).Order("due_time ASC").Find(&raw)
	tasks := make([]map[string]interface{}, len(raw))
	for i, r := range raw {
		tasks[i] = map[string]interface{}{
			"id":           r["id"],
			"taskType":     r["task_type"],
			"dueTime":      r["due_time"],
			"status":       r["status"],
			"prompt":       r["prompt"],
			"cancelReason": r["cancel_reason"],
			"retryCount":   r["retry_count"],
			"source":       r["source"],
			"createdAt":    r["created_at"],
			"updatedAt":    r["updated_at"],
			"lockUntil":    r["lock_until"],
			"maxRetry":     r["max_retry"],
			"sendResult":   r["send_result"],
			"payload":      r["payload"],
		}
	}
	if tasks == nil {
		tasks = []map[string]interface{}{}
	}
	return tasks
}

func (s *service) RegenerateActiveMessageTasks(characterID string) map[string]interface{} {
	s.db.Exec("UPDATE active_message_task SET status='CANCELLED', cancel_reason='regenerate', updated_at=datetime('now', 'localtime') WHERE date(due_time)=date('now', 'localtime') AND status='PENDING' AND character_id = ?", characterID)
	return map[string]interface{}{"regenerated": true}
}

func (s *service) RunActiveMessageTask(id int, characterID string) map[string]interface{} {
	var task map[string]interface{}
	s.db.Table("active_message_task").Where("id = ? AND character_id = ?", id, characterID).Limit(1).Find(&task)
	if len(task) == 0 {
		return map[string]interface{}{"id": id, "status": "NOT_FOUND"}
	}
	prompt, _ := task["prompt"].(string)
	taskType, _ := task["task_type"].(string)
	if prompt == "" {
		return map[string]interface{}{"id": id, "status": "NO_PROMPT"}
	}
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	messageCreatedAt := nowStr
	if dueTime, ok := task["due_time"].(string); ok && dueTime != "" {
		messageCreatedAt = dueTime
	}
	msgID := fmt.Sprintf("proactive-%d", now.UnixNano())
	generated := s.generateLLMReply(prompt)
	if generated == "" {
		switch taskType {
		case "morning_share":
			generated = "早上好！新的一天开始了。"
		case "noon_daily":
			generated = "午安，记得按时吃饭哦。"
		case "evening_reflection":
			generated = "傍晚好，今天辛苦了。"
		case "bedtime_mood":
			generated = "夜深了，早点休息。"
		default:
			generated = "你好呀！"
		}
	}
	convRow := s.db.Table("conversations").Select("id").Limit(1).Row()
	var convID string
	convRow.Scan(&convID)
	var channelSetting string
	s.db.Table("active_message_settings").Select("COALESCE(channel, 'all')").Where("character_id = ?", characterID).Limit(1).Row().Scan(&channelSetting)
	if channelSetting == "" {
		channelSetting = "all"
	}
	s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?)", msgID, convID, generated, messageCreatedAt)
	s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, ?, 'sent', ?, ?)", convID, generated, channelSetting, nowStr, nowStr)
	s.db.Exec("UPDATE active_message_task SET status='SENT', sent_at=?, updated_at=datetime('now', 'localtime') WHERE id=? AND character_id=?", nowStr, id, characterID)

	if s.isDefaultCharacter(characterID) {
		if strings.Contains(channelSetting, "wechat") || channelSetting == "all" {
			wcID := s.getWechatConvIDForChar(characterID)
			if wcID != "" {
				s.sendToWechatSidecar(wcID, generated)
			}
		}
		if strings.Contains(channelSetting, "qq") || channelSetting == "all" {
			qqID := s.getQQConvIDForChar(characterID)
			if qqID != "" {
				s.sendToQQSidecar(qqID, generated)
			}
		}
	}

	log.Printf("[Companion] RunActiveMessageTask sent type=%s id=%d channel=%s", taskType, id, channelSetting)
	return map[string]interface{}{"id": id, "status": "SENT", "taskType": taskType, "channel": channelSetting}
}

func (s *service) CancelActiveMessageTask(id int, characterID string) map[string]interface{} {
	s.db.Exec("UPDATE active_message_task SET status='CANCELLED', cancel_reason='manual', updated_at=datetime('now', 'localtime') WHERE id=? AND character_id=?", id, characterID)
	return map[string]interface{}{"id": id, "cancelled": true}
}

func (s *service) ListDelayedReplies(characterID string) []map[string]interface{} {
	var raw []map[string]interface{}
	q := s.db.Table("delayed_replies").Where("status = 'pending'")
	if characterID != "" {
		q = q.Where("character_id = ?", characterID)
	}
	q.Order("scheduled_at ASC").Find(&raw)
	replies := make([]map[string]interface{}, len(raw))
	for i, r := range raw {
		triggerState := "delay"
		if ch, _ := r["channel"].(string); ch != "" {
			triggerState = ch
		}
		replies[i] = map[string]interface{}{
			"id":                 r["id"],
			"status":             r["status"],
			"triggerState":       triggerState,
			"userMessage":        r["content"],
			"expectedReplyAfter": r["scheduled_at"],
			"channel":            r["channel"],
		}
	}
	if replies == nil {
		replies = []map[string]interface{}{}
	}
	return replies
}

func (s *service) CancelDelayedReply(id int, characterID string) map[string]interface{} {
	s.db.Exec("UPDATE delayed_replies SET status='cancelled', updated_at=datetime('now', 'localtime') WHERE id=? AND character_id=?", id, characterID)
	return map[string]interface{}{"id": id, "cancelled": true}
}

func (s *service) ProcessDelayedReplies(characterID string) map[string]interface{} {
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")

	var tasks []map[string]interface{}
	s.db.Table("delayed_replies").Where("status = 'pending' AND scheduled_at <= ? AND character_id = ?", nowStr, characterID).
		Order("scheduled_at ASC").Limit(20).Find(&tasks)

	var processed, sent, delayed, failed int

	for _, t := range tasks {
		processed++
		id, _ := t["id"]
		content, _ := t["content"].(string)
		convID, _ := t["conversation_id"].(string)
		channel, _ := t["channel"].(string)

		if content == "" {
			continue
		}

		canSend := true
		stateResult := s.GetState(characterID)
		currentState, _ := stateResult["currentState"].(string)

		if currentState == "SLEEPING" || currentState == "NAPPING" {
			canSend = false
			schedule := s.buildTodaySchedule(now.Format("2006-01-02"), characterID)
			wakeTime := schedule.WakeTime
			if currentState == "NAPPING" && schedule.NapEndTime != nil {
				wakeTime = *schedule.NapEndTime
			}
			if wakeTime.Before(now) {
				wakeTime = wakeTime.Add(24 * time.Hour)
			}
			s.db.Exec("UPDATE delayed_replies SET scheduled_at = ?, updated_at = datetime('now', 'localtime') WHERE id = ?",
				wakeTime.Format("2006-01-02 15:04:05"), id)
			delayed++
		} else if currentState == "IN_CLASS" || currentState == "IN_EXAM" || currentState == "BUSY" {
			canSend = false
			delayMin := 10 + rand.Intn(21)
			newTime := now.Add(time.Duration(delayMin) * time.Minute)
			s.db.Exec("UPDATE delayed_replies SET scheduled_at = ?, updated_at = datetime('now', 'localtime') WHERE id = ?",
				newTime.Format("2006-01-02 15:04:05"), id)
			delayed++
		}

		if canSend {
			if convID == "" {
				row := s.db.Table("conversations").Select("id").Limit(1).Row()
				row.Scan(&convID)
			}
			if convID == "" {
				failed++
				continue
			}
			if channel == "" {
				channel = "web"
			}

			msgID := fmt.Sprintf("reply-%d", now.UnixNano())
			displayContent := "💬 " + content
			err := s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'delayed_reply', 'normal', 'sent', 1, ?)",
				msgID, convID, displayContent, nowStr).Error
			if err != nil {
				retryCount := 0
				if rc, ok := t["retry_count"]; ok {
					switch v := rc.(type) {
					case int64:
						retryCount = int(v)
					case float64:
						retryCount = int(v)
					}
				}
				retryCount++
				if retryCount >= 3 {
					s.db.Exec("UPDATE delayed_replies SET status='FAILED', retry_count=?, updated_at=datetime('now', 'localtime') WHERE id = ?", retryCount, id)
					failed++
				} else {
					s.db.Exec("UPDATE delayed_replies SET retry_count=?, updated_at=datetime('now', 'localtime') WHERE id = ?", retryCount, id)
				}
			} else {
				s.db.Exec("UPDATE delayed_replies SET status='SENT', sent_at=?, updated_at=datetime('now', 'localtime') WHERE id = ?", nowStr, id)
				sendProactiveNotification(s.db, convID, msgID, content)
				sent++
			}
		}
	}

	return map[string]interface{}{
		"processed": processed,
		"sent":      sent,
		"delayed":   delayed,
		"failed":    failed,
	}
}

func (s *service) GetDebugOverview(characterID string) map[string]interface{} {

	now := time.Now()

	nowStr := now.Format("2006-01-02 15:04:05")

	schedule := s.GetScheduleToday(characterID)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	lt := s.GetLifestyleTendency(characterID)
	intensity := 50
	if v, ok := lt["intensity"].(int); ok {
		intensity = v
	}
	if v, ok := lt["intensity"].(float64); ok {
		intensity = int(v)
	}
	jitterMax := intensity / 10
	if jitterMax < 2 {
		jitterMax = 2
	}
	if jitterMax > 15 {
		jitterMax = 15
	}
	if st, ok := schedule["wakeTime"].(string); ok {
		if t, err := time.ParseInLocation("2006-01-02T15:04:05", st, time.Local); err == nil {
			off := time.Duration(rng.Intn(jitterMax*2+1)-jitterMax) * time.Minute
			schedule["wakeTime"] = t.Add(off).Format("2006-01-02T15:04:05")
		}
	}
	if st, ok := schedule["lunchTime"].(string); ok {
		if t, err := time.ParseInLocation("2006-01-02T15:04:05", st, time.Local); err == nil {
			off := time.Duration(rng.Intn(jitterMax*2+1)-jitterMax) * time.Minute
			schedule["lunchTime"] = t.Add(off).Format("2006-01-02T15:04:05")
		}
	}
	if st, ok := schedule["dinnerTime"].(string); ok {
		if t, err := time.ParseInLocation("2006-01-02T15:04:05", st, time.Local); err == nil {
			off := time.Duration(rng.Intn(jitterMax*2+1)-jitterMax) * time.Minute
			schedule["dinnerTime"] = t.Add(off).Format("2006-01-02T15:04:05")
		}
	}
	if st, ok := schedule["sleepTime"].(string); ok {
		if t, err := time.ParseInLocation("2006-01-02T15:04:05", st, time.Local); err == nil {
			off := time.Duration(rng.Intn(jitterMax*2+1)-jitterMax) * time.Minute
			schedule["sleepTime"] = t.Add(off).Format("2006-01-02T15:04:05")
		}
	}

	timeline := s.GetTimelineToday(characterID)

	currentState := s.GetState(characterID)

	stateLife := s.GetStateLife(characterID)

	activeMsgSetting := s.GetActiveMessageSetting(characterID)

	activeTasks := s.GetActiveMessageTasksToday(characterID)

	conflicts := s.GetScheduleConflicts("", characterID)

	effectiveClasses := s.GetEffectiveClasses("", characterID)

	var pendingReplies int64

	s.db.Table("delayed_replies").Where("status = 'pending'").Count(&pendingReplies)

	var todayTaskCount int64

	s.db.Table("active_message_task").Where("date(due_time) = date(?) AND character_id = ?", nowStr, characterID).Count(&todayTaskCount)

	var todaySentCount int64

	s.db.Table("active_message_task").Where("date(due_time) = date(?) AND status = 'SENT' AND character_id = ?", nowStr, characterID).Count(&todaySentCount)

	var todayLLMCalls int64

	s.db.Table("proactive_messages").Where("date(created_at) = date(?)", nowStr).Count(&todayLLMCalls)

	var maxDailyCalls int64

	s.db.Table("active_message_settings").Select("COALESCE(max_daily_calls, 10)").Where("character_id = ?", characterID).Limit(1).Row().Scan(&maxDailyCalls)

	if maxDailyCalls == 0 {
		maxDailyCalls = 10
	}

	delayedRepliesList := s.ListDelayedReplies(characterID)
	if delayedRepliesList == nil {
		delayedRepliesList = []map[string]interface{}{}
	}
	recentRuleLogs := s.GetRuleLogs(characterID)
	if recentRuleLogs == nil {
		recentRuleLogs = []map[string]interface{}{}
	}

	return map[string]interface{}{

		"now": nowStr,

		"todaySchedule": schedule,
		"schedule":      schedule,

		"timeline": timeline["events"],

		"currentState": currentState,

		"stateLife": stateLife,

		"activeMessageSetting": activeMsgSetting,

		"activeMessageTasks": activeTasks,

		"scheduleConflicts": conflicts,

		"effectiveClasses": effectiveClasses,

		"delayedReplies": delayedRepliesList,

		"recentRuleLogs": recentRuleLogs,

		"stats": map[string]interface{}{

			"todayTaskCount": todayTaskCount,

			"todaySentCount": todaySentCount,

			"todayLLMCalls": todayLLMCalls,

			"maxDailyCalls":     maxDailyCalls,
			"remainingLLMCalls": maxDailyCalls - todayLLMCalls,
		},
	}
}

func (s *service) RegenerateAllDebug(characterID string) map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	scheduleResult := s.RegenerateSchedule(characterID)
	s.ScheduleBasedGenerator(today, characterID)
	return map[string]interface{}{
		"regenerated": true,
		"schedule":    scheduleResult["schedule"],
		"timeline":    scheduleResult["timeline"],
		"taskCount":   len(s.GetActiveMessageTasksToday(characterID)),
	}
}
func (s *service) ProcessActiveMessagesDebug(characterID string) map[string]interface{} {
	return s.ProcessDueActiveMessageTasks(characterID)
}

func (s *service) ProcessDueActiveMessageTasks(characterID string) map[string]interface{} {
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	s.db.Exec("UPDATE active_message_task SET status='PENDING', lock_until=NULL, updated_at=datetime('now', 'localtime') WHERE status='PROCESSING' AND updated_at < datetime('now', 'localtime', '-5 minutes') AND character_id = ?", characterID)
	var tasks []map[string]interface{}
	s.db.Table("active_message_task").Where("status = 'PENDING' AND due_time <= ? AND character_id = ?", nowStr, characterID).Order("due_time ASC").Limit(20).Find(&tasks)
	var processed, sent, delayed, failed int
	var channelSetting string
	channelRow := s.db.Table("active_message_settings").Select("COALESCE(channel, 'all')").Where("character_id = ?", characterID).Limit(1).Row()
	channelRow.Scan(&channelSetting)
	if channelSetting == "" {
		channelSetting = "all"
	}
	for _, t := range tasks {
		processed++
		id, _ := t["id"]
		prompt, _ := t["prompt"].(string)
		if prompt == "" {
			continue
		}
		result := s.db.Exec("UPDATE active_message_task SET status='PROCESSING', lock_until=datetime('now', 'localtime', '+5 minutes') WHERE id = ? AND status='PENDING' AND character_id = ?", id, characterID)
		if result.RowsAffected == 0 {
			continue
		}
		stateResult := s.GetState(characterID)
		currentState, _ := stateResult["currentState"].(string)
		if currentState == "SLEEPING" || currentState == "IN_CLASS" || currentState == "IN_EXAM" || currentState == "BUSY" {
			delayMin := 10
			newDue := now.Add(time.Duration(delayMin) * time.Minute).Format("2006-01-02 15:04:05")
			s.db.Exec("UPDATE active_message_task SET status='PENDING', lock_until=NULL, due_time=?, updated_at=datetime('now', 'localtime') WHERE id=? AND character_id=?", newDue, id, characterID)
			delayed++
			continue
		}
		convRow := s.db.Table("conversations").Select("id").Limit(1).Row()
		var convID string
		convRow.Scan(&convID)
		if convID == "" {
			failed++
			continue
		}
		msgID := fmt.Sprintf("proactive-%d", now.UnixNano())
		generated := s.generateLLMReply(prompt)
		if generated == "" {
			failed++
			continue
		}
		displayContent := generated
		messageCreatedAt := nowStr
		if dueTime, ok := t["due_time"].(string); ok && dueTime != "" {
			messageCreatedAt = dueTime
		}
		insErr := s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?)", msgID, convID, displayContent, messageCreatedAt).Error
		if insErr != nil {
			retryCount := 0
			if rc, ok := t["retry_count"]; ok {
				switch v := rc.(type) {
				case int64:
					retryCount = int(v)
				case float64:
					retryCount = int(v)
				}
			}
			retryCount++
			if retryCount >= 3 {
				s.db.Exec("UPDATE active_message_task SET status='FAILED', retry_count=?, updated_at=datetime('now', 'localtime') WHERE id=? AND character_id=?", retryCount, id, characterID)
				failed++
			} else {
				newDue := now.Add(time.Duration(5*retryCount) * time.Minute).Format("2006-01-02 15:04:05")
				s.db.Exec("UPDATE active_message_task SET status='PENDING', lock_until=NULL, due_time=?, retry_count=?, updated_at=datetime('now', 'localtime') WHERE id=? AND character_id=?", newDue, retryCount, id, characterID)
				delayed++
			}
			continue
		}
		taskType, _ := t["task_type"].(string)
		s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, ?, 'sent', ?, ?)", convID, generated, channelSetting, nowStr, nowStr)
		s.db.Exec("UPDATE active_message_task SET status='SENT', sent_at=?, updated_at=datetime('now', 'localtime') WHERE id=? AND character_id=?", nowStr, id, characterID)
		log.Printf("[Companion] ProcessDueActiveMessageTasks sent type=%s id=%v", taskType, id)
		sent++

		if s.isDefaultCharacter(characterID) {
			if strings.Contains(channelSetting, "wechat") || channelSetting == "all" {
				wcID := s.getWechatConvIDForChar(characterID)
				if wcID != "" {
					s.sendToWechatSidecar(wcID, generated)
				}
			}
			if strings.Contains(channelSetting, "qq") || channelSetting == "all" {
				qqID := s.getQQConvIDForChar(characterID)
				if qqID != "" {
					s.sendToQQSidecar(qqID, generated)
				}
			}
		}
	}
	return map[string]interface{}{"processed": processed, "sent": sent, "delayed": delayed, "failed": failed}
}
func (s *service) ProcessDelayedRepliesDebug(characterID string) map[string]interface{} {
	return s.ProcessDelayedReplies(characterID)
}

func (s *service) GetRuleLogs(characterID string) []map[string]interface{} {
	var logs []map[string]interface{}
	q := s.db.Table("proactive_rule_logs")
	if characterID != "" {
		q = q.Where("character_id = ?", characterID)
	}
	q.Order("triggered_at DESC").Limit(50).Find(&logs)
	if logs == nil {
		logs = []map[string]interface{}{}
	}
	return logs
}

func (s *service) RegenerateSchedule(characterID string) map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	schedule := s.buildTodaySchedule(today, characterID)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	lt := s.GetLifestyleTendency(characterID)
	intensity := 50
	if v, ok := lt["intensity"].(int); ok {
		intensity = v
	}
	if v, ok := lt["intensity"].(float64); ok {
		intensity = int(v)
	}
	jitterMax := intensity / 10
	if jitterMax < 2 {
		jitterMax = 2
	}
	if jitterMax > 15 {
		jitterMax = 15
	}
	jitterMin := func(t time.Time, maxMin int) time.Time {
		off := time.Duration(rng.Intn(maxMin*2+1)-maxMin) * time.Minute
		return t.Add(off)
	}
	schedule.WakeTime = jitterMin(schedule.WakeTime, jitterMax)
	schedule.LunchTime = jitterMin(schedule.LunchTime, jitterMax)
	schedule.DinnerTime = jitterMin(schedule.DinnerTime, jitterMax)
	schedule.SleepTime = jitterMin(schedule.SleepTime, jitterMax)
	if schedule.HasNap && schedule.NapStartTime != nil && schedule.NapEndTime != nil {
		ns := jitterMin(*schedule.NapStartTime, jitterMax/2)
		ne := jitterMin(*schedule.NapEndTime, jitterMax/2)
		if ne.After(ns) {
			schedule.NapStartTime = &ns
			schedule.NapEndTime = &ne
		}
	}
	timeline := s.buildTimeline(today, schedule, characterID)
	timelineMaps := make([]map[string]interface{}, len(timeline))
	for i, e := range timeline {
		timelineMaps[i] = map[string]interface{}{
			"startTime":  e.StartTime.Format("2006-01-02T15:04:05"),
			"endTime":    e.EndTime.Format("2006-01-02T15:04:05"),
			"state":      e.State,
			"sourceType": e.SourceType,
			"priority":   e.Priority,
			"reason":     e.Reason,
		}
	}
	if timelineMaps == nil {
		timelineMaps = []map[string]interface{}{}
	}
	return map[string]interface{}{
		"schedule":    scheduleToMap(schedule),
		"timeline":    timelineMaps,
		"regenerated": true,
	}
}
func (s *service) RegenerateTimeline(characterID string) map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	schedule := s.buildTodaySchedule(today, characterID)
	timeline := s.buildTimeline(today, schedule, characterID)
	result := make([]map[string]interface{}, len(timeline))
	for i, e := range timeline {
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
	return map[string]interface{}{"events": result, "regenerated": true}
}

func parseDayOfWeek(date string) int {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return int(time.Now().Weekday())
	}
	return int(t.Weekday())
}
func toJSON(v interface{}) string { b, _ := json.Marshal(v); return string(b) }

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
	if err := s.db.Limit(1).First(&lt); err == nil {
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

func parseDate(date string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return time.Now()
	}
	return t
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

func parseWorkDays(s string) map[int]bool {
	result := map[int]bool{}
	parts := []string{}
	current := ""
	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	dayMap := map[string]int{"1": 1, "2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "0": 0, "7": 0}
	for _, p := range parts {
		p = trimSpace(p)
		if d, ok := dayMap[p]; ok {
			result[d] = true
			continue
		}
		if idx := indexOf(p, "-"); idx >= 0 {
			from := trimSpace(p[:idx])
			to := trimSpace(p[idx+1:])
			fd, fok := dayMap[from]
			td, tok := dayMap[to]
			if fok && tok {
				for d := fd; d <= td; d++ {
					result[d] = true
				}
			}
		}
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func buildStateResult(state, reason, startedAt, endsAt string) map[string]interface{} {
	sleeping := state == "SLEEPING" || state == "NAPPING"
	busy := state == "IN_CLASS" || state == "WORKING" || state == "IN_EXAM" || state == "BUSY" || state == "OVERTIME"
	available := state == "IDLE" || state == "AFTER_WORK" || state == "AFTER_CLASS" || state == "LIBRARY_BREAK" || state == "LUNCH_BREAK"
	result := map[string]interface{}{
		"state":          state,
		"currentState":   state,
		"sleeping":       sleeping,
		"busy":           busy,
		"available":      available,
		"reason":         reason,
		"stateStartedAt": startedAt,
		"stateEndsAt":    endsAt,
	}
	return result
}

func calculateEnergy(now time.Time, schedule TodaySchedule, currentState string) int {
	wake := schedule.WakeTime
	sleep := schedule.SleepTime
	if sleep.Before(wake) || sleep.Equal(wake) {
		sleep = sleep.Add(24 * time.Hour)
	}
	lunch := schedule.LunchTime
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	dinner := schedule.DinnerTime

	wakeHour := wake.Sub(today).Hours()
	if wakeHour > 24 {
		wakeHour -= 24
	}
	if wakeHour < 0 {
		wakeHour += 24
	}

	nowHour := float64(now.Hour()) + float64(now.Minute())/60.0
	if now.Before(today) {
		nowHour += 24
	}

	lunchHour := lunch.Sub(today).Hours()
	if lunchHour < 0 {
		lunchHour += 24
	}
	dinnerHour := dinner.Sub(today).Hours()
	if dinnerHour < 0 {
		dinnerHour += 24
	}
	sleepHour := sleep.Sub(today).Hours()

	if currentState == "SLEEPING" || currentState == "NAPPING" {
		return 10 + hashInt(now.Minute())%15
	}
	if currentState == "SICK_RESTING" || currentState == "LOW_ENERGY" || currentState == "LOW_ENERGY_AFTER_WORK" {
		return 10 + hashInt(now.Minute())%31
	}

	if nowHour >= wakeHour && nowHour < wakeHour+1 {
		return 60 + hashInt(now.Minute())%16
	}
	if nowHour >= wakeHour+1 && nowHour < lunchHour-1 {
		return 70 + hashInt(now.Hour()*60+now.Minute())%21
	}
	if nowHour >= lunchHour-1 && nowHour < lunchHour {
		return 50 + hashInt(now.Minute())%26
	}
	if schedule.HasNap && schedule.NapEndTime != nil {
		napEndHour := schedule.NapEndTime.Sub(today).Hours()
		if nowHour >= napEndHour && nowHour < napEndHour+2 {
			return 70 + hashInt(now.Minute())%21
		}
	}
	if nowHour >= lunchHour && nowHour < dinnerHour-1 {
		base := 65
		hoursSinceLunch := nowHour - lunchHour
		base -= int(hoursSinceLunch) * 3
		if base < 40 {
			base = 40
		}
		return base + hashInt(now.Minute())%16
	}
	if nowHour >= dinnerHour && nowHour < sleepHour-2 {
		return 50 + hashInt(now.Minute())%26
	}
	if nowHour >= sleepHour-2 {
		return 20 + hashInt(now.Minute())%26
	}
	return 50 + hashInt(now.Minute())%31
}

func hashInt(n int) int {
	n = ((n >> 16) ^ n) * 0x45d9f3b
	n = ((n >> 16) ^ n) * 0x45d9f3b
	n = (n >> 16) ^ n
	if n < 0 {
		n = -n
	}
	return n
}

func sendProactiveNotification(db *gorm.DB, convID, msgID, content string) {
	db.Exec("UPDATE conversations SET message_count=message_count+1, updated_at=datetime('now', 'localtime') WHERE id=?", convID)
}

func (s *service) ScheduleBasedGenerator(date string, characterID string) map[string]interface{} {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	today := parseDate(date)

	schedule := s.buildTodaySchedule(date, characterID)
	timeline := s.buildTimeline(date, schedule, characterID)
	stateLife := s.GetStateLife(characterID)
	mood, _ := stateLife["mood"].(string)
	if mood == "" {
		mood = "neutral"
	}
	energy, _ := stateLife["energy"].(int)

	lt := s.GetLifestyleTendency(characterID)
	dailyShareTendency := 50
	if v, ok := lt["intensity"].(int); ok {
		dailyShareTendency = v
	}
	if v, ok := lt["intensity"].(float64); ok {
		dailyShareTendency = int(v)
	}

	var tasks []ShareTask
	now := today

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomMinutes := func(base time.Time, minOff, maxOff int) time.Time {
		offset := rng.Intn(maxOff-minOff+1) + minOff
		return base.Add(time.Duration(offset) * time.Minute)
	}

	isBlocked := func(t time.Time) bool {
		for _, e := range timeline {
			if (t.After(e.StartTime) || t.Equal(e.StartTime)) && t.Before(e.EndTime) {
				s := e.State
				if s == "SLEEPING" || s == "IN_CLASS" || s == "IN_EXAM" || s == "BUSY" || s == "WORKING_OUT" || s == "OVERTIME" {
					return true
				}
			}
		}
		return false
	}

	addTask := func(taskType string, dueTime time.Time, reason string) bool {
		if isBlocked(dueTime) {
			return false
		}
		if dueTime.Before(now) {
			return false
		}
		prompt := s.GenerateSharePrompt(taskType, schedule, mood, energy)
		tasks = append(tasks, ShareTask{
			Type: taskType, DueTime: dueTime,
			Prompt: prompt, Reason: reason,
		})
		return true
	}

	wake := schedule.WakeTime
	lunch := schedule.LunchTime
	dinner := schedule.DinnerTime
	sleep := schedule.SleepTime
	if sleep.Before(wake) || sleep.Equal(wake) {
		sleep = sleep.Add(24 * time.Hour)
	}

	added := 0
	maxTasks := 3
	if dailyShareTendency >= 60 {
		maxTasks = 5
	}
	if dailyShareTendency < 30 {
		maxTasks = 2
	}
	idleDuration := s.getIdleDuration()
	if idleDuration > 48*time.Hour {
		maxTasks = 0
	} else if idleDuration > 24*time.Hour {
		maxTasks = 1
	} else if idleDuration > 12*time.Hour {
		if maxTasks > 2 {
			maxTasks = 2
		}
	} else if idleDuration > 6*time.Hour {
		if maxTasks > 3 {
			maxTasks = 3
		}
	}

	if added < maxTasks {
		morningTime := randomMinutes(wake, 5, 20)
		if addTask("morning_share", morningTime, "早安分享") {
			added++
		}
	}

	if added < maxTasks {
		noonTime := randomMinutes(lunch, -10, 0)
		if addTask("noon_daily", noonTime, "午间日常") {
			added++
		}
	}

	if added < maxTasks {
		eveningTime := randomMinutes(dinner, 30, 90)
		if addTask("evening_reflection", eveningTime, "傍晚分享") {
			added++
		}
	}

	if added < maxTasks {
		bedtime := randomMinutes(sleep, -60, -30)
		if addTask("bedtime_mood", bedtime, "睡前心情") {
			added++
		}
	}

	if added < maxTasks && schedule.HasNap && schedule.NapEndTime != nil {
		napWake := randomMinutes(*schedule.NapEndTime, 0, 10)
		if addTask("nap_wake", napWake, "午睡唤醒") {
			added++
		}
	}

	if len(tasks) > 1 {
		sort.Slice(tasks, func(i, j int) bool { return tasks[i].DueTime.Before(tasks[j].DueTime) })
		filtered := []ShareTask{tasks[0]}
		for i := 1; i < len(tasks); i++ {
			if tasks[i].DueTime.Sub(filtered[len(filtered)-1].DueTime) >= 60*time.Minute {
				filtered = append(filtered, tasks[i])
			}
		}
		tasks = filtered
	}

	s.db.Exec("UPDATE active_message_task SET status='CANCELLED', cancel_reason='regenerated', updated_at=datetime('now', 'localtime') WHERE date(due_time)=? AND status='PENDING' AND source='system' AND character_id = ?", date, characterID)

	if idleDuration > 12*time.Hour {
		var lastChase string
		s.db.Table("active_message_task").Select("due_time").Where("task_type = 'chase_up' AND status IN ('SENT','PROCESSING') AND character_id = ?", characterID).Order("due_time DESC").Limit(1).Row().Scan(&lastChase)
		if lastChase == "" {
			chaseTime := now.Add(time.Duration(5+rng.Intn(11)) * time.Minute)
			if !isBlocked(chaseTime) {
				idleHours := int(idleDuration.Hours())
				prompt := fmt.Sprintf("你已经%d小时没收到回复了。你有点失落，但不是指责。请生成一条自然的追问，1-2句，像微信里随口发的那种。", idleHours)
				tasks = append(tasks, ShareTask{Type: "chase_up", DueTime: chaseTime, Prompt: prompt, Reason: fmt.Sprintf("追问(%dh未回复)", idleHours)})
			}
		}
	}

	for _, t := range tasks {
		s.db.Exec("INSERT INTO active_message_task (task_type, due_time, prompt, status, source, character_id, created_at, updated_at) VALUES (?, ?, ?, 'PENDING', 'system', ?, datetime('now', 'localtime'), datetime('now', 'localtime'))",
			t.Type, t.DueTime.Format("2006-01-02 15:04:05"), t.Prompt, characterID)
	}

	resultMaps := make([]map[string]interface{}, len(tasks))
	for i, t := range tasks {
		resultMaps[i] = map[string]interface{}{
			"type": t.Type, "dueTime": t.DueTime.Format("2006-01-02T15:04:05"),
			"prompt": t.Prompt, "reason": t.Reason,
		}
	}
	if resultMaps == nil {
		resultMaps = []map[string]interface{}{}
	}
	return map[string]interface{}{
		"generated":         true,
		"tasks":             resultMaps,
		"taskCount":         len(tasks),
		"estimatedLLMCalls": len(tasks),
	}
}

func (s *service) GenerateSharePrompt(taskType string, schedule TodaySchedule, mood string, energy int) string {
	dateStr := schedule.WakeTime.Format("2006-01-02")
	sleepSummary := "正常"

	var recentMemories []string
	queryText := fmt.Sprintf("心情%s 精力%d", mood, energy)
	if taskType == "morning_share" {
		queryText = fmt.Sprintf("早晨 起床 心情%s", mood)
	} else if taskType == "evening_reflection" || taskType == "bedtime_mood" {
		queryText = fmt.Sprintf("晚上 睡觉前 心情%s", mood)
	}
	if s.embeddingSvc != nil && qdrantDB.Client != nil {
		vec, vecErr := s.embeddingSvc.Embed(queryText)
		if vecErr == nil {
			points, searchErr := qdrantDB.SearchVectors(vec, 5, nil)
			if searchErr == nil {
				for _, p := range points {
					if val, ok := p.Payload["value"]; ok {
						recentMemories = append(recentMemories, val.GetStringValue())
					}
				}
			}
		}
	}
	if len(recentMemories) == 0 {
		rows, err := s.db.Table("memories").Select("value").Order("created_at DESC").Limit(5).Rows()
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var v string
				rows.Scan(&v)
				if v != "" {
					recentMemories = append(recentMemories, v)
				}
			}
		}
	}

	history := s.getShareHistory()
	recentTopicsStr := strings.Join(history.RecentTopics, "、")
	if recentTopicsStr == "" {
		recentTopicsStr = "无"
	}

	scheduleSummary := fmt.Sprintf("起床 %s，午饭 %s，晚饭 %s，睡觉 %s",
		schedule.WakeTime.Format("15:04"),
		schedule.LunchTime.Format("15:04"),
		schedule.DinnerTime.Format("15:04"),
		schedule.SleepTime.Format("15:04"))

	memoriesStr := "无"
	if len(recentMemories) > 0 {
		memoriesStr = strings.Join(recentMemories, "；")
	}

	var prompt string
	switch taskType {
	case "morning_share":
		prompt = fmt.Sprintf(
			"你刚睡醒。昨晚睡眠状态：%s。现在心情：%s，精力：%d/100。今天的计划：%s。最近记忆：%s。"+
				"请生成一条自然的早安分享，像微信里随手发给熟人的消息，1-3句，不要客服腔，不要emoji，不要解释。避免重复这些话题：%s。",
			sleepSummary, mood, energy, scheduleSummary, memoriesStr, recentTopicsStr)
	case "noon_daily":
		prompt = fmt.Sprintf(
			"现在是午间。现在心情：%s，精力：%d/100。今天的计划：%s。"+
				"请生成一条午间日常分享，像微信短消息，1-3句，不要emoji，不要解释。避免重复这些话题：%s。",
			mood, energy, scheduleSummary, recentTopicsStr)
	case "evening_reflection":
		prompt = fmt.Sprintf(
			"现在是傍晚。今天的日期：%s。当前心情：%s，精力：%d/100。最近记忆：%s。"+
				"请生成一条傍晚小感受，语气自然，1-3句，不要emoji，不要解释。避免重复这些话题：%s。",
			dateStr, mood, energy, memoriesStr, recentTopicsStr)
	case "bedtime_mood":
		prompt = fmt.Sprintf(
			"快睡觉了。今天的日期：%s。当前心情：%s，精力：%d/100。最近记忆：%s。"+
				"请生成一条睡前分享，轻松、自然、不要肉麻，1-3句，不要emoji，不要解释。避免重复这些话题：%s。",
			dateStr, mood, energy, memoriesStr, recentTopicsStr)
	case "nap_wake":
		prompt = fmt.Sprintf(
			"刚午睡醒来。当前心情：%s，精力恢复到：%d/100。最近记忆：%s。"+
				"请生成一条刚醒来的自然分享，1-2句，不要emoji，不要解释。避免重复这些话题：%s。",
			mood, energy, memoriesStr, recentTopicsStr)
	default:
		prompt = fmt.Sprintf(
			"当前心情：%s，精力：%d/100。请生成一条自然的日常分享，像微信消息，1-2句，不要emoji，不要解释。",
			mood, energy)
	}
	return prompt
}

func (s *service) GetShareHistory() ShareHistory {
	var topics []string
	var lastAt string

	var rows []map[string]interface{}
	s.db.Table("proactive_messages").Select("message_content, created_at").Order("created_at DESC").Limit(30).Find(&rows)

	for _, r := range rows {
		if content, ok := r["message_content"].(string); ok && len(content) > 0 {
			if len([]rune(content)) <= 100 {
				topic := extractTopic(content)
				if topic != "" {
					topics = append(topics, topic)
				}
			}
		}
		if lastAt == "" {
			if ca, ok := r["created_at"].(string); ok {
				lastAt = ca
			}
		}
		if len(topics) >= 5 {
			break
		}
	}

	if topics == nil {
		topics = []string{}
	}
	return ShareHistory{RecentTopics: topics, LastShareAt: lastAt}
}

func (s *service) getShareHistory() ShareHistory { return s.GetShareHistory() }

func (s *service) TriggerDailyRegeneration(characterID string) map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	return s.ScheduleBasedGenerator(today, characterID)
}

func (s *service) generateLLMReply(prompt string) string {
	var baseURL, apiKey, modelName string
	err := s.db.Table("model_configs").Select("base_url, api_key, model_name").Where("is_active = 1").Limit(1).Row().Scan(&baseURL, &apiKey, &modelName)
	if err != nil || baseURL == "" || apiKey == "" {
		return ""
	}
	var charName, identity string
	s.db.Table("characters").Select("name, COALESCE(identity,'')").Where("is_active = 1").Limit(1).Row().Scan(&charName, &identity)
	if charName == "" {
		charName = "AI助手"
	}
	if identity == "" {
		identity = "一个AI伙伴"
	}
	now := time.Now()
	sys := fmt.Sprintf("你是%s，%s。\n当前时间：%s，周%s。\n你的语气自然、口语化。字数控制在8-40字。不要调用工具，直接输出纯文本。不要使用emoji。", charName, identity, now.Format("15:04"), now.Weekday().String())
	msgs := []map[string]interface{}{{"role": "system", "content": sys}, {"role": "user", "content": prompt}}
	reqBody, _ := json.Marshal(map[string]interface{}{"model": modelName, "messages": msgs, "temperature": 0.9, "max_tokens": 200, "stream": false})
	baseURL = strings.TrimRight(baseURL, "/")
	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", strings.NewReader(string(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	var r struct {
		Choices []struct{ Message struct{ Content string } }
	}
	json.Unmarshal(rb, &r)
	if len(r.Choices) > 0 {
		return strings.TrimSpace(r.Choices[0].Message.Content)
	}
	return ""
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
func extractTopic(content string) string {
	runes := []rune(content)
	if len(runes) < 6 {
		return ""
	}
	maxLen := 10
	if maxLen > len(runes) {
		maxLen = len(runes)
	}
	return string(runes[:maxLen])
}

func (s *service) RandomBurstTrigger(characterID string) map[string]interface{} {
	setting := s.GetActiveMessageSetting(characterID)
	enabled, _ := setting["enabled"].(bool)
	if !enabled {
		return map[string]interface{}{"triggered": false, "reason": "disabled"}
	}

	stateLife := s.GetStateLife(characterID)
	currentState, _ := stateLife["currentState"].(string)
	blockedStates := map[string]bool{"SLEEPING": true, "IN_CLASS": true, "IN_EXAM": true, "BUSY": true, "WORKING": true, "WORKING_OUT": true, "OVERTIME": true}
	if blockedStates[currentState] {
		return map[string]interface{}{"triggered": false, "reason": "blocked:" + currentState}
	}

	quietStart, _ := setting["quietStart"].(string)
	quietEnd, _ := setting["quietEnd"].(string)
	if quietStart == "" {
		quietStart = "23:00"
	}
	if quietEnd == "" {
		quietEnd = "07:00"
	}
	now := time.Now()
	nowStr := now.Format("15:04")
	if quietStart <= quietEnd {
		if nowStr >= quietStart && nowStr <= quietEnd {
			return map[string]interface{}{"triggered": false, "reason": "quiet:" + quietStart + "-" + quietEnd}
		}
	} else {
		if nowStr >= quietStart || nowStr <= quietEnd {
			return map[string]interface{}{"triggered": false, "reason": "quiet:" + quietStart + "-" + quietEnd}
		}
	}

	if s.lastBurstAt.Format("2006-01-02") != now.Format("2006-01-02") {
		s.todayBurstCount = 0
	}
	minInterval, _ := setting["minInterval"].(int)
	if time.Since(s.lastBurstAt) < time.Duration(minInterval)*time.Minute {
		return map[string]interface{}{"triggered": false, "reason": "minInterval"}
	}

	maxPerDay, _ := setting["maxPerDay"].(int)
	if s.todayBurstCount >= maxPerDay {
		return map[string]interface{}{"triggered": false, "reason": "maxPerDay"}
	}

	maxDailyCalls, _ := setting["maxDailyCalls"].(int)
	if maxDailyCalls == 0 {
		maxDailyCalls = 10
	}
	todayStr := now.Format("2006-01-02")
	var todayLLMCalls int64
	s.db.Table("proactive_messages").Where("date(created_at) = date(?)", todayStr).Count(&todayLLMCalls)
	if int(todayLLMCalls) >= maxDailyCalls {
		return map[string]interface{}{"triggered": false, "reason": "maxDailyCalls"}
	}

	activeLevel, _ := setting["activeLevel"].(int)
	if activeLevel == 0 {
		activeLevel = 40
	}
	baseProb := float64(activeLevel) / 100.0 * 0.05

	energy, _ := stateLife["energy"].(int)
	mood, _ := stateLife["mood"].(string)
	idleSec, _ := stateLife["idleDuration"].(float64)
	idleDuration := time.Duration(idleSec) * time.Second

	energyMod := 1.0
	if energy > 70 {
		energyMod = 1.2
	} else if energy < 30 {
		energyMod = 0.3
	}

	moodMod := 1.0
	if mood == "happy" {
		moodMod = 1.3
	} else if mood == "sad" || mood == "depressed" || mood == "ignored" {
		moodMod = 1.5
	} else if mood == "tired" || mood == "lonely" {
		moodMod = 0.7
	}

	stateMod := 1.0
	switch currentState {
	case "IDLE", "AFTER_WORK", "AFTER_CLASS", "LIBRARY_BREAK":
		stateMod = 1.0
	case "LOW_ENERGY", "SICK_RESTING":
		stateMod = 0.3
	default:
		stateMod = 0.6
	}

	budgetRemaining := maxDailyCalls - int(todayLLMCalls)
	if budgetRemaining < 1 {
		budgetRemaining = 1
	}
	budgetMod := float64(budgetRemaining) / float64(maxDailyCalls)

	finalProb := baseProb * energyMod * moodMod * stateMod * budgetMod

	if idleDuration > 48*time.Hour {
		finalProb = finalProb * 0.1
	}
	if idleDuration > 24*time.Hour {
		finalProb = finalProb * 0.3
	}

	rng := rand.New(rand.NewSource(now.UnixNano()))
	if rng.Float64() >= finalProb {
		return map[string]interface{}{"triggered": false, "reason": "probability", "prob": finalProb}
	}

	history := s.getShareHistory()
	recentTopics := strings.Join(history.RecentTopics, "、")
	if recentTopics == "" {
		recentTopics = "无"
	}

	var recentMemoriesStr string
	queryText := fmt.Sprintf("心情%s 状态%s", mood, currentState)
	if s.embeddingSvc != nil && qdrantDB.Client != nil {
		vec, vecErr := s.embeddingSvc.Embed(queryText)
		if vecErr == nil {
			points, searchErr := qdrantDB.SearchVectors(vec, 3, nil)
			if searchErr == nil {
				var mems []string
				for _, p := range points {
					if val, ok := p.Payload["value"]; ok {
						mems = append(mems, val.GetStringValue())
					}
				}
				recentMemoriesStr = strings.Join(mems, "；")
			}
		}
	}
	if recentMemoriesStr == "" {
		rows, err := s.db.Table("memories").Select("value").Where("importance >= 2").Order("created_at DESC").Limit(3).Rows()
		if err == nil {
			defer rows.Close()
			var mems []string
			for rows.Next() {
				var v string
				rows.Scan(&v)
				if v != "" {
					mems = append(mems, v)
				}
			}
			recentMemoriesStr = strings.Join(mems, "；")
		}
	}
	if recentMemoriesStr == "" {
		recentMemoriesStr = "无"
	}

	prompt := fmt.Sprintf("当前你处于 %s 状态，心情 %s，精力 %d/100。最近记忆：%s。请生成一条像微信里突然想到就发出的自然短消息，1-2句，不要客服腔，不要解释，不要 emoji，避免重复这些话题：%s。", currentState, mood, energy, recentMemoriesStr, recentTopics)

	msgID := fmt.Sprintf("burst-%d", now.UnixNano())
	generated := s.generateLLMReply(prompt)
	if generated == "" {
		return map[string]interface{}{"triggered": false, "reason": "llmFailed"}
	}
	displayContent := generated

	var convID string
	s.db.Table("conversations").Select("id").Limit(1).Row().Scan(&convID)
	if convID == "" {
		return map[string]interface{}{"triggered": false, "reason": "noConversation"}
	}

	s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?)",
		msgID, convID, displayContent, now.Format("2006-01-02 15:04:05"))

	s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, 'all', 'sent', ?, ?)",
		convID, prompt, now.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"))

	if s.isDefaultCharacter(characterID) {
		wcID := s.getWechatConvIDForChar(characterID)
		if wcID != "" {
			s.sendToWechatSidecar(wcID, generated)
		}
		qqID := s.getQQConvIDForChar(characterID)
		if qqID != "" {
			s.sendToQQSidecar(qqID, generated)
		}
	}

	s.lastBurstAt = now
	s.todayBurstCount++

	log.Printf("[Companion] RandomBurst triggered: prob=%.4f energyMod=%.2f moodMod=%.2f stateMod=%.2f budgetMod=%.2f", finalProb, energyMod, moodMod, stateMod, budgetMod)

	return map[string]interface{}{"triggered": true, "prob": finalProb, "burstCount": s.todayBurstCount, "prompt": prompt}

}
func (s *service) sendToWechatSidecar(toUserID, content string) {
	if strings.HasPrefix(toUserID, "conv-") {
		toUserID = toUserID[5:]
	}
	body, _ := json.Marshal(map[string]string{"toUserId": toUserID, "text": content})
	req, _ := http.NewRequest("POST", "http://127.0.0.1:9876/api/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		log.Printf("[Companion] 微信发送失败: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[Companion] 微信发送失败 HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		return
	}
	log.Printf("[Companion] 微信已发送 to=%s", toUserID)
}

func (s *service) sendToQQSidecar(toUserID, content string) {
	if strings.HasPrefix(toUserID, "conv-qq-") {
		toUserID = toUserID[8:]
	} else if strings.HasPrefix(toUserID, "conv-") {
		toUserID = toUserID[5:]
	}
	body, _ := json.Marshal(map[string]string{"toUserId": toUserID, "text": content})
	req, _ := http.NewRequest("POST", "http://127.0.0.1:9877/api/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		log.Printf("[Companion] QQ发送失败: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[Companion] QQ发送失败 HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		return
	}
	log.Printf("[Companion] QQ已发送 to=%s", toUserID)
}

func (s *service) isDefaultCharacter(characterID string) bool {
	var isActive int
	s.db.Table("characters").Select("is_default").Where("id = ?", characterID).Limit(1).Row().Scan(&isActive)
	return isActive == 1
}

func (s *service) getWechatConvIDForChar(characterID string) string {
	var id string
	s.db.Table("conversations").Select("id").
		Where("channel = 'wechat' AND peer_id != '' AND peer_id IS NOT NULL").
		Order("updated_at DESC").
		Limit(1).Row().Scan(&id)
	return id
}

func (s *service) getQQConvIDForChar(characterID string) string {
	var id string
	s.db.Table("conversations").Select("id").
		Where("channel = 'qq' AND peer_id != '' AND peer_id IS NOT NULL").
		Order("updated_at DESC").
		Limit(1).Row().Scan(&id)
	return id
}
