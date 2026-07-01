// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

func (s *service) RandomBurstTrigger(characterID string) map[string]interface{} {
	setting := s.GetActiveMessageSetting(characterID)
	enabled, _ := setting["enabled"].(bool)
	if !enabled {
		return map[string]interface{}{"triggered": false, "reason": "disabled"}
	}

	stateLife := s.GetStateLife(characterID)
	currentState, _ := stateLife["currentState"].(string)
	blockedStates := map[string]bool{"SLEEPING": true, "IN_CLASS": true, "IN_EXAM": true, "BUSY": true, "WORKING": true, "WORKING_OUT": true, "OVERTIME": true}
	if blockedStates[currentState] {
		return map[string]interface{}{"triggered": false, "reason": "blocked:" + currentState}
	}

	quietStart, _ := setting["quietStart"].(string)
	quietEnd, _ := setting["quietEnd"].(string)
	if quietStart == "" {
		quietStart = "23:00"
	}
	if quietEnd == "" {
		quietEnd = "07:00"
	}
	now := time.Now()
	nowStr := now.Format("15:04")
	if quietStart <= quietEnd {
		if nowStr >= quietStart && nowStr <= quietEnd {
			return map[string]interface{}{"triggered": false, "reason": "quiet:" + quietStart + "-" + quietEnd}
		}
	} else {
		if nowStr >= quietStart || nowStr <= quietEnd {
			return map[string]interface{}{"triggered": false, "reason": "quiet:" + quietStart + "-" + quietEnd}
		}
	}

	if s.lastBurstAt.Format("2006-01-02") != now.Format("2006-01-02") {
		s.todayBurstCount = 0
	}
	minInterval, _ := setting["minInterval"].(int)
	if time.Since(s.lastBurstAt) < time.Duration(minInterval)*time.Minute {
		return map[string]interface{}{"triggered": false, "reason": "minInterval"}
	}

	maxPerDay, _ := setting["maxPerDay"].(int)
	if s.todayBurstCount >= maxPerDay {
		return map[string]interface{}{"triggered": false, "reason": "maxPerDay"}
	}

	maxDailyCalls, _ := setting["maxDailyCalls"].(int)
	if maxDailyCalls == 0 {
		maxDailyCalls = 10
	}
	todayStr := now.Format("2006-01-02")
	var todayLLMCalls int64
	s.db.Table("proactive_messages").Where("date(created_at) = date(?)", todayStr).Count(&todayLLMCalls)
	if int(todayLLMCalls) >= maxDailyCalls {
		return map[string]interface{}{"triggered": false, "reason": "maxDailyCalls"}
	}

	activeLevel, _ := setting["activeLevel"].(int)
	if activeLevel == 0 {
		activeLevel = 40
	}
	baseProb := float64(activeLevel) / 100.0 * 0.05

	energy, _ := stateLife["energy"].(int)
	mood, _ := stateLife["mood"].(string)
	idleSec, _ := stateLife["idleDuration"].(float64)
	idleDuration := time.Duration(idleSec) * time.Second

	energyMod := 1.0
	if energy > 70 {
		energyMod = 1.2
	} else if energy < 30 {
		energyMod = 0.3
	}

	moodMod := 1.0
	if mood == "happy" {
		moodMod = 1.3
	} else if mood == "sad" || mood == "depressed" || mood == "ignored" {
		moodMod = 1.5
	} else if mood == "tired" || mood == "lonely" {
		moodMod = 0.7
	}

	stateMod := 1.0
	switch currentState {
	case "IDLE", "AFTER_WORK", "AFTER_CLASS", "LIBRARY_BREAK":
		stateMod = 1.0
	case "LOW_ENERGY", "SICK_RESTING":
		stateMod = 0.3
	default:
		stateMod = 0.6
	}

	budgetRemaining := maxDailyCalls - int(todayLLMCalls)
	if budgetRemaining < 1 {
		budgetRemaining = 1
	}
	budgetMod := float64(budgetRemaining) / float64(maxDailyCalls)

	finalProb := baseProb * energyMod * moodMod * stateMod * budgetMod

	if idleDuration > 48*time.Hour {
		finalProb = finalProb * 0.1
	}
	if idleDuration > 24*time.Hour {
		finalProb = finalProb * 0.3
	}

	rng := rand.New(rand.NewSource(now.UnixNano()))
	if rng.Float64() >= finalProb {
		return map[string]interface{}{"triggered": false, "reason": "probability", "prob": finalProb}
	}

	history := s.getShareHistory()
	recentTopics := strings.Join(history.RecentTopics, "、")
	if recentTopics == "" {
		recentTopics = "无"
	}

	var recentMemoriesStr string
	queryText := fmt.Sprintf("心情%s 状态%s", mood, currentState)
	if s.embeddingSvc != nil && qdrantDB.Client != nil {
		vec, vecErr := s.embeddingSvc.Embed(queryText)
		if vecErr == nil {
			points, searchErr := qdrantDB.SearchVectors(vec, 3, nil)
			if searchErr == nil {
				var mems []string
				for _, p := range points {
					if val, ok := p.Payload["value"]; ok {
						mems = append(mems, val.GetStringValue())
					}
				}
				recentMemoriesStr = strings.Join(mems, "；")
			}
		}
	}
	if recentMemoriesStr == "" {
		rows, err := s.db.Table("memories").Select("value").Where("importance >= 2").Order("created_at DESC").Limit(3).Rows()
		if err == nil {
			defer rows.Close()
			var mems []string
			for rows.Next() {
				var v string
				rows.Scan(&v)
				if v != "" {
					mems = append(mems, v)
				}
			}
			recentMemoriesStr = strings.Join(mems, "；")
		}
	}
	if recentMemoriesStr == "" {
		recentMemoriesStr = "无"
	}

	prompt := fmt.Sprintf("当前你处于 %s 状态，心情 %s，精力 %d/100。最近记忆：%s。请生成一条像微信里突然想到就发出的自然短消息，1-2句，不要客服腔，不要解释，不要 emoji，避免重复这些话题：%s。", currentState, mood, energy, recentMemoriesStr, recentTopics)

	msgID := fmt.Sprintf("burst-%d", now.UnixNano())
	generated := s.generateLLMReply(prompt)
	if generated == "" {
		return map[string]interface{}{"triggered": false, "reason": "llmFailed"}
	}
	displayContent := generated

	var convID string
	convID = s.resolveConversationID(characterID, "all", "")
	if convID == "" {
		return map[string]interface{}{"triggered": false, "reason": "noConversation"}
	}

	s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?)",
		msgID, convID, displayContent, now.Format("2006-01-02 15:04:05"))

	s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, 'all', 'sent', ?, ?)",
		convID, displayContent, now.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"))

	if s.isDefaultCharacter(characterID) {
		wcID := s.getWechatConvIDForChar(characterID)
		if wcID != "" {
			s.sendToWechatSidecar(wcID, generated)
		}
		qqID := s.getQQConvIDForChar(characterID)
		if qqID != "" {
			s.sendToQQSidecar(qqID, generated)
		}
	}

	s.lastBurstAt = now
	s.todayBurstCount++

	log.Printf("[Companion] RandomBurst triggered: prob=%.4f energyMod=%.2f moodMod=%.2f stateMod=%.2f budgetMod=%.2f", finalProb, energyMod, moodMod, stateMod, budgetMod)

	return map[string]interface{}{"triggered": true, "prob": finalProb, "burstCount": s.todayBurstCount, "prompt": prompt}

}
