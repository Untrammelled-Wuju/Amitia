package affect

import "time"
type Repository interface {
	LoadState(characterID string) (*AffectState, error)
	SaveState(characterID string, state AffectState) error
}

type StateVersion string

const (
	StateVersionV1 StateVersion = "affect-state-v1"
)

type PersonalityReference struct {
	Version        string  `json:"version,omitempty"`
	Sensitivity    float64 `json:"sensitivity"`
	Stability      float64 `json:"stability"`
	RecoveryBias   float64 `json:"recoveryBias"`
	ConfidenceBias float64 `json:"confidenceBias,omitempty"`
	ControlBias    float64 `json:"controlBias,omitempty"`
	MoodStickiness float64 `json:"moodStickiness"`
	Boundary       float64 `json:"boundary"`
}

type EventAppraisal struct {
	EventID         string    `json:"eventId,omitempty"`
	OccurredAt      time.Time `json:"occurredAt,omitempty"`
	Valence         float64   `json:"valence"`
	Arousal         float64   `json:"arousal"`
	SocialRelevance float64   `json:"socialRelevance"`
	BoundaryThreat  float64   `json:"boundaryThreat"`
	Confidence      float64   `json:"confidence"`
	Control         float64   `json:"control,omitempty"`
	ExpectationGap  float64   `json:"expectationGap"`
	Intensity       float64   `json:"intensity"`
}

type ChangeBudget struct {
	MaxEmotionDelta   float64 `json:"maxEmotionDelta"`
	MaxMoodDelta      float64 `json:"maxMoodDelta"`
	MaxDominanceDelta float64 `json:"maxDominanceDelta,omitempty"`
	MaxStressDelta    float64 `json:"maxStressDelta"`
}

type EmotionState struct {
	Positive    float64   `json:"positive"`
	Negative    float64   `json:"negative"`
	Arousal     float64   `json:"arousal"`
	Dominance   float64   `json:"dominance,omitempty"`
	LastEventID string    `json:"lastEventId,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

type MoodState struct {
	PAD       string    `json:"pad,omitempty"`
	Valence   float64   `json:"valence"`
	Tension   float64   `json:"tension"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type AffectState struct {
	Version   StateVersion `json:"version"`
	Emotion   EmotionState `json:"emotion"`
	Mood      MoodState    `json:"mood"`
	Stress    float64      `json:"stress"`
	UpdatedAt time.Time    `json:"updatedAt,omitempty"`
}

type AffectDelta struct {
	EmotionPositive float64 `json:"emotionPositive"`
	EmotionNegative float64 `json:"emotionNegative"`
	Dominance       float64 `json:"dominance,omitempty"`
	EmotionArousal  float64 `json:"emotionArousal"`
	MoodValence     float64 `json:"moodValence"`
	MoodTension     float64 `json:"moodTension"`
	Stress          float64 `json:"stress"`
}

type AffectAudit struct {
	RecoveryFactor float64      `json:"recoveryFactor"`
	ElapsedHours   float64      `json:"elapsedHours"`
	Budget         ChangeBudget `json:"budget"`
	Diagnostics    []string     `json:"diagnostics,omitempty"`
}

type EngineInput struct {
	Current     AffectState          `json:"current"`
	Personality PersonalityReference `json:"personality"`
	Appraisal   EventAppraisal       `json:"appraisal"`
	Budget      ChangeBudget         `json:"budget"`
	Now         time.Time            `json:"now"`
}

type EngineOutput struct {
	State AffectState `json:"state"`
	Delta AffectDelta `json:"delta"`
	Audit AffectAudit `json:"audit"`
}