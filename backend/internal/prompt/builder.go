package prompt

type BuildRequest struct {
	CharacterName       string
	CharacterConfig     string
	CompiledPersonality string
	RuntimePlan         string
	ExpressionPlan      string

	ProfileContext string
	MemoryContext  string
	Worldbook      string
	History        string

	ToolResults    string
	MultimodalText string

	CurrentUserInput string
}

type Builder struct{}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) Build(req BuildRequest) GwIR {
	var sections []GwSection

	sections = append(sections, GwSection{
		ID:              "platform_policy",
		Type:            GwSectionPlatformPolicy,
		TrustLevel:      TrustTrusted,
		InstructionMode: ModeAuthoritative,
		Source:          "platform",
		Priority:        1000,
		Content:         platformPolicy(),
	})

	sections = append(sections, GwSection{
		ID:              "app_contract",
		Type:            GwSectionAppContract,
		TrustLevel:      TrustTrusted,
		InstructionMode: ModeAuthoritative,
		Source:          "app",
		Priority:        900,
		Content:         appContract(),
	})

	sections = append(sections, GwSection{
		ID:              "cognitive_contract",
		Type:            GwSectionCognitiveContract,
		TrustLevel:      TrustTrusted,
		InstructionMode: ModeAuthoritative,
		Source:          "app",
		Priority:        870,
		Content:         cognitiveContract(),
	})

	sections = append(sections, GwSection{
		ID:              "anti_flattery_contract",
		Type:            GwSectionAntiFlatteryContract,
		TrustLevel:      TrustTrusted,
		InstructionMode: ModeAuthoritative,
		Source:          "app",
		Priority:        860,
		Content:         antiFlatteryContract(),
	})

	sections = append(sections, GwSection{
		ID:              "technical_task_contract",
		Type:            GwSectionTechnicalTaskContract,
		TrustLevel:      TrustTrusted,
		InstructionMode: ModeAuthoritative,
		Source:          "app",
		Priority:        850,
		Content:         technicalTaskContract(),
	})

	if req.CharacterConfig != "" || req.CompiledPersonality != "" {
		sections = append(sections, GwSection{
			ID:              "character_contract",
			Type:            GwSectionCharacterContract,
			TrustLevel:      TrustSemiTrusted,
			InstructionMode: ModeStyle,
			Source:          "character_config",
			Priority:        800,
			Content:         req.CharacterConfig + "\n" + req.CompiledPersonality,
		})
	}

	if req.RuntimePlan != "" {
		sections = append(sections, GwSection{
			ID:              "runtime_plan",
			Type:            GwSectionRuntimePlan,
			TrustLevel:      TrustSemiTrusted,
			InstructionMode: ModeRuntime,
			Source:          "runtime",
			Priority:        700,
			Content:         req.RuntimePlan,
		})
	}

	if req.ExpressionPlan != "" {
		sections = append(sections, GwSection{
			ID:              "expression_plan",
			Type:            GwSectionExpressionPlan,
			TrustLevel:      TrustSemiTrusted,
			InstructionMode: ModeRuntime,
			Source:          "runtime",
			Priority:        650,
			Content:         req.ExpressionPlan,
		})
	}

	appendDataOnly := func(id string, typ GwSectionType, source string, content string) {
		if content == "" {
			return
		}
		sections = append(sections, GwSection{
			ID:              id,
			Type:            typ,
			TrustLevel:      TrustUntrusted,
			InstructionMode: ModeDataOnly,
			Source:          source,
			Priority:        300,
			Content:         content,
		})
	}

	appendDataOnly("profile_context", GwSectionProfileContext, "profile", req.ProfileContext)
	appendDataOnly("memory_context", GwSectionMemoryContext, "memory", req.MemoryContext)
	appendDataOnly("worldbook_context", GwSectionWorldbookContext, "worldbook", req.Worldbook)
	appendDataOnly("conversation_history", GwSectionConversationHistory, "history", req.History)
	appendDataOnly("tool_result", GwSectionToolResult, "tool", req.ToolResults)
	appendDataOnly("multimodal_text", GwSectionMultimodalText, "multimodal", req.MultimodalText)

	sections = append(sections, GwSection{
		ID:              "current_user_message",
		Type:            GwSectionCurrentUserMessage,
		TrustLevel:      TrustUntrusted,
		InstructionMode: ModeUserRequest,
		Source:          "user",
		Priority:        100,
		Content:         req.CurrentUserInput,
	})

	return GwIR{Sections: sections}
}
