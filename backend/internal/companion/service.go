package companion

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"io"
	"net/http"
	"time"

	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	GetSleepSetting() map[string]interface{}
	UpdateSleepSetting(body map[string]interface{}) map[string]interface{}
	GetSchedule(date string) map[string]interface{}
	GetScheduleConflicts(date string) []map[string]interface{}
	GetScheduleToday() map[string]interface{}
	GetStateLife() map[string]interface{}
	GetState() map[string]interface{}
	GetTimelineToday() map[string]interface{}
	ListFixedEvents(date string) []map[string]interface{}
	GetFixedEvent(id int) map[string]interface{}
	CreateFixedEvent(body map[string]interface{}) map[string]interface{}
	UpdateFixedEvent(id int, body map[string]interface{}) map[string]interface{}
	DeleteFixedEvent(id int) bool
	ToggleFixedEventEnabled(id int) map[string]interface{}
	ListSpecialEvents() []map[string]interface{}
	CreateSpecialEvent(body map[string]interface{}) map[string]interface{}
	UpdateSpecialEvent(id int, body map[string]interface{}) map[string]interface{}
	DeleteSpecialEvent(id int) bool
	ToggleSpecialEventEnabled(id int) map[string]interface{}
	ListClassAdjustments() []map[string]interface{}
	CreateClassAdjustment(body map[string]interface{}) map[string]interface{}
	UpdateClassAdjustment(id int, body map[string]interface{}) map[string]interface{}
	DeleteClassAdjustment(id int) bool
	GetEffectiveClasses(date string) []map[string]interface{}
	GetLifestyleTendency() map[string]interface{}
	UpdateLifestyleTendency(body map[string]interface{}) map[string]interface{}
	ResetLifestyleTendency() map[string]interface{}
	GetWorkProfile() map[string]interface{}
	UpdateWorkProfile(body map[string]interface{}) map[string]interface{}
	GetActiveMessageSetting() map[string]interface{}
	UpdateActiveMessageSetting(body map[string]interface{}) map[string]interface{}
	GetActiveMessageTasksToday() []map[string]interface{}
	RegenerateActiveMessageTasks() map[string]interface{}
	RunActiveMessageTask(id int) map[string]interface{}
	CancelActiveMessageTask(id int) map[string]interface{}
	ListDelayedReplies() []map[string]interface{}
	CancelDelayedReply(id int) map[string]interface{}
	ProcessDelayedReplies() map[string]interface{}
	ProcessDueActiveMessageTasks() map[string]interface{}
	GetDebugOverview() map[string]interface{}
	RegenerateAllDebug() map[string]interface{}
	ProcessActiveMessagesDebug() map[string]interface{}
	ProcessDelayedRepliesDebug() map[string]interface{}
	GetRuleLogs() []map[string]interface{}
	RegenerateSchedule() map[string]interface{}
	RegenerateTimeline() map[string]interface{}
	ScheduleBasedGenerator(date string) map[string]interface{}
	GenerateSharePrompt(taskType string, schedule TodaySchedule, mood string, energy int) string
	GetShareHistory() ShareHistory
	TriggerDailyRegeneration() map[string]interface{}
	RandomBurstTrigger() map[string]interface{}
}

type service struct {
	db              *gorm.DB
	lastBurstAt     time.Time
	todayBurstCount int
}

func NewService(ctx *app.AppContext) Service {
	return &service{db: ctx.DB}
}

func (s *service) getSetting(key string) string {
	var v string; s.db.Table("app_settings").Select("value").Where("key = ?", key).Row().Scan(&v); return v
}

func (s *service) setSetting(key, value string) {
	s.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at", key, value)
}

func (s *service) GetSleepSetting() map[string]interface{} {
	var bed, wake string; var enabled int
	err := s.db.Table("sleep_settings").Select("bed_time, wake_time, enabled").Limit(1).Row().Scan(&bed, &wake, &enabled)
	if err != nil { return map[string]interface{}{"bedTime": "23:00", "wakeTime": "07:00", "enabled": true} }
	return map[string]interface{}{"bedTime": bed, "wakeTime": wake, "enabled": enabled == 1}
}

func (s *service) UpdateSleepSetting(body map[string]interface{}) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["bedTime"].(string); ok { updates["bed_time"] = v }
	if v, ok := body["wakeTime"].(string); ok { updates["wake_time"] = v }
	if v, ok := body["enabled"].(bool); ok { if v { updates["enabled"] = 1 } else { updates["enabled"] = 0 } }
	if len(updates) > 0 { s.db.Table("sleep_settings").Where("1=1").Updates(updates); go s.scheduleChanged() }
	return s.GetSleepSetting()
}

func (s *service) GetSchedule(date string) map[string]interface{} {
	if date == "" { date = time.Now().Format("2006-01-02") }
	return scheduleToMap(s.buildTodaySchedule(date))
}

func (s *service) GetScheduleConflicts(date string) []map[string]interface{} {
	if date == "" { date = time.Now().Format("2006-01-02") }
	schedule := s.buildTodaySchedule(date)
	timeline := s.buildTimeline(date, schedule)

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
			if a.EndTime.Before(b.StartTime) || a.EndTime.Equal(b.StartTime) { continue }
			if b.EndTime.Before(a.StartTime) || b.EndTime.Equal(a.StartTime) { continue }
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
			if e.State == "SLEEPING" { continue }
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
	if result == nil { result = []map[string]interface{}{} }
	return result
}
func (s *service) GetScheduleToday() map[string]interface{} { return s.GetSchedule(time.Now().Format("2006-01-02")) }

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

