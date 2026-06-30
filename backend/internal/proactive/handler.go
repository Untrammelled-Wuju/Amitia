// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"fmt"
	"strconv"
	"time"

	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/sse"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type Handler struct {
	service Service
	db      *gorm.DB
}

func NewHandler(srv Service, db *gorm.DB) *Handler {
	return &Handler{service: srv, db: db}
}

func (h *Handler) ListRules(c *gin.Context) {
	characterID := c.Query("characterId")
	rules, err := h.service.ListRules(characterID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, rules)
}

func (h *Handler) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "名称不能为空", nil)
		return
	}
	rule, err := h.service.CreateRule(&req)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "规则创建成功", rule)
}

func (h *Handler) UpdateRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	rule, err := h.service.UpdateRule(id, updates)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "规则更新成功", rule)
}

func (h *Handler) DeleteRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.service.DeleteRule(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "规则已删除", nil)
}

func (h *Handler) ToggleRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	rule, err := h.service.ToggleRule(id)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "操作失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "状态已切换", rule)
}

func (h *Handler) Status(c *gin.Context) {
	characterID := c.Query("characterId")
	rules, _ := h.service.ListRules(characterID)
	enabled := 0
	total := len(rules)
	for _, r := range rules {
		if v, ok := r["enabled"]; ok {
			switch n := v.(type) {
			case int:
				if n == 1 {
					enabled++
				}
			case int64:
				if n == 1 {
					enabled++
				}
			case float64:
				if int(n) == 1 {
					enabled++
				}
			}
		}
	}
	util.SuccessResponse(c, gin.H{"schedulerRunning": SchedulerRunning, "enabledRuleCount": enabled, "totalRuleCount": total})
}

func (h *Handler) ListReminders(c *gin.Context) {
	items, err := h.service.ListReminders()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) CreateReminder(c *gin.Context) {
	var req CreateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "标题和提醒时间不能为空", nil)
		return
	}
	rem, err := h.service.CreateReminder(&req)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	if rem.RemindAt <= time.Now().Format("2006-01-02 15:04:05") {
		h.service.DeleteReminder(rem.ID)
		util.ErrorResponse(c, response.InvalidParams, "提醒时间不能早于当前时间", nil)
		return
	}
	util.SuccessMsgResponse(c, "提醒创建成功", rem)
}

func (h *Handler) UpdateReminder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	rem, err := h.service.UpdateReminder(id, updates)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "提醒更新成功", rem)
}

func (h *Handler) DeleteReminder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.service.DeleteReminder(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "提醒已删除", nil)
}

func (h *Handler) ToggleReminder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	rem, err := h.service.ToggleReminder(id)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "操作失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "状态已切换", rem)
}

func (h *Handler) TestRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rule ProactiveRule
	if err := h.db.First(&rule, id).Error; err != nil {
		util.ErrorResponse(c, response.NotFound, "规则不存在", nil)
		return
	}
	var charName, identity string
	if rule.CharacterID != "" {
		h.db.Table("characters").Select("name, COALESCE(identity,'')").Where("id = ?", rule.CharacterID).Limit(1).Row().Scan(&charName, &identity)
	}
	if charName == "" {
		h.db.Table("characters").Select("name, COALESCE(identity,'')").Where("is_active = 1").Limit(1).Row().Scan(&charName, &identity)
	}
	if charName == "" {
		charName = "AI助手"
	}
	content := h.generateRuleContent(rule.Name, rule.RuleType, rule.PromptTemplate, charName, identity)
	if content == "" {
		util.ErrorResponse(c, response.InternalError, "AI生成失败，请检查模型配置", nil)
		return
	}
	util.SuccessResponse(c, gin.H{
		"id":             rule.ID,
		"tested":         true,
		"ruleName":       rule.Name,
		"messageContent": content,
		"channel":        rule.Channel,
		"safetyCheck":    gin.H{"safe": true},
	})
}

