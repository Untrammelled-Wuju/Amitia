package prompt

func appendPolicySections(ctx *buildContext) {
	ctx.appendSection("platform_policy", GwSectionPlatformPolicy, TrustTrusted, ModeAuthoritative, "platform", 1000, platformPolicy(), "GwSectionPlatformPolicy")
	ctx.appendSection("app_contract", GwSectionAppContract, TrustTrusted, ModeAuthoritative, "app", 900, appContract(), "GwSectionAppContract")
	ctx.appendSection("cognitive_contract", GwSectionCognitiveContract, TrustTrusted, ModeAuthoritative, "app", 870, cognitiveContract(), "GwSectionCognitiveContract")
	ctx.appendSection("anti_flattery_contract", GwSectionAntiFlatteryContract, TrustTrusted, ModeAuthoritative, "app", 860, antiFlatteryContract(), "GwSectionAntiFlatteryContract")
	ctx.appendSection("technical_task_contract", GwSectionTechnicalTaskContract, TrustTrusted, ModeAuthoritative, "app", 850, technicalTaskContract(), "GwSectionTechnicalTaskContract")
}

func (ctx *buildContext) appendSection(id string, typ GwSectionType, trust TrustLevel, mode InstructionMode, source string, priority int, content, constant string) {
	ctx.sections = append(ctx.sections, GwSection{Enabled: true, ID: id, Type: typ, TrustLevel: trust, InstructionMode: mode, Source: source, Priority: priority, Content: content, SourceProject: "prompt", SourceFile: "builder.go", SourceConstant: constant})
}
