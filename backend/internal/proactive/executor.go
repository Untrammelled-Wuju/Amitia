// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/tts"
	"gorm.io/gorm"
	"math/rand"
)

type Executor struct {
	db           *gorm.DB
	runningRules sync.Map
}

func NewExecutor(db *gorm.DB) *Executor {
	return &Executor{db: db}
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
		charID, lastSentAt                                          string
	}

	rows, err := e.db.Table("proactive_rules").
		Select("id, name, enabled, channel, character_id, rule_type, schedule_cron, quiet_start, quiet_end, max_per_day, sent_count_today, prompt_template, random_minutes, COALESCE(last_sent_at,'')").
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
		rows.Scan(&r.id, &r.name, &r.enabled, &r.channel, &r.charID, &r.ruleType,
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
	id, enabled, maxPerDay, sentToday, randomMinutes                                int
	name, channel, ruleType, cron, quietStart, quietEnd, prompt, charID, lastSentAt string
}) {
	var charName, identity, convID string
	if r.charID != "" {
		row := e.db.Table("characters").Select("name, COALESCE(identity,''), conversation_id").
			Where("id = ?", r.charID).Limit(1).Row()
		row.Scan(&charName, &identity, &convID)
	}
	if charName == "" || convID == "" {
		row := e.db.Table("characters").Select("name, identity, conversation_id").
			Where("is_active = 1").Limit(1).Row()
		if err := row.Scan(&charName, &identity, &convID); err != nil || convID == "" {
			return
		}
	}

	content := e.generateContent(r.name, r.ruleType, r.prompt, charName, identity)
	if content == "" {
		return
	}

	channel := r.channel
	if channel == "" {
		channel = "all"
	}

	sentWeb, sentWechat, sentQQ := false, false, false
	if channel == "web" || channel == "all" || strings.Contains(channel, "web") {
		e.sendToWeb(convID, content)
		sentWeb = true
	}
	if channel == "wechat" || channel == "all" || strings.Contains(channel, "wechat") {
		wcID := e.getWechatConvID(convID)
		if wcID != "" {
			e.sendToWechat(wcID, content)
			sentWechat = true
		}
	}
	if channel == "qq" || channel == "all" || strings.Contains(channel, "qq") {
		qqID := e.getQQConvID(r.charID)
		if qqID != "" {
			e.sendToQQ(qqID, content)
			sentQQ = true
		}
	}

	status := "sent"
	if !sentWeb && !sentWechat && !sentQQ {
		status = "failed"
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
		if r.charID != "" {
			row := e.db.Table("conversations").Select("id").
				Where("character_id = ? AND channel = 'web'", r.charID).
				Limit(1).Row()
			row.Scan(&convID)
		}
		if convID == "" {
			row := e.db.Table("conversations").Select("id").
				Where("channel = 'web'").
				Limit(1).Row()
			row.Scan(&convID)
		}
	}
	if convID == "" {
		log.Printf("[Reminder] 提醒 id=%d 无可用对话", r.id)
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

	sentWeb, sentWechat, sentQQ := false, false, false
	if channel == "web" || channel == "all" || strings.Contains(channel, "web") {
		e.sendToWeb(convID, content)
		sentWeb = true
	}
	if channel == "wechat" || channel == "all" || strings.Contains(channel, "wechat") {
		wcID := e.getWechatConvID(r.charID)
		if wcID != "" {
			if e.sendToWechat(wcID, content) {
				sentWechat = true
			}
		}
	}
	if channel == "qq" || channel == "all" || strings.Contains(channel, "qq") {
		if e.sendToQQ(convID, content) {
			sentQQ = true
		}
	}

	status := "sent"
	if !sentWeb && !sentWechat && !sentQQ {
		status = "failed"
	}

	e.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status) VALUES (?, ?, ?, ?, ?)",
		r.id, convID, content, channel, status)

	log.Printf("[Reminder] 提醒 id=%d title=%s 已发送 (web=%v wechat=%v qq=%v)", r.id, r.title, sentWeb, sentWechat, sentQQ)
}

