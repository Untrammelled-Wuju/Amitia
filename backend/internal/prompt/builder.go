package prompt

import (
	"github.com/u-ai/backend/config"
)

type BuildRequest struct {
	SystemPrompt string

	CharacterName       string
	CharacterConfig     string
	CompiledPersonality string
	RuntimePlan         string
	ExpressionPlan      string

	BaseIdentity     string
	PersonalityRaw   string
	EmotionFusionRaw string
	AdultIntimacyRaw string
	MemoryInjectRaw  string
	MemoryExtractRaw string
	OutputShapeRaw   string
	AntiRepeatRaw    string
	ChannelShortRaw  string

	ProactiveRaw             string
	ProactivePersonality     string
	ProactiveRelationship    string
	ProactiveEmotion         string
	ProactiveMemory          string
	ProactiveScene           string
	ProactiveTimeContext     string
	ProactiveRecentContext   string
	ProactiveTaskInstruction string

	ProfileContext string
	MemoryContext  string
	Worldbook      string
	History        string

	ToolResults    string
	MultimodalText string

	CurrentUserInput  string
	TraceOnly         string
	DropEmptySections bool
}

type Builder struct{}

func NewBuilder() *Builder {
	return &Builder{}
}

func zeroPromptFlags() config.PromptFeatureFlags {
	return config.PromptFeatureFlags{}
}