func (s *service) GetStateLife() map[string]interface{} {
	stateResult := s.GetState()
	currentState, _ := stateResult["currentState"].(string)
	if currentState == "" { currentState = "IDLE" }
	sleeping, _ := stateResult["sleeping"].(bool)
	busy, _ := stateResult["busy"].(bool)
	available, _ := stateResult["available"].(bool)
	stateStartedAt, _ := stateResult["stateStartedAt"].(string)
	stateEndsAt, _ := stateResult["stateEndsAt"].(string)

	mood := "neutral"
	var moods []map[string]interface{}
	s.db.Table("moods").Order("created_at DESC").Limit(1).Find(&moods)
	if len(moods) > 0 {
		if m, ok := moods[0]["mood"].(string); ok && m != "" { mood = m }
		if m, ok := moods[0]["mood_value"].(string); ok && m != "" { mood = m }
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
	schedule := s.buildTodaySchedule(today)
	energy := calculateEnergy(now, schedule, currentState)

	var currentActivity string
	if currentState == "SLEEPING" { currentActivity = "正在睡觉" }
	if currentState == "WAKING_UP" { currentActivity = "刚睡醒" }
	if currentState == "EATING_LUNCH" || currentState == "EATING_DINNER" { currentActivity = "正在吃饭" }
	if currentState == "NAPPING" { currentActivity = "正在午睡" }
	if currentState == "WORKING" { currentActivity = "正在工作" }
	if currentState == "IN_CLASS" { currentActivity = "正在上课" }
	if currentState == "STUDYING" { currentActivity = "正在学习" }
	if currentState == "COMMUTING_TO_WORK" { currentActivity = "上班路上" }
	if currentState == "COMMUTING_HOME" { currentActivity = "下班路上" }
	if currentState == "BEFORE_SLEEP" { currentActivity = "准备睡觉" }
	if currentState == "IDLE" { currentActivity = "空闲中" }
	if currentState == "AFTER_WORK" { currentActivity = "下班放松" }
	if currentActivity == "" { currentActivity = currentState }

	sleep := s.GetSleepSetting()
	result := map[string]interface{}{
		"currentState":    currentState,
		"currentActivity": currentActivity,
		"mood":            mood,
		"energy":          energy,
		"idleDuration":  idleDuration.Seconds(),
		"sleeping":        sleeping,
		"busy":            busy,
		"available":       available,
		"sleepSetting":    sleep,
	}
	if stateStartedAt != "" { result["stateStartedAt"] = stateStartedAt }
	if stateEndsAt != "" { result["stateEndsAt"] = stateEndsAt }
	return result
}

func (s *service) GetState() map[string]interface{} {
	now := time.Now()
	today := now.Format("2006-01-02")

	timelineRes := s.GetTimelineToday()
	entries, _ := timelineRes["events"].([]map[string]interface{})

	var matchedEntry map[string]interface{}
	for _, e := range entries {
		startStr, _ := e["startTime"].(string)
		endStr, _ := e["endTime"].(string)
		if startStr == "" || endStr == "" { continue }
		start, err1 := time.ParseInLocation("2006-01-02T15:04:05", startStr, time.Local)
		end, err2 := time.ParseInLocation("2006-01-02T15:04:05", endStr, time.Local)
		if err1 != nil || err2 != nil { continue }
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

	schedule := s.buildTodaySchedule(today)
	wake := schedule.WakeTime
	sleep := schedule.SleepTime
	if sleep.Before(wake) || sleep.Equal(wake) { sleep = sleep.Add(24 * time.Hour) }

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

func (s *service) GetTimelineToday() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	schedule := s.buildTodaySchedule(today)
	entries := s.buildTimeline(today, schedule)
	result := make([]map[string]interface{}, len(entries))
	for i, e := range entries {
		result[i] = map[string]interface{}{
			"startTime": e.StartTime.Format("2006-01-02T15:04:05"),
			"endTime":   e.EndTime.Format("2006-01-02T15:04:05"),
			"state":     e.State,
			"sourceType": e.SourceType,
			"priority":  e.Priority,
			"reason":    e.Reason,
		}
	}
	if result == nil { result = []map[string]interface{}{} }
	return map[string]interface{}{"date": today, "events": result, "schedule": scheduleToMap(schedule)}
}

func (s *service) ListFixedEvents(date string) []map[string]interface{} {
	var events []FixedEvent
	q := s.db.Where("enabled = 1")
	if date != "" { dayOfWeek := parseDayOfWeek(date); q = q.Where("(week_day = ? OR week_day = -1)", dayOfWeek) }
	q.Order("start_time").Find(&events)
	result := make([]map[string]interface{}, len(events))
	for i, e := range events { result[i] = map[string]interface{}{"id": e.ID, "name": e.Name, "description": e.Description, "weekDay": e.WeekDay, "startTime": e.StartTime, "endTime": e.EndTime, "eventType": e.EventType, "location": e.Location, "enabled": e.Enabled == 1} }
	return result
}

func (s *service) GetFixedEvent(id int) map[string]interface{} {
	var e FixedEvent; s.db.First(&e, id)
	return map[string]interface{}{"id": e.ID, "name": e.Name, "description": e.Description, "weekDay": e.WeekDay, "startTime": e.StartTime, "endTime": e.EndTime, "eventType": e.EventType, "location": e.Location, "enabled": e.Enabled == 1}
}

func (s *service) CreateFixedEvent(body map[string]interface{}) map[string]interface{} {
	name := ""; if v, ok := body["name"].(string); ok { name = v } else { name = "新事件" }
	e := FixedEvent{Name: name, EventType: "custom", Enabled: 1}
	if v, ok := body["description"].(string); ok { e.Description = v }
	if v, ok := body["weekDay"].(float64); ok { e.WeekDay = int(v) }
	if v, ok := body["startTime"].(string); ok { e.StartTime = v }
	if v, ok := body["endTime"].(string); ok { e.EndTime = v }
	if v, ok := body["eventType"].(string); ok { e.EventType = v }
	if v, ok := body["location"].(string); ok { e.Location = v }
	s.db.Create(&e); go s.scheduleChanged()
	return s.GetFixedEvent(e.ID)
}

func (s *service) UpdateFixedEvent(id int, body map[string]interface{}) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["name"].(string); ok { updates["name"] = v }
	if v, ok := body["description"].(string); ok { updates["description"] = v }
	if v, ok := body["weekDay"].(float64); ok { updates["week_day"] = int(v) }
	if v, ok := body["startTime"].(string); ok { updates["start_time"] = v }
	if v, ok := body["endTime"].(string); ok { updates["end_time"] = v }
	if v, ok := body["eventType"].(string); ok { updates["event_type"] = v }
	if v, ok := body["location"].(string); ok { updates["location"] = v }
	if v, ok := body["enabled"].(bool); ok { if v { updates["enabled"] = 1 } else { updates["enabled"] = 0 } }
	if len(updates) > 0 { s.db.Model(&FixedEvent{}).Where("id = ?", id).Updates(updates); go s.scheduleChanged() }
	return s.GetFixedEvent(id)
}

func (s *service) DeleteFixedEvent(id int) bool { ok := s.db.Delete(&FixedEvent{}, id).RowsAffected > 0; if ok { go s.scheduleChanged() }; return ok }

func (s *service) ToggleFixedEventEnabled(id int) map[string]interface{} {
	s.db.Model(&FixedEvent{}).Where("id = ?", id).Update("enabled", gorm.Expr("CASE WHEN enabled = 1 THEN 0 ELSE 1 END"))
	return s.GetFixedEvent(id)
}

func (s *service) ListSpecialEvents() []map[string]interface{} {
	var events []SpecialEvent; s.db.Order("event_date, start_time").Find(&events)
	result := make([]map[string]interface{}, len(events))
	for i, e := range events { result[i] = map[string]interface{}{"id": e.ID, "name": e.Name, "description": e.Description, "eventDate": e.EventDate, "startTime": e.StartTime, "endTime": e.EndTime, "eventType": e.EventType, "location": e.Location, "enabled": e.Enabled == 1} }
	return result
}

func (s *service) CreateSpecialEvent(body map[string]interface{}) map[string]interface{} {
	name := ""; if v, ok := body["name"].(string); ok { name = v } else { name = "特殊事件" }
	e := SpecialEvent{Name: name, EventType: "custom", Enabled: 1}
	if v, ok := body["description"].(string); ok { e.Description = v }
	if v, ok := body["eventDate"].(string); ok { e.EventDate = v }
	if v, ok := body["startTime"].(string); ok { e.StartTime = v }
	if v, ok := body["endTime"].(string); ok { e.EndTime = v }
	if v, ok := body["eventType"].(string); ok { e.EventType = v }
	if v, ok := body["location"].(string); ok { e.Location = v }
	s.db.Create(&e); go s.scheduleChanged()
	return map[string]interface{}{"id": e.ID, "name": e.Name, "eventDate": e.EventDate}
}

func (s *service) UpdateSpecialEvent(id int, body map[string]interface{}) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["name"].(string); ok { updates["name"] = v }
	if v, ok := body["description"].(string); ok { updates["description"] = v }
	if v, ok := body["eventDate"].(string); ok { updates["event_date"] = v }
	if v, ok := body["startTime"].(string); ok { updates["start_time"] = v }
	if v, ok := body["endTime"].(string); ok { updates["end_time"] = v }
	if v, ok := body["enabled"].(bool); ok { if v { updates["enabled"] = 1 } else { updates["enabled"] = 0 } }
	if len(updates) > 0 { s.db.Model(&SpecialEvent{}).Where("id = ?", id).Updates(updates); go s.scheduleChanged() }
	return map[string]interface{}{"id": id, "updated": true}
}

