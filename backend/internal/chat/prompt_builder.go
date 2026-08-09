// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/expression"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/pkg/app"

	"strings"
)

func (s *service) compiledSystemInstruction(channel string) string {
	kind := expression.ChannelKind(strings.ToLower(channel))
	if channel == "" {
		kind = expression.ChannelWeb
	}
	cp := expression.CompileChannelPrompt(kind)
	return cp.SystemInstruction
}

func (s *service) compiledChannelPrompt(channel string) expression.CompiledPrompt {
	kind := expression.ChannelKind(strings.ToLower(channel))
	if channel == "" {
		kind = expression.ChannelWeb
	}
	return expression.CompileChannelPrompt(kind)
}

type sys1Result struct {
	CharacterConfig     string
	ProfileContext      string
	EpisodicContext     string
	Worldbook           string
	PersonalityPresetID string
}

func (s *service) sys1Builder(profile *character.RoleRuntimeProfile, userMessage string, runtime *interaction.RuntimeAssembly) sys1Result {
	parts := buildRoleSystemParts(profile, runtime)
	characterID := ""
	if profile != nil {
		characterID = strings.TrimSpace(profile.CharacterID)
	}
	var profileCtx, epiCtx, wbCtx string
	if s.profileSvc != nil {
		profilePrompt := s.profileSvc.ToSystemPrompt(characterID, characterID)
		if profilePrompt != "" {
			profileCtx = profilePrompt
		}
	}
	if s.episodicSvc != nil {
		epiPrompt := s.episodicSvc.ToSystemPrompt(characterID)
		if epiPrompt != "" {
			epiCtx = epiPrompt
		}
	}
	if s.worldBookSvc != nil {
		wbPrompt := s.worldBookSvc.ToSystemPrompt(userMessage, "")
		if wbPrompt != "" {
			wbCtx = wbPrompt
		}
	}

	presetID := profile.PersonalityPresetID()

	return sys1Result{
		CharacterConfig:     strings.Join(parts, "\n\n"),
		ProfileContext:      profileCtx,
		EpisodicContext:     epiCtx,
		Worldbook:           wbCtx,
		PersonalityPresetID: presetID,
	}
}

type sys2Result struct {
	SystemInstruction string
	MemoryContext     string
	MemoryInjectRaw   string
}

func (s *service) sys2Builder(convID, charID, requestID, channel, userMessage string) sys2Result {
	sysInstruction := s.compiledSystemInstruction(channel)
	var internalParts []string
	var memoryInjectRaw string
	workingSummary := ""
	if s.stateProvider != nil {
		state := s.stateProvider.GetState(convID)
		if state != nil && state.LastInteractionSummary != "" {
			workingSummary = state.LastInteractionSummary
		}
	}
	if workingSummary == "" && s.wmCache != nil {
		wm := s.wmCache.Get(convID)
		if wm != nil && wm.State != nil && wm.State.Summary != "" {
			workingSummary = wm.State.Summary
		}
	}
	if workingSummary != "" {
		internalParts = append(internalParts, "【工作记忆】\n"+workingSummary)
	}
	if s.compressor != nil {
		status := s.compressor.GetCompressionStatus(convID)
		if summary, ok := status["latestSummary"].(string); ok && summary != "" {
			internalParts = append(internalParts, "【对话历史摘要】\n"+summary)
		}
	}
	if s.memorySvc != nil && userMessage != "" {
		results, err := s.memorySvc.HybridSearch(&memory.VectorSearchRequest{
			Query:          userMessage,
			CharacterID:    charID,
			ConversationID: convID,
			RequestID:      requestID,
			Channel:        channel,
			Limit:          8,
		})
		if err == nil && len(results) > 0 {
			layerLines := map[string][]string{}
			layerOrder := []string{"当前摘要", "用户画像", "情景回忆", "事实记忆"}
			for _, r := range results {
				layer := r.MemoryLayer
				if layer == "" {
					layer = "事实记忆"
				}
				typeLabel := r.Memory.MemoryType
				if typeLabel == "" {
					typeLabel = "fact"
				}
				line := fmt.Sprintf("- [%s %s %.0f%% 置信%d%%] %s", typeLabel, r.MatchType, r.Score*100, r.Memory.Confidence, r.Memory.Value)
				layerLines[layer] = append(layerLines[layer], line)
			}
			for _, layer := range layerOrder {
				if lines := layerLines[layer]; len(lines) > 0 {
					internalParts = append(internalParts, "【"+layer+"】\n"+strings.Join(lines, "\n"))
				}
			}
			memoryInjectRaw = s.buildMemoryInjectItems(results)
			for _, r := range results {
				go s.memorySvc.RecordUse(r.Memory.ID)
			}
		}
	}
	return sys2Result{
		SystemInstruction: sysInstruction,
		MemoryContext:     strings.Join(internalParts, "\n\n"),
		MemoryInjectRaw:   memoryInjectRaw,
	}
}