func (e *Executor) generateContent(name, ruleType, prompt, charName, identity string) string {
	if identity == "" {
		identity = "一个AI伙伴"
	}
	if prompt == "" {
		prompt = "发一条自然的主动消息。"
	}

	sys := fmt.Sprintf("你是%s，%s。\n你的语气自然、口语化。\n字数控制在8-40字。\n【重要】不要调用工具，直接输出纯文本。", charName, identity)
	usr := fmt.Sprintf("【主动消息 - 不要调用工具】\n任务：%s (%s)\n要求：%s\n直接输出消息（无前缀无引号）：", name, ruleType, prompt)

	cfg := e.getActiveModelCron()
	if cfg == nil {
		return ""
	}

	msgs := []map[string]interface{}{
		{"role": "system", "content": "【系统规则】回复中不要使用任何emoji表情符号。"},
		{"role": "system", "content": sys},
		{"role": "user", "content": usr},
	}

	baseURL := strings.TrimRight(cfg["baseUrl"], "/")
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": cfg["modelName"], "messages": msgs,
		"temperature": 0.9, "max_tokens": 200, "stream": false,
	})

	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg["apiKey"])

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

func (e *Executor) getActiveModelCron() map[string]string {
	var baseURL, apiKey, modelName string
	err := e.db.Table("model_configs").
		Select("base_url, api_key, model_name").
		Where("is_active = 1").Limit(1).Row().
		Scan(&baseURL, &apiKey, &modelName)
	if err != nil {
		return nil
	}
	return map[string]string{"baseUrl": baseURL, "apiKey": apiKey, "modelName": modelName}
}

func (e *Executor) sendToWeb(convID, content string) {
	now := time.Now()
	msgID := fmt.Sprintf("proactive-%d", now.UnixNano())
	displayContent := content
	audioUrl := ""
	var audioDuration float64

	ttsRepo := tts.NewRepository(e.db)
	activeCfg, cfgErr := ttsRepo.GetActive()
	if cfgErr == nil && activeCfg.ApiKey != "" && content != "" {
		cfg := &tts.TtsConfig{ApiKey: activeCfg.ApiKey, ResourceId: activeCfg.ResourceId, VoiceType: activeCfg.VoiceType, Speed: activeCfg.Speed, Pitch: activeCfg.Pitch, Volume: activeCfg.Volume}
		if cfg.VoiceType == "" {
			cfg.VoiceType = "zh_female_vv_uranus_bigtts"
		}
		if cfg.Speed == 0 {
			cfg.Speed = 1.0
		}
		if cfg.Pitch == 0 {
			cfg.Pitch = 1.0
		}
		if cfg.Volume == 0 {
			cfg.Volume = 1.0
		}
		if rand.Float64() < 0.20 {
			synthResult, synthErr := tts.Synthesize(cfg, content)
			if synthErr == nil {
				audioUrl = synthResult.AudioURL
				audioDuration = synthResult.Duration
				displayContent = ""
			}
		}
	}

	e.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, audio_url, audio_duration, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?, ?, ?)",
		msgID, convID, displayContent, audioUrl, audioDuration, now)
}

