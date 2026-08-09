package decision

import (
	"errors"
	"fmt"
	"time"
)

type ExpressionType string

const (
	ExpressionTypeText      ExpressionType = "text"
	ExpressionTypeEmoji     ExpressionType = "emoji"
	ExpressionTypeQuestion  ExpressionType = "question"
	ExpressionTypeStatement ExpressionType = "statement"
	ExpressionTypeGreeting  ExpressionType = "greeting"
	ExpressionTypeBoundary  ExpressionType = "boundary"
	ExpressionTypeSilence   ExpressionType = "silence"
)

type ExpressionTone string

const (
	ExpressionToneWarm      ExpressionTone = "warm"
	ExpressionToneNeutral   ExpressionTone = "neutral"
	ExpressionToneFirm      ExpressionTone = "firm"
	ExpressionToneSoft      ExpressionTone = "soft"
	ExpressionTonePlayful   ExpressionTone = "playful"
	ExpressionToneConcerned ExpressionTone = "concerned"
)

type ExpressionLength string

const (
	ExpressionLengthShort  ExpressionLength = "short"
	ExpressionLengthMedium ExpressionLength = "medium"
	ExpressionLengthLong   ExpressionLength = "long"
)

type ExpressionPlan struct {
	ID               string           `json:"id"`
	BehaviorPlanID   string           `json:"behaviorPlanId"`
	CreatedAt        time.Time        `json:"createdAt"`
	ExpressionType   ExpressionType   `json:"expressionType"`
	Tone             ExpressionTone   `json:"tone"`
	Length           ExpressionLength `json:"length"`
	EmotionIntensity float64          `json:"emotionIntensity"`
	ScaleFactor      float64          `json:"scaleFactor"`
	Suppressed       bool             `json:"suppressed"`
	CopingStrategy   CopingStrategy   `json:"copingStrategy,omitempty"`
	Channel          BehaviorChannel  `json:"channel"`
	DoNotSend        bool             `json:"doNotSend"`
	SafetyBlocked    bool             `json:"safetyBlocked"`
	Metadata         map[string]any   `json:"metadata,omitempty"`
}

type ExpressionPlanInput struct {
	BehaviorPlan               BehaviorPlan
	Psyche                     PsycheSignalSet
	ExpressionCtrl             ExpressionControlInput
	CopingStrategy             CopingStrategy
	SafetyResult               SafetyCheckResult
	PersonalityExpressionStyle map[string]float64
	Now                        time.Time
}

func GenerateExpressionPlan(input ExpressionPlanInput) (ExpressionPlan, error) {
	if input.BehaviorPlan.ID == "" {
		return ExpressionPlan{}, errors.New("expression plan: behavior plan ID is required")
	}
	if !input.BehaviorPlan.NeedsExpression {
		return ExpressionPlan{}, errors.New("expression plan: behavior plan does not need expression")
	}
	if input.BehaviorPlan.DoNotSend {
		return ExpressionPlan{}, errors.New("expression plan: behavior plan is do-not-send")
	}
	if input.BehaviorPlan.ExpressionPlanID == "" {
		return ExpressionPlan{}, errors.New("expression plan: behavior plan expression_plan_id is empty")
	}
	if input.Now.IsZero() {
		return ExpressionPlan{}, errors.New("expression plan: Now is required")
	}

	plan := ExpressionPlan{
		ID:             input.BehaviorPlan.ExpressionPlanID,
		BehaviorPlanID: input.BehaviorPlan.ID,
		CreatedAt:      input.Now,
		Channel:        input.BehaviorPlan.Selected.Channel,
		CopingStrategy: input.CopingStrategy,
		Metadata:       make(map[string]any),
	}

	ctrlConfig := DefaultExpressionControlConfig()
	ctrlResult := ControlExpression(input.ExpressionCtrl, ctrlConfig)
	plan.ExpressionType = mapExpressionType(input.BehaviorPlan.Selected.Tag)
	plan.Tone = deriveExpressionTone(input.Psyche, input.BehaviorPlan.Selected, input.PersonalityExpressionStyle)
	plan.Length = deriveExpressionLength(input.BehaviorPlan.Selected, input.PersonalityExpressionStyle)
	plan.EmotionIntensity = ClampEmotionIntensity(input.ExpressionCtrl.EmotionIntensity, ctrlConfig.SafetyCeiling)
	plan.ScaleFactor = ctrlResult.ScaleFactor
	plan.Suppressed = ctrlResult.Suppressed

	if input.SafetyResult.Blocked {
		plan.SafetyBlocked = true
		plan.DoNotSend = true
		plan.Suppressed = true
		plan.ScaleFactor = 0
		plan.EmotionIntensity = 0
	}

	return plan, nil
}

