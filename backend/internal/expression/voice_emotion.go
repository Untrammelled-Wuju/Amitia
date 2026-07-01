package expression

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

type VoiceEmotionTier string

const (
	VoiceEmotionPositive VoiceEmotionTier = "positive"
	VoiceEmotionNeutral  VoiceEmotionTier = "neutral"
	VoiceEmotionNegative VoiceEmotionTier = "negative"
	VoiceEmotionCaring   VoiceEmotionTier = "caring"
	VoiceEmotionHumorous VoiceEmotionTier = "humorous"
)

const (
	negativeEmotionSafetyCap = 0.6
)

type VoiceParams struct {
	Speed        float64         `json:"speed"`
	Pause        float64         `json:"pause"`
	EmotionTier  VoiceEmotionTier `json:"emotionTier"`
	Intensity    float64         `json:"intensity"`
	Trace        VoiceTrace      `json:"trace,omitempty"`
}

type VoiceTrace struct {
	SourcePlanID     string    `json:"sourcePlanId,omitempty"`
	MappedTones      []string  `json:"mappedTones,omitempty"`
	MappedEmotions   []string  `json:"mappedEmotions,omitempty"`
	FallbackReason   string    `json:"fallbackReason,omitempty"`
	GeneratedAt      time.Time `json:"generatedAt"`
	SafetyClamped    bool      `json:"safetyClamped"`
}

func neutralVoiceParams() VoiceParams {
	return VoiceParams{
		Speed:       1.0,
		Pause:       0.5,
		EmotionTier: VoiceEmotionNeutral,
		Intensity:   0.0,
		Trace: VoiceTrace{
			GeneratedAt: time.Now(),
		},
	}
}

func MapExpressionToVoice(plan interaction.ExpressionPlan) VoiceParams {
	return mapPlanToVoice(plan, true)
}

func MapExpressionToVoiceSafe(plan interaction.ExpressionPlan, supportsVoice bool) VoiceParams {
	return mapPlanToVoice(plan, supportsVoice)
}

func mapPlanToVoice(plan interaction.ExpressionPlan, supportsVoice bool) VoiceParams {
	if !supportsVoice {
		vp := neutralVoiceParams()
		vp.Trace.FallbackReason = "channel_unsupported"
		return vp
	}

	vp := VoiceParams{
		Speed:       1.0,
		Pause:       0.5,
		EmotionTier: VoiceEmotionNeutral,
		Intensity:   0.0,
		Trace: VoiceTrace{
			SourcePlanID: plan.ID,
			GeneratedAt:  time.Now(),
		},
	}

	tier, intensity := deriveEmotionTier(plan)
	if tier == VoiceEmotionNegative && intensity > negativeEmotionSafetyCap {
		intensity = negativeEmotionSafetyCap
		vp.Trace.SafetyClamped = true
	}

	vp.EmotionTier = tier
	vp.Intensity = intensity

	switch tier {
	case VoiceEmotionPositive:
		vp.Speed = 1.05 + intensity*0.05
		vp.Pause = 0.4 - intensity*0.05
	case VoiceEmotionNegative:
		vp.Speed = 0.9 - intensity*0.05
		vp.Pause = 0.6 + intensity*0.05
	case VoiceEmotionCaring:
		vp.Speed = 0.95 + intensity*0.03
		vp.Pause = 0.45 - intensity*0.03
	case VoiceEmotionHumorous:
		vp.Speed = 1.1 + intensity*0.05
		vp.Pause = 0.35 - intensity*0.03
	default:
		vp.Speed = 1.0
		vp.Pause = 0.5
	}

	if vp.Speed < 0.7 {
		vp.Speed = 0.7
	}
	if vp.Speed > 1.3 {
		vp.Speed = 1.3
	}
	if vp.Pause < 0.2 {
		vp.Pause = 0.2
	}
	if vp.Pause > 0.8 {
		vp.Pause = 0.8
	}

	for _, tone := range plan.Tones {
		vp.Trace.MappedTones = append(vp.Trace.MappedTones, string(tone))
	}
	for _, ep := range plan.EmotionPresentation {
		vp.Trace.MappedEmotions = append(vp.Trace.MappedEmotions, ep.Kind)
	}

	return vp
}

func deriveEmotionTier(plan interaction.ExpressionPlan) (VoiceEmotionTier, float64) {
	if len(plan.EmotionPresentation) == 0 {
		return tierFromTones(plan.Tones)
	}

	primary := plan.EmotionPresentation[0]

	kind := primary.Kind
	intensity := primary.Intensity

	negativeEmotions := map[string]bool{
		"sadness": true, "anger": true, "fear": true, "disgust": true,
		"frustration": true, "anxiety": true, "grief": true, "jealousy": true,
	}
	positiveEmotions := map[string]bool{
		"joy": true, "excitement": true, "happiness": true, "gratitude": true,
		"surprise": true, "hope": true, "pride": true, "relief": true,
	}
	caringEmotions := map[string]bool{
		"care": true, "love": true, "affection": true, "concern": true,
		"warmth": true, "empathy": true, "compassion": true, "support": true,
	}
	humorousEmotions := map[string]bool{
		"humor": true, "playful": true, "amusement": true, "wit": true,
		"teasing": true, "banter": true,
	}

	switch {
	case negativeEmotions[kind]:
		return VoiceEmotionNegative, intensity
	case positiveEmotions[kind]:
		return VoiceEmotionPositive, intensity
	case caringEmotions[kind]:
		return VoiceEmotionCaring, intensity
	case humorousEmotions[kind]:
		return VoiceEmotionHumorous, intensity
	}

	for _, ep := range plan.EmotionPresentation {
		switch {
		case negativeEmotions[ep.Kind]:
			return VoiceEmotionNegative, ep.Intensity
		case positiveEmotions[ep.Kind]:
			return VoiceEmotionPositive, ep.Intensity
		case caringEmotions[ep.Kind]:
			return VoiceEmotionCaring, ep.Intensity
		case humorousEmotions[ep.Kind]:
			return VoiceEmotionHumorous, ep.Intensity
		}
	}

	return tierFromTones(plan.Tones)
}

func tierFromTones(tones []interaction.ExpressionTone) (VoiceEmotionTier, float64) {
	for _, tone := range tones {
		switch tone {
		case interaction.ExpressionToneWarm:
			return VoiceEmotionCaring, 0.4
		case interaction.ExpressionTonePlayful:
			return VoiceEmotionHumorous, 0.5
		case interaction.ExpressionToneRepairing:
			return VoiceEmotionCaring, 0.3
		case interaction.ExpressionToneIntimate:
			return VoiceEmotionCaring, 0.5
		case interaction.ExpressionToneReserved:
			return VoiceEmotionNeutral, 0.2
		case interaction.ExpressionToneRational:
			return VoiceEmotionNeutral, 0.3
		}
	}
	return VoiceEmotionNeutral, 0.0
}

func MarshalVoiceTrace(vp VoiceParams) ([]byte, error) {
	return json.Marshal(vp.Trace)
}

func BuildAudioRequest(vp VoiceParams, text string) map[string]interface{} {
	return map[string]interface{}{
		"text":        text,
		"speed":       vp.Speed,
		"pause":       vp.Pause,
		"emotionTier": string(vp.EmotionTier),
		"intensity":   vp.Intensity,
	}
}

func BuildAudioRequestWithTrace(vp VoiceParams, text string) (map[string]interface{}, []byte) {
	req := BuildAudioRequest(vp, text)
	traceBytes, _ := MarshalVoiceTrace(vp)
	return req, traceBytes
}
