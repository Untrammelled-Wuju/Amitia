// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type scheduledRule struct {
	id, enabled, maxPerDay, sentToday, randomMinutes                    int
	userID, name, channel, ruleType, cron, quietStart, quietEnd, prompt string
	charID, convID, lastSentAt                                          string
}

type scheduledReminder struct {
	id, enabled                                     int
	userID, title, content, channel, charID, convID string
	remindAt, repeatRule, lastTriggeredAt           string
}

type Executor struct {
	db           *gorm.DB
	runningRules sync.Map
	dispatch     ProactiveDispatch
}

func NewExecutor(db *gorm.DB) *Executor {
	return &Executor{db: db}
}

func (e *Executor) SetDispatch(d ProactiveDispatch) {
	e.dispatch = d
}

func (e *Executor) isRuleRunning(id int) bool {
	_, ok := e.runningRules.Load(id)
	return ok
}

func (e *Executor) markRuleRunning(id int) {
	e.runningRules.Store(id, true)
}

func (e *Executor) markRuleDone(id int) {
	e.runningRules.Delete(id)
}

func (e *Executor) ScanAndExecute() {
	e.ScanRules()
	e.ScanReminders()
}

func (e *Executor) ScanRules() {
	rows, err := e.db.Table("proactive_rules").
		Select("id, user_id, name, enabled, channel, character_id, conversation_id, rule_type, schedule_cron, quiet_start, quiet_end, max_per_day, sent_count_today, prompt_template, random_minutes, COALESCE(last_sent_at,'')").
		Where("enabled = 1").Rows()
	if err != nil {
		return
	}
	defer rows.Close()

	now := time.Now()
	nowTotalMins := now.Hour()*60 + now.Minute()
	timeStr := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	for rows.Next() {
		var r scheduledRule
		if err := rows.Scan(&r.id, &r.userID, &r.name, &r.enabled, &r.channel, &r.charID, &r.convID, &r.ruleType,
			&r.cron, &r.quietStart, &r.quietEnd, &r.maxPerDay, &r.sentToday,
			&r.prompt, &r.randomMinutes, &r.lastSentAt); err != nil {
			continue
		}
		r.userID = strings.TrimSpace(r.userID)
		if r.userID == "" {
			continue
		}

		if r.cron == "" || r.sentToday >= r.maxPerDay {
			continue
		}
		if !quietHoursAllow(r.quietStart, r.quietEnd, timeStr) {
			continue
		}

		if len(r.lastSentAt) >= 19 {
			lastTime, err := time.Parse("2006-01-02 15:04:05", r.lastSentAt[:19])
			if err == nil && now.Sub(lastTime) < time.Duration(r.randomMinutes+10)*time.Minute {
				continue
			}
		}

		baseMin := parseCronMinute(r.cron)
		if baseMin < 0 {
			continue
		}

		window := r.randomMinutes
		if window <= 0 {
			window = 30
		}
		ws := baseMin - window
		if ws < 0 {
			ws = 0
		}
		we := baseMin + window
		if we > 1439 {
			we = 1439
		}

		if nowTotalMins < ws || nowTotalMins > we {
			continue
		}

		if e.isRuleRunning(r.id) {
			continue
		}
		e.markRuleRunning(r.id)
		log.Printf("[Proactive] 触发规则 id=%d name=%s", r.id, r.name)
		ruleCopy := r
		go func() {
			defer e.markRuleDone(ruleCopy.id)
			e.executeRule(ruleCopy)
		}()
	}
}

func (e *Executor) executeRule(r scheduledRule) {
	character, ok := resolveProactiveCharacterForUser(e.db, r.userID, r.charID, r.convID)
	if !ok {
		log.Printf("[Proactive] 规则 id=%d 缺少当前用户的有效角色作用域", r.id)
		return
	}

	channel := r.channel
	if channel == "" {
		channel = "all"
	}
	convID := resolveProactiveConversationForUser(e.db, r.userID, r.convID, character.ID, channel, false)
	if convID == "" {
		log.Printf("[Proactive] 规则 id=%d 无当前用户可用对话", r.id)
		return
	}

	if e.dispatch == nil {
		log.Printf("[Proactive] 规则 id=%d 主动消息统一调度未配置，无法发送", r.id)
		status := "failed"
		e.db.Exec("INSERT INTO proactive_messages (user_id, rule_id, conversation_id, message_content, channel, status) VALUES (?, ?, ?, ?, ?, ?)",
			r.userID, r.id, convID, "", channel, status)
		e.db.Exec("UPDATE proactive_rules SET sent_count_today=sent_count_today+1, last_sent_at=?, updated_at=? WHERE id=? AND user_id=?",
			time.Now(), time.Now(), r.id, r.userID)
		return
	}

	requestID := fmt.Sprintf("proactive-rule-%d-%d", r.id, time.Now().Unix())
	result, err := e.dispatch.DispatchProactive(context.Background(), ProactiveDispatchRequest{
		UserID:         r.userID,
		CharacterID:    character.ID,
		ConversationID: convID,
		Channel:        channel,
		Prompt:         r.prompt,
		RequestID:      requestID,
	})
	status := "pending"
	content := ""
	if err != nil || (result != nil && !result.Success) {
		status = "failed"
		log.Printf("[Proactive] 规则 id=%d 统一调度失败: %v", r.id, err)
	} else if result != nil {
		content = result.Content
	}
	e.db.Exec("INSERT INTO proactive_messages (user_id, rule_id, conversation_id, message_content, channel, status) VALUES (?, ?, ?, ?, ?, ?)",
		r.userID, r.id, convID, content, channel, status)
	e.db.Exec("UPDATE proactive_rules SET sent_count_today=sent_count_today+1, last_sent_at=?, updated_at=? WHERE id=? AND user_id=?",
		time.Now(), time.Now(), r.id, r.userID)
}

