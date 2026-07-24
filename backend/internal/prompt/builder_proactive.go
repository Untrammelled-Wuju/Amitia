package prompt

func appendProactiveSections(ctx *buildContext) {
	req := ctx.req
	if req.ProactiveRaw != "" && ctx.flags.ProactiveRawEnabled {
		ctx.appendSection("proactive_raw", GwSectionProactiveRaw, TrustTrusted, ModeAuthoritative, "proactive", 500, req.ProactiveRaw, "GwSectionProactiveRaw")
	}
	if req.ProactivePersonality != "" && ctx.flags.ProactiveRawEnabled {
		ctx.appendSection("proactive_personality", GwSectionProactivePersonality, TrustSemiTrusted, ModeStyle, "proactive", 490, req.ProactivePersonality, "GwSectionProactivePersonality")
	}
	if req.ProactiveRelationship != "" && ctx.flags.ProactiveRawEnabled {
		ctx.appendSection("proactive_relationship", GwSectionProactiveRelationship, TrustUntrusted, ModeDataOnly, "proactive", 480, req.ProactiveRelationship, "GwSectionProactiveRelationship")
	}
	if req.ProactiveEmotion != "" && ctx.flags.ProactiveRawEnabled {
		ctx.appendSection("proactive_emotion", GwSectionProactiveEmotion, TrustUntrusted, ModeDataOnly, "proactive", 470, req.ProactiveEmotion, "GwSectionProactiveEmotion")
	}
	if req.ProactiveMemory != "" && ctx.flags.ProactiveRawEnabled {
		ctx.appendSection("proactive_memory", GwSectionProactiveMemory, TrustUntrusted, ModeDataOnly, "proactive", 460, req.ProactiveMemory, "GwSectionProactiveMemory")
	}
	if req.ProactiveScene != "" && ctx.flags.ProactiveRawEnabled {
		ctx.appendSection("proactive_scene", GwSectionProactiveScene, TrustTrusted, ModeAuthoritative, "proactive", 450, req.ProactiveScene, "GwSectionProactiveScene")
	}
	if req.TemporalContext == "" && req.ProactiveTimeContext != "" && ctx.flags.ProactiveRawEnabled {
		ctx.appendSection("proactive_time_context", GwSectionProactiveTimeContext, TrustUntrusted, ModeDataOnly, "proactive", 440, req.ProactiveTimeContext, "GwSectionProactiveTimeContext")
	}
	if req.ProactiveRecentContext != "" && ctx.flags.ProactiveRawEnabled {
		ctx.appendSection("proactive_recent_context", GwSectionProactiveRecentContext, TrustUntrusted, ModeDataOnly, "proactive", 430, req.ProactiveRecentContext, "GwSectionProactiveRecentContext")
	}
}
