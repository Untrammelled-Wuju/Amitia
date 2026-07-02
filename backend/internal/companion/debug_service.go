// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"context"
	"log"
	"math/rand"
	"time"
)

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
		convID := s.resolveConversationID(characterID, channelSetting, "")
		if convID == "" {
			failed++
			continue
		}
		dispatchResult, dispatchErr := s.submitProactiveMessage(context.Background(), characterID, convID, channelSetting, prompt, proactiveRequestID("proactive-due", id, now))
		if dispatchErr != nil {
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
		messageContent := prompt
		if dispatchResult != nil && dispatchResult.Response != nil && dispatchResult.Response.Reply != "" {
			messageContent = dispatchResult.Response.Reply
		}
		s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, ?, 'queued', ?, ?)", convID, messageContent, channelSetting, nowStr, nowStr)
		s.db.Exec("UPDATE active_message_task SET status='SENT', sent_at=?, updated_at=datetime('now', 'localtime') WHERE id=? AND character_id=?", nowStr, id, characterID)
		log.Printf("[Companion] ProcessDueActiveMessageTasks sent type=%s id=%v", taskType, id)
		sent++
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
