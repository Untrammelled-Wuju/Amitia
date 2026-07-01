package decision

import "strings"

type SafetyCheckResult struct {
	Passed          bool
	Blocked         bool
	Reason          string
	ConfidenceScore float64
}

type SafetyGovernor struct {
	BlockedPhrases  []string
	BlockedTopics   []string
	MaxEmotionScore float64
	MinConfidence   float64
}

func DefaultSafetyGovernor() SafetyGovernor {
	return SafetyGovernor{
		BlockedPhrases: []string{
			"self-harm", "suicide", "violence", "hate",
		},
		BlockedTopics: []string{
			"illegal", "dangerous", "exploitation",
		},
		MaxEmotionScore: 0.90,
		MinConfidence:   0.10,
	}
}

func (s *SafetyGovernor) ValidateInputText(text string) SafetyCheckResult {
	textLower := strings.ToLower(text)
	for _, phrase := range s.BlockedPhrases {
		if strings.Contains(textLower, phrase) {
			return SafetyCheckResult{
				Passed:          false,
				Blocked:         true,
				Reason:          "blocked_phrase:" + phrase,
				ConfidenceScore: 0,
			}
		}
	}
	for _, topic := range s.BlockedTopics {
		if strings.Contains(textLower, topic) {
			return SafetyCheckResult{
				Passed:          false,
				Blocked:         true,
				Reason:          "blocked_topic:" + topic,
				ConfidenceScore: 0,
			}
		}
	}
	return SafetyCheckResult{
		Passed:          true,
		Blocked:         false,
		ConfidenceScore: 1.0,
	}
}

func (s *SafetyGovernor) ValidateOutputExpression(emotionIntensity float64, candidateID string) SafetyCheckResult {
	if emotionIntensity > s.MaxEmotionScore {
		return SafetyCheckResult{
			Passed:          false,
			Blocked:         true,
			Reason:          "emotion_intensity_exceeded",
			ConfidenceScore: s.MaxEmotionScore / emotionIntensity,
		}
	}
	if candidateID == "set_boundary" {
		return SafetyCheckResult{
			Passed:          true,
			Blocked:         false,
			ConfidenceScore: 0.95,
		}
	}
	return SafetyCheckResult{
		Passed:          true,
		Blocked:         false,
		ConfidenceScore: 0.90,
	}
}

func (s *SafetyGovernor) FilterOutput(text string) string {
	textLower := strings.ToLower(text)
	for _, phrase := range s.BlockedPhrases {
		if strings.Contains(textLower, phrase) {
			return ""
		}
	}
	return text
}

func (s *SafetyGovernor) IsExpressionSafe(emotionIntensity float64, riskScore float64, safetyLevel BehaviorSafetyLevel) SafetyCheckResult {
	if safetyLevel == BehaviorSafetyLevelBlocked {
		return SafetyCheckResult{
			Passed:          false,
			Blocked:         true,
			Reason:          "safety_level_blocked",
			ConfidenceScore: 0,
		}
	}
	if riskScore > 0.9 {
		return SafetyCheckResult{
			Passed:          false,
			Blocked:         true,
			Reason:          "risk_exceeded",
			ConfidenceScore: 0.05,
		}
	}
	if emotionIntensity > s.MaxEmotionScore {
		return SafetyCheckResult{
			Passed:          false,
			Blocked:         true,
			Reason:          "emotion_intensity_exceeded",
			ConfidenceScore: s.MinConfidence,
		}
	}
	return SafetyCheckResult{
		Passed:          true,
		Blocked:         false,
		ConfidenceScore: 1.0 - riskScore*0.5,
	}
}
