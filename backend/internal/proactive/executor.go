// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

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
	type rule struct {
		id, enabled, maxPerDay, sentToday, randomMinutes            int
		name, channel, ruleType, cron, quietStart, quietEnd, prompt string
		charID, convID, lastSentAt                                  string
	}

	rows, err := e.db.Table("proactive_rules").
		Select("id, name, enabled, channel, character_id, conversation_id, rule_type, schedule_cron, quiet_start, quiet_end, max_per_day, sent_count_today, prompt_template, random_minutes, COALESCE(last_sent_at,'')").
		Where("enabled = 1").Rows()
	if err != nil {
		return
	}
	defer rows.Close()

	now := time.Now()
	nowTotalMins := now.Hour()*60 + now.Minute()
	timeStr := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	for rows.Next() {
		var r rule
		rows.Scan(&r.id, &r.name, &r.enabled, &r.channel, &r.charID, &r.convID, &r.ruleType,
			&r.cron, &r.quietStart, &r.quietEnd, &r.maxPerDay, &r.sentToday,
			&r.prompt, &r.randomMinutes, &r.lastSentAt)

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
		log.Printf("[Proactive] 触发规则 id=%d name=%s channel=%s", r.id, r.name, r.channel)
		ruleCopy := r
		go func() {
			defer e.markRuleDone(ruleCopy.id)
			e.executeRule(ruleCopy)
		}()
	}
}

func (e *Executor) executeRule(r struct {
	id, enabled, maxPerDay, sentToday, randomMinutes                                        int
	name, channel, ruleType, cron, quietStart, quietEnd, prompt, charID, convID, lastSentAt string
}) {
	character, ok := resolveProactiveCharacter(e.db, r.charID, r.convID)
	if !ok {
		log.Printf("[Proactive] 规则 id=%d 缺少有效角色作用域", r.id)
		return
	}

	channel := r.channel
	if channel == "" {
		channel = "all"
	}
	convID := resolveProactiveConversation(e.db, r.convID, character.ID, channel, false)
	if convID == "" {
		log.Printf("[Proactive] 规则 id=%d 无可用对话", r.id)
		return
	}

	if e.dispatch == nil {
		log.Printf("[Proactive] 规则 id=%d 主动消息统一调度未配置，无法发送", r.id)
		status := "failed"
		e.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status) VALUES (?, ?, ?, ?, ?)",
			r.id, convID, "", channel, status)
		e.db.Exec("UPDATE proactive_rules SET sent_count_today=sent_count_today+1, last_sent_at=?, updated_at=? WHERE id=?",
			time.Now(), time.Now(), r.id)
		return
	}

	requestID := fmt.Sprintf("proactive-rule-%d-%d", r.id, time.Now().Unix())
	result, err := e.dispatch.DispatchProactive(context.Background(), ProactiveDispatchRequest{
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
	e.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status) VALUES (?, ?, ?, ?, ?)",
		r.id, convID, content, channel, status)
	e.db.Exec("UPDATE proactive_rules SET sent_count_today=sent_count_today+1, last_sent_at=?, updated_at=? WHERE id=?",
		time.Now(), time.Now(), r.id)
}

func (e *Executor) ScanReminders() {
	type rem struct {
		id, enabled                             int
		title, content, channel, charID, convID string
		remindAt, repeatRule, lastTriggeredAt   string
	}

	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	nowDate := now.Format("2006-01-02")
	nowTime := now.Format("15:04")

	rows, err := e.db.Table("reminders").
		Select("id, title, content, channel, character_id, conversation_id, remind_at, repeat_rule, enabled, last_triggered_at").
		Where("enabled = 1 AND remind_at <= ?", nowStr).
		Order("remind_at ASC").Limit(20).Rows()
	if err != nil {
		return
	}
	defer rows.Close()

	var pendingRems []rem
	for rows.Next() {
		var r rem
		rows.Scan(&r.id, &r.title, &r.content, &r.channel, &r.charID, &r.convID,
			&r.remindAt, &r.repeatRule, &r.enabled, &r.lastTriggeredAt)
		pendingRems = append(pendingRems, r)
	}
	rows.Close()

	for _, r := range pendingRems {
		log.Printf("[Reminder] 触发提醒 id=%d title=%s channel=%s", r.id, r.title, r.channel)
		go e.executeReminder(r)

		if r.repeatRule != "" && r.repeatRule != "none" {
			nextAt := calcNextRemindAt(r.remindAt, r.repeatRule, nowDate, nowTime)
			if nextAt != "" {
				e.db.Exec("UPDATE reminders SET remind_at=?, last_triggered_at=?, updated_at=? WHERE id=?",
					nextAt, nowStr, nowStr, r.id)
			} else {
				tomorrow := now.Add(24 * time.Hour).Format("2006-01-02")
				nextFull := tomorrow + " " + r.remindAt[11:19]
				e.db.Exec("UPDATE reminders SET remind_at=?, last_triggered_at=?, updated_at=? WHERE id=?",
					nextFull, nowStr, nowStr, r.id)
			}
		} else {
			e.db.Exec("UPDATE reminders SET enabled=0, last_triggered_at=?, updated_at=? WHERE id=?",
				nowStr, nowStr, r.id)
		}
	}
}

