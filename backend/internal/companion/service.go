package companion

import (
	"encoding/json"
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
	GetDebugOverview() map[string]interface{}
	RegenerateAllDebug() map[string]interface{}
	ProcessActiveMessagesDebug() map[string]interface{}
	ProcessDelayedRepliesDebug() map[string]interface{}
	GetRuleLogs() []map[string]interface{}
	RegenerateSchedule() map[string]interface{}
	RegenerateTimeline() map[string]interface{}
}

type service struct {
	db *gorm.DB
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
	if len(updates) > 0 { s.db.Table("sleep_settings").Where("1=1").Updates(updates) }
	return s.GetSleepSetting()
}

func (s *service) GetSchedule(date string) map[string]interface{} {
	if date == "" { date = time.Now().Format("2006-01-02") }
	var events []FixedEvent; s.db.Where("enabled = 1").Find(&events)
	var specials []SpecialEvent; s.db.Where("enabled = 1 AND event_date = ?", date).Find(&specials)
	slots := buildScheduleSlots(events, specials, date)
	return map[string]interface{}{"date": date, "slots": slots}
}

func (s *service) GetScheduleConflicts(date string) []map[string]interface{} { _ = date; return []map[string]interface{}{} }
func (s *service) GetScheduleToday() map[string]interface{} { return s.GetSchedule(time.Now().Format("2006-01-02")) }

func (s *service) GetStateLife() map[string]interface{} {
	sleep := s.GetSleepSetting()
	return map[string]interface{}{"currentState": "awake", "sleepSetting": sleep, "mood": "neutral", "energy": 80}
}

func (s *service) GetState() map[string]interface{} {
	return map[string]interface{}{"state": "awake", "sleeping": false, "lastWakeTime": time.Now().Format("2006-01-02 07:00:00"), "nextSleepTime": time.Now().Format("2006-01-02 23:00:00")}
}

func (s *service) GetTimelineToday() map[string]interface{} {
	return map[string]interface{}{"date": time.Now().Format("2006-01-02"), "events": []map[string]interface{}{}, "schedule": s.GetScheduleToday()}
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
	s.db.Create(&e)
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
	if len(updates) > 0 { s.db.Model(&FixedEvent{}).Where("id = ?", id).Updates(updates) }
	return s.GetFixedEvent(id)
}

func (s *service) DeleteFixedEvent(id int) bool { return s.db.Delete(&FixedEvent{}, id).RowsAffected > 0 }

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
	s.db.Create(&e)
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
	if len(updates) > 0 { s.db.Model(&SpecialEvent{}).Where("id = ?", id).Updates(updates) }
	return map[string]interface{}{"id": id, "updated": true}
}

func (s *service) DeleteSpecialEvent(id int) bool { return s.db.Delete(&SpecialEvent{}, id).RowsAffected > 0 }

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
	s.db.Create(&a)
	return map[string]interface{}{"id": a.ID, "className": a.ClassName}
}

func (s *service) UpdateClassAdjustment(id int, body map[string]interface{}) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["date"].(string); ok { updates["date"] = v }
	if v, ok := body["slotIndex"].(float64); ok { updates["slot_index"] = int(v) }
	if v, ok := body["className"].(string); ok { updates["class_name"] = v }
	if v, ok := body["adjustType"].(string); ok { updates["adjust_type"] = v }
	if v, ok := body["description"].(string); ok { updates["description"] = v }
	if len(updates) > 0 { s.db.Model(&ClassAdjustment{}).Where("id = ?", id).Updates(updates) }
	return map[string]interface{}{"id": id, "updated": true}
}

func (s *service) DeleteClassAdjustment(id int) bool { return s.db.Delete(&ClassAdjustment{}, id).RowsAffected > 0 }

func (s *service) GetEffectiveClasses(date string) []map[string]interface{} {
	if date == "" { date = time.Now().Format("2006-01-02") }
	return []map[string]interface{}{{"date": date, "dayOfWeek": parseDayOfWeek(date), "slots": []map[string]interface{}{}}}
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
	if len(updates) > 0 { s.db.Model(&LifestyleTendency{}).Where("1=1").Updates(updates) }
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
	if len(updates) > 0 { s.db.Model(&WorkProfile{}).Where("1=1").Updates(updates) }
	return s.GetWorkProfile()
}

func (s *service) GetActiveMessageSetting() map[string]interface{} {
	var enabled, minInterval, maxPerDay int; var channel string
	err := s.db.Table("active_message_settings").Select("enabled, min_interval, max_per_day, channel").Limit(1).Row().Scan(&enabled, &minInterval, &maxPerDay, &channel)
	if err != nil { return map[string]interface{}{"enabled": true, "minInterval": 60, "maxPerDay": 6, "channel": "all"} }
	return map[string]interface{}{"enabled": enabled == 1, "minInterval": minInterval, "maxPerDay": maxPerDay, "channel": channel}
}

