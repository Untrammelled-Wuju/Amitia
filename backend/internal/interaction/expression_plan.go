package interaction

import "time"

type ExpressionPlanVersion string

const (
	ExpressionPlanVersionV1 ExpressionPlanVersion = "expression-plan-v1"
)

type ExpressionTone string

const (
	ExpressionToneWarm      ExpressionTone = "warm"
	ExpressionToneRational  ExpressionTone = "rational"
	ExpressionTonePlayful   ExpressionTone = "playful"
	ExpressionToneIntimate  ExpressionTone = "intimate"
	ExpressionToneReserved  ExpressionTone = "reserved"
	ExpressionToneRepairing ExpressionTone = "repairing"
)

type ExpressionDirectness string

const (
	ExpressionDirectnessSoft     ExpressionDirectness = "soft"
	ExpressionDirectnessBalanced ExpressionDirectness = "balanced"
	ExpressionDirectnessDirect   ExpressionDirectness = "direct"
)

type ExpressionPolicy struct {
	Version              ExpressionPlanVersion `json:"version"`
	PolicyKey            string                `json:"policyKey,omitempty"`
	MinCharacters        int                   `json:"minCharacters"`
	MaxCharacters        int                   `json:"maxCharacters"`
	MinSentences         int                   `json:"minSentences"`
	MaxSentences         int                   `json:"maxSentences"`
	Directness           ExpressionDirectness  `json:"directness"`
	AdviceBias           float64               `json:"adviceBias"`
	Warmth               float64               `json:"warmth"`
	Rationality          float64               `json:"rationality"`
	Playfulness          float64               `json:"playfulness"`
	Intimacy             float64               `json:"intimacy"`
	EmotionalDisclosure  float64               `json:"emotionalDisclosure"`
	ForbiddenExpressions []string              `json:"forbiddenExpressions,omitempty"`
	ChannelOverlayKey    string                `json:"channelOverlayKey,omitempty"`
	NormalizationRules   []string              `json:"normalizationRules,omitempty"`
}

type ExpressionPlan struct {
	Version             ExpressionPlanVersion `json:"version"`
	ID                  string                `json:"id,omitempty"`
	BehaviorPlanID      string                `json:"behaviorPlanId,omitempty"`
	UserID              string                `json:"userId,omitempty"`
	CharacterID         string                `json:"characterId,omitempty"`
	CreatedAt           time.Time             `json:"createdAt"`
	Policy              ExpressionPolicy      `json:"policy"`
	Tones               []ExpressionTone      `json:"tones,omitempty"`
	EmotionPresentation []EmotionPresentation `json:"emotionPresentation,omitempty"`
	PromptSections      []PromptSectionRef    `json:"promptSections,omitempty"`
	OutputGuards        OutputGuardSet        `json:"outputGuards"`
	Audit               ExpressionAudit       `json:"audit"`
}

type EmotionPresentation struct {
	Kind      string  `json:"kind"`
	Intensity float64 `json:"intensity"`
	Mode      string  `json:"mode"`
	Reason    string  `json:"reason,omitempty"`
}

type PromptSectionRef struct {
	Type        string `json:"type"`
	Priority    int    `json:"priority"`
	TokenBudget int    `json:"tokenBudget"`
	Source      string `json:"source"`
	Sensitivity string `json:"sensitivity"`
	Trimmable   bool   `json:"trimmable"`
	DataOnly    bool   `json:"dataOnly"`
}

type OutputGuardSet struct {
	RespectHardBoundaries bool     `json:"respectHardBoundaries"`
	TreatMemoryAsData     bool     `json:"treatMemoryAsData"`
	TreatWorldbookAsData  bool     `json:"treatWorldbookAsData"`
	InjectionPatterns     []string `json:"injectionPatterns,omitempty"`
	BlockedClaims         []string `json:"blockedClaims,omitempty"`
}

type ExpressionAudit struct {
	PersonalityVersion string   `json:"personalityVersion,omitempty"`
	ConflictIDs        []string `json:"conflictIds,omitempty"`
	SnapshotID         string   `json:"snapshotId,omitempty"`
	Diagnostics        []string `json:"diagnostics,omitempty"`
}
