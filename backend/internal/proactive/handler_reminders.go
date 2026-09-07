package proactive

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/sse"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) ListReminders(c *gin.Context) {
	userID := requestidentity.ResolveGin(c, "")
	var items []Reminder
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		items, err = scoped.ListRemindersForUser(userID)
	} else {
		items, err = h.service.ListReminders()
	}
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
	userID := requestidentity.ResolveGin(c, "")
	var rem *Reminder
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rem, err = scoped.CreateReminderForUser(&req, userID)
	} else {
		rem, err = h.service.CreateReminder(&req)
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	if rem.RemindAt <= time.Now().Format("2006-01-02 15:04:05") {
		if scoped, ok := h.service.(scopedProactiveService); ok {
			_ = scoped.DeleteReminderForUser(rem.ID, userID)
		} else {
			_ = h.service.DeleteReminder(rem.ID)
		}
		util.ErrorResponse(c, response.InvalidParams, "提醒时间不能早于当前时间", nil)
		return
	}
	h.broadcastReminderChange(userID)
	util.SuccessMsgResponse(c, "提醒创建成功", rem)
}

func (h *Handler) UpdateReminder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	userID := requestidentity.ResolveGin(c, "")
	var rem *Reminder
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rem, err = scoped.UpdateReminderForUser(id, updates, userID)
	} else {
		rem, err = h.service.UpdateReminder(id, updates)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	h.broadcastReminderChange(userID)
	util.SuccessMsgResponse(c, "提醒更新成功", rem)
}

func (h *Handler) DeleteReminder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID := requestidentity.ResolveGin(c, "")
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		err = scoped.DeleteReminderForUser(id, userID)
	} else {
		err = h.service.DeleteReminder(id)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
		return
	}
	h.broadcastReminderChange(userID)
	util.SuccessMsgResponse(c, "提醒已删除", nil)
}

func (h *Handler) ToggleReminder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID := requestidentity.ResolveGin(c, "")
	var rem *Reminder
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rem, err = scoped.ToggleReminderForUser(id, userID)
	} else {
		rem, err = h.service.ToggleReminder(id)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "操作失败", nil)
		return
	}
	h.broadcastReminderChange(userID)
	util.SuccessMsgResponse(c, "状态已切换", rem)
}

func (h *Handler) TestReminder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID := requestidentity.ResolveGin(c, "")
	var rem *Reminder
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rem, err = scoped.FindReminderForUser(id, userID)
	} else {
		var fallback Reminder
		err = h.db.First(&fallback, id).Error
		rem = &fallback
	}
	if err != nil || rem == nil {
		util.ErrorResponse(c, response.NotFound, "提醒不存在", nil)
		return
	}
	convID := resolveProactiveConversationForUser(h.db, userID, rem.ConversationID, rem.CharacterID, rem.Channel, false)
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
	userID := requestidentity.ResolveGin(c, "")
	var rem *Reminder
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rem, err = scoped.FindReminderForUser(id, userID)
	} else {
		var fallback Reminder
		err = h.db.First(&fallback, id).Error
		rem = &fallback
	}
	if err != nil || rem == nil {
		util.ErrorResponse(c, response.NotFound, "提醒不存在", nil)
		return
	}
	msgID, convID := h.triggerReminderNow(rem, userID)
	if convID == "" {
		util.ErrorResponse(c, response.OperationFailed, "无可用对话", nil)
		return
	}
	h.broadcastReminderChange(userID)
	util.SuccessResponse(c, gin.H{"id": id, "triggered": true, "title": rem.Title, "conversationId": convID, "messageId": msgID})
}

