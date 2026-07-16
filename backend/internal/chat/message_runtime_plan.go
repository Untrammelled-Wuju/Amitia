// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"strings"

	"github.com/u-ai/backend/internal/interaction"
)

func buildBehaviorPlanFromRuntime(runtime *interaction.RuntimeAssembly) string {
	if runtime == nil {
		return ""
	}
	if runtime.BehaviorPlan == nil {
		return buildMinimalRuntimeContextPrompt(runtime)
	}
	bp, lines := runtime.BehaviorPlan, []string{}
	if bp.Intent != "" {
		lines = append(lines, "意图: "+bp.Intent)
	}
	if bp.Strategy != "" {
		lines = append(lines, "策略: "+bp.Strategy)
	}
	if bp.ResponseGoal != "" {
		lines = append(lines, "回复目标: "+bp.ResponseGoal)
	}
	if bp.ToneHint != "" {
		lines = append(lines, "语气提示: "+bp.ToneHint)
	}
	if len(bp.AllowedTopics) > 0 {
		lines = append(lines, "允许话题: "+strings.Join(bp.AllowedTopics, " / "))
	}
	if len(bp.ForbiddenTopics) > 0 {
		lines = append(lines, "禁止话题: "+strings.Join(bp.ForbiddenTopics, " / "))
	}
	if bp.Priority != "" {
		lines = append(lines, "优先级: "+string(bp.Priority))
	}
	if bp.SafetyLevel != "" {
		lines = append(lines, "安全级别: "+string(bp.SafetyLevel))
	}
	return strings.Join(lines, "\n")
}
func buildMinimalRuntimeContextPrompt(runtime *interaction.RuntimeAssembly) string {
	lines := []string{}
	if runtime.Path != "" {
		lines = append(lines, "路径: "+string(runtime.Path))
	}
	if len(runtime.Safety.Reasons) > 0 {
		lines = append(lines, "安全因素: "+strings.Join(runtime.Safety.Reasons, ", "))
	}
	return strings.Join(lines, "\n")
}
func buildExpressionPlanFromRuntime(runtime *interaction.RuntimeAssembly) string {
	if runtime == nil || runtime.ExpressionPlan == nil {
		return ""
	}
	ep, lines := runtime.ExpressionPlan, []string{"【回复约束 - 必须遵守】"}
	switch ep.Length {
	case "short":
		lines = append(lines, "回复长度: 短（1-3句话，不超过80字）")
	case "medium":
		lines = append(lines, "回复长度: 中（3-6句话）")
	case "long":
		lines = append(lines, "回复长度: 长（5句以上，可充分展开）")
	default:
		lines = append(lines, "回复长度: 适中")
	}
	switch ep.Tone {
	case "warm":
		lines = append(lines, "语气: 温暖亲切，用词柔和有温度")
	case "neutral":
		lines = append(lines, "语气: 中性平和，客观自然")
	case "firm":
		lines = append(lines, "语气: 坚定明确，态度清晰")
	case "soft":
		lines = append(lines, "语气: 柔和克制，避免强烈表达")
	case "playful":
		lines = append(lines, "语气: 俏皮活泼，可适当幽默")
	case "concerned":
		lines = append(lines, "语气: 关切体贴，表达在意和关注")
	default:
		lines = append(lines, "语气: 自然适中")
	}
	if ep.EmotionIntensity > 0 {
		label := "高（充分表达情绪感受）"
		if ep.EmotionIntensity < 0.3 {
			label = "低（克制冷静，减少情绪化表达）"
		} else if ep.EmotionIntensity < 0.6 {
			label = "中（适度表达情绪）"
		}
		lines = append(lines, "情绪表达强度: "+label)
	}
	switch ep.ExpressionType {
	case "question":
		lines = append(lines, "表达类型: 提问 - 应在回复中包含追问以推进对话")
	case "statement":
		lines = append(lines, "表达类型: 陈述 - 以提供信息和建议为主")
	case "greeting":
		lines = append(lines, "表达类型: 问候 - 以打招呼和寒暄为主")
	case "boundary":
		lines = append(lines, "表达类型: 设立边界 - 明确表达底线，不展开")
	case "silence":
		lines = append(lines, "表达类型: 沉默 - 尽量简短或不回复实质内容")
	}
	if ep.Suppressed {
		lines = append(lines, "表达抑制: 已启用 - 整体表达应该更加克制和收敛")
	}
	lines = append(lines, "内容密度: 每次回复必须包含有效信息，不使用无意义附和", "结论优先: 对技术/项目/代码/架构/审计/方案类问题，先给明确结论再解释", "禁用默认开头: 不得使用\"嗯嗯\"\"你说得对\"\"这个想法挺好\"\"我理解你\"\"确实可以\"\"稍微优化一下\"作为默认开头或万能缓冲句", "可以反驳: 允许指出用户错误和不完整，不默认认同用户")
	return strings.Join(lines, "\n")
}
