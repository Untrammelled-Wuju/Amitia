package emotionstate

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/expression"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/relationship"
)

type CurrentExpression struct {
	Primary       string   `json:"primary"`
	Secondary     string   `json:"secondary,omitempty"`
	Intensity     float64  `json:"intensity"`
	Warmth        float64  `json:"warmth"`
	Coldness      float64  `json:"coldness"`
	Energy        float64  `json:"energy"`
	Pace          float64  `json:"pace"`
	Concern       float64  `json:"concern"`
	AngerDisplay  float64  `json:"anger_display"`
	HurtDisplay   float64  `json:"hurt_display"`
	Smile         float64  `json:"smile"`
	PauseBeforeMS int      `json:"pause_before_ms"`
	PolicyReason  string   `json:"policy_reason"`
	Evidence      []string `json:"evidence,omitempty"`
}

type Context struct {
	UserAffect          UserAffect
	RelationshipEmotion RelationshipEmotion
	Signals             Signals
	Baseline            Baseline
	CurrentExpression   CurrentExpression
	SchedulePrompt      string
	Prompt              string
	VoiceInstruction    string
}

func Compose(psycheState *psyche.PsycheState, relation *relationship.RelationshipState, transient RelationshipEmotion, affect UserAffect, signals Signals) CurrentExpression {
	out := CurrentExpression{
		Primary:       "calm",
		Intensity:     0.28,
		Warmth:        0.5,
		Energy:        0.5,
		Pace:          0.5,
		PauseBeforeMS: 80,
		PolicyReason:  "baseline",
	}
	if psycheState != nil {
		valence := psycheState.Emotion.Valence
		arousal := psycheState.Emotion.Arousal
		dominance := psycheState.Emotion.Dominance
		out.Intensity = clamp01(1 - dominance*0.35 + math.Abs(valence-0.5)*0.55)
		out.Warmth = clamp01(0.42 + valence*0.34 + dominance*0.08)
		out.Energy = clamp01(psycheState.Energy*0.7 + arousal*0.3)
		out.Pace = clamp01(0.42 + arousal*0.38 - psycheState.Stress*0.18)
		out.Smile = clamp01((valence - 0.5) * 1.4)
		if psycheState.Energy <= 0.28 || psycheState.Stress >= 0.72 {
			out.Primary = "tired"
			out.Intensity = math.Max(out.Intensity, psycheState.Stress)
			out.PauseBeforeMS = 140
			out.PolicyReason = "psyche_fatigue_or_stress"
		}
	}
	if relation != nil {
		out.Warmth = clamp01(out.Warmth*0.62 + relation.Security*0.15 + relation.Trust*0.13 + relation.RepairConfidence*0.10)
		out.Coldness = clamp01(relation.Tension * 0.62)
		if relation.Tension >= 0.45 {
			out.Primary = "restrained_conflict"
			out.Intensity = math.Max(out.Intensity, relation.Tension)
			out.PauseBeforeMS = 130
			out.PolicyReason = "relationship_tension"
		}
	}
	out.Warmth = clamp01(out.Warmth*0.72 + transient.Warmth*0.28)
	out.Concern = transient.Concern
	out.AngerDisplay = transient.Anger
	out.HurtDisplay = transient.Hurt
	out.Coldness = clamp01(out.Coldness + transient.Anger*0.30 + transient.Hurt*0.24 + transient.Disappointment*0.22)
	if transient.Anger >= 0.42 || transient.Hurt >= 0.42 || transient.Disappointment >= 0.48 {
		out.Primary = "restrained_conflict"
		out.Intensity = math.Max(out.Intensity, math.Max(transient.Anger, math.Max(transient.Hurt, transient.Disappointment)))
		out.PolicyReason = "persistent_relationship_emotion"
		out.PauseBeforeMS = 130
	}
	if transient.Concern >= 0.35 {
		out.Concern = math.Max(out.Concern, transient.Concern)
	}
	if affect.Severity >= 0.55 || affect.Stress >= 0.65 || affect.Need == "comfort" || affect.Need == "gentle_check" {
		persistedAnger := out.AngerDisplay
		persistedHurt := out.HurtDisplay
		out.Primary = "restrained_concern"
		out.Concern = math.Max(out.Concern, 0.45+math.Max(affect.Severity, affect.Stress)*0.45)
		out.AngerDisplay = persistedAnger * (0.18 + 0.22*(1-clamp01(affect.Severity)))
		out.HurtDisplay = persistedHurt * (0.45 + 0.25*(1-clamp01(affect.Severity)))
		out.Warmth = math.Max(out.Warmth, 0.44+clamp01(affect.Severity)*0.18)
		out.Coldness *= 0.45
		out.Pace -= 0.10
		out.Energy -= 0.08
		out.Smile = 0
		out.PauseBeforeMS = 180
		out.PolicyReason = "user_condition_priority_without_state_reset"
		if persistedHurt >= 0.30 {
			out.Secondary = "residual_hurt"
		} else if persistedAnger >= 0.30 {
			out.Secondary = "residual_anger"
		}
	}
	out.Intensity = clamp01(out.Intensity)
	out.Warmth = clamp01(out.Warmth)
	out.Coldness = clamp01(out.Coldness)
	out.Energy = clamp01(out.Energy)
	out.Pace = clamp01(out.Pace)
	out.Concern = clamp01(out.Concern)
	out.AngerDisplay = clamp01(out.AngerDisplay)
	out.HurtDisplay = clamp01(out.HurtDisplay)
	out.Smile = clamp01(out.Smile)
	out.Evidence = append(out.Evidence, signals.Evidence...)
	return out
}

