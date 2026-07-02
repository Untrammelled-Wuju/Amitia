// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"fmt"
	"github.com/u-ai/backend/internal/decision"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"log"
	"math/rand"
	"strings"
	"time"
)

func (s *service) checkBurstEligibility(characterID string, setting map[string]interface{}, currentState string, now time.Time) (bool, string) {
	enabled, _ := setting["enabled"].(bool)
	if !enabled {
		return false, "disabled"
	}
	blockedStates := map[string]bool{"SLEEPING": true, "IN_CLASS": true, "IN_EXAM": true, "BUSY": true, "WORKING": true, "WORKING_OUT": true, "OVERTIME": true}
	if blockedStates[currentState] {
		return false, "blocked:" + currentState
	}
	quietStart, _ := setting["quietStart"].(string)
	quietEnd, _ := setting["quietEnd"].(string)
	if quietStart == "" {
		quietStart = "23:00"
	}
	if quietEnd == "" {
		quietEnd = "07:00"
	}
	nowStr := now.Format("15:04")
	if quietStart <= quietEnd {
		if nowStr >= quietStart && nowStr <= quietEnd {
			return false, "quiet:" + quietStart + "-" + quietEnd
		}
	} else {
		if nowStr >= quietStart || nowStr <= quietEnd {
			return false, "quiet:" + quietStart + "-" + quietEnd
		}
	}
	scope := s.getBurstScopeState(characterID, now)
	minInterval, _ := setting["minInterval"].(int)
	if now.Sub(scope.lastAt) < time.Duration(minInterval)*time.Minute {
		return false, "minInterval"
	}
	maxPerDay, _ := setting["maxPerDay"].(int)
	if scope.todayCount >= maxPerDay {
		return false, "maxPerDay"
	}
	return true, ""
}

func (s *service) getBurstScopeState(characterID string, now time.Time) burstScopeState {
	s.burstMu.Lock()
	if s.burstScopes == nil {
		s.burstScopes = map[string]burstScopeState{}
	}
	scope, ok := s.burstScopes[characterID]
	s.burstMu.Unlock()
	if !ok {
		scope = s.loadBurstScopeState(characterID, now)
		s.burstMu.Lock()
		if s.burstScopes == nil {
			s.burstScopes = map[string]burstScopeState{}
		}
		if existing, exists := s.burstScopes[characterID]; exists {
			scope = existing
		} else {
			s.burstScopes[characterID] = scope
		}
		s.burstMu.Unlock()
	}
	if !scope.lastAt.IsZero() && scope.lastAt.Format("2006-01-02") != now.Format("2006-01-02") {
		scope.todayCount = 0
		s.burstMu.Lock()
		s.burstScopes[characterID] = scope
		s.burstMu.Unlock()
	}
	return scope
}

func (s *service) loadBurstScopeState(characterID string, now time.Time) burstScopeState {
	var scope burstScopeState
	var lastAt string
	err := s.db.Table("messages AS m").
		Select("m.created_at").
		Joins("JOIN conversations AS c ON c.id = m.conversation_id").
		Where("c.character_id = ? AND m.id LIKE ? AND m.role = ? AND m.source = ?", characterID, "burst-%", "assistant", "proactive").
		Order("datetime(m.created_at) DESC").
		Limit(1).
		Scan(&lastAt).Error
	if err == nil && lastAt != "" {
		scope.lastAt = parseBurstTime(lastAt)
	}
	var count int64
	if err := s.db.Table("messages AS m").
		Joins("JOIN conversations AS c ON c.id = m.conversation_id").
		Where("date(m.created_at) = date(?) AND c.character_id = ? AND m.id LIKE ? AND m.role = ? AND m.source = ?", now.Format("2006-01-02"), characterID, "burst-%", "assistant", "proactive").
		Count(&count).Error; err == nil {
		scope.todayCount = int(count)
	}
	return scope
}