func (h *Handler) TriggerRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rule ProactiveRule
	if err := h.db.First(&rule, id).Error; err != nil {
		util.ErrorResponse(c, response.NotFound, "规则不存在", nil)
		return
	}
	var charName, identity string
	if rule.CharacterID != "" {
		h.db.Table("characters").Select("name, COALESCE(identity,'')").Where("id = ?", rule.CharacterID).Limit(1).Row().Scan(&charName, &identity)
	}
	if charName == "" {
		h.db.Table("characters").Select("name, COALESCE(identity,'')").Where("is_active = 1").Limit(1).Row().Scan(&charName, &identity)
	}
	if charName == "" {
		charName = "AI助手"
	}
	content := h.generateRuleContent(rule.Name, rule.RuleType, rule.PromptTemplate, charName, identity)
	if content == "" {
		util.ErrorResponse(c, response.InternalError, "AI生成失败，请检查模型配置", nil)
		return
	}
	channel := rule.Channel
	if channel == "" {
		channel = "web"
	}
	var convID string
	h.db.Table("conversations").Select("id").Limit(1).Row().Scan(&convID)
	if convID == "" {
		util.ErrorResponse(c, response.OperationFailed, "无可用对话", nil)
		return
	}
	now := time.Now()
	msgID := uuid.New().String()
	h.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?)", msgID, convID, content, now)
	h.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (?, ?, ?, ?, 'sent', ?, ?)", rule.ID, convID, content, channel, now, now)
	h.db.Exec("UPDATE proactive_rules SET sent_count_today=sent_count_today+1, last_sent_at=?, updated_at=? WHERE id=?", now, now, rule.ID)
	if channel == "wechat" || channel == "all" {
		wcID := h.getWechatConvIDForTrigger(rule.CharacterID)
		if wcID != "" {
			h.sendToWechatSidecar(wcID, content)
		}
	}
	if channel == "qq" || channel == "all" {
		qqID := h.getQQConvIDForTrigger(rule.CharacterID)
		if qqID != "" {
			h.sendToQQSidecarForTrigger(qqID, content)
		}
	}
	util.SuccessResponse(c, gin.H{"id": rule.ID, "triggered": true, "messageContent": content, "channel": channel})
}

