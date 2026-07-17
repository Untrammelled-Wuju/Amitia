package prompt

type TrustLevel string

const (
	TrustTrusted     TrustLevel = "trusted"
	TrustSemiTrusted TrustLevel = "semi_trusted"
	TrustUntrusted   TrustLevel = "untrusted"
)

type InstructionMode string

const (
	ModeAuthoritative InstructionMode = "authoritative_instruction"
	ModeStyle         InstructionMode = "style_constraint"
	ModeRuntime       InstructionMode = "runtime_guidance"
	ModeDataOnly      InstructionMode = "data_only"
	ModeUserRequest   InstructionMode = "user_request"
)

type GwSectionType string

const (
	GwSectionPlatformPolicy           GwSectionType = "platform_policy"
	GwSectionAppContract              GwSectionType = "app_contract"
	GwSectionCognitiveContract        GwSectionType = "cognitive_contract"
	GwSectionAntiFlatteryContract     GwSectionType = "anti_flattery_contract"
	GwSectionTechnicalTaskContract    GwSectionType = "technical_task_contract"
	GwSectionCharacterContract        GwSectionType = "character_contract"
	GwSectionRuntimePlan              GwSectionType = "runtime_plan"
	GwSectionExpressionPlan           GwSectionType = "expression_plan"
	GwSectionMemoryContext            GwSectionType = "memory_context"
	GwSectionProfileContext           GwSectionType = "profile_context"
	GwSectionWorldbookContext         GwSectionType = "worldbook_context"
	GwSectionPluginContext            GwSectionType = "plugin_context"
	GwSectionAgentSkillInstructions   GwSectionType = "agent_skill_instructions"
	GwSectionConversationHistory      GwSectionType = "conversation_history"
	GwSectionToolResult               GwSectionType = "tool_result"
	GwSectionMultimodalText           GwSectionType = "multimodal_text"
	GwSectionCurrentUserMessage       GwSectionType = "current_user_message"
	GwSectionBaseIdentity             GwSectionType = "base_identity"
	GwSectionPersonalityRaw           GwSectionType = "personality_raw"
	GwSectionEmotionFusionRaw         GwSectionType = "emotion_fusion_raw"
	GwSectionAdultIntimacyRaw         GwSectionType = "adult_intimacy_raw"
	GwSectionMemoryInjectRaw          GwSectionType = "memory_inject_raw"
	GwSectionMemoryExtractRaw         GwSectionType = "memory_extract_raw"
	GwSectionOutputShapeRaw           GwSectionType = "output_shape_raw"
	GwSectionAntiRepeatRaw            GwSectionType = "anti_repeat_raw"
	GwSectionProactiveRaw             GwSectionType = "proactive_raw"
	GwSectionProactivePersonality     GwSectionType = "proactive_personality"
	GwSectionProactiveRelationship    GwSectionType = "proactive_relationship"
	GwSectionProactiveEmotion         GwSectionType = "proactive_emotion"
	GwSectionProactiveMemory          GwSectionType = "proactive_memory"
	GwSectionProactiveScene           GwSectionType = "proactive_scene"
	GwSectionProactiveTimeContext     GwSectionType = "proactive_time_context"
	GwSectionProactiveRecentContext   GwSectionType = "proactive_recent_context"
	GwSectionProactiveTaskInstruction GwSectionType = "proactive_task_instruction"
	GwSectionChannelShortRaw          GwSectionType = "channel_short_raw"
	GwSectionSystemPrompt             GwSectionType = "system_prompt"

	GwSectionTraceOnly GwSectionType = "trace_only"
)

type GwSection struct {
	ID              string
	Type            GwSectionType
	TrustLevel      TrustLevel
	InstructionMode InstructionMode
	Source          string
	Priority        int
	TokenBudget     int
	Content         string
	SourceProject   string
	SourceFile      string
	SourceConstant  string
	Enabled         bool
}

type GwIR struct {
	Sections []GwSection
	Trace    PromptTrace
}

type GwMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