func (s *service) DeleteSpecialEvent(id int) bool { ok := s.db.Delete(&SpecialEvent{}, id).RowsAffected > 0; if ok { go s.scheduleChanged() }; return ok }

func (s *service) ToggleSpecialEventEnabled(id int) map[string]interface{} {
	s.db.Model(&SpecialEvent{}).Where("id = ?", id).Update("enabled", gorm.Expr("CASE WHEN enabled = 1 THEN 0 ELSE 1 END"))
	return map[string]interface{}{"id": id, "toggled": true}
}

func (s *service) ListClassAdjustments() []map[string]interface{} {
	var items []ClassAdjustment; s.db.Order("date, slot_index").Find(&items)
	result := make([]map[string]interface{}, len(items))
	for i, a := range items { result[i] = map[string]interface{}{"id": a.ID, "date": a.Date, "slotIndex": a.SlotIndex, "className": a.ClassName, "adjustType": a.AdjustType, "description": a.Description} }
	return result
}

func (s *service) CreateClassAdjustment(body map[string]interface{}) map[string]interface{} {
	a := ClassAdjustment{AdjustType: "swap"}
	if v, ok := body["date"].(string); ok { a.Date = v }
	if v, ok := body["slotIndex"].(float64); ok { a.SlotIndex = int(v) }
	if v, ok := body["className"].(string); ok { a.ClassName = v }
	if v, ok := body["adjustType"].(string); ok { a.AdjustType = v }
	if v, ok := body["description"].(string); ok { a.Description = v }
	s.db.Create(&a); go s.scheduleChanged()
	return map[string]interface{}{"id": a.ID, "className": a.ClassName}
}

func (s *service) UpdateClassAdjustment(id int, body map[string]interface{}) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["date"].(string); ok { updates["date"] = v }
	if v, ok := body["slotIndex"].(float64); ok { updates["slot_index"] = int(v) }
	if v, ok := body["className"].(string); ok { updates["class_name"] = v }
	if v, ok := body["adjustType"].(string); ok { updates["adjust_type"] = v }
	if v, ok := body["description"].(string); ok { updates["description"] = v }
	if len(updates) > 0 { s.db.Model(&ClassAdjustment{}).Where("id = ?", id).Updates(updates); go s.scheduleChanged() }
	return map[string]interface{}{"id": id, "updated": true}
}

func (s *service) DeleteClassAdjustment(id int) bool { ok := s.db.Delete(&ClassAdjustment{}, id).RowsAffected > 0; if ok { go s.scheduleChanged() }; return ok }

func (s *service) GetEffectiveClasses(date string) []map[string]interface{} {
	if date == "" { date = time.Now().Format("2006-01-02") }
	type classSlot struct {
		Title          string `json:"title"`
		StartTime      string `json:"startTime"`
		EndTime        string `json:"endTime"`
		Location       string `json:"location"`
		SourceType     string `json:"sourceType"`
		AdjustmentType string `json:"adjustmentType"`
	}

	var adjustments []ClassAdjustment
	s.db.Where("date = ?", date).Order("slot_index ASC").Find(&adjustments)

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
		if adj.AdjustType == "canceled" { continue }
		slots = append(slots, slot)
	}

	var lt LifestyleTendency
	if err := s.db.Limit(1).First(&lt); err == nil && lt.Schedule != "" {
		lines := strings.Split(lt.Schedule, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" { continue }
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 { continue }
			slotIdx := 0
			fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &slotIdx)
			className := strings.TrimSpace(parts[1])
			exists := false
			for _, s := range slots {
				stH := 0
				fmt.Sscanf(s.StartTime, "%d", &stH)
				if stH-8 == slotIdx && s.AdjustmentType != "canceled" { exists = true; break }
			}
			if !exists {
				sh := 8 + slotIdx
				slots = append(slots, classSlot{
					Title: className, StartTime: fmt.Sprintf("%02d:00", sh),
					EndTime: fmt.Sprintf("%02d:50", sh), Location: "教室",
					SourceType: "lifestyle", AdjustmentType: "",
				})
			}
		}
	}

	var specials []SpecialEvent
	s.db.Where("enabled = 1 AND event_date = ?", date).Find(&specials)
	for _, sp := range specials {
		if sp.EventType == "class" || sp.EventType == "exam" {
			slots = append(slots, classSlot{
				Title: sp.Name, StartTime: sp.StartTime, EndTime: sp.EndTime,
				Location: sp.Location, SourceType: "special_event",
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
	if result == nil { result = []map[string]interface{}{} }
	return []map[string]interface{}{{"date": date, "dayOfWeek": parseDayOfWeek(date), "slots": result}}
}

func (s *service) GetLifestyleTendency() map[string]interface{} {
	var t LifestyleTendency
	if err := s.db.Limit(1).First(&t); err != nil { return map[string]interface{}{"activity": "", "intensity": 50, "schedule": "", "preference": ""} }
	return map[string]interface{}{"id": t.ID, "activity": t.Activity, "intensity": t.Intensity, "schedule": t.Schedule, "preference": t.Preference}
}

func (s *service) UpdateLifestyleTendency(body map[string]interface{}) map[string]interface{} {
	var count int64; s.db.Model(&LifestyleTendency{}).Count(&count)
	updates := make(map[string]interface{})
	if v, ok := body["activity"].(string); ok { updates["activity"] = v }
	if v, ok := body["intensity"].(float64); ok { updates["intensity"] = int(v) }
	if v, ok := body["schedule"].(string); ok { updates["schedule"] = v }
	if v, ok := body["preference"].(string); ok { updates["preference"] = v }
	if count == 0 { s.db.Create(&LifestyleTendency{Activity: "", Intensity: 50, Schedule: "", Preference: ""}) }
	if len(updates) > 0 { s.db.Model(&LifestyleTendency{}).Where("1=1").Updates(updates); go s.scheduleChanged() }
	return s.GetLifestyleTendency()
}

func (s *service) ResetLifestyleTendency() map[string]interface{} {
	s.db.Where("1=1").Delete(&LifestyleTendency{})
	return s.GetLifestyleTendency()
}

func (s *service) GetWorkProfile() map[string]interface{} {
	var w WorkProfile
	if err := s.db.Limit(1).First(&w); err != nil { return map[string]interface{}{"jobTitle": "", "workHours": "", "workDays": "", "description": ""} }
	return map[string]interface{}{"id": w.ID, "jobTitle": w.JobTitle, "workHours": w.WorkHours, "workDays": w.WorkDays, "description": w.Description}
}

func (s *service) UpdateWorkProfile(body map[string]interface{}) map[string]interface{} {
	var count int64; s.db.Model(&WorkProfile{}).Count(&count)
	updates := make(map[string]interface{})
	if v, ok := body["jobTitle"].(string); ok { updates["job_title"] = v }
	if v, ok := body["workHours"].(string); ok { updates["work_hours"] = v }
	if v, ok := body["workDays"].(string); ok { updates["work_days"] = v }
	if v, ok := body["description"].(string); ok { updates["description"] = v }
	if count == 0 { s.db.Create(&WorkProfile{JobTitle: "", WorkHours: "", WorkDays: "", Description: ""}) }
	if len(updates) > 0 { s.db.Model(&WorkProfile{}).Where("1=1").Updates(updates); go s.scheduleChanged() }
	return s.GetWorkProfile()
}

func (s *service) GetActiveMessageSetting() map[string]interface{} {
	var enabled, activeLevel, minInterval, maxPerDay, maxDailyCalls int; var channel, quietStart, quietEnd string
	err := s.db.Table("active_message_settings").Select("enabled, COALESCE(active_level, 40) as active_level, min_interval, COALESCE(quiet_start, '23:00') as quiet_start, COALESCE(quiet_end, '07:00') as quiet_end, max_per_day, COALESCE(max_daily_calls, 10) as max_daily_calls, channel").Limit(1).Row().Scan(&enabled, &activeLevel, &minInterval, &quietStart, &quietEnd, &maxPerDay, &maxDailyCalls, &channel)
	if err != nil { return map[string]interface{}{"enabled": true, "activeLevel": 40, "quietStart": "23:00", "quietEnd": "07:00", "minInterval": 60, "maxPerDay": 6, "maxDailyCalls": 10, "channel": "all"} }
	if quietStart == "" { quietStart = "23:00" }
	if quietEnd == "" { quietEnd = "07:00" }
	if activeLevel == 0 { activeLevel = 40 }
	return map[string]interface{}{"enabled": enabled == 1, "activeLevel": activeLevel, "quietStart": quietStart, "quietEnd": quietEnd, "minInterval": minInterval, "maxPerDay": maxPerDay, "maxDailyCalls": maxDailyCalls, "channel": channel}
}
func (s *service) UpdateActiveMessageSetting(body map[string]interface{}) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["enabled"].(bool); ok { if v { updates["enabled"] = 1 } else { updates["enabled"] = 0 } }
	if v, ok := body["activeLevel"].(float64); ok { vv := int(v); if vv < 1 { vv = 1 }; if vv > 100 { vv = 100 }; updates["active_level"] = vv }
	if v, ok := body["minInterval"].(float64); ok { updates["min_interval"] = int(v) }
	if v, ok := body["quietStart"].(string); ok { updates["quiet_start"] = v }
	if v, ok := body["quietEnd"].(string); ok { updates["quiet_end"] = v }
	if v, ok := body["maxPerDay"].(float64); ok { updates["max_per_day"] = int(v) }
	if v, ok := body["maxDailyCalls"].(float64); ok { vv := int(v); if vv < 1 { vv = 1 }; if vv > 50 { vv = 50 }; updates["max_daily_calls"] = vv }
	if v, ok := body["channel"].(string); ok { updates["channel"] = v }
	if len(updates) > 0 {
		var count int64; s.db.Table("active_message_settings").Count(&count)
		if count == 0 { s.db.Exec("INSERT INTO active_message_settings (enabled, active_level, min_interval, quiet_start, quiet_end, max_per_day, max_daily_calls, channel) VALUES (1, 40, 60, '23:00', '07:00', 6, 10, 'all')") }
		s.db.Table("active_message_settings").Where("1=1").Updates(updates)
	}
	return s.GetActiveMessageSetting()
}
func (s *service) GetActiveMessageTasksToday() []map[string]interface{} {
	var raw []map[string]interface{}
	s.db.Table("active_message_task").Where("date(due_time) = date('now')").Order("due_time ASC").Find(&raw)
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
	if tasks == nil { tasks = []map[string]interface{}{} }
	return tasks
}

func (s *service) RegenerateActiveMessageTasks() map[string]interface{} {
	s.db.Exec("UPDATE active_message_task SET status='CANCELLED', cancel_reason='regenerate', updated_at=datetime('now') WHERE date(due_time)=date('now') AND status='PENDING'")
	return map[string]interface{}{"regenerated": true}
}

func (s *service) RunActiveMessageTask(id int) map[string]interface{} {
	var task map[string]interface{}
	s.db.Table("active_message_task").Where("id = ?", id).Limit(1).Find(&task)
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
	msgID := fmt.Sprintf("proactive-%d", now.UnixNano())
	generated := s.generateLLMReply(prompt)
	if generated == "" { generated = prompt }
	convRow := s.db.Table("conversations").Select("id").Limit(1).Row()
	var convID string
	convRow.Scan(&convID)
	var channelSetting string
	s.db.Table("active_message_settings").Select("COALESCE(channel, 'all')").Limit(1).Row().Scan(&channelSetting)
	if channelSetting == "" { channelSetting = "all" }
	s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?)", msgID, convID, generated, nowStr)
	s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, ?, 'sent', ?, ?)", convID, prompt, channelSetting, nowStr, nowStr)
	s.db.Exec("UPDATE active_message_task SET status='SENT', sent_at=?, updated_at=datetime('now') WHERE id=?", nowStr, id)
	log.Printf("[Companion] RunActiveMessageTask sent type=%s id=%d channel=%s", taskType, id, channelSetting)
	return map[string]interface{}{"id": id, "status": "SENT", "taskType": taskType, "channel": channelSetting}
}

