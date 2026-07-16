package prompt

func appendCharacterSections(ctx *buildContext) {
	req := ctx.req
	if req.BaseIdentity != "" {
		ctx.appendSection("base_identity", GwSectionBaseIdentity, TrustTrusted, ModeAuthoritative, "base_identity", 880, req.BaseIdentity, "GwSectionBaseIdentity")
	}
	if req.SystemPrompt != "" {
		ctx.appendSection("system_prompt", GwSectionSystemPrompt, TrustSemiTrusted, ModeAuthoritative, "system_prompt", 840, "【角色专属提示词 - 最高优先级角色指令】"+"\n"+req.SystemPrompt, "GwSectionSystemPrompt")
	}
	if req.CharacterConfig != "" || req.CompiledPersonality != "" {
		ctx.appendSection("character_contract", GwSectionCharacterContract, TrustTrusted, ModeAuthoritative, "character_config", 800, req.CharacterConfig+"\n"+req.CompiledPersonality, "GwSectionCharacterContract")
	}
	if req.PersonalityRaw != "" && ctx.flags.PersonalityRawEnabled {
		ctx.appendSection("personality_raw", GwSectionPersonalityRaw, TrustSemiTrusted, ModeStyle, "personality", 780, req.PersonalityRaw, "GwSectionPersonalityRaw")
	}
	if req.EmotionFusionRaw != "" && ctx.flags.EmotionFusionEnabled {
		ctx.appendSection("emotion_fusion_raw", GwSectionEmotionFusionRaw, TrustSemiTrusted, ModeStyle, "emotion", 760, req.EmotionFusionRaw, "GwSectionEmotionFusionRaw")
	}
	if req.AdultIntimacyRaw != "" && ctx.flags.IntimacyDefaultEnabled {
		ctx.appendSection("adult_intimacy_raw", GwSectionAdultIntimacyRaw, TrustSemiTrusted, ModeStyle, "intimacy", 740, req.AdultIntimacyRaw, "GwSectionAdultIntimacyRaw")
	}
	if req.RuntimePlan != "" {
		ctx.appendSection("runtime_plan", GwSectionRuntimePlan, TrustSemiTrusted, ModeRuntime, "runtime", 700, req.RuntimePlan, "GwSectionRuntimePlan")
	}
	if req.ExpressionPlan != "" {
		ctx.appendSection("expression_plan", GwSectionExpressionPlan, TrustSemiTrusted, ModeRuntime, "runtime", 650, req.ExpressionPlan, "GwSectionExpressionPlan")
	}
}