func (e *Executor) executeReminder(r struct {
	id, enabled                             int
	title, content, channel, charID, convID string
	remindAt, repeatRule, lastTriggeredAt   string
}) {
	convID := r.convID
	if convID == "" {
		convID = resolveProactiveConversation(e.db, "", r.charID, r.channel, false)
	} else {
		convID = resolveProactiveConversation(e.db, r.convID, r.charID, r.channel, false)
	}
	if convID == "" {
		log.Printf("[Reminder] 提醒 id=%d 无可用对话", r.id)
		e.recordTriggerHistory(r.id, r.title, "reminder", r.channel, "failed", "无可用对话")
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
		e.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status) VALUES (?, ?, ?, ?, ?)",
			r.id, convID, "", channel, "failed")
		e.recordTriggerHistory(r.id, r.title, "reminder", channel, "failed", "统一调度未配置")
		return
	}

	requestID := fmt.Sprintf("proactive-reminder-%d-%d", r.id, time.Now().Unix())
	result, err := e.dispatch.DispatchProactive(context.Background(), ProactiveDispatchRequest{
		CharacterID:    r.charID,
		ConversationID: convID,
		Channel:        channel,
		Prompt:         content,
		RequestID:      requestID,
	})
	status := "pending"
	contentStr := ""
	if err != nil || (result != nil && !result.Success) {
		status = "failed"
	}
	if result != nil {
		contentStr = result.Content
	}
	e.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status) VALUES (?, ?, ?, ?, ?)",
		r.id, convID, contentStr, channel, status)
	finalState := "sent"
	if status == "failed" {
		finalState = "failed"
	}
	e.recordTriggerHistory(r.id, r.title, "reminder", channel, finalState, "")
	log.Printf("[Reminder] 提醒 id=%d title=%s 已通过统一调度发送", r.id, r.title)
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

func (e *Executor) ExecuteShareTask(prompt, conversationID, characterID string) string {
	character, ok := resolveProactiveCharacter(e.db, characterID, conversationID)
	if !ok {
		log.Println("[Proactive] ExecuteShareTask: missing scoped character")
		return ""
	}
	convID := resolveProactiveConversation(e.db, conversationID, character.ID, "all", false)
	if convID == "" {
		log.Println("[Proactive] ExecuteShareTask: no scoped conversation")
		return ""
	}

	if e.dispatch == nil {
		log.Println("[Proactive] ExecuteShareTask: 主动消息统一调度未配置")
		e.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, ?, ?, ?, ?)",
			convID, "", "all", "failed", time.Now(), time.Now())
		return ""
	}

	requestID := fmt.Sprintf("proactive-share-%d", time.Now().UnixNano())
	result, err := e.dispatch.DispatchProactive(context.Background(), ProactiveDispatchRequest{
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
	e.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, ?, ?, ?, ?)",
		convID, content, "all", status, time.Now(), time.Now())
	log.Printf("[Proactive] ExecuteShareTask dispatched via unified entry: success=%v", err == nil && result != nil && result.Success)
	return content
}

func (e *Executor) recordTriggerHistory(triggerID int, title, triggerType, channel, state, lastError string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	id := uuid.New().String()
	e.db.Exec("INSERT INTO trigger_histories (id, trigger_id, trigger_type, title, channel, state, priority, reason, attempt_count, last_error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'normal', '系统触发', 0, ?, ?, ?)", id, fmt.Sprintf("%d", triggerID), triggerType, title, channel, state, lastError, now, now)
}
