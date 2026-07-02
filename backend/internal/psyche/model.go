package psyche

import (
	"encoding/json"
	"time"
)

type PersonalityConfig struct {
	SchemaVersion            string   `json:"schemaVersion"`
	PersonalitySchemaVersion string   `json:"personality_schema_version,omitempty"`
	Initiative               *float64 `json:"initiative"`
	Sensitivity              *float64 `json:"sensitivity"`
	Tolerance                *float64 `json:"tolerance"`
	Stability                *float64 `json:"stability"`
	Boundary                 *float64 `json:"boundary"`
	Warmth                   *float64 `json:"warmth"`
	Directness               *float64 `json:"directness"`
	Humor                    *float64 `json:"humor"`
	Affection                *float64 `json:"affection"`
	Verbosity                *float64 `json:"verbosity"`
	ConflictAvoidance        *float64 `json:"conflictAvoidance"`
}

type PersonalityMigration struct {
	FromSchema    string                     `json:"fromSchema"`
	ToSchema      string                     `json:"toSchema"`
	Snapshot      PersonalityConfig          `json:"snapshot"`
	Sources       map[string]string          `json:"sources"`
	UnknownFields map[string]json.RawMessage `json:"unknownFields,omitempty"`
	Diagnostics   []string                   `json:"diagnostics,omitempty"`
}

type ResolvedConfig struct {
	SchemaVersion     string  `json:"schemaVersion"`
	Initiative        float64 `json:"initiative"`
	Sensitivity       float64 `json:"sensitivity"`
	Tolerance         float64 `json:"tolerance"`
	Stability         float64 `json:"stability"`
	Boundary          float64 `json:"boundary"`
	Warmth            float64 `json:"warmth"`
	Directness        float64 `json:"directness"`
	Humor             float64 `json:"humor"`
	Affection         float64 `json:"affection"`
	Verbosity         float64 `json:"verbosity"`
	ConflictAvoidance float64 `json:"conflictAvoidance"`
}

type InternalModel struct {
	StableCore  StableCoreLayer  `json:"stableCore"`
	Growth      GrowthLayer      `json:"growth"`
	Situational SituationalLayer `json:"situational"`
}

type StableCoreLayer struct {
	SocialInitiative     float64 `json:"socialInitiative"`
	RejectionSensitivity float64 `json:"rejectionSensitivity"`
	UncertaintyTolerance float64 `json:"uncertaintyTolerance"`
	EmotionStability     float64 `json:"emotionStability"`
	BoundaryStrength     float64 `json:"boundaryStrength"`
}

type GrowthLayer struct {
	Warmth      float64 `json:"warmth"`
	Humor       float64 `json:"humor"`
	Affection   float64 `json:"affection"`
	SupportBias float64 `json:"supportBias"`
}

type SituationalLayer struct {
	Directness        float64 `json:"directness"`
	Verbosity         float64 `json:"verbosity"`
	ConflictAvoidance float64 `json:"conflictAvoidance"`
}

type AppraisalCoefficients struct {
	Version               string  `json:"version"`
	Rejection             float64 `json:"rejection"`
	RelationshipRelevance float64 `json:"relationshipRelevance"`
	ExpectationGap        float64 `json:"expectationGap"`
	Uncertainty           float64 `json:"uncertainty"`
	Boundary              float64 `json:"boundary"`
	AmplificationCap      float64 `json:"amplificationCap"`
	Explanation           string  `json:"explanation"`
}

type RecoveryProfile struct {
	Version              string  `json:"version"`
	EmotionHalfLifeHours float64 `json:"emotionHalfLifeHours"`
	MoodHalfLifeHours    float64 `json:"moodHalfLifeHours"`
	StressHalfLifeHours  float64 `json:"stressHalfLifeHours"`
	NeedHalfLifeHours    float64 `json:"needHalfLifeHours"`
	MinRecoveryRate      float64 `json:"minRecoveryRate"`
	MaxRecoveryRate      float64 `json:"maxRecoveryRate"`
	ResilienceBias       float64 `json:"resilienceBias"`
}

type BehaviorProfile struct {
	Version             string  `json:"version"`
	InitiateWeight      float64 `json:"initiateWeight"`
	DirectWeight        float64 `json:"directWeight"`
	HumorWeight         float64 `json:"humorWeight"`
	ConflictAvoidWeight float64 `json:"conflictAvoidWeight"`
	SupportWeight       float64 `json:"supportWeight"`
	InitiationThreshold float64 `json:"initiationThreshold"`
}

type ExpressionPolicy struct {
	Version             string   `json:"version"`
	MinReplyChars       int      `json:"minReplyChars"`
	MaxReplyChars       int      `json:"maxReplyChars"`
	MinSentences        int      `json:"minSentences"`
	MaxSentences        int      `json:"maxSentences"`
	ShortSentenceBias   float64  `json:"shortSentenceBias"`
	Warmth              float64  `json:"warmth"`
	Rationality         float64  `json:"rationality"`
	Teasing             float64  `json:"teasing"`
	Intimacy            float64  `json:"intimacy"`
	SuggestionBias      float64  `json:"suggestionBias"`
	EmotionalDisclosure float64  `json:"emotionalDisclosure"`
	ForbiddenStyles     []string `json:"forbiddenStyles"`
}