func (s *service) CancelActiveMessageTask(id int) map[string]interface{} {
	s.db.Exec("UPDATE active_message_task SET status='CANCELLED', cancel_reason='manual', updated_at=datetime('now') WHERE id=?", id)
	return map[string]interface{}{"id": id, "cancelled": true}
}

func (s *service) ListDelayedReplies() []map[string]interface{} {
	var raw []map[string]interface{}
	s.db.Table("delayed_replies").Where("status = 'pending'").Order("scheduled_at ASC").Find(&raw)
	replies := make([]map[string]interface{}, len(raw))
	for i, r := range raw {
		triggerState := "delay"
		if ch, _ := r["channel"].(string); ch != "" { triggerState = ch }
		replies[i] = map[string]interface{}{
			"id":                r["id"],
			"status":            r["status"],
			"triggerState":      triggerState,
			"userMessage":       r["content"],
			"expectedReplyAfter": r["scheduled_at"],
			"channel":           r["channel"],
		}
	}
	if replies == nil { replies = []map[string]interface{}{} }
	return replies
}

func (s *service) CancelDelayedReply(id int) map[string]interface{} {
	s.db.Exec("UPDATE delayed_replies SET status='cancelled', updated_at=datetime('now') WHERE id=?", id)
	return map[string]interface{}{"id": id, "cancelled": true}
}