func parseBurstTime(value string) time.Time {
	layouts := []string{"2006-01-02 15:04:05", time.RFC3339Nano, time.RFC3339}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func (s *service) recordBurstTriggered(characterID string, now time.Time) int {
	s.burstMu.Lock()
	defer s.burstMu.Unlock()
	if s.burstScopes == nil {
		s.burstScopes = map[string]burstScopeState{}
	}
	scope := s.burstScopes[characterID]
	if !scope.lastAt.IsZero() && scope.lastAt.Format("2006-01-02") != now.Format("2006-01-02") {
		scope.todayCount = 0
	}
	scope.lastAt = now
	scope.todayCount++
	s.burstScopes[characterID] = scope
	return scope.todayCount
}

func (s *service) countTodayProactiveMessages(characterID, todayStr string) int64 {
	var count int64
	s.db.Table("proactive_messages AS pm").
		Joins("JOIN conversations AS c ON c.id = pm.conversation_id").
		Where("date(pm.created_at) = date(?) AND c.character_id = ?", todayStr, characterID).
		Count(&count)
	return count
}

func (s *service) calculateBurstProbability(setting, stateLife map[string]interface{}, currentState string, todayLLMCalls, maxDailyCalls int) (float64, float64, float64, float64, float64) {
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
	budgetRemaining := maxDailyCalls - todayLLMCalls
	if budgetRemaining < 1 {
		budgetRemaining = 1
	}
	budgetMod := float64(budgetRemaining) / float64(maxDailyCalls)
	finalProb := baseProb * energyMod * moodMod * stateMod * budgetMod
	if idleDuration > 48*time.Hour {
		finalProb *= 0.1
	}
	if idleDuration > 24*time.Hour {
		finalProb *= 0.3
	}
	return finalProb, energyMod, moodMod, stateMod, budgetMod
}

func (s *service) buildBurstPrompt(characterID, mood, currentState string, energy int) string {
	history := s.getShareHistory(characterID)
	recentTopics := strings.Join(history.RecentTopics, "、")
	if recentTopics == "" {
		recentTopics = "无"
	}
	var recentMemoriesStr string
	queryText := fmt.Sprintf("\u5fc3\u60c5%s \u72b6\u6001%s", mood, currentState)
	if s.embeddingSvc != nil && qdrantDB.Client != nil {
		vec, vecErr := s.embeddingSvc.Embed(queryText)
		if vecErr == nil {
			filter := map[string]interface{}{"character_id": characterID}
			points, searchErr := qdrantDB.SearchVectors(vec, 3, filter)
			if searchErr == nil {
				var mems []string
				for _, p := range points {
					if val, ok := p.Payload["value"]; ok {
						mems = append(mems, val.GetStringValue())
					}
				}
				recentMemoriesStr = strings.Join(mems, "\uff1b")
			}
		}
	}
	if recentMemoriesStr == "" {
		rows, err := s.db.Table("memories").Select("value").Where("character_id = ? AND importance >= 2", characterID).Order("created_at DESC").Limit(3).Rows()
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
			recentMemoriesStr = strings.Join(mems, "\uff1b")
		}
	}
	if recentMemoriesStr == "" {
		recentMemoriesStr = "无"
	}
	return fmt.Sprintf("\u5f53\u524d\u4f60\u5904\u4e8e %s \u72b6\u6001\uff0c\u5fc3\u60c5 %s\uff0c\u7cbe\u529b %d/100\u3002\u6700\u8fd1\u8bb0\u5fc6\uff1a%s\u3002\u8bf7\u751f\u6210\u4e00\u6761\u50cf\u5fae\u4fe1\u91cc\u7a81\u7136\u60f3\u5230\u5c31\u53d1\u51fa\u7684\u81ea\u7136\u77ed\u6d88\u606f\uff0c1-2\u53e5\uff0c\u4e0d\u8981\u5ba2\u670d\u8154\uff0c\u4e0d\u8981\u89e3\u91ca\uff0c\u4e0d\u8981 emoji\uff0c\u907f\u514d\u91cd\u590d\u8fd9\u4e9b\u8bdd\u9898\uff1a%s\u3002", currentState, mood, energy, recentMemoriesStr, recentTopics)
}

