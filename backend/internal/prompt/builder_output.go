package prompt

func appendSanitizerSections(ctx *buildContext) {
	req := ctx.req
	if req.OutputShapeRaw != "" && ctx.flags.ReplySanitizerEnabled {
		ctx.appendSection("output_shape_raw", GwSectionOutputShapeRaw, TrustTrusted, ModeAuthoritative, "sanitizer", 600, req.OutputShapeRaw, "GwSectionOutputShapeRaw")
	}
	if req.AntiRepeatRaw != "" && ctx.flags.ReplySanitizerEnabled {
		ctx.appendSection("anti_repeat_raw", GwSectionAntiRepeatRaw, TrustTrusted, ModeAuthoritative, "sanitizer", 590, req.AntiRepeatRaw, "GwSectionAntiRepeatRaw")
	}
	if req.ChannelShortRaw != "" && ctx.flags.TextlibRawEnabled {
		ctx.appendSection("channel_short_raw", GwSectionChannelShortRaw, TrustTrusted, ModeAuthoritative, "textlib", 580, req.ChannelShortRaw, "GwSectionChannelShortRaw")
	}
}

func appendUserAndTraceSections(ctx *buildContext) {
	content := ctx.req.CurrentUserInput
	ctx.appendSection("current_user_message", GwSectionCurrentUserMessage, TrustUntrusted, ModeUserRequest, "user", 100, content, "GwSectionCurrentUserMessage")
	if ctx.req.TraceOnly != "" {
		ctx.appendSection("trace_only", GwSectionTraceOnly, TrustUntrusted, ModeDataOnly, "trace", 10, ctx.req.TraceOnly, "GwSectionTraceOnly")
	}
}
