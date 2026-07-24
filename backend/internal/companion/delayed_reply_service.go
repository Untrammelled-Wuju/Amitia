// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

func (s *service) ListDelayedReplies(characterID string) []map[string]interface{} {
	var raw []map[string]interface{}
	q := s.db.Table("delayed_replies").Where("status = 'pending'")
	if characterID != "" {
		q = q.Where("character_id = ?", characterID)
	}
	q.Order("scheduled_at ASC").Find(&raw)
	replies := make([]map[string]interface{}, len(raw))
	for i, r := range raw {
		triggerState := "delay"
		if ch, _ := r["channel"].(string); ch != "" {
			triggerState = ch
		}
		replies[i] = map[string]interface{}{
			"id":                 r["id"],
			"status":             r["status"],
			"triggerState":       triggerState,
			"userMessage":        r["content"],
			"expectedReplyAfter": r["scheduled_at"],
			"channel":            r["channel"],
		}
	}
	if replies == nil {
		replies = []map[string]interface{}{}
	}
	return replies
}

func (s *service) CancelDelayedReply(id int, characterID string) map[string]interface{} {
	s.db.Exec("UPDATE delayed_replies SET status='cancelled', updated_at=datetime('now', 'localtime') WHERE id=? AND character_id=?", id, characterID)
	return map[string]interface{}{"id": id, "cancelled": true}
}

func (s *service) ProcessDelayedReplies(characterID string) map[string]interface{} {
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")

	var tasks []map[string]interface{}
	s.db.Table("delayed_replies").Where("status = 'pending' AND scheduled_at <= ? AND character_id = ?", nowStr, characterID).
		Order("scheduled_at ASC").Limit(20).Find(&tasks)

	var processed, sent, delayed, failed int

	for _, t := range tasks {
		processed++
		id, _ := t["id"]
		content, _ := t["content"].(string)
		convID, _ := t["conversation_id"].(string)
		channel, _ := t["channel"].(string)

		if content == "" {
			continue
		}

		canSend := true
		stateResult := s.GetState(characterID)
		currentState, _ := stateResult["currentState"].(string)

		if currentState == "SLEEPING" || currentState == "NAPPING" {
			canSend = false
			schedule := s.buildTodaySchedule(now.Format("2006-01-02"), characterID)
			wakeTime := schedule.WakeTime
			if currentState == "NAPPING" && schedule.NapEndTime != nil {
				wakeTime = *schedule.NapEndTime
			}
			if wakeTime.Before(now) {
				wakeTime = wakeTime.Add(24 * time.Hour)
			}
			s.db.Exec("UPDATE delayed_replies SET scheduled_at = ?, updated_at = datetime('now', 'localtime') WHERE id = ?",
				wakeTime.Format("2006-01-02 15:04:05"), id)
			delayed++
		} else if currentState == "IN_CLASS" || currentState == "IN_EXAM" || currentState == "BUSY" {
			canSend = false
			delayMin := 10 + rand.Intn(21)
			newTime := now.Add(time.Duration(delayMin) * time.Minute)
			s.db.Exec("UPDATE delayed_replies SET scheduled_at = ?, updated_at = datetime('now', 'localtime') WHERE id = ?",
				newTime.Format("2006-01-02 15:04:05"), id)
			delayed++
		}

		if canSend {
			convID = s.resolveConversationID(characterID, channel, convID)
			if convID == "" {
				failed++
				continue
			}
			if channel == "" {
				channel = "web"
			}

			requestID := fmt.Sprintf("delayed-reply-%s", uuid.New().String())
			dispatchResult, dispatchErr := s.submitProactiveMessage(context.Background(), characterID, convID, channel, content, requestID)
			if dispatchErr != nil {
				retryCount := 0
				if rc, ok := t["retry_count"]; ok {
					switch v := rc.(type) {
					case int64:
						retryCount = int(v)
					case float64:
						retryCount = int(v)
					}
				}
				retryCount++
				if retryCount >= 3 {
					s.db.Exec("UPDATE delayed_replies SET status='FAILED', retry_count=?, updated_at=datetime('now', 'localtime') WHERE id = ?", retryCount, id)
					failed++
				} else {
					s.db.Exec("UPDATE delayed_replies SET retry_count=?, updated_at=datetime('now', 'localtime') WHERE id = ?", retryCount, id)
				}
			} else if dispatchResult == nil {
				newTime := now.Add(30 * time.Minute)
				s.db.Exec("UPDATE delayed_replies SET scheduled_at = ?, updated_at = datetime('now', 'localtime') WHERE id = ?",
					newTime.Format("2006-01-02 15:04:05"), id)
				delayed++
			} else {
				messageContent := content
				if dispatchResult.Response != nil && dispatchResult.Response.Reply != "" {
					messageContent = dispatchResult.Response.Reply
				}
				interactionID := ""
				if dispatchResult != nil {
					interactionID = dispatchResult.InteractionID
				}
				deliveryID := uuid.New().String()
				s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, interaction_id, delivery_id, request_id, delivery_status, created_at, updated_at) VALUES (0, ?, ?, ?, 'queued', ?, ?, ?, 'PENDING', ?, ?)",
					convID, messageContent, channel, interactionID, deliveryID, requestID, nowStr, nowStr)
				s.db.Exec("UPDATE delayed_replies SET status='PROCESSED', sent_at=?, updated_at=datetime('now', 'localtime') WHERE id = ?", nowStr, id)
				sent++
			}
		}
	}

	return map[string]interface{}{
		"processed": processed,
		"sent":      sent,
		"delayed":   delayed,
		"failed":    failed,
	}
}