func (s *service) rewriteQueryForSearch(userMessage string) string {
	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		return userMessage
	}
	prompt := []map[string]interface{}{
		{"role": "system", "content": "把用户输入转成用于记忆检索的简洁关键词，去除语气词和寒暄，直接输出关键词不要解释。如果输入已经是简短的关键词则原样返回。"},
		{"role": "user", "content": userMessage},
	}
	rewritten, _, err := s.callLLM(context.Background(), cfg, prompt)
	if err != nil || rewritten == "" {
		return userMessage
	}
	rewritten = strings.TrimSpace(rewritten)
	if len([]rune(rewritten)) < 2 {
		return userMessage
	}
	return rewritten
}

func shouldRetrieveMemory(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if len([]rune(trimmed)) < 4 {
		return false
	}
	greetings := []string{"嗯", "好", "哦", "啊", "哈", "嗨", "喂", "在吗", "在不在", "好的", "好吧", "行", "可以", "知道了", "明白了", "懂了", "嗯嗯", "哈哈", "呵呵", "嘿嘿", "谢谢", "多谢", "再见", "拜拜", "晚安", "早安", "早上好", "晚上好", "ok", "OK", "Ok", "hi", "Hi", "hello", "Hello", "bye", "Bye"}
	lower := strings.ToLower(trimmed)
	for _, g := range greetings {
		if lower == strings.ToLower(g) {
			return false
		}
	}
	return true
}

func buildRoleSystemParts(profile *character.RoleRuntimeProfile, runtime *interaction.RuntimeAssembly) []string {
	parts := []string{}
	if profile == nil {
		return parts
	}
	identity := profile.Identity
	if identity == "" {
		identity = "一个AI伙伴"
	}
	parts = append(parts, fmt.Sprintf("你是%s，%s。", profile.Name, identity))
	if profile.Personality != "" {
		parts = append(parts, "【角色性格】\n"+profile.Personality)
	}
	if profile.SpeakingStyle != "" {
		parts = append(parts, "【聊天风格】\n"+profile.SpeakingStyle)
	}
	if profile.RelationshipStyle != "" {
		parts = append(parts, "【关系风格】\n"+profile.RelationshipStyle)
	}
	if profile.BoundaryRules != "" {
		parts = append(parts, "【场景规则】\n"+profile.BoundaryRules)
	}
	appendCompiledPersonality(runtime, &parts)
	appendRuntimeConfig := func(label string, data map[string]interface{}) {
		if len(data) == 0 {
			return
		}
		if len(data) == 1 {
			if _, ok := data["version"]; ok {
				return
			}
		}
		raw, err := json.Marshal(data)
		if err == nil && string(raw) != "{}" {
			parts = append(parts, label+"\n"+string(raw))
		}
	}
	appendRuntimeConfig("【性格配置】", profile.PersonalityConfig)
	appendRuntimeConfig("【对话风格配置】", profile.ChatStyleConfig)
	appendRuntimeConfig("【场景配置】", profile.SceneRules)
	return parts
}

func appendCompiledPersonality(runtime *interaction.RuntimeAssembly, parts *[]string) {
	if runtime == nil || runtime.Personality == nil {
		return
	}
	summary := runtime.Personality.ToPersonalitySummary()
	if summary != "" {
		*parts = append(*parts, "【性格行为指令 - 编译自滑块配置】\n"+summary)
	}

	if runtime.BehaviorPlan != nil {
		bpSummary := renderBehaviorPlanSummary(runtime.BehaviorPlan)
		if bpSummary != "" {
			*parts = append(*parts, "【本轮行为策略】\n"+bpSummary)
		}
	}

	if runtime.ExpressionPlan != nil {
		epSummary := renderExpressionPlanSummary(runtime.ExpressionPlan)
		if epSummary != "" {
			*parts = append(*parts, "【本轮表达约束】\n"+epSummary)
		}
	}
}