func (h *Handler) ResetPresets(c *gin.Context) {
	var body struct {
		CharacterID string `json:"characterId"`
	}
	c.ShouldBindJSON(&body)
	characterID := body.CharacterID
	if characterID == "" {
		characterID = c.Query("characterId")
	}

	var amsEnabled int
	h.db.Table("active_message_settings").Select("COALESCE(enabled, 1)").Limit(1).Row().Scan(&amsEnabled)

	genericRules := []ProactiveRule{
		{Name: "工作间歇", Channel: "all", CharacterID: characterID, RuleType: "cron", ScheduleCron: "0 15 * * 1-5", PromptTemplate: "工作累了就起来活动一下，喝杯水休息一会吧。", MaxPerDay: 20, Enabled: 1, RandomMinutes: 30},
		{Name: "晚间闲聊", Channel: "all", CharacterID: characterID, RuleType: "cron", ScheduleCron: "0 20 * * *", PromptTemplate: "晚上好！放松一下，想聊点什么吗？", MaxPerDay: 20, Enabled: 1, RandomMinutes: 45},
	}

	scheduleRules := []ProactiveRule{
		{Name: "早安问候", Channel: "all", CharacterID: characterID, RuleType: "cron", ScheduleCron: "0 8 * * *", PromptTemplate: "早上好！新的一天开始了，有什么计划吗？", MaxPerDay: 20, Enabled: 1, RandomMinutes: 30},
		{Name: "晚安提醒", Channel: "all", CharacterID: characterID, RuleType: "cron", ScheduleCron: "0 22 * * *", PromptTemplate: "夜深了，早点休息哦。今天过得怎么样？", MaxPerDay: 20, Enabled: 1, RandomMinutes: 30},
		{Name: "午饭时间", Channel: "all", CharacterID: characterID, RuleType: "cron", ScheduleCron: "0 12 * * *", PromptTemplate: "到午饭时间啦，别忘了按时吃饭哦！", MaxPerDay: 20, Enabled: 1, RandomMinutes: 15},
		{Name: "早安心情", Channel: "all", CharacterID: characterID, RuleType: "daily_greeting", ScheduleCron: "30 7 * * *", PromptTemplate: "分享你刚起床的心情和今天的小期待，语气轻松愉快，像朋友发早安消息。不要使用emoji。", MaxPerDay: 20, Enabled: 1, RandomMinutes: 20},
		{Name: "午间日常", Channel: "all", CharacterID: characterID, RuleType: "custom", ScheduleCron: "30 12 * * *", PromptTemplate: "分享一下你此刻的状态或者在想什么，随意的日常片段，像朋友聊天。不要使用emoji。", MaxPerDay: 20, Enabled: 1, RandomMinutes: 30},
		{Name: "傍晚时光", Channel: "all", CharacterID: characterID, RuleType: "custom", ScheduleCron: "0 18 * * *", PromptTemplate: "分享你今天的一个小感受或注意到的事情，温暖随意，像分享生活。不要使用emoji。", MaxPerDay: 20, Enabled: 1, RandomMinutes: 30},
		{Name: "睡前分享", Channel: "all", CharacterID: characterID, RuleType: "sleep_reminder", ScheduleCron: "30 21 * * *", PromptTemplate: "分享今天让你开心的瞬间或此刻的心情，轻松温暖。不要道别，像睡前聊天。不要使用emoji。", MaxPerDay: 20, Enabled: 1, RandomMinutes: 20, QuietStart: "", QuietEnd: ""},
	}

	h.service.DeleteRulesByCharacter(characterID)

	scheduleSkipped := []string{}
	if amsEnabled == 1 {
		for _, r := range genericRules {
			h.service.CreateRuleDirect(&r)
		}
		for _, r := range scheduleRules {
			scheduleSkipped = append(scheduleSkipped, r.Name)
		}
	} else {
		for _, r := range genericRules {
			h.service.CreateRuleDirect(&r)
		}
		for _, r := range scheduleRules {
			r.Enabled = 0
			h.service.CreateRuleDirect(&r)
			scheduleSkipped = append(scheduleSkipped, r.Name+"(已禁用)")
		}
	}

	createdCount := len(genericRules)
	if amsEnabled != 1 {
		createdCount += len(scheduleRules)
	}

	util.SuccessMsgResponse(c, "已恢复系统预设规则", gin.H{
		"count":           createdCount,
		"genericCreated":  len(genericRules),
		"scheduleSkipped": scheduleSkipped,
		"amsEnabled":      amsEnabled == 1,
	})
}
func (h *Handler) RuleMessages(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var msgs []map[string]interface{}
	h.db.Table("proactive_messages").Where("rule_id = ?", id).Order("created_at DESC").Limit(50).Find(&msgs)
	if msgs == nil {
		msgs = []map[string]interface{}{}
	}
	util.SuccessResponse(c, msgs)
}

func (h *Handler) TestReminder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rem Reminder
	if err := h.db.First(&rem, id).Error; err != nil {
		util.ErrorResponse(c, response.NotFound, "提醒不存在", nil)
		return
	}
	var convID string
	if rem.ConversationID != "" {
		convID = rem.ConversationID
	} else if rem.CharacterID != "" {
		h.db.Table("conversations").Select("id").Where("character_id = ?", rem.CharacterID).Limit(1).Row().Scan(&convID)
	}
	if convID == "" {
		h.db.Table("conversations").Select("id").Limit(1).Row().Scan(&convID)
	}
	content := rem.Content
	if content == "" {
		content = fmt.Sprintf("[提醒测试] %s", rem.Title)
	}
	util.SuccessResponse(c, gin.H{
		"id":             id,
		"tested":         true,
		"title":          rem.Title,
		"remindAt":       rem.RemindAt,
		"messageContent": content,
		"channel":        rem.Channel,
		"conversationId": convID,
	})
}