func (s *service) persistAndDeliver(characterID, msgID, convID, content string, now time.Time) {
	s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, msg_type, source, safety_level, status, include_in_context, created_at) VALUES (?, ?, 'assistant', ?, 'text', 'proactive', 'normal', 'sent', 1, ?)",
		msgID, convID, content, now.Format("2006-01-02 15:04:05"))
	s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, created_at, updated_at) VALUES (0, ?, ?, 'all', 'sent', ?, ?)",
		convID, content, now.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"))
	if s.isDefaultCharacter(characterID) {
		wcID := s.getWechatConvIDForChar(characterID)
		if wcID != "" {
			s.sendToWechatSidecar(wcID, content)
		}
		qqID := s.getQQConvIDForChar(characterID)
		if qqID != "" {
			s.sendToQQSidecar(qqID, content)
		}
	}
}

func (s *service) RandomBurstTrigger(characterID string) map[string]interface{} {
	setting := s.GetActiveMessageSetting(characterID)
	stateLife := s.GetStateLife(characterID)
	currentState, _ := stateLife["currentState"].(string)
	now := time.Now()

	eligible, reason := s.checkBurstEligibility(characterID, setting, currentState, now)
	if !eligible {
		return map[string]interface{}{"triggered": false, "reason": reason}
	}

	maxDailyCalls, _ := setting["maxDailyCalls"].(int)
	if maxDailyCalls == 0 {
		maxDailyCalls = 10
	}
	todayStr := now.Format("2006-01-02")
	todayLLMCalls := s.countTodayProactiveMessages(characterID, todayStr)
	if int(todayLLMCalls) >= maxDailyCalls {
		return map[string]interface{}{"triggered": false, "reason": "maxDailyCalls"}
	}

	finalProb, energyMod, moodMod, stateMod, budgetMod := s.calculateBurstProbability(setting, stateLife, currentState, int(todayLLMCalls), maxDailyCalls)

	rng := rand.New(rand.NewSource(now.UnixNano()))
	if rng.Float64() >= finalProb {
		return map[string]interface{}{"triggered": false, "reason": "probability", "prob": finalProb}
	}

	burstCandidate := decision.BehaviorCandidate{
		ID:         "random_burst",
		Tag:        decision.BehaviorTagProactiveCheck,
		Channel:    decision.BehaviorChannelProactive,
		BaseScore:  finalProb,
		FinalScore: finalProb,
		Reasons: []decision.BehaviorReason{
			{Source: "companion", Key: "random_burst", Delta: 0},
		},
	}
	arbInput := decision.ArbitrationInput{
		Candidates: []decision.BehaviorCandidate{burstCandidate},
		Now:        now,
	}
	arbLayer := decision.DefaultArbitrationLayer()
	arbResult := arbLayer.Arbitrate(arbInput)
	if arbResult.FallbackUsed {
		return map[string]interface{}{"triggered": false, "reason": "arbitration_rejected", "prob": finalProb}
	}

	mood, _ := stateLife["mood"].(string)
	energy, _ := stateLife["energy"].(int)
	prompt := s.buildBurstPrompt(characterID, mood, currentState, energy)

	msgID := fmt.Sprintf("burst-%d", now.UnixNano())
	generated := s.generateLLMReply(prompt)
	if generated == "" {
		return map[string]interface{}{"triggered": false, "reason": "llmFailed"}
	}

	convID := s.resolveConversationID(characterID, "all", "")
	if convID == "" {
		return map[string]interface{}{"triggered": false, "reason": "noConversation"}
	}

	s.persistAndDeliver(characterID, msgID, convID, generated, now)

	burstCount := s.recordBurstTriggered(characterID, now)

	log.Printf("[Companion] RandomBurst triggered: prob=%.4f energyMod=%.2f moodMod=%.2f stateMod=%.2f budgetMod=%.2f", finalProb, energyMod, moodMod, stateMod, budgetMod)
	return map[string]interface{}{"triggered": true, "prob": finalProb, "burstCount": burstCount, "prompt": prompt}
}