func mapExpressionType(tag BehaviorTag) ExpressionType {
	switch tag {
	case BehaviorTagReply:
		return ExpressionTypeText
	case BehaviorTagAskClarify:
		return ExpressionTypeQuestion
	case BehaviorTagOfferSupport:
		return ExpressionTypeStatement
	case BehaviorTagSetBoundary:
		return ExpressionTypeBoundary
	case BehaviorTagProactiveCheck:
		return ExpressionTypeGreeting
	case BehaviorTagDelay:
		return ExpressionTypeSilence
	default:
		return ExpressionTypeText
	}
}

func deriveExpressionTone(psyche PsycheSignalSet, candidate BehaviorCandidate, personalityStyle map[string]float64) ExpressionTone {
	stressVal := psyche.Stress.Value
	posEmotion := 0.0
	negEmotion := 0.0
	for _, e := range psyche.Emotions {
		if e.Kind == "joy" || e.Kind == "care" || e.Kind == "gratitude" {
			posEmotion += e.Intensity
		}
		if e.Kind == "anger" || e.Kind == "sadness" || e.Kind == "fear" {
			negEmotion += e.Intensity
		}
	}

	emotionalExpr := getStyleValue(personalityStyle, "emotionalExpression", 0.5)
	formality := getStyleValue(personalityStyle, "formality", 0.5)
	toneWords := getStyleValue(personalityStyle, "toneWords", 0.5)

	if candidate.Tag == BehaviorTagSetBoundary {
		return ExpressionToneFirm
	}
	if candidate.Tag == BehaviorTagOfferSupport && posEmotion > 0.4 {
		if emotionalExpr > 0.6 {
			return ExpressionToneWarm
		}
	}
	if negEmotion > 0.6 || stressVal > 0.7 {
		if emotionalExpr > 0.5 {
			return ExpressionToneConcerned
		}
		if formality > 0.6 {
			return ExpressionToneFirm
		}
		return ExpressionToneSoft
	}
	if posEmotion > 0.5 && stressVal < 0.4 {
		if toneWords > 0.6 {
			return ExpressionTonePlayful
		}
		if emotionalExpr > 0.5 {
			return ExpressionToneWarm
		}
		return ExpressionToneNeutral
	}
	if posEmotion > 0.3 {
		if toneWords > 0.5 {
			return ExpressionTonePlayful
		}
		if emotionalExpr > 0.4 {
			return ExpressionToneWarm
		}
		return ExpressionToneWarm
	}
	if negEmotion > 0.3 {
		if emotionalExpr > 0.4 {
			return ExpressionToneConcerned
		}
		return ExpressionToneSoft
	}
	if formality > 0.7 {
		return ExpressionToneFirm
	}
	return ExpressionToneNeutral
}

func deriveExpressionLength(candidate BehaviorCandidate, personalityStyle map[string]float64) ExpressionLength {
	verbosity := getStyleValue(personalityStyle, "verbosity", 0.5)
	shortSentence := getStyleValue(personalityStyle, "shortSentence", 0.5)

	baseLength := baseExpressionLength(candidate.Tag)

	if verbosity > 0.7 && baseLength == ExpressionLengthShort {
		return ExpressionLengthMedium
	}
	if verbosity > 0.7 && baseLength == ExpressionLengthMedium {
		return ExpressionLengthLong
	}
	if verbosity < 0.3 && baseLength == ExpressionLengthLong {
		return ExpressionLengthMedium
	}
	if verbosity < 0.3 && baseLength == ExpressionLengthMedium {
		return ExpressionLengthShort
	}
	if shortSentence > 0.7 {
		if baseLength == ExpressionLengthLong {
			return ExpressionLengthMedium
		}
		if baseLength == ExpressionLengthMedium {
			return ExpressionLengthShort
		}
	}
	return baseLength
}

func baseExpressionLength(tag BehaviorTag) ExpressionLength {
	switch tag {
	case BehaviorTagReply:
		return ExpressionLengthMedium
	case BehaviorTagAskClarify:
		return ExpressionLengthShort
	case BehaviorTagOfferSupport:
		return ExpressionLengthMedium
	case BehaviorTagSetBoundary:
		return ExpressionLengthShort
	case BehaviorTagProactiveCheck:
		return ExpressionLengthShort
	case BehaviorTagDelay:
		return ExpressionLengthShort
	default:
		return ExpressionLengthShort
	}
}

func getStyleValue(style map[string]float64, key string, defaultVal float64) float64 {
	if style == nil {
		return defaultVal
	}
	if v, ok := style[key]; ok {
		return v
	}
	return defaultVal
}

var _ = fmt.Sprintf
