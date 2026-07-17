package prompt

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