func (s *service) ProcessDelayedReplies() map[string]interface{} {
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")

	var tasks []map[string]interface{}
	s.db.Table("delayed_replies").Where("status = 'pending' AND scheduled_at <= ?", nowStr).
		Order("scheduled_at ASC").Limit(20).Find(&tasks)

	var processed, sent, delayed, failed int

	for _, t := range tasks {
		processed++
		id, _ := t["id"]
		content, _ := t["content"].(string)
		convID, _ := t["conversation_id"].(string)
		channel, _ := t["channel"].(string)

		if content == "" { continue }

		canSend := true
		stateResult := s.GetState()
		currentState, _ := stateResult["currentState"].(string)

		if currentState == "SLEEPING" || currentState == "NAPPING" {
			canSend = false
			schedule := s.buildTodaySchedule(now.Format("2006-01-02"))
			wakeTime := schedule.WakeTime
			if currentState == "NAPPING" && schedule.NapEndTime != nil {
				wakeTime = *schedule.NapEndTime
			}
			if wakeTime.Before(now) { wakeTime = wakeTime.Add(24 * time.Hour) }
			s.db.Exec("UPDATE delayed_replies SET scheduled_at = ?, updated_at = datetime('now') WHERE id = ?",
				wakeTime.Format("2006-01-02 15:04:05"), id)
			delayed++
		} else if currentState == "IN_CLASS" || currentState == "IN_EXAM" || currentState == "BUSY" {
			canSend = false
			delayMin := 10 + rand.Intn(21)
			newTime := now.Add(time.Duration(delayMin) * time.Minute)
			s.db.Exec("UPDATE delayed_replies SET scheduled_at = ?, updated_at = datetime('now') WHERE id = ?",
				newTime.Format("2006-01-02 15:04:05"), id)
			delayed++
		}

		if canSend {
			if convID == "" {
				row := s.db.Table("conversations").Select("id").Limit(1).Row()
				row.Scan(&convID)
			}
			if convID == "" { failed++; continue }
			if channel == "" { channel = "web" }

			msgID := fmt.Sprintf("reply-%d", now.UnixNano())
			displayContent := "💬 " + content
			err := s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'delayed_reply', 'normal', 'sent', 1, ?)",
				msgID, convID, displayContent, nowStr).Error
			if err != nil {
				retryCount := 0
				if rc, ok := t["retry_count"]; ok {
					switch v := rc.(type) {
					case int64: retryCount = int(v)
					case float64: retryCount = int(v)
					}
				}
				retryCount++
				if retryCount >= 3 {
					s.db.Exec("UPDATE delayed_replies SET status='FAILED', retry_count=?, updated_at=datetime('now') WHERE id = ?", retryCount, id)
					failed++
				} else {
					s.db.Exec("UPDATE delayed_replies SET retry_count=?, updated_at=datetime('now') WHERE id = ?", retryCount, id)
				}
			} else {
				s.db.Exec("UPDATE delayed_replies SET status='SENT', sent_at=?, updated_at=datetime('now') WHERE id = ?", nowStr, id)
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

func (s *service) GetDebugOverview() map[string]interface{} {
	
now := time.Now()
	
nowStr := now.Format("2006-01-02 15:04:05")
	
schedule := s.GetScheduleToday()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	lt := s.GetLifestyleTendency()
	intensity := 50
	if v, ok := lt["intensity"].(int); ok { intensity = v }
	if v, ok := lt["intensity"].(float64); ok { intensity = int(v) }
	jitterMax := intensity / 10
	if jitterMax < 2 { jitterMax = 2 }
	if jitterMax > 15 { jitterMax = 15 }
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
	
timeline := s.GetTimelineToday()
	
currentState := s.GetState()
	
stateLife := s.GetStateLife()
	
activeMsgSetting := s.GetActiveMessageSetting()
	
activeTasks := s.GetActiveMessageTasksToday()
	
conflicts := s.GetScheduleConflicts("")
	
effectiveClasses := s.GetEffectiveClasses("")
	
var pendingReplies int64
	
s.db.Table("delayed_replies").Where("status = 'pending'").Count(&pendingReplies)
	
var todayTaskCount int64
	
s.db.Table("active_message_task").Where("date(due_time) = date(?)", nowStr).Count(&todayTaskCount)
	
var todaySentCount int64
	
s.db.Table("active_message_task").Where("date(due_time) = date(?) AND status = 'SENT'", nowStr).Count(&todaySentCount)
	
var todayLLMCalls int64
	
s.db.Table("proactive_messages").Where("date(created_at) = date(?)", nowStr).Count(&todayLLMCalls)
	
var maxDailyCalls int64
	
s.db.Table("active_message_settings").Select("COALESCE(max_daily_calls, 10)").Limit(1).Row().Scan(&maxDailyCalls)
	
if maxDailyCalls == 0 { maxDailyCalls = 10 }
	
	delayedRepliesList := s.ListDelayedReplies()
	if delayedRepliesList == nil { delayedRepliesList = []map[string]interface{}{} }
	recentRuleLogs := s.GetRuleLogs()
	if recentRuleLogs == nil { recentRuleLogs = []map[string]interface{}{} }
	
return map[string]interface{}{
	
	
"now":                    nowStr,
	
	
"todaySchedule":           schedule,
		"schedule":               schedule,
	
	
"timeline":               timeline["events"],
	
	
"currentState":           currentState,
	
	
"stateLife":              stateLife,
	
	
"activeMessageSetting":   activeMsgSetting,
	
	
"activeMessageTasks": activeTasks,
	
	
"scheduleConflicts":      conflicts,
	
	
"effectiveClasses":       effectiveClasses,
	
	
"delayedReplies":         delayedRepliesList,
	
	
"recentRuleLogs":         recentRuleLogs,
	
	
	"stats": map[string]interface{}{
	
	
	
"todayTaskCount": todayTaskCount,
	
	
	
"todaySentCount": todaySentCount,
	
	
	
"todayLLMCalls":  todayLLMCalls,
	
	
	
"maxDailyCalls":  maxDailyCalls,
			"remainingLLMCalls": maxDailyCalls - todayLLMCalls,
	
	
},
	
}
}

func (s *service) RegenerateAllDebug() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	scheduleResult := s.RegenerateSchedule()
	s.ScheduleBasedGenerator(today)
	return map[string]interface{}{
		"regenerated": true,
		"schedule":    scheduleResult["schedule"],
		"timeline":    scheduleResult["timeline"],
		"taskCount":   len(s.GetActiveMessageTasksToday()),
	}
}
func (s *service) ProcessActiveMessagesDebug() map[string]interface{} { return s.ProcessDueActiveMessageTasks() }

func (s *service) ProcessDueActiveMessageTasks() map[string]interface{} {
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
		s.db.Exec("UPDATE active_message_task SET status='PENDING', lock_until=NULL, updated_at=datetime('now') WHERE status='PROCESSING' AND updated_at < datetime('now', '-5 minutes')")
	var tasks []map[string]interface{}
	s.db.Table("active_message_task").Where("status = 'PENDING' AND due_time <= ?", nowStr).Order("due_time ASC").Limit(20).Find(&tasks)
	var processed, sent, delayed, failed int
	var channelSetting string
	channelRow := s.db.Table("active_message_settings").Select("COALESCE(channel, 'all')").Limit(1).Row()
	channelRow.Scan(&channelSetting)
	if channelSetting == "" { channelSetting = "all" }
	for _, t := range tasks {
		processed++
		id, _ := t["id"]
		prompt, _ := t["prompt"].(string)
		if prompt == "" { continue }
		result := s.db.Exec("UPDATE active_message_task SET status='PROCESSING', lock_until=datetime('now', '+5 minutes') WHERE id = ? AND status='PENDING'", id)
		if result.RowsAffected == 0 { continue }
		stateResult := s.GetState()
		currentState, _ := stateResult["currentState"].(string)
		if currentState == "SLEEPING" || currentState == "IN_CLASS" || currentState == "IN_EXAM" || currentState == "BUSY" {
			delayMin := 10
			newDue := now.Add(time.Duration(delayMin) * time.Minute).Format("2006-01-02 15:04:05")
			s.db.Exec("UPDATE active_message_task SET status='PENDING', lock_until=NULL, due_time=?, updated_at=datetime('now') WHERE id=?", newDue, id)
			delayed++
			continue
		}
		convRow := s.db.Table("conversations").Select("id").Limit(1).Row()
		var convID string
		convRow.Scan(&convID)
		if convID == "" { failed++; continue }
		msgID := fmt.Sprintf("proactive-%d", now.UnixNano())
		generated := s.generateLLMReply(prompt)
		if generated == "" { generated = prompt }
		displayContent := generated
		insErr := s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?)", msgID, convID, displayContent, nowStr).Error
		if insErr != nil {
			retryCount := 0
			if rc, ok := t["retry_count"]; ok { switch v := rc.(type) { case int64: retryCount = int(v); case float64: retryCount = int(v) } }
			retryCount++
			if retryCount >= 3 {
				s.db.Exec("UPDATE active_message_task SET status='FAILED', retry_count=?, updated_at=datetime('now') WHERE id=?", retryCount, id)
				failed++
			} else {
				newDue := now.Add(time.Duration(5*retryCount) * time.Minute).Format("2006-01-02 15:04:05")
				s.db.Exec("UPDATE active_message_task SET status='PENDING', lock_until=NULL, due_time=?, retry_count=?, updated_at=datetime('now') WHERE id=?", newDue, retryCount, id)
				delayed++
			}
			continue
		}
		taskType, _ := t["task_type"].(string)
		s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, ?, 'sent', ?, ?)", convID, prompt, channelSetting, nowStr, nowStr)
		s.db.Exec("UPDATE active_message_task SET status='SENT', sent_at=?, updated_at=datetime('now') WHERE id=?", nowStr, id)
		log.Printf("[Companion] ProcessDueActiveMessageTasks sent type=%s id=%v", taskType, id)
		sent++
	}
	return map[string]interface{}{"processed": processed, "sent": sent, "delayed": delayed, "failed": failed}
}
func (s *service) ProcessDelayedRepliesDebug() map[string]interface{} { return s.ProcessDelayedReplies() }

