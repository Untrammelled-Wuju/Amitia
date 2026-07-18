package prompt

import "strings"

const relationshipTimePolicy = "关系时间规则：当前任务优先；不得责备用户、索取离线解释或暗示用户亏欠；不得把自然离线解释为疏离、背叛或关系受损。"

func appendContextSections(ctx *buildContext) {
	req := ctx.req
	if req.AgentSkillContext != "" {
		ctx.appendSection("agent_skill_instructions", GwSectionAgentSkillInstructions, TrustSemiTrusted, ModeRuntime, "agent-skill-host", 325, req.AgentSkillContext, "GwSectionAgentSkillInstructions")
	}
	if req.MemoryInjectRaw != "" && ctx.flags.MemoryRawEnabled {
		ctx.appendSection("memory_inject_raw", GwSectionMemoryInjectRaw, TrustUntrusted, ModeDataOnly, "memory", 350, req.MemoryInjectRaw, "GwSectionMemoryInjectRaw")
	}
	if req.MemoryExtractRaw != "" && ctx.flags.MemoryRawEnabled {
		ctx.appendSection("memory_extract_raw", GwSectionMemoryExtractRaw, TrustUntrusted, ModeDataOnly, "memory", 340, req.MemoryExtractRaw, "GwSectionMemoryExtractRaw")
	}
	appendDataOnly(ctx, "profile_context", GwSectionProfileContext, "profile", req.ProfileContext, "GwSectionProfileContext")
	if req.TemporalContext != "" {
		ctx.sections = append(ctx.sections, GwSection{Enabled: true, ID: "temporal_context", Type: GwSectionTemporalContext, TrustLevel: TrustTrusted, InstructionMode: ModeDataOnly, Source: "temporal-runtime", Priority: 430, TokenBudget: 220, Content: req.TemporalContext, SourceProject: "prompt", SourceFile: "builder_context.go", SourceConstant: "GwSectionTemporalContext"})
	}
	if relationshipTime := strings.TrimSpace(req.RelationshipTimeContext); relationshipTime != "" {
		ctx.sections = append(ctx.sections, GwSection{Enabled: true, ID: "relationship_time", Type: GwSectionRelationshipTime, TrustLevel: TrustTrusted, InstructionMode: ModeDataOnly, Source: "temporal-runtime", Priority: 440, TokenBudget: 160, Content: relationshipTimePolicy + "\n" + relationshipTime, SourceProject: "prompt", SourceFile: "builder_context.go", SourceConstant: "GwSectionRelationshipTime"})
	}
	appendDataOnly(ctx, "memory_context", GwSectionMemoryContext, "memory", req.MemoryContext, "GwSectionMemoryContext")
	appendDataOnly(ctx, "worldbook_context", GwSectionWorldbookContext, "worldbook", req.Worldbook, "GwSectionWorldbookContext")
	appendDataOnly(ctx, "plugin_context", GwSectionPluginContext, "plugin", req.PluginContext, "GwSectionPluginContext")
	appendDataOnly(ctx, "conversation_history", GwSectionConversationHistory, "history", req.History, "GwSectionConversationHistory")
	appendDataOnly(ctx, "tool_result", GwSectionToolResult, "tool", req.ToolResults, "GwSectionToolResult")
	appendDataOnly(ctx, "multimodal_text", GwSectionMultimodalText, "multimodal", req.MultimodalText, "GwSectionMultimodalText")
}

func appendDataOnly(ctx *buildContext, id string, typ GwSectionType, source, content, constant string) {
	if content != "" {
		ctx.appendSection(id, typ, TrustUntrusted, ModeDataOnly, source, 300, content, constant)
	}
}