func (s *service) UpdateActiveMessageSetting(body map[string]interface{}) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["enabled"].(bool); ok { if v { updates["enabled"] = 1 } else { updates["enabled"] = 0 } }
	if v, ok := body["minInterval"].(float64); ok { updates["min_interval"] = int(v) }
	if v, ok := body["maxPerDay"].(float64); ok { updates["max_per_day"] = int(v) }
	if v, ok := body["channel"].(string); ok { updates["channel"] = v }
	if len(updates) > 0 {
		var count int64; s.db.Table("active_message_settings").Count(&count)
		if count == 0 { s.db.Exec("INSERT INTO active_message_settings (enabled, min_interval, max_per_day, channel) VALUES (1, 60, 6, 'all')") }
		s.db.Table("active_message_settings").Where("1=1").Updates(updates)
	}
	return s.GetActiveMessageSetting()
}

func (s *service) GetActiveMessageTasksToday() []map[string]interface{} {
	var tasks []map[string]interface{}
	s.db.Table("active_message_task").Where("date(due_time) = date('now')").Order("due_time ASC").Find(&tasks)
	if tasks == nil { tasks = []map[string]interface{}{} }
	return tasks
}

func (s *service) RegenerateActiveMessageTasks() map[string]interface{} {
	s.db.Exec("UPDATE active_message_task SET status='CANCELLED', cancel_reason='regenerate', updated_at=datetime('now') WHERE date(due_time)=date('now') AND status='PENDING'")
	return map[string]interface{}{"regenerated": true}
}

func (s *service) RunActiveMessageTask(id int) map[string]interface{} {
	s.db.Exec("UPDATE active_message_task SET status='RUNNING', updated_at=datetime('now') WHERE id=?", id)
	return map[string]interface{}{"id": id, "status": "RUNNING"}
}

func (s *service) CancelActiveMessageTask(id int) map[string]interface{} {
	s.db.Exec("UPDATE active_message_task SET status='CANCELLED', cancel_reason='manual', updated_at=datetime('now') WHERE id=?", id)
	return map[string]interface{}{"id": id, "cancelled": true}
}

func (s *service) ListDelayedReplies() []map[string]interface{} {
	var replies []map[string]interface{}
	s.db.Table("delayed_replies").Where("status = 'pending'").Order("scheduled_at ASC").Find(&replies)
	if replies == nil { replies = []map[string]interface{}{} }
	return replies
}

func (s *service) CancelDelayedReply(id int) map[string]interface{} {
	s.db.Exec("UPDATE delayed_replies SET status='cancelled', updated_at=datetime('now') WHERE id=?", id)
	return map[string]interface{}{"id": id, "cancelled": true}
}

func (s *service) ProcessDelayedReplies() map[string]interface{} {
	return map[string]interface{}{"processed": 0}
}

func (s *service) GetDebugOverview() map[string]interface{} {
	var totalRules, totalTasks, totalReplies int64
	s.db.Table("proactive_rules").Count(&totalRules); s.db.Table("active_message_task").Count(&totalTasks); s.db.Table("messages").Where("source = 'proactive'").Count(&totalReplies)
	return map[string]interface{}{"totalRules": totalRules, "totalTasks": totalTasks, "totalProactiveReplies": totalReplies, "sleepSetting": s.GetSleepSetting(), "currentState": "awake"}
}

func (s *service) RegenerateAllDebug() map[string]interface{} { return map[string]interface{}{"regenerated": true} }
func (s *service) ProcessActiveMessagesDebug() map[string]interface{} { return map[string]interface{}{"processed": 0} }
func (s *service) ProcessDelayedRepliesDebug() map[string]interface{} { return map[string]interface{}{"processed": 0} }

func (s *service) GetRuleLogs() []map[string]interface{} {
	var logs []map[string]interface{}
	s.db.Table("proactive_rule_logs").Order("triggered_at DESC").Limit(50).Find(&logs)
	if logs == nil { logs = []map[string]interface{}{} }
	return logs
}

func (s *service) RegenerateSchedule() map[string]interface{} { return map[string]interface{}{"regenerated": true} }
func (s *service) RegenerateTimeline() map[string]interface{} { return map[string]interface{}{"regenerated": true} }

func buildScheduleSlots(events []FixedEvent, specials []SpecialEvent, date string) []ScheduleSlot {
	slots := []ScheduleSlot{}; weekDay := parseDayOfWeek(date)
	for _, e := range events { if e.WeekDay == -1 || e.WeekDay == weekDay { slots = append(slots, ScheduleSlot{DayOfWeek: weekDay, StartTime: e.StartTime, EndTime: e.EndTime, Name: e.Name, Type: "fixed"}) } }
	for _, s := range specials { if s.EventDate == date { slots = append(slots, ScheduleSlot{DayOfWeek: weekDay, StartTime: s.StartTime, EndTime: s.EndTime, Name: s.Name, Type: "special"}) } }
	return slots
}

func parseDayOfWeek(date string) int { t, err := time.Parse("2006-01-02", date); if err != nil { return int(time.Now().Weekday()) }; return int(t.Weekday()) }
func toJSON(v interface{}) string { b, _ := json.Marshal(v); return string(b) }
