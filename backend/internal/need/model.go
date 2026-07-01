package need

import "time"

type EngineVersion string

const (
	EngineVersionV1 EngineVersion = "need-engine-v1"
)

type NeedKind string

const (
	NeedKindReassurance NeedKind = "reassurance"
	NeedKindConnection  NeedKind = "connection"
	NeedKindAutonomy    NeedKind = "autonomy"
	NeedKindClarity     NeedKind = "clarity"
	NeedKindRest        NeedKind = "rest"
	NeedKindExpression  NeedKind = "expression"
	NeedKindNovelty     NeedKind = "novelty"
)

type NeedState struct {
	Level     float64   `json:"level"`
	Baseline  float64   `json:"baseline"`
	Trend     float64   `json:"trend"`
	Saturated bool      `json:"saturated"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type NeedSnapshot struct {
	Version   EngineVersion          `json:"version"`
	States    map[NeedKind]NeedState `json:"states"`
	UpdatedAt time.Time              `json:"updatedAt,omitempty"`
}

type NeedSignal struct {
	Kind       NeedKind `json:"kind"`
	Pressure   float64  `json:"pressure"`
	Relief     float64  `json:"relief"`
	Confidence float64  `json:"confidence"`
}

type PersonalityRef struct {
	Version          string  `json:"version,omitempty"`
	Sensitivity      float64 `json:"sensitivity"`
	Stability        float64 `json:"stability"`
	RecoveryBias     float64 `json:"recoveryBias"`
	AttachmentBias   float64 `json:"attachmentBias"`
	BoundaryStrength float64 `json:"boundaryStrength"`
}

type ChangeBudget struct {
	MaxLevelDelta float64 `json:"maxLevelDelta"`
	MaxTrendDelta float64 `json:"maxTrendDelta"`
}

type UpdateInput struct {
	Current     NeedSnapshot   `json:"current"`
	Signals     []NeedSignal   `json:"signals,omitempty"`
	Personality PersonalityRef `json:"personality"`
	Budget      ChangeBudget   `json:"budget"`
	Now         time.Time      `json:"now"`
}

type NeedDelta struct {
	Level float64 `json:"level"`
	Trend float64 `json:"trend"`
}

type UpdateResult struct {
	Version EngineVersion          `json:"version"`
	Before  NeedSnapshot           `json:"before"`
	Delta   map[NeedKind]NeedDelta `json:"delta"`
	After   NeedSnapshot           `json:"after"`
	Budget  ChangeBudget           `json:"budget"`
	Audit   Audit                  `json:"audit"`
}

type Audit struct {
	FormulaVersion     string   `json:"formulaVersion"`
	PersonalityVersion string   `json:"personalityVersion,omitempty"`
	ElapsedHours       float64  `json:"elapsedHours"`
	Diagnostics        []string `json:"diagnostics,omitempty"`
}
