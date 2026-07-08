package prompt

type SectionType string

const (
	SectionTypeSystem                   SectionType = "system"
	SectionTypeIdentity                 SectionType = "identity"
	SectionTypeBehaviorPlan             SectionType = "behavior_plan"
	SectionTypePsyche                   SectionType = "psyche"
	SectionTypeMemory                   SectionType = "memory"
	SectionTypeHistory                  SectionType = "history"
	SectionTypeCurrentInput             SectionType = "current_input"
	SectionTypeWorldbook                SectionType = "worldbook"
	SectionTypeBaseIdentity             SectionType = "base_identity"
	SectionTypePersonalityRaw           SectionType = "personality_raw"
	SectionTypeEmotionFusionRaw         SectionType = "emotion_fusion_raw"
	SectionTypeAdultIntimacyRaw         SectionType = "adult_intimacy_raw"
	SectionTypeMemoryInjectRaw          SectionType = "memory_inject_raw"
	SectionTypeMemoryExtractRaw         SectionType = "memory_extract_raw"
	SectionTypeOutputShapeRaw           SectionType = "output_shape_raw"
	SectionTypeAntiRepeatRaw            SectionType = "anti_repeat_raw"
	SectionTypeProactiveRaw             SectionType = "proactive_raw"
	SectionTypeProactivePersonality     SectionType = "proactive_personality"
	SectionTypeProactiveRelationship    SectionType = "proactive_relationship"
	SectionTypeProactiveEmotion         SectionType = "proactive_emotion"
	SectionTypeProactiveMemory          SectionType = "proactive_memory"
	SectionTypeProactiveScene           SectionType = "proactive_scene"
	SectionTypeProactiveTimeContext     SectionType = "proactive_time_context"
	SectionTypeProactiveRecentContext   SectionType = "proactive_recent_context"
	SectionTypeProactiveTaskInstruction SectionType = "proactive_task_instruction"
	SectionTypeChannelShortRaw          SectionType = "channel_short_raw"
	SectionTypeTraceOnly                SectionType = "trace_only"
)

type SensitivityLevel string

const (
	SensitivityPublic   SensitivityLevel = "public"
	SensitivityInternal SensitivityLevel = "internal"
	SensitivityUserData SensitivityLevel = "user_data"
	SensitivityPrivate  SensitivityLevel = "private"
	SensitivitySecret   SensitivityLevel = "secret"
)

type IRVersion string

const (
	IRVersionV1 IRVersion = "prompt-ir-v1"
)

type Section struct {
	Type        SectionType      `json:"type"`
	Priority    int              `json:"priority"`
	TokenBudget int              `json:"tokenBudget"`
	Source      string           `json:"source"`
	Sensitivity SensitivityLevel `json:"sensitivity"`
	Trimmable   bool             `json:"trimmable"`
	DataOnly    bool             `json:"dataOnly"`
	Content     string           `json:"content"`
}

type IR struct {
	Version  IRVersion `json:"version"`
	Sections []Section `json:"sections"`
	Audit    Audit     `json:"audit"`
}

type Audit struct {
	CompilerVersion string       `json:"compilerVersion"`
	Diagnostics     []string     `json:"diagnostics,omitempty"`
	TrimRecords     []TrimRecord `json:"trimRecords,omitempty"`
}

type CompileOptions struct {
	MaxSections        int
	MinTokenBudget     int
	MaxTokenBudget     int
	RedactSensitive    bool
	DropEmptySections  bool
	ForceDataOnlyTypes []SectionType
}

type TrimRecord struct {
	SectionType  SectionType `json:"sectionType"`
	Source       string      `json:"source"`
	Reason       string      `json:"reason"`
	BeforeTokens int         `json:"beforeTokens"`
	AfterTokens  int         `json:"afterTokens"`
	Summary      string      `json:"summary"`
}

type SectionTrace struct {
	SectionName    string `json:"section_name"`
	SourceProject  string `json:"source_project"`
	SourceFile     string `json:"source_file"`
	SourceConstant string `json:"source_constant"`
	PromptHash     string `json:"prompt_hash"`
	RenderedLength int    `json:"rendered_length"`
	Enabled        bool   `json:"enabled"`
}

type QualityFlags struct {
	ThinkRemoved         bool `json:"think_removed"`
	MarkdownRemoved      bool `json:"markdown_removed"`
	PersonaSectionUsed   bool `json:"persona_section_used"`
	EmotionSectionUsed   bool `json:"emotion_section_used"`
	MemorySectionUsed    bool `json:"memory_section_used"`
	IntimacyBoundaryUsed bool `json:"intimacy_boundary_used"`
}

type PromptTrace struct {
	PromptHash   string         `json:"prompt_hash"`
	Sections     []SectionTrace `json:"sections"`
	QualityFlags QualityFlags   `json:"quality_flags"`
}
