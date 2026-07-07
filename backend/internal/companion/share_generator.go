// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"fmt"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/proactive"
)

func calculateMaxShareTasks(dailyShareTendency int, idleDuration time.Duration) int {
	maxTasks := 3
	if dailyShareTendency >= 60 {
		maxTasks = 5
	}
	if dailyShareTendency < 30 {
		maxTasks = 2
	}
	if idleDuration > 48*time.Hour {
		return 0
	}
	if idleDuration > 24*time.Hour {
		return 1
	}
	if proactive.IdleChaseThreshold(proactive.ClassifyIdle(idleDuration)) {
		if maxTasks > 2 {
			return 2
		}
	}
	if idleDuration > 6*time.Hour && maxTasks > 3 {
		maxTasks = 3
	}
	return maxTasks
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
		prompt := s.GenerateSharePrompt(characterID, taskType, schedule, mood, energy)
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

	idleDuration := s.getIdleDuration(characterID)
	maxTasks := calculateMaxShareTasks(dailyShareTendency, idleDuration)
	added := 0
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

	if proactive.IdleChaseThreshold(proactive.ClassifyIdle(idleDuration)) {
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

func (s *service) GenerateSharePrompt(characterID string, taskType string, schedule TodaySchedule, mood string, energy int) string {
	dateStr := schedule.WakeTime.Format("2006-01-02")
	sleepSummary := "正常"

	var recentMemories []string
	queryText := fmt.Sprintf("心情%s 精力%d", mood, energy)
	if taskType == "morning_share" {
		queryText = fmt.Sprintf("早晨 起床 心情%s", mood)
	} else if taskType == "evening_reflection" || taskType == "bedtime_mood" {
		queryText = fmt.Sprintf("晚上 睡觉前 心情%s", mood)
	}
	if !s.isMemoryAccessAllowed(characterID) {
	} else if s.embeddingSvc != nil && qdrantDB.Client != nil {
		vec, vecErr := s.embeddingSvc.Embed(queryText)
		if vecErr == nil {
			filter := map[string]interface{}{"character_id": characterID}
			points, searchErr := qdrantDB.SearchVectors(vec, 5, filter)
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
		rows, err := s.db.Table("memories").Select("value").Where("character_id = ? AND allow_proactive_mention = 1 AND verified_status NOT IN ('deleted','invalidated','expired','rejected','tombstone','tombstoned','inactive') AND (expires_at IS NULL OR expires_at > datetime('now'))", characterID).Order("created_at DESC").Limit(5).Rows()
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

	history := s.getShareHistory(characterID)
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

func (s *service) GetShareHistory(characterID string) ShareHistory {
	var topics []string
	var lastAt string

	var rows []map[string]interface{}
	query := s.db.Table("proactive_messages").Select("message_content, created_at").Order("created_at DESC").Limit(30)
	if characterID != "" {
		query = s.db.Table("proactive_messages").Select("proactive_messages.message_content, proactive_messages.created_at").Joins("JOIN conversations ON conversations.id = proactive_messages.conversation_id").Where("conversations.character_id = ?", characterID).Order("proactive_messages.created_at DESC").Limit(30)
	}
	query.Find(&rows)

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

func (s *service) getShareHistory(characterID string) ShareHistory {
	return s.GetShareHistory(characterID)
}

func (s *service) TriggerDailyRegeneration(characterID string) map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	return s.ScheduleBasedGenerator(today, characterID)
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
