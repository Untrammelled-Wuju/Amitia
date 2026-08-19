// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"context"
	"log"
	"time"

	"github.com/u-ai/backend/internal/delivery"
)

func (s *service) GetActiveMessageSetting(characterID string) map[string]interface{} {
	var enabled, activeLevel, minInterval, maxPerDay, maxDailyCalls int
	var channel, quietStart, quietEnd string
	err := s.db.Table("active_message_settings").Select("enabled, COALESCE(active_level, 40) as active_level, min_interval, COALESCE(quiet_start, '23:00') as quiet_start, COALESCE(quiet_end, '07:00') as quiet_end, max_per_day, COALESCE(max_daily_calls, 10) as max_daily_calls, channel").Where("character_id = ?", characterID).Limit(1).Row().Scan(&enabled, &activeLevel, &minInterval, &quietStart, &quietEnd, &maxPerDay, &maxDailyCalls, &channel)
	if err != nil {
		return map[string]interface{}{"enabled": true, "activeLevel": 40, "quietStart": "23:00", "quietEnd": "07:00", "minInterval": 60, "maxPerDay": 6, "maxDailyCalls": 10, "channel": "all"}
	}
	if quietStart == "" {
		quietStart = "23:00"
	}
	if quietEnd == "" {
		quietEnd = "07:00"
	}
	if activeLevel == 0 {
		activeLevel = 40
	}
	return map[string]interface{}{"enabled": enabled == 1, "activeLevel": activeLevel, "quietStart": quietStart, "quietEnd": quietEnd, "minInterval": minInterval, "maxPerDay": maxPerDay, "maxDailyCalls": maxDailyCalls, "channel": channel}
}

func (s *service) UpdateActiveMessageSetting(body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["enabled"].(bool); ok {
		if v {
			updates["enabled"] = 1
		} else {
			updates["enabled"] = 0
		}
	}
	if v, ok := body["activeLevel"].(float64); ok {
		vv := int(v)
		if vv < 1 {
			vv = 1
		}
		if vv > 100 {
			vv = 100
		}
		updates["active_level"] = vv
	}
	if v, ok := body["minInterval"].(float64); ok {
		updates["min_interval"] = int(v)
	}
	if v, ok := body["quietStart"].(string); ok {
		updates["quiet_start"] = v
	}
	if v, ok := body["quietEnd"].(string); ok {
		updates["quiet_end"] = v
	}
	if v, ok := body["maxPerDay"].(float64); ok {
		updates["max_per_day"] = int(v)
	}
	if v, ok := body["maxDailyCalls"].(float64); ok {
		vv := int(v)
		if vv < 1 {
			vv = 1
		}
		if vv > 50 {
			vv = 50
		}
		updates["max_daily_calls"] = vv
	}
	if v, ok := body["channel"].(string); ok {
		updates["channel"] = v
	}
	if len(updates) > 0 {
		var count int64
		s.db.Table("active_message_settings").Where("character_id = ?", characterID).Count(&count)
		if count == 0 {
			s.db.Exec("INSERT INTO active_message_settings (character_id, enabled, active_level, min_interval, quiet_start, quiet_end, max_per_day, max_daily_calls, channel) VALUES (?, 1, 40, 60, '23:00', '07:00', 6, 10, 'all')", characterID)
		}
		s.db.Table("active_message_settings").Where("character_id = ?", characterID).Updates(updates)
	}
	return s.GetActiveMessageSetting(characterID)
}

func (s *service) GetActiveMessageTasksToday(characterID string) []map[string]interface{} {
	var raw []map[string]interface{}
	todayDate := time.Now().Format("2006-01-02")
	s.db.Table("active_message_task").Where("date(due_time) = ? AND character_id = ?", todayDate, characterID).Order("due_time ASC").Find(&raw)
	tasks := make([]map[string]interface{}, len(raw))
	for i, r := range raw {
		tasks[i] = map[string]interface{}{
			"id":           r["id"],
			"taskType":     r["task_type"],
			"dueTime":      r["due_time"],
			"status":       r["status"],
			"prompt":       r["prompt"],
			"cancelReason": r["cancel_reason"],
			"retryCount":   r["retry_count"],
			"source":       r["source"],
			"createdAt":    r["created_at"],
			"updatedAt":    r["updated_at"],
			"lockUntil":    r["lock_until"],
			"maxRetry":     r["max_retry"],
			"sendResult":   r["send_result"],
			"payload":      r["payload"],
		}
	}
	if tasks == nil {
		tasks = []map[string]interface{}{}
	}
	return tasks
}