func (h *Handler) triggerReminderNow(rem *Reminder, userID string) (msgID, convID string) {
	if rem == nil {
		return "", ""
	}
	owner := normalizeProactiveOwner(userID)
	convID = resolveProactiveConversationForUser(h.db, owner, rem.ConversationID, rem.CharacterID, rem.Channel, false)
	if convID == "" {
		return "", ""
	}
	content := rem.Content
	if content == "" {
		content = fmt.Sprintf("[提醒] %s", rem.Title)
	}
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")

	if h.compSvc == nil {
		msgID = uuid.New().String()
		h.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'pending', 1, ?)", msgID, convID, content, nowStr)
		h.db.Exec("INSERT INTO proactive_messages (user_id, rule_id, conversation_id, message_content, channel, status, created_at) VALUES (?, ?, ?, ?, ?, 'pending', ?)", owner, rem.ID, convID, content, rem.Channel, nowStr)
		h.db.Exec("UPDATE conversations SET message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?), updated_at = ? WHERE id = ? AND user_id = ?", convID, nowStr, convID, owner)
		proactiveOwnerQuery(h.db.Table("reminders").Where("id = ?", rem.ID), owner).Updates(map[string]interface{}{"enabled": 0, "last_triggered_at": nowStr, "updated_at": nowStr})
		sse.Global.BroadcastToUser(owner, "proactive_message", map[string]interface{}{"userId": owner, "conversationId": convID, "messageId": msgID, "content": content, "role": "assistant", "source": "proactive", "createdAt": nowStr})
		return msgID, convID
	}

	requestID := fmt.Sprintf("proactive-reminder-now-%d-%d", rem.ID, now.UnixNano())
	generatedContent, err := h.compSvc.DispatchProactiveMessage(context.Background(), owner, rem.CharacterID, convID, rem.Channel, content, requestID)
	if err != nil || generatedContent == "" {
		generatedContent = content
	}

	lines := util.SplitLongMessage(generatedContent, util.MaxWebMessageLen)
	for _, line := range lines {
		msgID = uuid.New().String()
		h.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'pending', 1, ?)", msgID, convID, line, nowStr)
		h.db.Exec("INSERT INTO proactive_messages (user_id, rule_id, conversation_id, message_content, channel, status, created_at) VALUES (?, ?, ?, ?, ?, 'pending', ?)", owner, rem.ID, convID, line, rem.Channel, nowStr)
	}
	proactiveOwnerQuery(h.db.Table("conversations").Where("id = ?", convID), owner).Updates(map[string]interface{}{"updated_at": nowStr})
	h.db.Exec("UPDATE conversations SET message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ? AND user_id = ?", convID, convID, owner)
	proactiveOwnerQuery(h.db.Table("reminders").Where("id = ?", rem.ID), owner).Updates(map[string]interface{}{"enabled": 0, "last_triggered_at": nowStr, "updated_at": nowStr})
	sse.Global.BroadcastToUser(owner, "proactive_message", map[string]interface{}{"userId": owner, "conversationId": convID, "messageId": msgID, "content": generatedContent, "role": "assistant", "source": "proactive", "createdAt": nowStr})
	return msgID, convID
}

func (h *Handler) CancelRemindersByQuery(c *gin.Context) {
	var body struct {
		Title       string `json:"title"`
		CharacterID string `json:"characterId"`
	}
	_ = c.ShouldBindJSON(&body)
	userID := requestidentity.ResolveGin(c, "")

	var reminders []Reminder
	if scoped, ok := h.service.(scopedProactiveService); ok {
		reminders, _ = scoped.ListRemindersForUser(userID)
	} else {
		reminders, _ = h.service.ListReminders()
	}
	count := 0
	for _, r := range reminders {
		if body.Title != "" && r.Title != body.Title {
			continue
		}
		if body.CharacterID != "" && r.CharacterID != body.CharacterID {
			continue
		}
		var err error
		if scoped, ok := h.service.(scopedProactiveService); ok {
			err = scoped.DeleteReminderForUser(r.ID, userID)
		} else {
			err = h.service.DeleteReminder(r.ID)
		}
		if err == nil {
			count++
		}
	}
	if count > 0 {
		h.broadcastReminderChange(userID)
	}
	util.SuccessResponse(c, gin.H{"cancelled": count})
}

func (h *Handler) CancelLatestReminder(c *gin.Context) {
	userID := requestidentity.ResolveGin(c, "")
	var reminders []Reminder
	if scoped, ok := h.service.(scopedProactiveService); ok {
		reminders, _ = scoped.ListRemindersForUser(userID)
	} else {
		reminders, _ = h.service.ListReminders()
	}
	if len(reminders) == 0 {
		util.SuccessResponse(c, gin.H{"cancelled": false, "reason": "no reminders"})
		return
	}
	latest := reminders[0]
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		err = scoped.DeleteReminderForUser(latest.ID, userID)
	} else {
		err = h.service.DeleteReminder(latest.ID)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "取消失败", nil)
		return
	}
	h.broadcastReminderChange(userID)
	util.SuccessResponse(c, gin.H{"cancelled": true, "id": latest.ID, "title": latest.Title})
}

func (h *Handler) ReminderStatus(c *gin.Context) {
	userID := requestidentity.ResolveGin(c, "")
	var reminders []Reminder
	if scoped, ok := h.service.(scopedProactiveService); ok {
		reminders, _ = scoped.ListRemindersForUser(userID)
	} else {
		reminders, _ = h.service.ListReminders()
	}
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
	userID := requestidentity.ResolveGin(c, "")
	var items []Reminder
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		items, err = scoped.PendingRemindersForUser(userID)
	} else {
		items, err = h.service.PendingReminders()
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, items)
}