func (e *Executor) ScanReminders() {
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	nowDate := now.Format("2006-01-02")
	nowTime := now.Format("15:04")

	rows, err := e.db.Table("reminders").
		Select("id, user_id, title, content, channel, character_id, conversation_id, remind_at, repeat_rule, enabled, COALESCE(last_triggered_at,'')").
		Where("enabled = 1 AND remind_at <= ?", nowStr).
		Order("remind_at ASC").Limit(20).Rows()
	if err != nil {
		return
	}
	defer rows.Close()

	var pendingRems []scheduledReminder
	for rows.Next() {
		var r scheduledReminder
		if err := rows.Scan(&r.id, &r.userID, &r.title, &r.content, &r.channel, &r.charID, &r.convID,
			&r.remindAt, &r.repeatRule, &r.enabled, &r.lastTriggeredAt); err != nil {
			continue
		}
		r.userID = strings.TrimSpace(r.userID)
		if r.userID != "" {
			pendingRems = append(pendingRems, r)
		}
	}

	for _, r := range pendingRems {
		log.Printf("[Reminder] 触发提醒 id=%d title=%s", r.id, r.title)
		go e.executeReminder(r)

		if r.repeatRule != "" && r.repeatRule != "none" {
			nextAt := calcNextRemindAt(r.remindAt, r.repeatRule, nowDate, nowTime)
			if nextAt == "" && len(r.remindAt) >= 19 {
				tomorrow := now.Add(24 * time.Hour).Format("2006-01-02")
				nextAt = tomorrow + " " + r.remindAt[11:19]
			}
			if nextAt != "" {
				e.db.Exec("UPDATE reminders SET remind_at=?, last_triggered_at=?, updated_at=? WHERE id=? AND user_id=?",
					nextAt, nowStr, nowStr, r.id, r.userID)
			}
		} else {
			e.db.Exec("UPDATE reminders SET enabled=0, last_triggered_at=?, updated_at=? WHERE id=? AND user_id=?",
				nowStr, nowStr, r.id, r.userID)
		}
	}
}

func (e *Executor) executeReminder(r scheduledReminder) {
	convID := resolveProactiveConversationForUser(e.db, r.userID, r.convID, r.charID, r.channel, false)
	if convID == "" {
		log.Printf("[Reminder] 提醒 id=%d 无当前用户可用对话", r.id)
		e.recordTriggerHistory(r.userID, r.id, r.title, "reminder", r.channel, "failed", "无可用对话")
		return
	}

	content := r.content
	if content == "" {
		content = r.title
	}

	channel := r.channel
	if channel == "" {
		channel = "web"
	}

	if e.dispatch == nil {
		log.Printf("[Reminder] 提醒 id=%d 主动消息统一调度未配置，无法发送", r.id)
		e.db.Exec("INSERT INTO proactive_messages (user_id, rule_id, conversation_id, message_content, channel, status) VALUES (?, ?, ?, ?, ?, ?)",
			r.userID, r.id, convID, "", channel, "failed")
		e.recordTriggerHistory(r.userID, r.id, r.title, "reminder", channel, "failed", "统一调度未配置")
		return
	}

	requestID := fmt.Sprintf("proactive-reminder-%d-%d", r.id, time.Now().Unix())
	result, err := e.dispatch.DispatchProactive(context.Background(), ProactiveDispatchRequest{
		UserID:         r.userID,
		CharacterID:    r.charID,
		ConversationID: convID,
		Channel:        channel,
		Prompt:         content,
		RequestID:      requestID,
	})
	status := "pending"
	contentStr := ""
	lastError := ""
	if err != nil || (result != nil && !result.Success) {
		status = "failed"
		if err != nil {
			lastError = err.Error()
		}
	}
	if result != nil {
		contentStr = result.Content
	}
	e.db.Exec("INSERT INTO proactive_messages (user_id, rule_id, conversation_id, message_content, channel, status) VALUES (?, ?, ?, ?, ?, ?)",
		r.userID, r.id, convID, contentStr, channel, status)
	finalState := "sent"
	if status == "failed" {
		finalState = "failed"
	}
	e.recordTriggerHistory(r.userID, r.id, r.title, "reminder", channel, finalState, lastError)
	log.Printf("[Reminder] 提醒 id=%d title=%s 已通过统一调度处理", r.id, r.title)
}

