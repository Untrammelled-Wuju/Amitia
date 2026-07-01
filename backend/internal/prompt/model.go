package prompt

type SectionType string

const (
	SectionTypeSystem       SectionType = "system"
	SectionTypeIdentity     SectionType = "identity"
	SectionTypeBehaviorPlan SectionType = "behavior_plan"
	SectionTypePsyche       SectionType = "psyche"
	SectionTypeMemory       SectionType = "memory"
	SectionTypeHistory      SectionType = "history"
	SectionTypeCurrentInput SectionType = "current_input"
	SectionTypeWorldbook    SectionType = "worldbook"
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
