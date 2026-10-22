package chat

import (
	"context"
	"fmt"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/expression"
	"github.com/u-ai/backend/internal/extension/runtimegate"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/personality"
	promptir "github.com/u-ai/backend/internal/prompt"
	"strings"
)

func ownedExpressionPrompt(ctx context.Context, inference business.Inference, role *character.RoleRuntimeProfile) ([]string, []string) {
	if role == nil {
		return nil, nil
	}
	runtime, _ := interaction.RuntimeFromContext(ctx)
	preset := role.PersonalityPresetID()
	template := personality.CompilePersonalityTemplate(role.Name, preset, role.Gender)
	parts := []string{promptir.BuildPersonalityRawSection(role.Name, role.Gender, template), promptir.BuildAntiRepeatRawSection()}
	dynamic := []string{buildAdultIntimacyRaw(runtime, preset, inference.Message)}
	channel := "web"
	source := "manual"
	if inference.Ingress != nil {
		if inference.Ingress.Channel != "" {
			channel = inference.Ingress.Channel
		}
		if inference.Ingress.Source != "" {
			source = inference.Ingress.Source
		}
	}
	if inference.ScheduledMessage {
		source = "proactive"
	}
	kind := expression.ChannelKind(channel)
	if kind == "" {
		kind = expression.ChannelWeb
	}
	compiled := expression.CompileChannelPrompt(kind)
	parts = append(parts, compiled.SystemInstruction, compiled.StyleInstruction)
	appendCompiledPersonality(runtime, &dynamic)
	if source == "proactive" {
		aff := 0.5
		if runtime != nil && runtime.Appraisal != nil {
			if runtime.Appraisal.RelationshipDelta > 0 {
				aff = 0.3 + runtime.Appraisal.RelationshipDelta*0.7
			} else {
				aff = 0.3
			}
		}
		if runtime != nil && runtime.Context.Psyche.Status == interaction.LoadStatusReady && runtime.Context.Psyche.Value.Stress > 0.7 {
			aff *= 0.4
		}
		aff = min(aff, 1.0)
		proactive := personality.BuildProactivePersonalityBlock(role.Name, preset, role.Gender, aff, false)
		dynamic = append(dynamic, promptir.ProactiveSceneSection(), promptir.ProactivePersonalityInstructionHeader()+"\n\n"+proactive+"\n\n"+promptir.ProactivePersonalityBoundarySection(), buildProactiveEmotionFromPsyche(runtime))
	}
	filtered := parts[:0]
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			filtered = append(filtered, part)
		}
	}
	return filtered, dynamic
}

func ownedExpressionBlocked(ctx context.Context, inference business.Inference) (bool, error) {
	runtime, ok := interaction.RuntimeFromContext(ctx)
	if !ok || runtime == nil {
		return false, nil
	}
	bound := len(inference.Workspaces) > 0
	if runtime.ExpressionPlan != nil && (runtime.ExpressionPlan.SafetyBlocked || runtime.ExpressionPlan.DoNotSend && !bound) {
		return true, nil
	}
	if runtime.BehaviorPlan != nil && !bound {
		directive, err := decision.BuildActionDirective(runtime.BehaviorPlan)
		if err != nil {
			return false, err
		}
		if directive.Kind == decision.ActionDirectiveWait {
			return true, nil
		}
	}
	return false, nil
}

func requestedReasoningEnabled(supported bool, requested *bool) bool {
	if !supported {
		return false
	}
	if requested == nil {
		return true
	}
	return *requested
}

func resolveExpressionChannel(channel string, voiceMessage bool) expression.ChannelKind {
	if voiceMessage {
		return expression.ChannelVoice
	}
	return expression.ChannelWeb
}

func applyExpressionLengthLimit(reply string, rt *interaction.RuntimeAssembly) string {

	if rt == nil || rt.ExpressionPlan == nil {
		return reply
	}
	ep := rt.ExpressionPlan
	switch ep.Length {
	case "short":
		return truncateToRuneLimit(reply, 120)
	case "medium":
		return truncateToRuneLimit(reply, 350)
	case "long":
		return truncateToRuneLimit(reply, 800)
	default:
		return reply
	}
}

func proactiveScene(source string) string {
	if source == "proactive" {
		return promptir.ProactiveSceneSection()
	}
	return ""
}

func mergeContext(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "\n\n" + b
}

func renderPluginContributions(contributions []ContextContribution) (string, []string) {
	parts := make([]string, 0, len(contributions))
	sources := make([]string, 0, len(contributions))
	for _, contribution := range contributions {
		parts = append(parts, "来源: "+contribution.Source+"\n"+contribution.Content)
		sources = append(sources, contribution.Source)
	}
	return strings.Join(parts, "\n\n"), sources
}

func truncateToRuneLimit(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}

func buildAdultIntimacyRaw(runtime *interaction.RuntimeAssembly, personalityPresetID, userMessage string) string {
	label := ""
	if input := buildEmotionFusionInput(runtime); input != nil {
		label = input.PrimaryLabel
	}

	gate := promptir.IntimacyGate(userMessage, label)
	switch gate {
	case "hard_stop":
		return ""
	case "rejection":
		return promptir.BuildIntimacyDowngradeSection()
	case "blocked_emotion":
		return ""
	}

	return promptir.BuildAdultIntimacyDefaultSection(personalityPresetID)
}

func buildProactiveEmotionFromPsyche(runtime *interaction.RuntimeAssembly) string {
	if !runtimegate.IsEnabled(runtimegate.EmotionExtensionID) {
		return ""
	}
	if runtime == nil || runtime.Context.Psyche.Status != interaction.LoadStatusReady {
		return ""
	}
	p := runtime.Context.Psyche.Value
	return fmt.Sprintf("情绪：{\"valence\":%.2f,\"arousal\":%.2f,\"dominance\":%.2f}\n心情：{\"moodValence\":%.2f,\"moodArousal\":%.2f}\n压力：%.0f\n精力：%.0f",
		p.Valence, p.Arousal, p.Dominance, p.MoodValence, p.MoodArousal, p.Stress*100, (1-p.Fatigue)*100)
}
