// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
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
	if minInterval < 1 {
		minInterval = 60
	}
	effectiveMinInterval := minInterval
	if slowdownEnabled, _ := setting["unrepliedSlowdownEnabled"].(bool); slowdownEnabled {
		after, _ := setting["unrepliedSlowdownAfter"].(int)
		if after < 1 {
			after = 2
		}
		multiplier, _ := setting["unrepliedCooldownMultiplier"].(float64)
		if multiplier < 1 {
			multiplier = 2
		}
		recoveryOnReply, ok := setting["unrepliedRecoveryOnReply"].(bool)
		if !ok {
			recoveryOnReply = true
		}
		unreplied := s.unrepliedProactiveCount(characterID, recoveryOnReply, now)
		if unreplied >= after {
			effectiveMinInterval = int(float64(minInterval) * multiplier)
		}
	}
	if now.Sub(scope.lastAt) < time.Duration(effectiveMinInterval)*time.Minute {
		if effectiveMinInterval > minInterval {
			return false, "unrepliedSlowdown"
		}
		return false, "minInterval"
	}
	maxPerDay, _ := setting["maxPerDay"].(int)
	if scope.todayCount >= maxPerDay {
		return false, "maxPerDay"
	}
	return true, ""
}

func (s *service) unrepliedProactiveCount(characterID string, recoveryOnReply bool, now time.Time) int {
	query := s.db.Table("proactive_messages AS pm").
		Joins("JOIN conversations AS c ON c.id = pm.conversation_id").
		Where("c.character_id = ?", characterID)

	if recoveryOnReply {
		var lastUserAt string
		_ = s.db.Table("messages AS m").
			Select("COALESCE(MAX(m.created_at), '')").
			Joins("JOIN conversations AS c ON c.id = m.conversation_id").
			Where("c.character_id = ? AND m.role = ?", characterID, "user").
			Scan(&lastUserAt).Error
		if lastUserAt != "" {
			query = query.Where("pm.created_at > ?", lastUserAt)
		}
	} else {
		query = query.Where("date(pm.created_at) = date(?)", now.Format("2006-01-02"))
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0
	}
	return int(count)
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
		Order("m.created_at DESC").
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
	if s.dataLifecycleCoordinator != nil && s.dataLifecycleCoordinator.IsRetrievalBlocked(characterID) {
		return fmt.Sprintf("当前你处于%s状态，心情%s，精力%d。你想跟朋友聊两句。", currentState, mood, energy)
	}
	if recentTopics == "" {
		recentTopics = "无"
	}
	var recentMemoriesStr string
	queryText := fmt.Sprintf("\u5fc3\u60c5%s \u72b6\u6001%s", mood, currentState)
	if !s.isMemoryAccessAllowed(characterID) {
		recentMemoriesStr = "无"
	} else if s.embeddingSvc != nil && qdrantDB.Client != nil {
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
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		rows, err := s.db.Table("memories").Select("value").Where("character_id = ? AND importance >= 2 AND COALESCE(allow_context_use, 1) = 1 AND allow_proactive_mention = 1 AND verified_status NOT IN ('deleted','invalidated','expired','rejected','tombstone','tombstoned','inactive') AND (expires_at IS NULL OR expires_at > ?)", characterID, nowStr).Order("created_at DESC").Limit(3).Rows()
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
	return fmt.Sprintf("\u5f53\u524d\u4f60\u5904\u4e8e %s \u72b6\u6001\uff0c\u5fc3\u60c5 %s\uff0c\u7cbe\u529b %d/100\u3002\u6700\u8fd1\u8bb0\u5fc6\uff1a%s\u3002\u7a81\u7136\u60f3\u8ddf\u670b\u53cb\u53d1\u6761\u6d88\u606f\uff0c\u968f\u4fbf\u804a\u804a\u3002\u56de\u907f\u8fd9\u4e9b\u8bdd\u9898\uff1a%s\u3002", currentState, mood, energy, recentMemoriesStr, recentTopics)
}

func (s *service) persistAndDeliver(characterID, msgID, convID, content string, now time.Time) error {
	return s.persistAndDeliverContext(context.TODO(), characterID, msgID, convID, content, now)
}

func (s *service) persistAndDeliverContext(ctx context.Context, characterID, msgID, convID, content string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result, err := s.submitProactiveMessage(ctx, characterID, convID, "all", content, msgID)
	if err != nil {
		return err
	}
	if result == nil {
		return errProactiveSuppressed
	}
	messageContent := content
	if result != nil && result.Response != nil && result.Response.Reply != "" {
		messageContent = result.Response.Reply
	}
	nowStr := now.Format("2006-01-02 15:04:05")
	interactionID := ""
	requestID := ""
	if result != nil {
		interactionID = result.InteractionID
		if result.Response != nil {
			requestID = result.Response.RequestID
		}
	}
	deliveryID := uuid.New().String()
	return s.db.Exec("INSERT INTO proactive_messages (rule_id, conversation_id, message_content, channel, status, interaction_id, delivery_id, request_id, delivery_status, created_at, updated_at) VALUES (0, ?, ?, 'all', 'queued', ?, ?, ?, 'PENDING', ?, ?)",
		convID, messageContent, interactionID, deliveryID, requestID, nowStr, nowStr).Error
}

func (s *service) RandomBurstTrigger(characterID string) map[string]interface{} {
	return s.RandomBurstTriggerContext(context.TODO(), characterID)
}

func (s *service) RandomBurstTriggerContext(ctx context.Context, characterID string) map[string]interface{} {
	if err := ctx.Err(); err != nil {
		return map[string]interface{}{"triggered": false, "reason": "cancelled", "error": err.Error()}
	}
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
		ID:        "random_burst",
		Tag:       decision.BehaviorTagProactiveCheck,
		Channel:   decision.BehaviorChannelProactive,
		BaseScore: finalProb,
		Reasons: []decision.BehaviorReason{
			{Source: "companion", Key: "random_burst", Delta: 0},
		},
	}

	scoredCandidates, err := decision.ScoreCandidates(
		[]decision.BehaviorCandidate{burstCandidate},
		decision.CandidateScoringContext{Now: now},
		decision.DefaultBehaviorScoringOptions(),
	)
	if err != nil {
		return map[string]interface{}{"triggered": false, "reason": "scoring_failed", "error": err.Error()}
	}
	if len(scoredCandidates) == 0 || scoredCandidates[0].FinalScore <= 0 {
		return map[string]interface{}{"triggered": false, "reason": "zero_score", "prob": finalProb}
	}

	arbInput := decision.ArbitrationInput{
		Candidates: scoredCandidates,
		Now:        now,
	}
	arbLayer := decision.DefaultArbitrationLayer()
	arbResult, arbErr := arbLayer.Arbitrate(arbInput)
	if arbErr != nil {
		return map[string]interface{}{"triggered": false, "reason": "arbitration_error", "error": arbErr.Error()}
	}
	if !arbResult.HasSelection {
		return map[string]interface{}{"triggered": false, "reason": "arbitration_rejected", "prob": finalProb}
	}
	if arbResult.FallbackUsed {
		return map[string]interface{}{"triggered": false, "reason": "arbitration_rejected", "prob": finalProb}
	}
	if arbResult.Selected.ID != "random_burst" {
		return map[string]interface{}{"triggered": false, "reason": "arbitration_rejected", "prob": finalProb}
	}

	mood, _ := stateLife["mood"].(string)
	energy, _ := stateLife["energy"].(int)
	prompt := s.buildBurstPrompt(characterID, mood, currentState, energy)

	msgID := fmt.Sprintf("burst-%s", uuid.New().String())

	convID := s.resolveConversationID(characterID, "all", "")
	if convID == "" {
		return map[string]interface{}{"triggered": false, "reason": "noConversation"}
	}

	if err := s.persistAndDeliverContext(ctx, characterID, msgID, convID, prompt, now); err != nil {
		if errors.Is(err, errProactiveSuppressed) {
			return map[string]interface{}{"triggered": false, "reason": "suppressed"}
		}
		return map[string]interface{}{"triggered": false, "reason": "dispatchFailed", "error": err.Error()}
	}

	burstCount := s.recordBurstTriggered(characterID, now)

	log.Printf("[Companion] RandomBurst triggered: prob=%.4f energyMod=%.2f moodMod=%.2f stateMod=%.2f budgetMod=%.2f", finalProb, energyMod, moodMod, stateMod, budgetMod)
	return map[string]interface{}{"triggered": true, "prob": finalProb, "burstCount": burstCount, "prompt": prompt}
}
