package decision

type CopingStrategy string

const (
	CopingReappraisal    CopingStrategy = "reappraisal"
	CopingSuppression    CopingStrategy = "suppression"
	CopingSeekSupport    CopingStrategy = "seek_support"
	CopingProblemSolving CopingStrategy = "problem_solving"
	CopingAcceptance     CopingStrategy = "acceptance"
	CopingDistraction    CopingStrategy = "distraction"
	CopingRumination     CopingStrategy = "rumination"
	CopingAvoidance      CopingStrategy = "avoidance"
)

type CopingInput struct {
	PositiveEmotion  float64
	NegativeEmotion  float64
	Stress           float64
	CognitiveLoad    float64
	AvailableSupport float64
}

type CopingResult struct {
	PrimaryStrategy   CopingStrategy
	SecondaryStrategy CopingStrategy
	Confidence        float64
	Reason            string
}

func SelectCopingStrategy(input CopingInput) CopingResult {
	if input.Stress > 0.8 {
		if input.AvailableSupport > 0.5 {
			return CopingResult{
				PrimaryStrategy: CopingSeekSupport,
				Confidence:      0.85,
				Reason:          "high_stress_with_support",
			}
		}
		return CopingResult{
			PrimaryStrategy:   CopingAcceptance,
			SecondaryStrategy: CopingDistraction,
			Confidence:        0.60,
			Reason:            "high_stress_no_support",
		}
	}
	if input.NegativeEmotion > 0.7 {
		if input.CognitiveLoad > 0.7 {
			return CopingResult{
				PrimaryStrategy:   CopingDistraction,
				SecondaryStrategy: CopingSuppression,
				Confidence:        0.55,
				Reason:            "high_negative_high_load",
			}
		}
		return CopingResult{
			PrimaryStrategy: CopingReappraisal,
			Confidence:      0.75,
			Reason:          "high_negative_available_capacity",
		}
	}
	if input.PositiveEmotion > 0.5 {
		return CopingResult{
			PrimaryStrategy: CopingProblemSolving,
			Confidence:      0.80,
			Reason:          "positive_affect_engaged",
		}
	}
	if input.Stress > 0.5 {
		if input.CognitiveLoad > 0.6 {
			return CopingResult{
				PrimaryStrategy:   CopingAcceptance,
				SecondaryStrategy: CopingDistraction,
				Confidence:        0.60,
				Reason:            "moderate_stress_high_load",
			}
		}
		return CopingResult{
			PrimaryStrategy: CopingReappraisal,
			Confidence:      0.65,
			Reason:          "moderate_stress_manageable",
		}
	}
	return CopingResult{
		PrimaryStrategy: CopingProblemSolving,
		Confidence:      0.70,
		Reason:          "default_engaged",
	}
}

func CopingStrategyBoost(strategy CopingStrategy, candidateID string) float64 {
	switch strategy {
	case CopingReappraisal:
		if candidateID == "chat_reply" || candidateID == "express_emotion" {
			return 0.15
		}
	case CopingSuppression:
		if candidateID == "wait_observe" || candidateID == "delay" {
			return 0.20
		}
	case CopingSeekSupport:
		if candidateID == "offer_support" || candidateID == "chat_reply" {
			return 0.25
		}
	case CopingProblemSolving:
		if candidateID == "tool_search" || candidateID == "ask_clarify" {
			return 0.20
		}
	case CopingAcceptance:
		if candidateID == "wait_observe" || candidateID == "chat_reply" {
			return 0.10
		}
	case CopingDistraction:
		if candidateID == "proactive_greet" || candidateID == "chat_reply" {
			return 0.10
		}
	case CopingAvoidance:
		if candidateID == "wait_observe" || candidateID == "delay" {
			return 0.30
		}
	}
	return 0
}