func renderBehaviorPlanSummary(plan *decision.BehaviorPlan) string {
	if plan == nil {
		return ""
	}
	var lines []string

	if plan.Intent != "" {
		lines = append(lines, "- 意图: "+string(plan.Intent))
	}
	if plan.Strategy != "" {
		lines = append(lines, "- 策略: "+string(plan.Strategy))
	}
	if plan.ResponseGoal != "" {
		lines = append(lines, "- 回复目标: "+plan.ResponseGoal)
	}
	if plan.ToneHint != "" {
		lines = append(lines, "- 语气提示: "+string(plan.ToneHint))
	}
	if len(plan.PlanContentPolicy.AllowedTopics) > 0 {
		lines = append(lines, "- 允许话题: "+strings.Join(plan.PlanContentPolicy.AllowedTopics, "、"))
	}
	if len(plan.PlanContentPolicy.ForbiddenTopics) > 0 {
		lines = append(lines, "- 禁止话题: "+strings.Join(plan.PlanContentPolicy.ForbiddenTopics, "、"))
	}
	if plan.Priority != "" {
		lines = append(lines, "- 优先级: "+string(plan.Priority))
	}
	if plan.SafetyLevel != "" {
		lines = append(lines, "- 安全级别: "+string(plan.SafetyLevel))
	}

	return strings.Join(lines, "\n")
}

func renderExpressionPlanSummary(plan *decision.ExpressionPlan) string {
	if plan == nil {
		return ""
	}
	var lines []string

	if plan.Tone != "" {
		lines = append(lines, "- 语气: "+toneLabel(plan.Tone))
	}
	if plan.Length != "" {
		lines = append(lines, "- 回复长度: "+string(plan.Length))
	}
	if plan.EmotionIntensity > 0 {
		lines = append(lines, fmt.Sprintf("- 情绪强度: %.0f%%", plan.EmotionIntensity*100))
	}
	if plan.Suppressed {
		lines = append(lines, "- 表达抑制: 是")
	}

	return strings.Join(lines, "\n")
}

func behaviorTagLabel(tag decision.BehaviorTag) string {
	switch tag {
	case decision.BehaviorTagReply:
		return "正常回复"
	case decision.BehaviorTagAskClarify:
		return "请求澄清"
	case decision.BehaviorTagOfferSupport:
		return "提供支持"
	case decision.BehaviorTagSetBoundary:
		return "设立边界"
	case decision.BehaviorTagRepair:
		return "关系修复"
	case decision.BehaviorTagProactiveCheck:
		return "主动关心"
	case decision.BehaviorTagDelay:
		return "延迟/观察"
	default:
		return string(tag)
	}
}

func strategyFromTag(tag decision.BehaviorTag) string {
	switch tag {
	case decision.BehaviorTagReply:
		return "自然回应，保持对话流畅"
	case decision.BehaviorTagAskClarify:
		return "温和追问，帮助澄清模糊内容"
	case decision.BehaviorTagOfferSupport:
		return "提供情感支持或实际帮助"
	case decision.BehaviorTagSetBoundary:
		return "礼貌但坚定地设立边界"
	case decision.BehaviorTagRepair:
		return "修复关系裂痕，重建信任"
	case decision.BehaviorTagProactiveCheck:
		return "主动表达关心和陪伴"
	case decision.BehaviorTagDelay:
		return "保持克制，等待观察"
	default:
		return "保持自然沟通"
	}
}

func toneLabel(tone decision.ExpressionTone) string {
	switch tone {
	case decision.ExpressionToneWarm:
		return "温暖"
	case decision.ExpressionToneNeutral:
		return "中性"
	case decision.ExpressionToneFirm:
		return "坚定"
	case decision.ExpressionToneSoft:
		return "轻柔"
	case decision.ExpressionTonePlayful:
		return "俏皮"
	case decision.ExpressionToneConcerned:
		return "关切"
	default:
		return string(tone)
	}
}

func (s *service) getRoleRuntimeProfile(characterID string) (*character.RoleRuntimeProfile, error) {
	if s.charRepo != nil {
		return s.charRepo.GetRuntimeProfile(characterID)
	}
	return character.NewRepository(app.NewAppContext(s.db, nil)).GetRuntimeProfile(characterID)
}
