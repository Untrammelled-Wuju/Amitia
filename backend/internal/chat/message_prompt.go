// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"strings"

	"github.com/u-ai/backend/internal/interaction"
	promptir "github.com/u-ai/backend/internal/prompt"
	applog "github.com/u-ai/backend/log"
)

type processPromptInput struct {
	BaseIdentity, CharacterConfig, PersonalityConfig, PersonalityRaw, ProfileContext, MemoryContext, Worldbook, PluginContext, EmotionFusionRaw, AdultIntimacyRaw, MemoryInjectRaw, AntiRepeatRaw                       string
	History                                                                                                                                                                                                             []map[string]string
	Runtime                                                                                                                                                                                                             *interaction.RuntimeAssembly
	StyleInstruction, ProactiveScene, ProactiveTimeContext, ProactiveRecentContext, ProactivePersonality, ProactiveRelationship, ProactiveEmotion, ProactiveMemory, SystemPrompt, ProactiveTaskInstruction, UserContent string
}

func buildProcessPromptMessages(input processPromptInput) ([]map[string]interface{}, *promptir.PromptTrace) {
	gateway := promptir.NewGateway()
	runtimePlan := buildBehaviorPlanFromRuntime(input.Runtime)
	expressionPlan := buildExpressionPlanFromRuntime(input.Runtime)
	if input.StyleInstruction != "" {
		if expressionPlan != "" {
			expressionPlan += "\n" + input.StyleInstruction
		} else {
			expressionPlan = input.StyleInstruction
		}
	}
	request := promptir.BuildRequest{CharacterConfig: input.CharacterConfig, CompiledPersonality: input.PersonalityConfig, BaseIdentity: input.BaseIdentity, SystemPrompt: input.SystemPrompt, PersonalityRaw: input.PersonalityRaw, EmotionFusionRaw: input.EmotionFusionRaw, AdultIntimacyRaw: input.AdultIntimacyRaw, MemoryInjectRaw: input.MemoryInjectRaw, AntiRepeatRaw: input.AntiRepeatRaw, ProfileContext: input.ProfileContext, MemoryContext: input.MemoryContext, Worldbook: input.Worldbook, PluginContext: input.PluginContext, RuntimePlan: runtimePlan, ExpressionPlan: expressionPlan, History: renderHistoryForPromptIR(input.History), CurrentUserInput: input.UserContent, ProactiveTaskInstruction: input.ProactiveTaskInstruction, ProactiveScene: input.ProactiveScene, ProactiveTimeContext: input.ProactiveTimeContext, ProactiveRecentContext: input.ProactiveRecentContext, ProactivePersonality: input.ProactivePersonality, ProactiveRelationship: input.ProactiveRelationship, ProactiveEmotion: input.ProactiveEmotion, ProactiveMemory: input.ProactiveMemory}
	gwMessages, promptTrace, err := gateway.BuildMessages(request)
	if err != nil {
		applog.Warn("prompt gateway build failed, trying minimal build", applog.Fields{"error": err.Error()})
		request.ProfileContext, request.MemoryContext, request.Worldbook, request.PluginContext, request.History, request.ProactiveScene, request.ProactiveTimeContext, request.ProactiveRecentContext = "", "", "", "", "", "", "", ""
		gwMessages, promptTrace, err = gateway.BuildMessages(request)
		if err != nil {
			applog.Warn("minimal prompt build also failed, falling back to raw", applog.Fields{"error": err.Error()})
			safeContent := promptir.SanitizeCurrentUserMessage(input.UserContent)
			return []map[string]interface{}{{"role": "user", "content": "<current_user_message>\n" + safeContent + "\n</current_user_message>"}}, nil
		}
	}
	messages := make([]map[string]interface{}, 0, len(gwMessages))
	for _, message := range gwMessages {
		messages = append(messages, map[string]interface{}{"role": message.Role, "content": message.Content})
	}
	return messages, promptTrace
}

func renderHistoryForPromptIR(history []map[string]string) string {
	lines := make([]string, 0, len(history))
	for _, message := range history {
		role, content := strings.TrimSpace(message["role"]), strings.TrimSpace(message["content"])
		if content == "" {
			continue
		}
		if role == "" {
			role = "unknown"
		}
		lines = append(lines, role+": "+content)
	}
	return strings.Join(lines, "\n")
}