func (b *Builder) Build(req BuildRequest) GwIR {
	flags := zeroPromptFlags()
	if config.AppCfg != nil {
		flags = config.AppCfg.Prompt
	}
	var sections []GwSection

	sections = append(sections, GwSection{Enabled: true,
		ID:              "platform_policy",
		Type:            GwSectionPlatformPolicy,
		TrustLevel:      TrustTrusted,
		InstructionMode: ModeAuthoritative,
		Source:          "platform",
		Priority:        1000,
		Content:         platformPolicy(),
		SourceProject:   "prompt",
		SourceFile:      "builder.go",
		SourceConstant:  "GwSectionPlatformPolicy",
	})

	sections = append(sections, GwSection{Enabled: true,
		ID:              "app_contract",
		Type:            GwSectionAppContract,
		TrustLevel:      TrustTrusted,
		InstructionMode: ModeAuthoritative,
		Source:          "app",
		Priority:        900,
		Content:         appContract(),
		SourceProject:   "prompt",
		SourceFile:      "builder.go",
		SourceConstant:  "GwSectionAppContract",
	})

	sections = append(sections, GwSection{Enabled: true,
		ID:              "cognitive_contract",
		Type:            GwSectionCognitiveContract,
		TrustLevel:      TrustTrusted,
		InstructionMode: ModeAuthoritative,
		Source:          "app",
		Priority:        870,
		Content:         cognitiveContract(),
		SourceProject:   "prompt",
		SourceFile:      "builder.go",
		SourceConstant:  "GwSectionCognitiveContract",
	})

	sections = append(sections, GwSection{Enabled: true,
		ID:              "anti_flattery_contract",
		Type:            GwSectionAntiFlatteryContract,
		TrustLevel:      TrustTrusted,
		InstructionMode: ModeAuthoritative,
		Source:          "app",
		Priority:        860,
		Content:         antiFlatteryContract(),
		SourceProject:   "prompt",
		SourceFile:      "builder.go",
		SourceConstant:  "GwSectionAntiFlatteryContract",
	})

	sections = append(sections, GwSection{Enabled: true,
		ID:              "technical_task_contract",
		Type:            GwSectionTechnicalTaskContract,
		TrustLevel:      TrustTrusted,
		InstructionMode: ModeAuthoritative,
		Source:          "app",
		Priority:        850,
		Content:         technicalTaskContract(),
		SourceProject:   "prompt",
		SourceFile:      "builder.go",
		SourceConstant:  "GwSectionTechnicalTaskContract",
	})

	if req.BaseIdentity != "" {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "base_identity",
			Type:            GwSectionBaseIdentity,
			TrustLevel:      TrustTrusted,
			InstructionMode: ModeAuthoritative,
			Source:          "base_identity",
			Priority:        880,
			Content:         req.BaseIdentity,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionBaseIdentity",
		})
	}

	

	if req.SystemPrompt != "" {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "system_prompt",
			Type:            GwSectionSystemPrompt,
			TrustLevel:      TrustSemiTrusted,
			InstructionMode: ModeAuthoritative,
			Source:          "system_prompt",
			Priority:        840,
			Content:         "【角色专属提示词 - 最高优先级角色指令】" + "\n" + req.SystemPrompt,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionSystemPrompt",
		})
	}

	if req.CharacterConfig != "" || req.CompiledPersonality != "" {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "character_contract",
			Type:            GwSectionCharacterContract,
			TrustLevel:      TrustTrusted,
			InstructionMode: ModeAuthoritative,
			Source:          "character_config",
			Priority:        800,
			Content:         req.CharacterConfig + "\n" + req.CompiledPersonality,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionCharacterContract",
		})
	}

	if req.PersonalityRaw != "" && flags.PersonalityRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "personality_raw",
			Type:            GwSectionPersonalityRaw,
			TrustLevel:      TrustSemiTrusted,
			InstructionMode: ModeStyle,
			Source:          "personality",
			Priority:        780,
			Content:         req.PersonalityRaw,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionPersonalityRaw",
		})
	}

	if req.EmotionFusionRaw != "" && flags.EmotionFusionEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "emotion_fusion_raw",
			Type:            GwSectionEmotionFusionRaw,
			TrustLevel:      TrustSemiTrusted,
			InstructionMode: ModeStyle,
			Source:          "emotion",
			Priority:        760,
			Content:         req.EmotionFusionRaw,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionEmotionFusionRaw",
		})
	}

	if req.AdultIntimacyRaw != "" && flags.IntimacyDefaultEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "adult_intimacy_raw",
			Type:            GwSectionAdultIntimacyRaw,
			TrustLevel:      TrustSemiTrusted,
			InstructionMode: ModeStyle,
			Source:          "intimacy",
			Priority:        740,
			Content:         req.AdultIntimacyRaw,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionAdultIntimacyRaw",
		})
	}

	if req.RuntimePlan != "" {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "runtime_plan",
			Type:            GwSectionRuntimePlan,
			TrustLevel:      TrustSemiTrusted,
			InstructionMode: ModeRuntime,
			Source:          "runtime",
			Priority:        700,
			Content:         req.RuntimePlan,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionRuntimePlan",
		})
	}

	if req.ExpressionPlan != "" {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "expression_plan",
			Type:            GwSectionExpressionPlan,
			TrustLevel:      TrustSemiTrusted,
			InstructionMode: ModeRuntime,
			Source:          "runtime",
			Priority:        650,
			Content:         req.ExpressionPlan,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionExpressionPlan",
		})
	}

	if req.MemoryInjectRaw != "" && flags.MemoryRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "memory_inject_raw",
			Type:            GwSectionMemoryInjectRaw,
			TrustLevel:      TrustUntrusted,
			InstructionMode: ModeDataOnly,
			Source:          "memory",
			Priority:        350,
			Content:         req.MemoryInjectRaw,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionMemoryInjectRaw",
		})
	}

	if req.MemoryExtractRaw != "" && flags.MemoryRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "memory_extract_raw",
			Type:            GwSectionMemoryExtractRaw,
			TrustLevel:      TrustUntrusted,
			InstructionMode: ModeDataOnly,
			Source:          "memory",
			Priority:        340,
			Content:         req.MemoryExtractRaw,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionMemoryExtractRaw",
		})
	}

	appendDataOnly := func(id string, typ GwSectionType, source string, content string, constant string) {
		if content == "" {
			return
		}
		sections = append(sections, GwSection{Enabled: true,
			ID:              id,
			Type:            typ,
			TrustLevel:      TrustUntrusted,
			InstructionMode: ModeDataOnly,
			Source:          source,
			Priority:        300,
			Content:         content,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  constant,
		})
	}

	appendDataOnly("profile_context", GwSectionProfileContext, "profile", req.ProfileContext, "GwSectionProfileContext")
	appendDataOnly("memory_context", GwSectionMemoryContext, "memory", req.MemoryContext, "GwSectionMemoryContext")
	appendDataOnly("worldbook_context", GwSectionWorldbookContext, "worldbook", req.Worldbook, "GwSectionWorldbookContext")
	appendDataOnly("conversation_history", GwSectionConversationHistory, "history", req.History, "GwSectionConversationHistory")
	appendDataOnly("tool_result", GwSectionToolResult, "tool", req.ToolResults, "GwSectionToolResult")
	appendDataOnly("multimodal_text", GwSectionMultimodalText, "multimodal", req.MultimodalText, "GwSectionMultimodalText")

	if req.OutputShapeRaw != "" && flags.ReplySanitizerEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "output_shape_raw",
			Type:            GwSectionOutputShapeRaw,
			TrustLevel:      TrustTrusted,
			InstructionMode: ModeAuthoritative,
			Source:          "sanitizer",
			Priority:        600,
			Content:         req.OutputShapeRaw,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionOutputShapeRaw",
		})
	}

	if req.AntiRepeatRaw != "" && flags.ReplySanitizerEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "anti_repeat_raw",
			Type:            GwSectionAntiRepeatRaw,
			TrustLevel:      TrustTrusted,
			InstructionMode: ModeAuthoritative,
			Source:          "sanitizer",
			Priority:        590,
			Content:         req.AntiRepeatRaw,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionAntiRepeatRaw",
		})
	}

	if req.ChannelShortRaw != "" && flags.TextlibRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "channel_short_raw",
			Type:            GwSectionChannelShortRaw,
			TrustLevel:      TrustTrusted,
			InstructionMode: ModeAuthoritative,
			Source:          "textlib",
			Priority:        580,
			Content:         req.ChannelShortRaw,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionChannelShortRaw",
		})
	}

	if req.ProactiveTaskInstruction != "" && flags.ProactiveRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "proactive_task_instruction",
			Type:            GwSectionProactiveTaskInstruction,
			TrustLevel:      TrustTrusted,
			InstructionMode: ModeAuthoritative,
			Source:          "proactive",
			Priority:        510,
			Content:         req.ProactiveTaskInstruction,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionProactiveTaskInstruction",
		})
	}

	if req.ProactiveRaw != "" && flags.ProactiveRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "proactive_raw",
			Type:            GwSectionProactiveRaw,
			TrustLevel:      TrustTrusted,
			InstructionMode: ModeAuthoritative,
			Source:          "proactive",
			Priority:        500,
			Content:         req.ProactiveRaw,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionProactiveRaw",
		})
	}

	if req.ProactivePersonality != "" && flags.ProactiveRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "proactive_personality",
			Type:            GwSectionProactivePersonality,
			TrustLevel:      TrustSemiTrusted,
			InstructionMode: ModeStyle,
			Source:          "proactive",
			Priority:        490,
			Content:         req.ProactivePersonality,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionProactivePersonality",
		})
	}

	if req.ProactiveRelationship != "" && flags.ProactiveRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "proactive_relationship",
			Type:            GwSectionProactiveRelationship,
			TrustLevel:      TrustUntrusted,
			InstructionMode: ModeDataOnly,
			Source:          "proactive",
			Priority:        480,
			Content:         req.ProactiveRelationship,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionProactiveRelationship",
		})
	}

	if req.ProactiveEmotion != "" && flags.ProactiveRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "proactive_emotion",
			Type:            GwSectionProactiveEmotion,
			TrustLevel:      TrustUntrusted,
			InstructionMode: ModeDataOnly,
			Source:          "proactive",
			Priority:        470,
			Content:         req.ProactiveEmotion,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionProactiveEmotion",
		})
	}

	if req.ProactiveMemory != "" && flags.ProactiveRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "proactive_memory",
			Type:            GwSectionProactiveMemory,
			TrustLevel:      TrustUntrusted,
			InstructionMode: ModeDataOnly,
			Source:          "proactive",
			Priority:        460,
			Content:         req.ProactiveMemory,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionProactiveMemory",
		})
	}

	if req.ProactiveScene != "" && flags.ProactiveRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "proactive_scene",
			Type:            GwSectionProactiveScene,
			TrustLevel:      TrustTrusted,
			InstructionMode: ModeAuthoritative,
			Source:          "proactive",
			Priority:        450,
			Content:         req.ProactiveScene,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionProactiveScene",
		})
	}

	if req.ProactiveTimeContext != "" && flags.ProactiveRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "proactive_time_context",
			Type:            GwSectionProactiveTimeContext,
			TrustLevel:      TrustUntrusted,
			InstructionMode: ModeDataOnly,
			Source:          "proactive",
			Priority:        440,
			Content:         req.ProactiveTimeContext,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionProactiveTimeContext",
		})
	}

	if req.ProactiveRecentContext != "" && flags.ProactiveRawEnabled {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "proactive_recent_context",
			Type:            GwSectionProactiveRecentContext,
			TrustLevel:      TrustUntrusted,
			InstructionMode: ModeDataOnly,
			Source:          "proactive",
			Priority:        430,
			Content:         req.ProactiveRecentContext,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionProactiveRecentContext",
		})
	}

	currentUserContent := req.CurrentUserInput
	if req.ProactiveTaskInstruction != "" {
		currentUserContent = "(proactive)"
	}

	sections = append(sections, GwSection{Enabled: true,
		ID:              "current_user_message",
		Type:            GwSectionCurrentUserMessage,
		TrustLevel:      TrustUntrusted,
		InstructionMode: ModeUserRequest,
		Source:          "user",
		Priority:        100,
		Content:         currentUserContent,
		SourceProject:   "prompt",
		SourceFile:      "builder.go",
		SourceConstant:  "GwSectionCurrentUserMessage",
	})

	if req.TraceOnly != "" {
		sections = append(sections, GwSection{Enabled: true,
			ID:              "trace_only",
			Type:            GwSectionTraceOnly,
			TrustLevel:      TrustUntrusted,
			InstructionMode: ModeDataOnly,
			Source:          "trace",
			Priority:        10,
			Content:         req.TraceOnly,
			SourceProject:   "prompt",
			SourceFile:      "builder.go",
			SourceConstant:  "GwSectionTraceOnly",
		})
	}
	if req.DropEmptySections {
		var filtered []GwSection
		for _, s := range sections {
			if s.Content != "" {
				filtered = append(filtered, s)
			}
		}
		sections = filtered
	}

	return GwIR{Sections: sections}
}
