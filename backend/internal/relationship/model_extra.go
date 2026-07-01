package relationship

import "time"

type SlowEvidenceBuffer struct {
	Trust      SlowDimension `json:"trust"`
	Intimacy   SlowDimension `json:"intimacy"`
	Dependency SlowDimension `json:"dependency"`
	Conflict   SlowDimension `json:"conflict"`
	Repair     SlowDimension `json:"repair"`
}

type SlowDimension struct {
	PendingDelta  float64   `json:"pendingDelta"`
	VisibleChange float64   `json:"visibleChange"`
	EvidenceCount int       `json:"evidenceCount"`
	LastFlushedAt time.Time `json:"lastFlushedAt"`
}

type SlowUpdateConfig struct {
	TrustThreshold      float64 `json:"trustThreshold"`
	IntimacyThreshold   float64 `json:"intimacyThreshold"`
	DependencyThreshold float64 `json:"dependencyThreshold"`
	ConflictThreshold   float64 `json:"conflictThreshold"`
	RepairThreshold     float64 `json:"repairThreshold"`
	DecayRate           float64 `json:"decayRate"`
	MaxEvidenceAge      float64 `json:"maxEvidenceAge"`
}

type AttachmentStyle string

const (
	AttachmentSecure  AttachmentStyle = "secure"
	AttachmentAnxious AttachmentStyle = "anxious"
	AttachmentDismiss AttachmentStyle = "dismissing"
	AttachmentFearful AttachmentStyle = "fearful"
)

type AttachmentProfile struct {
	Style               AttachmentStyle `json:"style"`
	RecoverySpeed       float64         `json:"recoverySpeed"`
	ConflictSensitivity float64         `json:"conflictSensitivity"`
	ProtestIntensity    float64         `json:"protestIntensity"`
}

type ActiveConflict struct {
	ID         string     `json:"id,omitempty"`
	SourceID   string     `json:"sourceId,omitempty"`
	Intensity  float64    `json:"intensity"`
	StartedAt  time.Time  `json:"startedAt,omitempty"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	Escalated  bool       `json:"escalated"`
}

type RepairAttempt struct {
	ID          string    `json:"id,omitempty"`
	ConflictID  string    `json:"conflictId,omitempty"`
	Effective   bool      `json:"effective"`
	Confidence  float64   `json:"confidence"`
	AttemptedAt time.Time `json:"attemptedAt,omitempty"`
}

type ConflictState struct {
	ActiveConflicts   []ActiveConflict `json:"activeConflicts"`
	RepairAttempts    []RepairAttempt  `json:"repairAttempts"`
	ActiveRepair      bool             `json:"activeRepair"`
	RepairTriggeredAt time.Time        `json:"repairTriggeredAt,omitempty"`
	ConflictCount     int              `json:"conflictCount"`
	ResolvedCount     int              `json:"resolvedCount"`
}

type NarrativeTone string

const (
	NarrativePositive   NarrativeTone = "positive"
	NarrativeTense      NarrativeTone = "tense"
	NarrativeNeutral    NarrativeTone = "neutral"
	NarrativeRecovering NarrativeTone = "recovering"
	NarrativeDistant    NarrativeTone = "distant"
)

type NarrativeSummary struct {
	Tone       NarrativeTone `json:"tone"`
	Summary    string        `json:"summary"`
	Confidence float64       `json:"confidence"`
	UpdatedAt  time.Time     `json:"updatedAt,omitempty"`
}

type UnresolvedThread struct {
	ID              string     `json:"id,omitempty"`
	Topic           string     `json:"topic"`
	EventRefs       []string   `json:"eventRefs,omitempty"`
	Reason          string     `json:"reason"`
	Severity        float64    `json:"severity"`
	Duration        float64    `json:"duration"`
	CreatedAt       time.Time  `json:"createdAt,omitempty"`
	LastEscalatedAt time.Time  `json:"lastEscalatedAt,omitempty"`
	ResolvedAt      *time.Time `json:"resolvedAt,omitempty"`
	EscalationLevel int        `json:"escalationLevel"`
	DecayPerHour    float64    `json:"decayPerHour"`
	RelationImpact  float64    `json:"relationImpact"`
}

type UnresolvedConfig struct {
	BaseEscalationHours    float64 `json:"baseEscalationHours"`
	EscalationStepSeverity float64 `json:"escalationStepSeverity"`
	MaxEscalationLevel     int     `json:"maxEscalationLevel"`
	NaturalDecayPerHour    float64 `json:"naturalDecayPerHour"`
	ResolutionThreshold    float64 `json:"resolutionThreshold"`
}
