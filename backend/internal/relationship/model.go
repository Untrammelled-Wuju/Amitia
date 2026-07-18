package relationship

import "time"

type EngineVersion string

const (
	EngineVersionV1 EngineVersion = "relationship-engine-v1"
)

type EvidenceKind string

const (
	EvidenceKindPositive   EvidenceKind = "positive"
	EvidenceKindSupportive EvidenceKind = "supportive"
	EvidenceKindRepair     EvidenceKind = "repair"
	EvidenceKindConflict   EvidenceKind = "conflict"
	EvidenceKindSafety     EvidenceKind = "safety"
	EvidenceKindBoundary   EvidenceKind = "boundary"
	EvidenceKindWithdrawal EvidenceKind = "withdrawal"
)

type RelationshipState struct {
	Trust            float64 `json:"trust"`
	Familiarity      float64 `json:"familiarity"`
	Security         float64 `json:"security"`
	Tension          float64 `json:"tension"`
	RepairConfidence float64 `json:"repairConfidence"`
	Boundary         float64 `json:"boundary"`
}

type RelationshipDelta struct {
	Trust            float64 `json:"trust"`
	Familiarity      float64 `json:"familiarity"`
	Security         float64 `json:"security"`
	Tension          float64 `json:"tension"`
	RepairConfidence float64 `json:"repairConfidence"`
	Boundary         float64 `json:"boundary"`
}

type InteractionEvidence struct {
	ID         string       `json:"id,omitempty"`
	Kind       EvidenceKind `json:"kind"`
	Intensity  float64      `json:"intensity"`
	Confidence float64      `json:"confidence"`
}

type PersonalityRef struct {
	Version           string  `json:"version,omitempty"`
	Warmth            float64 `json:"warmth"`
	Affection         float64 `json:"affection"`
	Attachment        float64 `json:"attachment"`
	BoundaryStrength  float64 `json:"boundaryStrength"`
	Sensitivity       float64 `json:"sensitivity"`
	Tolerance         float64 `json:"tolerance"`
	ConflictAvoidance float64 `json:"conflictAvoidance"`
}

type ChangeBudget struct {
	MaxPositiveDelta float64 `json:"maxPositiveDelta"`
	MaxNegativeDelta float64 `json:"maxNegativeDelta"`
	MaxTensionDelta  float64 `json:"maxTensionDelta"`
	MaxBoundaryDelta float64 `json:"maxBoundaryDelta"`
}

type UpdateInput struct {
	Current     RelationshipState     `json:"current"`
	Evidence    []InteractionEvidence `json:"evidence,omitempty"`
	Personality PersonalityRef        `json:"personality"`
	Budget      ChangeBudget          `json:"budget"`
}

type UpdateResult struct {
	Version  EngineVersion     `json:"version"`
	Previous RelationshipState `json:"previous"`
	Delta    RelationshipDelta `json:"delta"`
	Next     RelationshipState `json:"next"`
	Budget   ChangeBudget      `json:"budget"`
	Audit    RelationshipAudit `json:"audit"`
}

type RelationshipAudit struct {
	FormulaVersion     string   `json:"formulaVersion"`
	PersonalityVersion string   `json:"personalityVersion,omitempty"`
	EvidenceIDs        []string `json:"evidenceIds,omitempty"`
	Diagnostics        []string `json:"diagnostics,omitempty"`
}

type ConflictEvent struct {
	ID             string    `json:"id,omitempty"`
	SourceEvidence string    `json:"sourceEvidence,omitempty"`
	Intensity      float64   `json:"intensity"`
	TensionBefore  float64   `json:"tensionBefore"`
	TensionAfter   float64   `json:"tensionAfter"`
	SafetyRelated  bool      `json:"safetyRelated"`
	BoundaryActive bool      `json:"boundaryActive"`
	OccurredAt     time.Time `json:"occurredAt,omitempty"`
}

type RepairRecord struct {
	ID               string    `json:"id,omitempty"`
	SourceEvidence   string    `json:"sourceEvidence,omitempty"`
	ConfidenceBefore float64   `json:"confidenceBefore"`
	ConfidenceAfter  float64   `json:"confidenceAfter"`
	TensionBefore    float64   `json:"tensionBefore"`
	TensionAfter     float64   `json:"tensionAfter"`
	Effective        bool      `json:"effective"`
	OccurredAt       time.Time `json:"occurredAt,omitempty"`
}

type TensionDecayProfile struct {
	BaseDecayHourly  float64 `json:"baseDecayHourly"`
	UnresolvedWeight float64 `json:"unresolvedWeight"`
	SafeDecay        bool    `json:"safeDecay"`
}

type DimensionState struct {
	Value       float64   `json:"value"`
	Velocity    float64   `json:"velocity"`
	LastUpdated time.Time `json:"lastUpdated"`
}

type RelationshipDimensions struct {
	Trust      DimensionState `json:"trust"`
	Intimacy   DimensionState `json:"intimacy"`
	Dependency DimensionState `json:"dependency"`
	Conflict   DimensionState `json:"conflict"`
	Repair     DimensionState `json:"repair"`
}

type RelationshipEventType string

const (
	EventTypePositiveInteraction  RelationshipEventType = "positive_interaction"
	EventTypeNegativeInteraction  RelationshipEventType = "negative_interaction"
	EventTypeRepairEffort         RelationshipEventType = "repair_effort"
	EventTypeRupture              RelationshipEventType = "rupture"
	EventTypeBoundaryCrossing     RelationshipEventType = "boundary_crossing"
	EventTypeWithdrawal           RelationshipEventType = "withdrawal"
	EventTypeVulnerabilityShare   RelationshipEventType = "vulnerability_share"
	EventTypeNeutralInteraction   RelationshipEventType = "neutral_interaction"
	EventTypeTemporalReengagement RelationshipEventType = "temporal_reengagement"
)

type RelationshipEvent struct {
	ID              string                `json:"id,omitempty"`
	Type            RelationshipEventType `json:"type"`
	Intensity       float64               `json:"intensity"`
	Confidence      float64               `json:"confidence"`
	SourceMessageID string                `json:"sourceMessageId,omitempty"`
	SourceConvID    string                `json:"sourceConvId,omitempty"`
	ParentEventID   string                `json:"parentEventId,omitempty"`
	CausalChain     []string              `json:"causalChain,omitempty"`
	OccurredAt      time.Time             `json:"occurredAt,omitempty"`
}

type EventImpact struct {
	Dimension string  `json:"dimension"`
	Delta     float64 `json:"delta"`
	Reason    string  `json:"reason"`
}

type EventAccumulation struct {
	MaxSingleDelta float64 `json:"maxSingleDelta"`
	MaxTotalDelta  float64 `json:"maxTotalDelta"`
	Accumulated    float64 `json:"accumulated"`
}

type EventApplyResult struct {
	Previous *RelationshipDimensions `json:"previous"`
	Next     *RelationshipDimensions `json:"next"`
	Impacts  []EventImpact           `json:"impacts"`
	Overflow []EventImpact           `json:"overflow,omitempty"`
}

type RelationshipSnapshot struct {
	State      RelationshipState      `json:"state"`
	Dimensions RelationshipDimensions `json:"dimensions"`
	CapturedAt time.Time              `json:"capturedAt"`
}
