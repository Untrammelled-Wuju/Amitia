// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"fmt"

	"github.com/google/uuid"
	"math/rand"
	"time"

	"gorm.io/gorm"
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

			msgID := fmt.Sprintf("reply-%s", uuid.New().String())
			displayContent := "💬 " + content
			err := s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'delayed_reply', 'normal', 'sent', 1, ?)",
				msgID, convID, displayContent, nowStr).Error
			if err != nil {
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
			} else {
				s.db.Exec("UPDATE delayed_replies SET status='SENT', sent_at=?, updated_at=datetime('now', 'localtime') WHERE id = ?", nowStr, id)
				sendProactiveNotification(s.db, convID, msgID, content)
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

func sendProactiveNotification(db *gorm.DB, convID, msgID, content string) {
	db.Exec("UPDATE conversations SET message_count=message_count+1, updated_at=datetime('now', 'localtime') WHERE id=?", convID)
}