type CompiledProfile struct {
	CompilerVersion string                `json:"compilerVersion"`
	Resolved        ResolvedConfig        `json:"resolved"`
	Internal        InternalModel         `json:"internal"`
	Appraisal       AppraisalCoefficients `json:"appraisal"`
	Recovery        RecoveryProfile       `json:"recovery"`
	Behavior        BehaviorProfile       `json:"behavior"`
	Expression      ExpressionPolicy      `json:"expression"`
	Sources         map[string]string     `json:"sources"`
	Diagnostics     []string              `json:"diagnostics"`
	Migration       PersonalityMigration  `json:"migration"`
}

type RuntimeStateInput struct {
	Stress        *float64 `json:"stress"`
	Fatigue       *float64 `json:"fatigue"`
	Arousal       *float64 `json:"arousal"`
	MoodPressure  *float64 `json:"moodPressure"`
	SocialLoad    *float64 `json:"socialLoad"`
	RecoveryHours *float64 `json:"recoveryHours"`
}

type RuntimeState struct {
	Stress        float64 `json:"stress"`
	Fatigue       float64 `json:"fatigue"`
	Arousal       float64 `json:"arousal"`
	MoodPressure  float64 `json:"moodPressure"`
	SocialLoad    float64 `json:"socialLoad"`
	RecoveryHours float64 `json:"recoveryHours"`
}

type RuntimeInfluence struct {
	StressImpact      float64 `json:"stressImpact"`
	FatigueImpact     float64 `json:"fatigueImpact"`
	RecoveryImpact    float64 `json:"recoveryImpact"`
	Regulation        float64 `json:"regulation"`
	ExpressionPenalty float64 `json:"expressionPenalty"`
	Pressure          float64 `json:"pressure"`
	Volatility        float64 `json:"volatility"`
}

type RuntimeModulation struct {
	Version     string                `json:"version"`
	State       RuntimeState          `json:"state"`
	Influence   RuntimeInfluence      `json:"influence"`
	Internal    InternalModel         `json:"internal"`
	Appraisal   AppraisalCoefficients `json:"appraisal"`
	Recovery    RecoveryProfile       `json:"recovery"`
	Behavior    BehaviorProfile       `json:"behavior"`
	Expression  ExpressionPolicy      `json:"expression"`
	Sources     map[string]string     `json:"sources"`
	Diagnostics []string              `json:"diagnostics"`
}

type EventType string

const (
	EventTypeInteraction EventType = "EVENT_TYPE_INTERACTION"
	EventTypeAppraisal   EventType = "EVENT_TYPE_APPRAISAL"
	EventTypeInternal    EventType = "EVENT_TYPE_INTERNAL"
	EventTypeRecovery    EventType = "EVENT_TYPE_RECOVERY"
)

type EmotionDimensions struct {
	Valence   float64 `json:"valence"`
	Arousal   float64 `json:"arousal"`
	Dominance float64 `json:"dominance"`
}

type MoodDimensions struct {
	MoodValence float64 `json:"moodValence"`
	MoodArousal float64 `json:"moodArousal"`
}

type PsycheState struct {
	CharacterID  string            `gorm:"primaryKey;column:character_id" json:"characterId"`
	Version      string            `gorm:"column:version" json:"version"`
	StateVersion int               `json:"stateVersion" gorm:"column:state_version;default:0"`
	Emotion      EmotionDimensions `gorm:"-" json:"emotion"`
	Mood         MoodDimensions    `gorm:"-" json:"mood"`
	Stress       float64           `gorm:"column:stress" json:"stress"`
	Energy       float64           `gorm:"column:energy" json:"energy"`
	CreatedAt    time.Time         `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time         `gorm:"column:updated_at" json:"updatedAt"`
}

type PsycheEvent struct {
	ID             string    `json:"id"`
	CharacterID    string    `json:"characterId"`
	Type           EventType `json:"type" gorm:"column:event_type"`
	Source         string    `json:"source"`
	ValenceDelta   float64   `json:"valenceDelta"`
	ArousalDelta   float64   `json:"arousalDelta"`
	DominanceDelta float64   `json:"dominanceDelta"`
	StressDelta    float64   `json:"stressDelta"`
	EnergyDelta    float64   `json:"energyDelta"`
	Timestamp      time.Time `json:"timestamp" gorm:"column:created_at"`
}

type PsycheSnapshot struct {
	ID               string    `json:"id"`
	CharacterID      string    `json:"characterId"`
	Version          string    `json:"version"`
	Timestamp        time.Time `json:"timestamp" gorm:"column:created_at"`
	EmotionValence   float64   `json:"emotionValence"`
	EmotionArousal   float64   `json:"emotionArousal"`
	EmotionDominance float64   `json:"emotionDominance"`
	MoodValence      float64   `json:"moodValence"`
	MoodArousal      float64   `json:"moodArousal"`
	Stress           float64   `json:"stress"`
	Energy           float64   `json:"energy"`
}