func (s *service) GetRuleLogs() []map[string]interface{} {
	var logs []map[string]interface{}
	s.db.Table("proactive_rule_logs").Order("triggered_at DESC").Limit(50).Find(&logs)
	if logs == nil { logs = []map[string]interface{}{} }
	return logs
}

func (s *service) RegenerateSchedule() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	schedule := s.buildTodaySchedule(today)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	lt := s.GetLifestyleTendency()
	intensity := 50
	if v, ok := lt["intensity"].(int); ok { intensity = v }
	if v, ok := lt["intensity"].(float64); ok { intensity = int(v) }
	jitterMax := intensity / 10
	if jitterMax < 2 { jitterMax = 2 }
	if jitterMax > 15 { jitterMax = 15 }
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
	timeline := s.buildTimeline(today, schedule)
	timelineMaps := make([]map[string]interface{}, len(timeline))
	for i, e := range timeline {
		timelineMaps[i] = map[string]interface{}{
			"startTime": e.StartTime.Format("2006-01-02T15:04:05"),
			"endTime":   e.EndTime.Format("2006-01-02T15:04:05"),
			"state":     e.State,
			"sourceType": e.SourceType,
			"priority":  e.Priority,
			"reason":    e.Reason,
		}
	}
	if timelineMaps == nil { timelineMaps = []map[string]interface{}{} }
	return map[string]interface{}{
		"schedule":    scheduleToMap(schedule),
		"timeline":    timelineMaps,
		"regenerated": true,
	}
}
func (s *service) RegenerateTimeline() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	schedule := s.buildTodaySchedule(today)
	timeline := s.buildTimeline(today, schedule)
	result := make([]map[string]interface{}, len(timeline))
	for i, e := range timeline {
		result[i] = map[string]interface{}{
			"startTime": e.StartTime.Format("2006-01-02T15:04:05"),
			"endTime":   e.EndTime.Format("2006-01-02T15:04:05"),
			"state":     e.State,
			"sourceType": e.SourceType,
			"priority":  e.Priority,
			"reason":    e.Reason,
		}
	}
	if result == nil { result = []map[string]interface{}{} }
	return map[string]interface{}{"events": result, "regenerated": true}
}

func parseDayOfWeek(date string) int { t, err := time.ParseInLocation("2006-01-02", date, time.Local); if err != nil { return int(time.Now().Weekday()) }; return int(t.Weekday()) }
func toJSON(v interface{}) string { b, _ := json.Marshal(v); return string(b) }

func (s *service) buildTodaySchedule(date string) TodaySchedule {
	today := parseDate(date)

	wakeTime := parseTimeStr("08:00", today)
	bedTime := parseTimeStr("23:00", today)

	var bed, wake string
	var sleepEnabled int
	err := s.db.Table("sleep_settings").Select("bed_time, wake_time, enabled").Limit(1).Row().Scan(&bed, &wake, &sleepEnabled)
	if err == nil {
		if wake != "" { wakeTime = parseTimeStr(wake, today) }
		if bed != "" { bedTime = parseTimeStr(bed, today) }
		if bedTime.Before(wakeTime) || bedTime.Equal(wakeTime) { bedTime = bedTime.Add(24 * time.Hour) }
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
			if e.StartTime != "" { lunchTime = parseTimeStr(e.StartTime, today) }
		case "meal_dinner":
			if e.StartTime != "" { dinnerTime = parseTimeStr(e.StartTime, today) }
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
		if lt.Intensity < 30 {
			if wakeTime.Hour() < 7 { wakeTime = wakeTime.Add(30 * time.Minute) }
		} else if lt.Intensity > 70 {
			if wakeTime.Hour() > 6 { wakeTime = wakeTime.Add(-15 * time.Minute) }
		}
	}

	isRestDay := false
	var specials []SpecialEvent
	s.db.Where("enabled = 1 AND event_date = ?", date).Find(&specials)
	for _, sp := range specials {
		if sp.EventType == "rest_day" || sp.StartTime == "" || (sp.StartTime == "00:00" && sp.EndTime == "23:59") {
			isRestDay = true
			break
		}
	}

	return TodaySchedule{
		WakeTime:  wakeTime,
		LunchTime: lunchTime,
		DinnerTime: dinnerTime,
		HasNap:    hasNap,
		NapStartTime: napStart,
		NapEndTime:   napEnd,
		SleepTime: bedTime,
		IsRestDay: isRestDay,
	}
}

