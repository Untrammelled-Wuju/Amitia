package interaction

import "strings"

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func tokenizeForMemory(text string) []string {
	parts := strings.Fields(text)
	result := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		runes := []rune(part)
		if len(runes) <= 1 {
			continue
		}
		result = append(result, part)
		if len(runes) >= 4 {
			result = append(result, string(runes[:len(runes)/2]))
		}
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func containsAny(message string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func semanticRelatesToGoal(message string, snapshot ContextSnapshot, category AppraisalEventCategory) bool {
	if category == AppraisalCatHelp || category == AppraisalCatApology || category == AppraisalCatComplaint || category == AppraisalCatCold || category == AppraisalCatBoundaryCross {
		return true
	}
	for _, marker := range []string{"目标", "goal", "计划", "plan", "想要", "want", "需要", "need", "希望", "hope", "决定", "decide"} {
		if strings.Contains(strings.ToLower(message), marker) {
			return true
		}
	}
	return category == AppraisalCatPraise
}

func semanticControllable(message string, category AppraisalEventCategory) bool {
	switch category {
	case AppraisalCatBoundaryCross, AppraisalCatApology:
		return false
	case AppraisalCatHelp, AppraisalCatEmotional:
		return true
	}
	for _, marker := range []string{"地震", "earthquake", "意外", "accident", "突然", "sudden", "死亡", "death", "灾难", "disaster", "生病", "sick", "被迫", "forced"} {
		if strings.Contains(strings.ToLower(message), marker) {
			return false
		}
	}
	return true
}

func semanticHasAlternativeExplanation(message string, category AppraisalEventCategory) bool {
	switch category {
	case AppraisalCatApology, AppraisalCatComplaint, AppraisalCatCold:
		return true
	default:
		return false
	}
}

func semanticSimilarPastCount(message string, snapshot ContextSnapshot, category AppraisalEventCategory) int {
	if snapshot.Memories.Status != LoadStatusReady {
		return 0
	}
	tokens, count := tokenizeForMemory(strings.ToLower(message)), 0
	for _, memory := range snapshot.Memories.Value.Memories {
		matches, content := 0, strings.ToLower(memory.Value)
		for _, token := range tokens {
			if len(token) >= 2 && strings.Contains(content, token) {
				matches++
			}
		}
		if float64(matches)/float64(maxInt(1, len(tokens))) > 0.3 {
			count++
		}
	}
	return count
}

func semanticBoundaryViolated(message string, snapshot ContextSnapshot, category AppraisalEventCategory, sensitivities appraisalSensitivities) bool {
	if category == AppraisalCatBoundaryCross {
		return true
	}
	for _, marker := range []string{"爱", "love", "喜欢", "like", "想见", "want to meet", "私聊", "private", "加好友", "add friend"} {
		if strings.Contains(strings.ToLower(message), marker) && snapshot.Relationship.Status == LoadStatusReady && snapshot.Relationship.Value.Familiarity < 0.3 {
			return true
		}
	}
	if snapshot.Relationship.Status == LoadStatusReady && snapshot.Relationship.Value.Familiarity < 0.15+sensitivities.boundaryStrength*0.25 && isEmotionalMessage(message) {
		return true
	}
	return false
}

func semanticNormViolated(message string, snapshot ContextSnapshot, category AppraisalEventCategory, sensitivities appraisalSensitivities) bool {
	if category == AppraisalCatComplaint {
		for _, marker := range []string{"骂", "侮辱", "insult", "人身攻击", "personal attack", "威胁", "threat", "骚扰", "harass"} {
			if strings.Contains(strings.ToLower(message), marker) {
				return true
			}
		}
	}
	if category == AppraisalCatBoundaryCross && sensitivities.boundaryStrength > 0.6 {
		return true
	}
	return category == AppraisalCatBoundaryCross || category == AppraisalCatComplaint && isEmotionalMessage(message)
}

func semanticGoalCongruent(message string, snapshot ContextSnapshot, cat AppraisalEventCategory, sens appraisalSensitivities) bool {
	switch cat {
	case AppraisalCatPraise:
		return true
	case AppraisalCatComplaint, AppraisalCatBoundaryCross, AppraisalCatCold:
		return false
	case AppraisalCatApology:
		return snapshot.Relationship.Status == LoadStatusReady && snapshot.Relationship.Value.Tension < 0.4
	case AppraisalCatHelp:
		return true
	}
	msg := strings.ToLower(message)
	negativeMarkers := []string{"失败", "fail", "放弃", "give up", "做不到", "can't", "不行", "impossible", "绝望", "hopeless"}
	for _, m := range negativeMarkers {
		if strings.Contains(msg, m) {
			return false
		}
	}
	positiveMarkers := []string{"成功", "success", "做到了", "did it", "开心", "happy", "感谢", "thanks", "很棒", "great"}
	for _, m := range positiveMarkers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	if snapshot.Memories.Status == LoadStatusReady {
		recentPositive, recentNegative := 0, 0
		for _, mem := range snapshot.Memories.Value.Memories {
			mv := strings.ToLower(mem.Value)
			if containsAny(mv, positiveMarkers) {
				recentPositive++
			}
			if containsAny(mv, negativeMarkers) {
				recentNegative++
			}
		}
		if recentPositive > recentNegative {
			return true
		}
		if recentNegative > recentPositive {
			return false
		}
	}
	return sens.warmth > 0.3
}

func semanticIsExpected(message string, snapshot ContextSnapshot, cat AppraisalEventCategory, sens appraisalSensitivities) float64 {
	base := 0.5
	if snapshot.Relationship.Status == LoadStatusReady {
		base = snapshot.Relationship.Value.Familiarity*0.4 + 0.3
	}
	switch cat {
	case AppraisalCatPraise, AppraisalCatChat:
		base += 0.15
	case AppraisalCatBoundaryCross:
		base -= 0.30
	case AppraisalCatApology:
		base -= 0.20
	case AppraisalCatComplaint:
		base -= 0.15
	case AppraisalCatCold:
		base += 0.10
	case AppraisalCatEmotional:
		base -= 0.05
	}
	msg, surpriseCount := strings.ToLower(message), 0
	for _, m := range []string{"居然", "没想到", "竟然", "突然", "surprise", "unexpected", "震惊", "怎么会"} {
		if strings.Contains(msg, m) {
			surpriseCount++
		}
	}
	base -= float64(surpriseCount) * 0.10
	base -= (1.0 - sens.boundaryStrength) * 0.15
	return clampFloat(base, 0.05, 0.95)
}

func semanticResponsibility(message string, snapshot ContextSnapshot, cat AppraisalEventCategory, sens appraisalSensitivities) float64 {
	base := 0.5
	switch cat {
	case AppraisalCatApology:
		base = 0.85
	case AppraisalCatComplaint:
		base = 0.25
	case AppraisalCatPraise:
		base = 0.60
	case AppraisalCatBoundaryCross:
		base = 0.30
	}
	msg := strings.ToLower(message)
	for _, m := range []string{"我错了", "my fault", "怪我", "blame me", "对不起", "sorry", "是我的错", "I was wrong"} {
		if strings.Contains(msg, m) {
			base = 0.90
			break
		}
	}
	for _, m := range []string{"你错了", "your fault", "怪你", "blame you", "是你的问题", "your problem"} {
		if strings.Contains(msg, m) {
			base = 0.10
			break
		}
	}
	if snapshot.Relationship.Status == LoadStatusReady {
		base -= snapshot.Relationship.Value.Tension * 0.30
	}
	base += (sens.affection - 0.5) * 0.20
	return clampFloat(base, 0.05, 0.95)
}

func semanticUncertainty(message string, snapshot ContextSnapshot, cat AppraisalEventCategory, sens appraisalSensitivities) float64 {
	base := 0.35
	switch cat {
	case AppraisalCatHelp:
		base = 0.65
	case AppraisalCatApology:
		base = 0.50
	case AppraisalCatBoundaryCross:
		base = 0.20
	case AppraisalCatComplaint:
		base = 0.30
	case AppraisalCatCold:
		base = 0.45
	}
	msg, uncertainCount := strings.ToLower(message), 0
	for _, m := range []string{"可能", "maybe", "也许", "perhaps", "不知道", "don't know", "不确定", "unsure", "好像", "似乎", "大概"} {
		if strings.Contains(msg, m) {
			uncertainCount++
		}
	}
	base += float64(uncertainCount) * 0.12
	if snapshot.Relationship.Status == LoadStatusReady {
		base += (1.0 - snapshot.Relationship.Value.Security) * 0.20
	}
	base += (sens.rejectionSens - 0.5) * 0.15
	return clampFloat(base, 0.05, 0.95)
}
