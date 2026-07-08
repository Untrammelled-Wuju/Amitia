package chat

import (
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/interaction"
	promptir "github.com/u-ai/backend/internal/prompt"
)

func deriveEmotionLabel(tone decision.ExpressionTone, emotionIntensity float64, suppressed bool) string {
	switch tone {
	case decision.ExpressionToneWarm:
		if emotionIntensity >= 0.6 {
			return "SWEET_ATTACHMENT"
		}
		return "QUIET_FOND"
	case decision.ExpressionToneNeutral:
		return "CALM_RATIONAL"
	case decision.ExpressionTonePlayful:
		return "TSUNDERE"
	case decision.ExpressionToneConcerned:
		if emotionIntensity >= 0.5 {
			return "HURT_GRIEVANCE"
		}
		return "FEARFUL_OBEDIENT"
	case decision.ExpressionToneFirm:
		return "ANGRY_ATTACK"
	case decision.ExpressionToneSoft:
		if suppressed {
			return "COLD_DETACHED"
		}
		return "SHY_HEARTBEAT"
	default:
		return "CALM_RATIONAL"
	}
}

func deriveAffValue(emotionIntensity float64, tone decision.ExpressionTone) float64 {
	base := 0.5
	switch tone {
	case decision.ExpressionToneWarm:
		base = 0.65 + emotionIntensity*0.35
	case decision.ExpressionTonePlayful:
		base = 0.55 + emotionIntensity*0.3
	case decision.ExpressionToneNeutral:
		base = 0.5
	case decision.ExpressionToneFirm:
		base = 0.3 - emotionIntensity*0.1
	case decision.ExpressionToneSoft:
		base = 0.45
	case decision.ExpressionToneConcerned:
		base = 0.4 + emotionIntensity*0.2
	default:
		base = 0.5
	}
	return clampFloat(base, -1.0, 1.0)
}

func deriveSecValue(security float64) float64 {
	return clampFloat(security*2.0-1.0, -1.0, 1.0)
}

func deriveAroValue(emotionIntensity float64) float64 {
	return clampFloat(emotionIntensity*2.0-1.0, -1.0, 1.0)
}

func deriveDomValue(tone decision.ExpressionTone, suppressed bool) float64 {
	if suppressed {
		return -0.5
	}
	switch tone {
	case decision.ExpressionToneFirm:
		return 0.6
	case decision.ExpressionToneWarm:
		return 0.3
	case decision.ExpressionTonePlayful:
		return 0.4
	case decision.ExpressionToneSoft:
		return -0.3
	case decision.ExpressionToneConcerned:
		return -0.2
	default:
		return 0.0
	}
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func buildEmotionFusionInput(runtime *interaction.RuntimeAssembly) *promptir.EmotionFusionInput {
	if runtime == nil {
		return nil
	}
	ep := runtime.ExpressionPlan
	if ep == nil {
		return nil
	}
	emotionIntensity := ep.EmotionIntensity
	if emotionIntensity <= 0 {
		emotionIntensity = 0.5
	}
	label := deriveEmotionLabel(ep.Tone, emotionIntensity, ep.Suppressed)

	security := 0.5
	if runtime.Appraisal != nil && runtime.Appraisal.RelationshipDelta > 0 {
		security = 0.6
	}

	aff := deriveAffValue(emotionIntensity, ep.Tone)
	sec := deriveSecValue(security)
	aro := deriveAroValue(emotionIntensity)
	dom := deriveDomValue(ep.Tone, ep.Suppressed)

	return &promptir.EmotionFusionInput{
		PrimaryLabel: label,
		Aff:          aff,
		Sec:          sec,
		Aro:          aro,
		Dom:          dom,
	}
}

func buildEmotionFusionRaw(runtime *interaction.RuntimeAssembly, name string) string {
	if runtime == nil {
		return ""
	}
	input := buildEmotionFusionInput(runtime)
	if input == nil {
		return ""
	}
	if name != "" {
		input.PersonalityLabel = name
	}
	input.CoreConflict = "嘴硬但心软"
	input.Catchphrases = []string{"嗯", "哼", "切"}
	input.SpeakingStyle = "自然口语化，多用短句和反问"
	return promptir.BuildEmotionFusionRawSection(*input)
}