func (h *Handler) TriggerReminder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rem Reminder
	if err := h.db.First(&rem, id).Error; err != nil {
		util.ErrorResponse(c, response.NotFound, "提醒不存在", nil)
		return
	}
	msgID, convID := h.triggerReminderNow(&rem)
	if convID == "" {
		util.ErrorResponse(c, response.OperationFailed, "无可用对话", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"id": id, "triggered": true, "title": rem.Title, "conversationId": convID, "messageId": msgID})
}

func (h *Handler) triggerReminderNow(rem *Reminder) (msgID, convID string) {
	convID = rem.ConversationID
	if convID == "" && rem.CharacterID != "" {
		row := h.db.Table("conversations").Select("id").Where("character_id = ? AND channel = ?", rem.CharacterID, rem.Channel).Limit(1).Row()
		row.Scan(&convID)
	}
	if convID == "" {
		row := h.db.Table("conversations").Select("id").Where("channel = ?", rem.Channel).Limit(1).Row()
		row.Scan(&convID)
	}
	if convID == "" {
		return
	}
	content := rem.Content
	if content == "" {
		content = fmt.Sprintf("[提醒] %s", rem.Title)
	}
	msgID = uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	h.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?)", msgID, convID, content, now)
	h.db.Exec("UPDATE conversations SET message_count=message_count+1, updated_at=? WHERE id=?", now, convID)
	h.db.Exec("UPDATE reminders SET enabled=0, last_triggered_at=?, updated_at=? WHERE id=?", now, now, rem.ID)
	sse.Global.Broadcast("proactive_message", map[string]interface{}{"conversationId": convID, "messageId": msgID, "content": content, "role": "assistant", "source": "proactive"})
	h.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at) VALUES (?, ?, ?, ?, 'sent', ?)", rem.ID, convID, content, rem.Channel, now)
	return
}

func (h *Handler) CancelRemindersByQuery(c *gin.Context) {
	var body struct {
		Title       string `json:"title"`
		CharacterID string `json:"characterId"`
	}
	c.ShouldBindJSON(&body)

	reminders, _ := h.service.ListReminders()
	count := 0
	for _, r := range reminders {
		match := true
		if body.Title != "" && r.Title != body.Title {
			match = false
		}
		if body.CharacterID != "" && r.CharacterID != body.CharacterID {
			match = false
		}
		if match {
			h.service.DeleteReminder(r.ID)
			count++
		}
	}
	util.SuccessResponse(c, gin.H{"cancelled": count})
}

func (h *Handler) CancelLatestReminder(c *gin.Context) {
	reminders, _ := h.service.ListReminders()
	if len(reminders) == 0 {
		util.SuccessResponse(c, gin.H{"cancelled": false, "reason": "no reminders"})
		return
	}
	latest := reminders[0]
	h.service.DeleteReminder(latest.ID)
	util.SuccessResponse(c, gin.H{"cancelled": true, "id": latest.ID, "title": latest.Title})
}

func (h *Handler) ReminderStatus(c *gin.Context) {
	reminders, _ := h.service.ListReminders()
	total := len(reminders)
	enabled := 0
	dueNow := 0
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	for _, r := range reminders {
		if r.Enabled == 1 {
			enabled++
		}
		if r.Enabled == 1 && r.RemindAt <= nowStr {
			dueNow++
		}
	}
	util.SuccessResponse(c, gin.H{"schedulerRunning": SchedulerRunning, "total": total, "enabled": enabled, "dueNow": dueNow})
}

func (h *Handler) PendingReminders(c *gin.Context) {
	items, err := h.service.PendingReminders()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) GetCleanupConfig(c *gin.Context) {
	var value string
	h.db.Raw("SELECT value FROM app_settings WHERE key = 'reminder_cleanup_days' LIMIT 1").Row().Scan(&value)
	if value == "" {
		value = "0"
	}
	util.SuccessResponse(c, gin.H{"cleanupDays": value})
}