func (e *Executor) sendToWechat(convID, content string) bool {
	toUserID := convID
	if strings.HasPrefix(convID, "conv-") {
		toUserID = convID[5:]
	}

	body, _ := json.Marshal(map[string]string{"toUserId": toUserID, "text": content})
	req, _ := http.NewRequest("POST", "http://127.0.0.1:9876/api/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Proactive] 微信发送失败: %v", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		var respBody bytes.Buffer
		respBody.ReadFrom(resp.Body)
		log.Printf("[Proactive] 微信返回 %d body=%s", resp.StatusCode, respBody.String())
		return false
	}
	log.Printf("[Proactive] 微信已发送 to=%s", toUserID)
	return true
}
func (e *Executor) getWechatConvID(charID string) string {
	var id string
	e.db.Table("conversations").Select("id").
		Where("channel = 'wechat' AND peer_id != '' AND peer_id IS NOT NULL").
		Order("updated_at DESC").
		Limit(1).Row().Scan(&id)
	return id
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
		// Try HH:MM format
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

func (e *Executor) sendToQQ(userID, content string) bool {
	convID := userID
	targetID := userID
	if strings.HasPrefix(userID, "conv-qq-") {
		targetID = userID[len("conv-qq-"):]
	} else if strings.HasPrefix(userID, "conv-") {
		targetID = userID[5:]
	}

	useVoice := rand.Float64() < 0.20
	voiceOK := false

	body, _ := json.Marshal(map[string]string{"toUserId": targetID, "text": content})
	client := &http.Client{Timeout: 60 * time.Second}

	if useVoice {
		voiceReq, _ := http.NewRequest("POST", "http://127.0.0.1:9877/api/send-voice", bytes.NewReader(body))
		voiceReq.Header.Set("Content-Type", "application/json")
		voiceResp, voiceErr := client.Do(voiceReq)
		if voiceErr == nil && voiceResp.StatusCode == 200 {
			voiceOK = true
		}
		if voiceResp != nil {
			voiceResp.Body.Close()
		}
	}

	if !voiceOK {
		textReq, _ := http.NewRequest("POST", "http://127.0.0.1:9877/api/send", bytes.NewReader(body))
		textReq.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(textReq)
		if err != nil {
			log.Printf("[Proactive] QQ发送失败: %v", err)
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			var respBody bytes.Buffer
			respBody.ReadFrom(resp.Body)
			log.Printf("[Proactive] QQ返回 %d body=%s", resp.StatusCode, respBody.String())
			return false
		}
	}

	now := time.Now()
	msgID := fmt.Sprintf("proactive-%d", now.UnixNano())
	displayContent := content
	e.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?)",
		msgID, convID, displayContent, now)

	log.Printf("[Proactive] QQ已发送 to=%s voice=%v voiceOK=%v", targetID, useVoice, voiceOK)
	return true
}

func (e *Executor) ExecuteShareTask(prompt string) string {
	var charName, identity, convID string
	row := e.db.Table("characters").Select("name, identity, conversation_id").
		Where("is_active = 1").Limit(1).Row()
	if err := row.Scan(&charName, &identity, &convID); err != nil || convID == "" {
		log.Println("[Proactive] ExecuteShareTask: no active character")
		return ""
	}
	content := e.generateContent("系统主动消息", "share", prompt, charName, identity)
	if content == "" {
		return ""
	}
	sentWeb, sentWechat, sentQQ := false, false, false
	e.sendToWeb(convID, content)
	sentWeb = true
	wcID := e.getWechatConvID(convID)
	if wcID != "" {
		e.sendToWechat(wcID, content)
		sentWechat = true
	}
	qqID := e.getQQConvID("")
	if qqID != "" {
		e.sendToQQ(qqID, content)
		sentQQ = true
	}
	status := "sent"
	if !sentWeb && !sentWechat && !sentQQ {
		status = "failed"
	}
	e.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, ?, ?, ?, ?)",
		convID, content, "all", status, time.Now(), time.Now())
	log.Printf("[Proactive] ExecuteShareTask sent: web=%v wechat=%v qq=%v", sentWeb, sentWechat, sentQQ)
	return content
}

func (e *Executor) getQQConvID(charID string) string {
	var id string
	e.db.Table("conversations").Select("id").
		Where("channel = 'qq' AND peer_id != '' AND peer_id IS NOT NULL").
		Order("updated_at DESC").
		Limit(1).Row().Scan(&id)
	return id
}