func (s *service) buildTimeline(date string, schedule TodaySchedule) []TimelineEntry {
	today := parseDate(date)
	midnight := today
	nextMidnight := today.Add(24 * time.Hour)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		var entries []TimelineEntry

	addEntry := func(start, end time.Time, state, sourceType string, priority int, reason string) {
		if end.Before(start) || end.Equal(start) { return }
		entries = append(entries, TimelineEntry{
			StartTime: start, EndTime: end,
			State: state, SourceType: sourceType,
			Priority: priority, Reason: reason,
		})
	}

	hasWork := false
	var workStart, workEnd time.Time
	var wp WorkProfile
	if err := s.db.Limit(1).First(&wp); err == nil && wp.JobTitle != "" && wp.WorkHours != "" {
		parts := splitTimeRange(wp.WorkHours)
		if len(parts) == 2 {
			workStart = parseTimeStr(parts[0], today)
			workEnd = parseTimeStr(parts[1], today)
			if workEnd.Before(workStart) || workEnd.Equal(workStart) { workEnd = workEnd.Add(12 * time.Hour) }
			todayWeekday := int(today.Weekday())
			if wp.WorkDays != "" {
				workDays := parseWorkDays(wp.WorkDays)
				if workDays[todayWeekday] { hasWork = true }
			} else {
				hasWork = todayWeekday >= 1 && todayWeekday <= 5
			}
		}
	}

	classes := s.buildClassEntries(date)

	wake := schedule.WakeTime
	lunch := schedule.LunchTime
	dinner := schedule.DinnerTime
	sleep := schedule.SleepTime

	if sleep.Before(wake) || sleep.Equal(wake) { sleep = sleep.Add(24 * time.Hour) }

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
			if ns.After(lunchEnd) { addEntry(lunchEnd, ns, "IDLE", "schedule", 40, "空闲") }
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
		if afterDinner.Before(afterWork) { afterDinner = afterWork }

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
			if ns.After(lunchEnd) { addEntry(lunchEnd, ns, "IDLE", "schedule", 40, "空闲") }
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
			if ns.After(lunchEnd) { addEntry(lunchEnd, ns, "IDLE", "schedule", 40, "空闲") }
			addEntry(ns, ne, "NAPPING", "schedule", 85, "午睡")
			afterLunchEnd := ne
			if afterLunchEnd.Before(lunchEnd) { afterLunchEnd = lunchEnd }
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

func (s *service) buildClassEntries(date string) []TimelineEntry {
	var entries []TimelineEntry
	today := parseDate(date)

	classes := s.GetEffectiveClasses(date)
	for _, c := range classes {
		slots, _ := c["slots"].([]map[string]interface{})
		for _, slot := range slots {
			name, _ := slot["className"].(string)
			if name == "" { name, _ = slot["name"].(string) }
			startStr, _ := slot["startTime"].(string)
			endStr, _ := slot["endTime"].(string)
			if startStr == "" || endStr == "" { continue }

			start := parseTimeStr(startStr, today)
			end := parseTimeStr(endStr, today)
			if end.Before(start) { continue }

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
			if end.Before(start) { continue }
			entries = append(entries, TimelineEntry{
				StartTime: start, EndTime: end,
				State: "STUDYING", SourceType: "fixed_event",
				Priority: 70, Reason: fmt.Sprintf("学习: %s", e.Name),
			})
		}
	}

	return entries
}

func scheduleToMap(s TodaySchedule) map[string]interface{} {
	result := map[string]interface{}{
		"wakeTime":  s.WakeTime.Format("2006-01-02T15:04:05"),
		"lunchTime": s.LunchTime.Format("2006-01-02T15:04:05"),
		"dinnerTime": s.DinnerTime.Format("2006-01-02T15:04:05"),
		"hasNap":    s.HasNap,
		"sleepTime": s.SleepTime.Format("2006-01-02T15:04:05"),
		"isRestDay": s.IsRestDay,
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
	if len(parts) < 2 { parts = []string{"08", "00"} }
	h := 0; m := 0
	fmt.Sscanf(parts[0], "%d", &h); fmt.Sscanf(parts[1], "%d", &m)
	return time.Date(date.Year(), date.Month(), date.Day(), h, m, 0, 0, time.Local)
}

func parseDate(date string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil { return time.Now() }
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
					if p2 != "" { parts = append(parts, p2) }
				}
				if len(parts) >= 2 { return parts }
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
			if current != "" { parts = append(parts, current); current = "" }
		} else {
			current += string(ch)
		}
	}
	if current != "" { parts = append(parts, current) }

	dayMap := map[string]int{"1": 1, "2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "0": 0, "7": 0}
	for _, p := range parts {
		p = trimSpace(p)
		if d, ok := dayMap[p]; ok { result[d] = true; continue }
		if idx := indexOf(p, "-"); idx >= 0 {
			from := trimSpace(p[:idx])
			to := trimSpace(p[idx+1:])
			fd, fok := dayMap[from]
			td, tok := dayMap[to]
			if fok && tok {
				for d := fd; d <= td; d++ { result[d] = true }
			}
		}
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr { return i }
	}
	return -1
}

func trimSpace(s string) string {
	start := 0; end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') { start++ }
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') { end-- }
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
	if sleep.Before(wake) || sleep.Equal(wake) { sleep = sleep.Add(24 * time.Hour) }
	lunch := schedule.LunchTime
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	dinner := schedule.DinnerTime

	wakeHour := wake.Sub(today).Hours()
	if wakeHour > 24 { wakeHour -= 24 }
	if wakeHour < 0 { wakeHour += 24 }

	nowHour := float64(now.Hour()) + float64(now.Minute())/60.0
	if now.Before(today) { nowHour += 24 }

	lunchHour := lunch.Sub(today).Hours()
	if lunchHour < 0 { lunchHour += 24 }
	dinnerHour := dinner.Sub(today).Hours()
	if dinnerHour < 0 { dinnerHour += 24 }
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
		if base < 40 { base = 40 }
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
	if n < 0 { n = -n }
	return n
}

func sendProactiveNotification(db *gorm.DB, convID, msgID, content string) {
	db.Exec("UPDATE conversations SET message_count=message_count+1, updated_at=datetime('now') WHERE id=?", convID)
}

func (s *service) ScheduleBasedGenerator(date string) map[string]interface{} {
	if date == "" { date = time.Now().Format("2006-01-02") }
	today := parseDate(date)

	schedule := s.buildTodaySchedule(date)
	timeline := s.buildTimeline(date, schedule)
	stateLife := s.GetStateLife()
	mood, _ := stateLife["mood"].(string)
	if mood == "" { mood = "neutral" }
	energy, _ := stateLife["energy"].(int)

	lt := s.GetLifestyleTendency()
	dailyShareTendency := 50
	if v, ok := lt["intensity"].(int); ok { dailyShareTendency = v }
	if v, ok := lt["intensity"].(float64); ok { dailyShareTendency = int(v) }

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
		if isBlocked(dueTime) { return false }
		if dueTime.Before(now) { return false }
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
	if sleep.Before(wake) || sleep.Equal(wake) { sleep = sleep.Add(24 * time.Hour) }

	added := 0
	maxTasks := 3
	if dailyShareTendency >= 60 { maxTasks = 5 }
	if dailyShareTendency < 30 { maxTasks = 2 }
	idleDuration := s.getIdleDuration()
	if idleDuration > 48*time.Hour { maxTasks = 0 } else if idleDuration > 24*time.Hour { maxTasks = 1 } else if idleDuration > 12*time.Hour { if maxTasks > 2 { maxTasks = 2 } } else if idleDuration > 6*time.Hour { if maxTasks > 3 { maxTasks = 3 } }

	if added < maxTasks {
		morningTime := randomMinutes(wake, 5, 20)
		if addTask("morning_share", morningTime, "早安分享") { added++ }
	}

	if added < maxTasks {
		noonTime := randomMinutes(lunch, -10, 0)
		if addTask("noon_daily", noonTime, "午间日常") { added++ }
	}

	if added < maxTasks {
		eveningTime := randomMinutes(dinner, 30, 90)
		if addTask("evening_reflection", eveningTime, "傍晚分享") { added++ }
	}

	if added < maxTasks {
		bedtime := randomMinutes(sleep, -60, -30)
		if addTask("bedtime_mood", bedtime, "睡前心情") { added++ }
	}

	if added < maxTasks && schedule.HasNap && schedule.NapEndTime != nil {
		napWake := randomMinutes(*schedule.NapEndTime, 0, 10)
		if addTask("nap_wake", napWake, "午睡唤醒") { added++ }
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

	s.db.Exec("UPDATE active_message_task SET status='CANCELLED', cancel_reason='regenerated', updated_at=datetime('now') WHERE date(due_time)=? AND status='PENDING' AND source='system'", date)

	if idleDuration > 12*time.Hour {
		var lastChase string
		s.db.Table("active_message_task").Select("due_time").Where("task_type = 'chase_up' AND status IN ('SENT','PROCESSING')").Order("due_time DESC").Limit(1).Row().Scan(&lastChase)
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
		s.db.Exec("INSERT INTO active_message_task (task_type, due_time, prompt, status, source, created_at, updated_at) VALUES (?, ?, ?, 'PENDING', 'system', datetime('now'), datetime('now'))",
			t.Type, t.DueTime.Format("2006-01-02 15:04:05"), t.Prompt)
	}

	resultMaps := make([]map[string]interface{}, len(tasks))
	for i, t := range tasks {
		resultMaps[i] = map[string]interface{}{
			"type": t.Type, "dueTime": t.DueTime.Format("2006-01-02T15:04:05"),
			"prompt": t.Prompt, "reason": t.Reason,
		}
	}
	if resultMaps == nil { resultMaps = []map[string]interface{}{} }
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
	rows, err := s.db.Table("memories").Select("value").Order("created_at DESC").Limit(5).Rows()
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var v string
			rows.Scan(&v)
			if v != "" { recentMemories = append(recentMemories, v) }
		}
	}

	history := s.getShareHistory()
	recentTopicsStr := strings.Join(history.RecentTopics, "、")
	if recentTopicsStr == "" { recentTopicsStr = "无" }

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
	s.db.Table("proactive_messages").Select("message_content, created_at").Order("created_at DESC").Limit(20).Find(&rows)

	for _, r := range rows {
		if content, ok := r["message_content"].(string); ok && len(content) > 0 {
			topic := extractTopic(content)
			if topic != "" { topics = append(topics, topic) }
		}
		if lastAt == "" {
			if ca, ok := r["created_at"].(string); ok { lastAt = ca }
		}
	}

	if topics == nil { topics = []string{} }
	return ShareHistory{RecentTopics: topics, LastShareAt: lastAt}
}