func calcNextRemindAt(remindAt, repeatRule, nowDate, nowTime string) string {
	if remindAt == "" || len(remindAt) < 16 {
		return ""
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", remindAt, time.Local)
	if err != nil {
		return ""
	}
	switch repeatRule {
	case "daily":
		return t.Add(24 * time.Hour).Format("2006-01-02 15:04:05")
	case "weekly":
		return t.Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	case "monthly":
		return t.AddDate(0, 1, 0).Format("2006-01-02 15:04:05")
	case "hourly":
		return t.Add(1 * time.Hour).Format("2006-01-02 15:04:05")
	default:
		return ""
	}
}

func parseCronMinute(cron string) int {
	parts := strings.Fields(cron)
	if len(parts) < 2 {
		t, err := time.Parse("15:04", cron)
		if err == nil {
			return t.Hour()*60 + t.Minute()
		}
		return -1
	}
	h := 0
	m := 0
	fmt.Sscanf(parts[1], "%d", &h)
	fmt.Sscanf(parts[0], "%d", &m)
	return h*60 + m
}

func quietHoursAllow(start, end, now string) bool {
	if start == "" || end == "" {
		return true
	}
	if start <= end {
		return now < start || now >= end
	}
	return now >= end && now < start
}

func (e *Executor) ownerForShareTask(conversationID, characterID string) string {
	var userID string
	if strings.TrimSpace(conversationID) != "" {
		e.db.Table("conversations").Select("user_id").Where("id = ?", conversationID).Limit(1).Row().Scan(&userID)
	}
	if strings.TrimSpace(userID) == "" && strings.TrimSpace(characterID) != "" {
		e.db.Table("characters").Select("user_id").Where("id = ?", characterID).Limit(1).Row().Scan(&userID)
	}
	return strings.TrimSpace(userID)
}

func (e *Executor) ExecuteShareTask(prompt, conversationID, characterID string) string {
	userID := e.ownerForShareTask(conversationID, characterID)
	if userID == "" {
		log.Println("[Proactive] ExecuteShareTask: missing owner scope")
		return ""
	}
	character, ok := resolveProactiveCharacterForUser(e.db, userID, characterID, conversationID)
	if !ok {
		log.Println("[Proactive] ExecuteShareTask: missing scoped character")
		return ""
	}
	convID := resolveProactiveConversationForUser(e.db, userID, conversationID, character.ID, "all", false)
	if convID == "" {
		log.Println("[Proactive] ExecuteShareTask: no scoped conversation")
		return ""
	}

	if e.dispatch == nil {
		log.Println("[Proactive] ExecuteShareTask: 主动消息统一调度未配置")
		e.db.Exec("INSERT INTO proactive_messages (user_id, rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (?, 0, ?, ?, ?, ?, ?, ?)",
			userID, convID, "", "all", "failed", time.Now(), time.Now())
		return ""
	}

	requestID := fmt.Sprintf("proactive-share-%d", time.Now().UnixNano())
	result, err := e.dispatch.DispatchProactive(context.Background(), ProactiveDispatchRequest{
		UserID:         userID,
		CharacterID:    character.ID,
		ConversationID: convID,
		Channel:        "all",
		Prompt:         prompt,
		RequestID:      requestID,
	})
	status := "pending"
	content := ""
	if err != nil || (result != nil && !result.Success) {
		status = "failed"
	} else if result != nil {
		content = result.Content
	}
	e.db.Exec("INSERT INTO proactive_messages (user_id, rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (?, 0, ?, ?, ?, ?, ?, ?)",
		userID, convID, content, "all", status, time.Now(), time.Now())
	log.Printf("[Proactive] ExecuteShareTask dispatched via unified entry: success=%v", err == nil && result != nil && result.Success)
	return content
}

func (e *Executor) recordTriggerHistory(userID string, triggerID int, title, triggerType, channel, state, lastError string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	id := uuid.New().String()
	e.db.Exec("INSERT INTO trigger_histories (id, user_id, trigger_id, trigger_type, title, channel, state, priority, reason, attempt_count, last_error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 'normal', '系统触发', 0, ?, ?, ?)", id, userID, fmt.Sprintf("%d", triggerID), triggerType, title, channel, state, lastError, now, now)
}
