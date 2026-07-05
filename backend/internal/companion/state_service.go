// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/proactive"
)

func (s *service) getIdleDuration(characterID string) time.Duration {
	var lastAt string
	err := s.db.Table("messages").
		Select("messages.created_at").
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("messages.role = 'user' AND conversations.character_id = ?", characterID).
		Order("messages.created_at DESC").Limit(1).Row().Scan(&lastAt)
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
	s.db.Table("moods").Where("character_id = ?", characterID).Order("created_at DESC").Limit(1).Find(&moods)
	if len(moods) > 0 {
		if m, ok := moods[0]["mood"].(string); ok && m != "" {
			mood = m
		}
		if m, ok := moods[0]["mood_value"].(string); ok && m != "" {
			mood = m
		}
	}

	idleDuration := s.getIdleDuration(characterID)
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
		"unifiedState":    proactive.UnifiedState{Busy: busy, Replyable: available},
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

func (s *service) GetMindState(characterID string) map[string]interface{} {
	result := map[string]interface{}{}

	psyche := s.readPsycheState(characterID)
	if psyche != nil {
		result["psyche"] = psyche
	}

	relationships := s.readRelationshipState(characterID)
	result["relationships"] = relationships

	needs := s.readNeedState(characterID)
	result["needs"] = needs

	return result
}

type psycheRecordLocal struct {
	CharacterID  string  `gorm:"column:character_id"`
	Version      string  `gorm:"column:version"`
	StateVersion int     `gorm:"column:state_version"`
	Emotion      string  `gorm:"column:emotion"`
	Mood         string  `gorm:"column:mood"`
	Stress       float64 `gorm:"column:stress"`
	Energy       float64 `gorm:"column:energy"`
	UpdatedAt    string  `gorm:"column:updated_at"`
}

func (psycheRecordLocal) TableName() string { return "psyche_states" }

type needStateLocalRecord struct {
	ID           string  `gorm:"column:id"`
	CharacterID  string  `gorm:"column:character_id"`
	NeedKey      string  `gorm:"column:need_key"`
	CurrentValue float64 `gorm:"column:current_value"`
	Baseline     float64 `gorm:"column:baseline"`
	Trend        float64 `gorm:"column:trend"`
	Saturated    bool    `gorm:"column:saturated"`
	UpdatedAt    string  `gorm:"column:updated_at"`
}

func (needStateLocalRecord) TableName() string { return "need_states" }

func (s *service) readPsycheState(characterID string) map[string]interface{} {
	var record psycheRecordLocal
	err := s.db.Where("character_id = ?", characterID).Take(&record).Error
	if err != nil {
		return nil
	}
	result := map[string]interface{}{
		"version":      record.Version,
		"stateVersion": record.StateVersion,
		"stress":       record.Stress,
		"energy":       record.Energy,
		"updatedAt":    record.UpdatedAt,
	}
	if record.Emotion != "" {
		var emotion map[string]interface{}
		if json.Unmarshal([]byte(record.Emotion), &emotion) == nil {
			result["emotion"] = emotion
		}
	}
	if record.Mood != "" {
		var mood map[string]interface{}
		if json.Unmarshal([]byte(record.Mood), &mood) == nil {
			result["mood"] = mood
		}
	}
	return result
}

func (s *service) readRelationshipState(characterID string) []map[string]interface{} {
	var records []map[string]interface{}
	s.db.Table("relationship_states").Where("character_id = ?", characterID).Order("updated_at DESC").Find(&records)
	if records == nil {
		records = []map[string]interface{}{}
	}
	for i, record := range records {
		if dataStr, ok := record["relation_data"].(string); ok && dataStr != "" {
			var data map[string]interface{}
			if json.Unmarshal([]byte(dataStr), &data) == nil {
				records[i]["data"] = data
			}
		}
	}
	return records
}

func (s *service) readNeedState(characterID string) []map[string]interface{} {
	var records []needStateLocalRecord
	s.db.Where("character_id = ?", characterID).Order("need_key ASC").Find(&records)
	result := make([]map[string]interface{}, 0, len(records))
	for _, r := range records {
		result = append(result, map[string]interface{}{
			"needKey":      r.NeedKey,
			"currentValue": r.CurrentValue,
			"baseline":     r.Baseline,
			"trend":        r.Trend,
			"saturated":    r.Saturated,
			"updatedAt":    r.UpdatedAt,
		})
	}
	return result
}