func (s *service) getShareHistory() ShareHistory { return s.GetShareHistory() }

func (s *service) TriggerDailyRegeneration() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	return s.ScheduleBasedGenerator(today)
}



func (s *service) generateLLMReply(prompt string) string {
	var baseURL, apiKey, modelName string
	err := s.db.Table("model_configs").Select("base_url, api_key, model_name").Where("is_active = 1").Limit(1).Row().Scan(&baseURL, &apiKey, &modelName)
	if err != nil || baseURL == "" || apiKey == "" {
		return ""
	}
	sys := "你的语气自然、口语化。字数控制在8-40字。不要调用工具，直接输出纯文本。不要使用emoji。"
	msgs := []map[string]interface{}{{"role": "system", "content": sys}, {"role": "user", "content": prompt}}
	reqBody, _ := json.Marshal(map[string]interface{}{"model": modelName, "messages": msgs, "temperature": 0.9, "max_tokens": 200, "stream": false})
	baseURL = strings.TrimRight(baseURL, "/")
	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", strings.NewReader(string(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil { return "" }
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	var r struct{ Choices []struct{ Message struct{ Content string } } }
	json.Unmarshal(rb, &r)
	if len(r.Choices) > 0 { return strings.TrimSpace(r.Choices[0].Message.Content) }
	return ""
}

func (s *service) scheduleChanged() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Companion] scheduleChanged panic recovered: %v", r)
		}
	}()
	s.ScheduleBasedGenerator(time.Now().Format("2006-01-02"))
}
func extractTopic(content string) string {
	runes := []rune(content)
	if len(runes) < 3 { return "" }
	middle := len(runes) / 2
	start := middle - 10
	if start < 0 { start = 0 }
	end := middle + 10
	if end > len(runes) { end = len(runes) }
	return string(runes[start:end])
}



func (s *service) RandomBurstTrigger() map[string]interface{} {
	setting := s.GetActiveMessageSetting()
	enabled, _ := setting["enabled"].(bool)
	if !enabled {
		return map[string]interface{}{"triggered": false, "reason": "disabled"}
	}

	stateLife := s.GetStateLife()
	currentState, _ := stateLife["currentState"].(string)
	blockedStates := map[string]bool{"SLEEPING": true, "IN_CLASS": true, "IN_EXAM": true, "BUSY": true, "WORKING": true, "WORKING_OUT": true, "OVERTIME": true}
	if blockedStates[currentState] {
		return map[string]interface{}{"triggered": false, "reason": "blocked:" + currentState}
	}

	quietStart, _ := setting["quietStart"].(string)
	quietEnd, _ := setting["quietEnd"].(string)
	if quietStart == "" { quietStart = "23:00" }
	if quietEnd == "" { quietEnd = "07:00" }
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

	if s.lastBurstAt.Format("2006-01-02") != now.Format("2006-01-02") { s.todayBurstCount = 0 }
	minInterval, _ := setting["minInterval"].(int)
	if time.Since(s.lastBurstAt) < time.Duration(minInterval)*time.Minute {
		return map[string]interface{}{"triggered": false, "reason": "minInterval"}
	}

	maxPerDay, _ := setting["maxPerDay"].(int)
	if s.todayBurstCount >= maxPerDay {
		return map[string]interface{}{"triggered": false, "reason": "maxPerDay"}
	}

	maxDailyCalls, _ := setting["maxDailyCalls"].(int)
	if maxDailyCalls == 0 { maxDailyCalls = 10 }
	todayStr := now.Format("2006-01-02")
	var todayLLMCalls int64
	s.db.Table("proactive_messages").Where("date(created_at) = date(?)", todayStr).Count(&todayLLMCalls)
	if int(todayLLMCalls) >= maxDailyCalls {
		return map[string]interface{}{"triggered": false, "reason": "maxDailyCalls"}
	}

	activeLevel, _ := setting["activeLevel"].(int)
	if activeLevel == 0 { activeLevel = 40 }
	baseProb := float64(activeLevel) / 100.0 * 0.05

	energy, _ := stateLife["energy"].(int)
	mood, _ := stateLife["mood"].(string)
	idleSec, _ := stateLife["idleDuration"].(float64)
	idleDuration := time.Duration(idleSec) * time.Second

	energyMod := 1.0
	if energy > 70 { energyMod = 1.2 } else if energy < 30 { energyMod = 0.3 }

	moodMod := 1.0
	if mood == "happy" { moodMod = 1.3 } else if mood == "sad" || mood == "depressed" || mood == "ignored" { moodMod = 1.5 } else if mood == "tired" || mood == "lonely" { moodMod = 0.7 }

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
	if budgetRemaining < 1 { budgetRemaining = 1 }
	budgetMod := float64(budgetRemaining) / float64(maxDailyCalls)

	finalProb := baseProb * energyMod * moodMod * stateMod * budgetMod

	if idleDuration > 48*time.Hour { finalProb = finalProb * 0.1 }
	if idleDuration > 24*time.Hour { finalProb = finalProb * 0.3 }

	rng := rand.New(rand.NewSource(now.UnixNano()))
	if rng.Float64() >= finalProb {
		return map[string]interface{}{"triggered": false, "reason": "probability", "prob": finalProb}
	}

	history := s.getShareHistory()
	recentTopics := strings.Join(history.RecentTopics, "、")
	if recentTopics == "" { recentTopics = "无" }

	var recentMemoriesStr string
	rows, err := s.db.Table("memories").Select("value").Where("importance >= 2").Order("created_at DESC").Limit(3).Rows()
	if err == nil {
		defer rows.Close()
		var mems []string
		for rows.Next() {
			var v string
			rows.Scan(&v)
			if v != "" { mems = append(mems, v) }
		}
		recentMemoriesStr = strings.Join(mems, "；")
	}
	if recentMemoriesStr == "" { recentMemoriesStr = "无" }

	prompt := fmt.Sprintf("当前你处于 %s 状态，心情 %s，精力 %d/100。最近记忆：%s。请生成一条像微信里突然想到就发出的自然短消息，1-2句，不要客服腔，不要解释，不要 emoji，避免重复这些话题：%s。", currentState, mood, energy, recentMemoriesStr, recentTopics)

	msgID := fmt.Sprintf("burst-%d", now.UnixNano())
	generated := s.generateLLMReply(prompt)
	if generated == "" { return map[string]interface{}{"triggered": false, "reason": "llmFailed"} }
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

	s.lastBurstAt = now
	s.todayBurstCount++

	log.Printf("[Companion] RandomBurst triggered: prob=%.4f energyMod=%.2f moodMod=%.2f stateMod=%.2f budgetMod=%.2f", finalProb, energyMod, moodMod, stateMod, budgetMod)

	return map[string]interface{}{"triggered": true, "prob": finalProb, "burstCount": s.todayBurstCount, "prompt": prompt}
}