func (h *Handler) SetCleanupConfig(c *gin.Context) {
	var body struct {
		CleanupDays string `json:"cleanupDays"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "参数无效", nil)
		return
	}
	h.db.Exec("INSERT OR REPLACE INTO app_settings (key, value, updated_at) VALUES ('reminder_cleanup_days', ?, datetime('now', 'localtime'))", body.CleanupDays)
	util.SuccessMsgResponse(c, "已更新", nil)
}
func (h *Handler) generateRuleContent(name, ruleType, prompt, charName, identity string) string {
	if identity == "" {
		identity = "一个AI伙伴"
	}
	if prompt == "" {
		prompt = "发一条自然的主动消息。"
	}

	now := time.Now()
	sys := fmt.Sprintf("你是%s，%s。\n当前时间：%s，周%s。\n你的语气自然、口语化。\n字数控制在8-40字。\n【重要】不要调用工具，直接输出纯文本。\n不要使用emoji表情符号。", charName, identity, now.Format("15:04"), now.Weekday().String())
	usr := fmt.Sprintf("【主动消息 - 不要调用工具】\n任务：%s (%s)\n要求：%s\n直接输出消息（无前缀无引号）：", name, ruleType, prompt)

	cfg := h.getActiveModelConfig()
	if cfg == nil {
		return ""
	}

	msgs := []map[string]interface{}{
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
		log.Printf("[Proactive] AI 生成失败: %v", err)
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

func (h *Handler) getActiveModelConfig() map[string]string {
	var baseURL, apiKey, modelName string
	err := h.db.Table("model_configs").
		Select("base_url, api_key, model_name").
		Where("is_active = 1").Limit(1).Row().
		Scan(&baseURL, &apiKey, &modelName)
	if err != nil {
		return nil
	}
	return map[string]string{"baseUrl": baseURL, "apiKey": apiKey, "modelName": modelName}
}

func (h *Handler) sendToWechatSidecar(convID, content string) {
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
		return
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("[Proactive] 微信返回 %d", resp.StatusCode)
		return
	}
	log.Printf("[Proactive] 微信已发送 to=%s", toUserID)
}

func (h *Handler) getWechatConvID(charID string) string {
	var id string
	query := h.db.Table("conversations").Select("id")
	if charID != "" {
		query = query.Where("character_id = ?", charID)
	}
	query.Where("channel = 'wechat' AND source = 'wechat'").Limit(1).Row().Scan(&id)
	return id
}

func (h *Handler) getWechatConvIDForTrigger(charID string) string {
	var id string
	h.db.Table("conversations").Select("id").
		Where("channel = 'wechat' AND peer_id != '' AND peer_id IS NOT NULL").
		Order("updated_at DESC").
		Limit(1).Row().Scan(&id)
	return id
}

func (h *Handler) getQQConvIDForTrigger(charID string) string {
	var id string
	h.db.Table("conversations").Select("id").
		Where("channel = 'qq' AND peer_id != '' AND peer_id IS NOT NULL").
		Order("updated_at DESC").
		Limit(1).Row().Scan(&id)
	return id
}

func (h *Handler) sendToQQSidecarForTrigger(toUserID, content string) {
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
		log.Printf("[Proactive] QQ发送失败: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[Proactive] QQ返回 %d body=%s", resp.StatusCode, string(bodyBytes))
		return
	}
	log.Printf("[Proactive] QQ已发送 to=%s", toUserID)
}

func (h *Handler) CleanupTriggeredReminders() {
	var daysStr string
	h.db.Raw("SELECT value FROM app_settings WHERE key = 'reminder_cleanup_days' LIMIT 1").Row().Scan(&daysStr)
	days := 0
	if daysStr != "" {
		fmt.Sscanf(daysStr, "%d", &days)
	}
	if days <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	h.db.Exec("DELETE FROM reminders WHERE enabled = 0 AND last_triggered_at < ?", cutoff)
}