func BuildContext(psycheState *psyche.PsycheState, relation *relationship.RelationshipState, state *State, schedulePrompt string) *Context {
	if state == nil {
		state = defaultState("", "", time.Now().UTC())
	}
	current := Compose(psycheState, relation, state.RelationshipEmotion, state.UserAffect, state.Signals)
	ctx := &Context{
		UserAffect:          state.UserAffect,
		RelationshipEmotion: state.RelationshipEmotion,
		Signals:             state.Signals,
		Baseline:            state.Baseline,
		CurrentExpression:   current,
		SchedulePrompt:      strings.TrimSpace(schedulePrompt),
	}
	ctx.Prompt = buildPrompt(ctx)
	ctx.VoiceInstruction = buildVoiceInstruction(current)
	return ctx
}

func buildPrompt(ctx *Context) string {
	if ctx == nil {
		return ""
	}
	affect := ctx.UserAffect
	expression := ctx.CurrentExpression
	parts := []string{
		"【当前情绪与表达状态】",
		fmt.Sprintf("角色外显主导情绪：%s，强度 %.2f", expression.Primary, expression.Intensity),
		fmt.Sprintf("外显温度：温暖 %.2f，冷淡 %.2f，关心 %.2f", expression.Warmth, expression.Coldness, expression.Concern),
		fmt.Sprintf("表达节奏：语速 %.2f，起始停顿 %dms，能量 %.2f", expression.Pace, expression.PauseBeforeMS, expression.Energy),
		fmt.Sprintf("用户当前情绪：%s，强度 %.2f，压力 %.2f，需求 %s，置信度 %.2f", affect.PrimaryEmotion, affect.Intensity, affect.Stress, affect.Need, affect.Confidence),
	}
	if affect.SecondaryEmotion != "" {
		parts = append(parts, "用户次要情绪："+affect.SecondaryEmotion)
	}
	if affect.PossibleConcealment {
		parts = append(parts, "用户可能在隐藏真实感受，优先温和确认，不要直接拆穿。")
	}
	if expression.AngerDisplay >= 0.3 || expression.HurtDisplay >= 0.3 {
		parts = append(parts, fmt.Sprintf("底层关系情绪仍有愤怒 %.2f、受伤 %.2f，可以被当前用户状态暂时压住，但不得清零。", expression.AngerDisplay, expression.HurtDisplay))
	}
	if ctx.SchedulePrompt != "" {
		parts = append(parts, ctx.SchedulePrompt)
	}
	if signals := formatSignals(ctx.Signals); signals != "" {
		parts = append(parts, signals)
	}
	parts = append(parts, "这些字段只用于决定表达，不得向用户复述。")
	return strings.Join(parts, "\n")
}

func buildVoiceInstruction(current CurrentExpression) string {
	plan := interaction.ExpressionPlan{
		Version: interaction.ExpressionPlanVersionV1,
		Tones:   []interaction.ExpressionTone{voiceTone(current)},
		EmotionPresentation: []interaction.EmotionPresentation{{
			Kind:      voiceEmotionKind(current),
			Intensity: current.Intensity,
			Mode:      "display",
		}},
	}
	params := expression.MapExpressionToVoice(plan)
	return fmt.Sprintf("%s；语速 %.2f；停顿 %.2f；情绪强度 %.2f", current.Primary, params.Speed, params.Pause, current.Intensity)
}

func voiceTone(current CurrentExpression) interaction.ExpressionTone {
	switch {
	case current.Warmth >= 0.65 || current.Concern >= 0.5:
		return interaction.ExpressionToneWarm
	case current.Smile >= 0.6:
		return interaction.ExpressionTonePlayful
	case current.Coldness >= 0.55:
		return interaction.ExpressionToneReserved
	case current.HurtDisplay >= 0.35 || current.AngerDisplay >= 0.35:
		return interaction.ExpressionToneRepairing
	default:
		return interaction.ExpressionToneRational
	}
}

func voiceEmotionKind(current CurrentExpression) string {
	switch {
	case current.Concern >= 0.5:
		return "concern"
	case current.Warmth >= 0.65:
		return "affection"
	case current.Smile >= 0.6:
		return "playful"
	case current.Coldness >= 0.55:
		return "reserved"
	case current.HurtDisplay >= 0.35:
		return "hurt"
	case current.AngerDisplay >= 0.35:
		return "anger"
	default:
		return "neutral"
	}
}

func formatSignals(signals Signals) string {
	var parts []string
	if math.Abs(signals.VolumeDeviation) >= 0.25 {
		parts = append(parts, fmt.Sprintf("音量相对个人基线变化 %.0f%%", signals.VolumeDeviation*100))
	}
	if math.Abs(signals.SpeechRateDeviation) >= 0.25 {
		parts = append(parts, fmt.Sprintf("语速相对个人基线变化 %.0f%%", signals.SpeechRateDeviation*100))
	}
	if signals.ResponseLatencyMS >= 1200 {
		parts = append(parts, fmt.Sprintf("用户本轮回应前停顿约 %dms", signals.ResponseLatencyMS))
	}
	if len(parts) == 0 {
		return ""
	}
	return "本地语音行为信号：" + strings.Join(parts, "，") + "。"
}
