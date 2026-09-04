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
	var unrepliedSlowdownEnabled, unrepliedSlowdownAfter, unrepliedRecoveryOnReply int
	var unrepliedCooldownMultiplier float64
	var channel, quietStart, quietEnd string
	err := s.db.Table("active_message_settings").Select("enabled, COALESCE(active_level, 40) as active_level, min_interval, COALESCE(quiet_start, '23:00') as quiet_start, COALESCE(quiet_end, '07:00') as quiet_end, max_per_day, COALESCE(max_daily_calls, 10) as max_daily_calls, channel, COALESCE(unreplied_slowdown_enabled, 1), COALESCE(unreplied_slowdown_after, 2), COALESCE(unreplied_cooldown_multiplier, 2.0), COALESCE(unreplied_recovery_on_reply, 1)").Where("character_id = ?", characterID).Limit(1).Row().Scan(&enabled, &activeLevel, &minInterval, &quietStart, &quietEnd, &maxPerDay, &maxDailyCalls, &channel, &unrepliedSlowdownEnabled, &unrepliedSlowdownAfter, &unrepliedCooldownMultiplier, &unrepliedRecoveryOnReply)
	if err != nil {
		return defaultActiveMessageSetting()
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
	if minInterval < 1 {
		minInterval = 60
	}
	if maxPerDay < 1 {
		maxPerDay = 6
	}
	if maxDailyCalls < 1 {
		maxDailyCalls = 10
	}
	if channel == "" {
		channel = "all"
	}
	if unrepliedSlowdownAfter < 1 {
		unrepliedSlowdownAfter = 2
	}
	if unrepliedCooldownMultiplier < 1 {
		unrepliedCooldownMultiplier = 2
	}
	return map[string]interface{}{
		"enabled":                     enabled == 1,
		"activeLevel":                 activeLevel,
		"quietStart":                  quietStart,
		"quietEnd":                    quietEnd,
		"minInterval":                 minInterval,
		"maxPerDay":                   maxPerDay,
		"maxDailyCalls":               maxDailyCalls,
		"channel":                     channel,
		"unrepliedSlowdownEnabled":    unrepliedSlowdownEnabled == 1,
		"unrepliedSlowdownAfter":      unrepliedSlowdownAfter,
		"unrepliedCooldownMultiplier": unrepliedCooldownMultiplier,
		"unrepliedRecoveryOnReply":    unrepliedRecoveryOnReply == 1,
	}
}

func defaultActiveMessageSetting() map[string]interface{} {
	return map[string]interface{}{
		"enabled":                     true,
		"activeLevel":                 40,
		"quietStart":                  "23:00",
		"quietEnd":                    "07:00",
		"minInterval":                 60,
		"maxPerDay":                   6,
		"maxDailyCalls":               10,
		"channel":                     "all",
		"unrepliedSlowdownEnabled":    true,
		"unrepliedSlowdownAfter":      2,
		"unrepliedCooldownMultiplier": 2.0,
		"unrepliedRecoveryOnReply":    true,
	}
}

func (s *service) UpdateActiveMessageSetting(body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["enabled"].(bool); ok {
		updates["enabled"] = boolInt(v)
	}
	if v, ok := numberValue(body["activeLevel"]); ok {
		updates["active_level"] = clampInt(int(v), 1, 100)
	}
	if v, ok := numberValue(body["minInterval"]); ok {
		updates["min_interval"] = clampInt(int(v), 1, 24*60)
	}
	if v, ok := body["quietStart"].(string); ok {
		updates["quiet_start"] = v
	}
	if v, ok := body["quietEnd"].(string); ok {
		updates["quiet_end"] = v
	}
	if v, ok := numberValue(body["maxPerDay"]); ok {
		updates["max_per_day"] = clampInt(int(v), 1, 100)
	}
	if v, ok := numberValue(body["maxDailyCalls"]); ok {
		updates["max_daily_calls"] = clampInt(int(v), 1, 50)
	}
	if v, ok := body["channel"].(string); ok {
		updates["channel"] = v
	}
	if v, ok := body["unrepliedSlowdownEnabled"].(bool); ok {
		updates["unreplied_slowdown_enabled"] = boolInt(v)
	}
	if v, ok := numberValue(body["unrepliedSlowdownAfter"]); ok {
		updates["unreplied_slowdown_after"] = clampInt(int(v), 1, 20)
	}
	if v, ok := numberValue(body["unrepliedCooldownMultiplier"]); ok {
		if v < 1 {
			v = 1
		}
		if v > 10 {
			v = 10
		}
		updates["unreplied_cooldown_multiplier"] = v
	}
	if v, ok := body["unrepliedRecoveryOnReply"].(bool); ok {
		updates["unreplied_recovery_on_reply"] = boolInt(v)
	}
	if len(updates) > 0 {
		var count int64
		s.db.Table("active_message_settings").Where("character_id = ?", characterID).Count(&count)
		if count == 0 {
			s.db.Exec("INSERT INTO active_message_settings (character_id, enabled, active_level, min_interval, quiet_start, quiet_end, max_per_day, max_daily_calls, channel, unreplied_slowdown_enabled, unreplied_slowdown_after, unreplied_cooldown_multiplier, unreplied_recovery_on_reply) VALUES (?, 1, 40, 60, '23:00', '07:00', 6, 10, 'all', 1, 2, 2.0, 1)", characterID)
		}
		s.db.Table("active_message_settings").Where("character_id = ?", characterID).Updates(updates)
	}
	return s.GetActiveMessageSetting(characterID)
}

func numberValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return 0, false
	}
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
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