func (s *service) RegenerateActiveMessageTasks(characterID string) map[string]interface{} {
	now := time.Now()
	nowDate := now.Format("2006-01-02")
	nowStr := now.Format("2006-01-02 15:04:05")
	s.db.Exec("UPDATE active_message_task SET status='CANCELLED', cancel_reason='regenerate', updated_at=? WHERE date(due_time)=? AND status='PENDING' AND character_id = ?", nowStr, nowDate, characterID)
	return map[string]interface{}{"regenerated": true}
}

func (s *service) RunActiveMessageTask(id int, characterID string) map[string]interface{} {
	return s.RunActiveMessageTaskContext(context.TODO(), id, characterID)
}

func (s *service) RunActiveMessageTaskContext(ctx context.Context, id int, characterID string) map[string]interface{} {
	if err := ctx.Err(); err != nil {
		return map[string]interface{}{"id": id, "status": "CANCELLED", "error": err.Error()}
	}
	var task map[string]interface{}
	s.db.Table("active_message_task").Where("id = ? AND character_id = ?", id, characterID).Limit(1).Find(&task)
	if len(task) == 0 {
		return map[string]interface{}{"id": id, "status": "NOT_FOUND"}
	}
	prompt, _ := task["prompt"].(string)
	taskType, _ := task["task_type"].(string)
	if prompt == "" {
		return map[string]interface{}{"id": id, "status": "NO_PROMPT"}
	}
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")
	var channelSetting string
	s.db.Table("active_message_settings").Select("COALESCE(channel, 'all')").Where("character_id = ?", characterID).Limit(1).Row().Scan(&channelSetting)
	if channelSetting == "" {
		channelSetting = "all"
	}
	convID := s.resolveConversationID(characterID, channelSetting, "")
	if convID == "" {
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		s.db.Exec("UPDATE active_message_task SET status='FAILED', updated_at=? WHERE id=? AND character_id=?", nowStr, id, characterID)
		return map[string]interface{}{"id": id, "status": "NO_CONVERSATION", "taskType": taskType, "channel": channelSetting}
	}

	scope := s.resolveProactiveDeliveryScope(convID, channelSetting, characterID)

	result, err := s.submitProactiveMessage(ctx, characterID, convID, channelSetting, prompt, proactiveRequestID("proactive-task", id))
	if err != nil {
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		s.db.Exec("UPDATE active_message_task SET status='FAILED', updated_at=? WHERE id=? AND character_id=?", nowStr, id, characterID)
		return map[string]interface{}{"id": id, "status": "FAILED", "taskType": taskType, "channel": channelSetting, "error": err.Error()}
	}
	if result == nil {
		return map[string]interface{}{"id": id, "status": "SUPPRESSED", "taskType": taskType, "channel": channelSetting}
	}
	messageContent := prompt
	if result != nil && result.Response != nil && result.Response.Reply != "" {
		messageContent = result.Response.Reply
	}
	interactionID := ""
	requestID := ""
	if result != nil {
		interactionID = result.InteractionID
		if result.Response != nil {
			requestID = result.Response.RequestID
		}
	}

	var deliveryID string
	if result != nil && result.Response != nil && result.Response.MessagePlan != nil && result.Response.MessagePlan.Managed {
		messagePlan := result.Response.MessagePlan
		if scope.channel != "web" && messagePlan.ResponseGroupID != "" && scope.peerID != "" {
			for _, item := range messagePlan.Items {
				if item.MessageID != "" {
					deliveryID = delivery.GenerateDeliveryID(messagePlan.ResponseGroupID, scope.channel, scope.peerID, item.MessageID)
					break
				}
			}
		}
	}
	nowStr = time.Now().Format("2006-01-02 15:04:05")
	s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, interaction_id, delivery_id, request_id, delivery_status, created_at, updated_at) VALUES (0, ?, ?, ?, 'queued', ?, ?, ?, 'PENDING', ?, ?)", convID, messageContent, channelSetting, interactionID, deliveryID, requestID, nowStr, nowStr)
	s.db.Exec("UPDATE active_message_task SET status='QUEUED', updated_at=? WHERE id=? AND character_id=?", nowStr, id, characterID)

	log.Printf("[Companion] RunActiveMessageTask queued type=%s id=%d channel=%s deliveryID=%s", taskType, id, channelSetting, deliveryID)
	return map[string]interface{}{"id": id, "status": "QUEUED", "taskType": taskType, "channel": channelSetting}
}

func (s *service) CancelActiveMessageTask(id int, characterID string) map[string]interface{} {
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	s.db.Exec("UPDATE active_message_task SET status='CANCELLED', cancel_reason='manual', updated_at=? WHERE id=? AND character_id=?", nowStr, id, characterID)
	return map[string]interface{}{"id": id, "cancelled": true}
}

