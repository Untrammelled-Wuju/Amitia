package decision

import "time"

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
	BehaviorPlan   BehaviorPlan
	Psyche         PsycheSignalSet
	ExpressionCtrl ExpressionControlInput
	CopingStrategy CopingStrategy
	SafetyResult   SafetyCheckResult
	Now            time.Time
}

func GenerateExpressionPlan(input ExpressionPlanInput) ExpressionPlan {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	plan := ExpressionPlan{
		ID:             "expr-" + input.BehaviorPlan.ID,
		BehaviorPlanID: input.BehaviorPlan.ID,
		CreatedAt:      now,
		Channel:        input.BehaviorPlan.Selected.Channel,
		CopingStrategy: input.CopingStrategy,
		Metadata:       make(map[string]any),
	}
	ctrlConfig := DefaultExpressionControlConfig()
	ctrlResult := ControlExpression(input.ExpressionCtrl, ctrlConfig)
	plan.ExpressionType = mapExpressionType(input.BehaviorPlan.Selected.Tag)
	plan.Tone = deriveExpressionTone(input.Psyche, input.BehaviorPlan.Selected)
	plan.Length = deriveExpressionLength(input.BehaviorPlan.Selected)
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
	return plan
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

func deriveExpressionTone(psyche PsycheSignalSet, candidate BehaviorCandidate) ExpressionTone {
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
	if candidate.Tag == BehaviorTagSetBoundary {
		return ExpressionToneFirm
	}
	if candidate.Tag == BehaviorTagOfferSupport && posEmotion > 0.4 {
		return ExpressionToneWarm
	}
	if negEmotion > 0.6 || stressVal > 0.7 {
		return ExpressionToneConcerned
	}
	if posEmotion > 0.5 && stressVal < 0.4 {
		return ExpressionTonePlayful
	}
	if posEmotion > 0.3 {
		return ExpressionToneWarm
	}
	if negEmotion > 0.3 {
		return ExpressionToneSoft
	}
	return ExpressionToneNeutral
}

func deriveExpressionLength(candidate BehaviorCandidate) ExpressionLength {
	switch candidate.Tag {
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
