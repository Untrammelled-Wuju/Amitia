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
	GwSectionPlatformPolicy        GwSectionType = "platform_policy"
	GwSectionAppContract           GwSectionType = "app_contract"
	GwSectionCognitiveContract     GwSectionType = "cognitive_contract"
	GwSectionAntiFlatteryContract  GwSectionType = "anti_flattery_contract"
	GwSectionTechnicalTaskContract GwSectionType = "technical_task_contract"
	GwSectionCharacterContract     GwSectionType = "character_contract"
	GwSectionRuntimePlan           GwSectionType = "runtime_plan"
	GwSectionExpressionPlan        GwSectionType = "expression_plan"
	GwSectionMemoryContext         GwSectionType = "memory_context"
	GwSectionProfileContext        GwSectionType = "profile_context"
	GwSectionWorldbookContext      GwSectionType = "worldbook_context"
	GwSectionConversationHistory   GwSectionType = "conversation_history"
	GwSectionToolResult            GwSectionType = "tool_result"
	GwSectionMultimodalText        GwSectionType = "multimodal_text"
	GwSectionCurrentUserMessage    GwSectionType = "current_user_message"
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
}

type GwIR struct {
	Sections []GwSection
}

type GwMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
