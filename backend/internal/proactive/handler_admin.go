package proactive

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"strconv"
	"time"
)

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
	h.broadcastReminderChange()
	util.SuccessMsgResponse(c, "已更新", nil)
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

func (h *Handler) ListTriggerHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	state := c.Query("state")
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	items, total, err := h.service.ListTriggerHistory(page, pageSize, state)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": total})
}

func (h *Handler) QueueSummary(c *gin.Context) {
	var pendingCount int64
	h.db.Model(&Reminder{}).Where("enabled = 1").Count(&pendingCount)

	var recentFailures int64
	cutoff := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	h.db.Model(&TriggerHistory{}).Where("state = 'failed' AND created_at > ?", cutoff).Count(&recentFailures)

	depth := int(pendingCount)

	var clearedAt string
	h.db.Raw("SELECT value FROM app_settings WHERE key = 'reminder_backpressure_cleared' LIMIT 1").Row().Scan(&clearedAt)
	cleared := false
	if clearedAt != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", clearedAt); err == nil {
			if time.Since(t) < 5*time.Minute {
				cleared = true
			}
		}
	}
	backpressure := !cleared && (pendingCount > 50 || recentFailures > 10)

	var oldestAgeMs int64
	var oldestCreated string
	if err := h.db.Model(&Reminder{}).Where("enabled = 1").Select("MIN(created_at)").Row().Scan(&oldestCreated); err == nil && oldestCreated != "" {
		if t, e := time.Parse("2006-01-02 15:04:05", oldestCreated); e == nil {
			oldestAgeMs = time.Since(t).Milliseconds()
		}
	}

	util.SuccessResponse(c, gin.H{
		"depth":          depth,
		"pendingCount":   pendingCount,
		"oldestAgeMs":    oldestAgeMs,
		"recentFailures": recentFailures,
		"backpressure":   backpressure,
	})
}

type ProspectiveReminder struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	RemindAt string `json:"remindAt"`
	Status   string `json:"status"`
}

func (h *Handler) Prospective(c *gin.Context) {
	characterID := c.Query("characterId")
	var items []ProspectiveReminder
	query := h.db.Table("prospective_memories").Where("status = ?", "pending")
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	query.Order("remind_at ASC").Limit(20).Find(&items)
	if items == nil {
		items = []ProspectiveReminder{}
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) ClearBackpressure(c *gin.Context) {
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	h.db.Exec("INSERT OR REPLACE INTO app_settings (key, value, updated_at) VALUES ('reminder_backpressure_cleared', ?, datetime('now', 'localtime'))", nowStr)
	h.broadcastReminderChange()
	util.SuccessMsgResponse(c, "背压标记已清除", nil)
}
