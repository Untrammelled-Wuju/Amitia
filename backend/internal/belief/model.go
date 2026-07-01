package belief

import "time"

type EvidenceSpan struct {
	SourceMsgID  string `json:"sourceMsgID"`
	SourceStart  int    `json:"sourceStart"`
	SourceEnd    int    `json:"sourceEnd"`
}

type EngineVersion string

const (
	EngineVersionV1 EngineVersion = "belief-resolver-v1"
)

type SourceKind string

const (
	SourceKindFact      SourceKind = "fact"
	SourceKindUser      SourceKind = "user"
	SourceKindMemory    SourceKind = "memory"
	SourceKindInference SourceKind = "inference"
)

type Candidate struct {
	ID         string     `json:"id,omitempty"`
	Key        string     `json:"key"`
	Value      string     `json:"value"`
	Source     SourceKind `json:"source"`
	Confidence float64    `json:"confidence"`
	ObservedAt time.Time  `json:"observedAt,omitempty"`
	ExpiresAt  time.Time     `json:"expiresAt,omitempty"`
	Evidence   EvidenceSpan `json:"evidence,omitempty"`
}

type ResolverPolicy struct {
	MinimumConfidence float64 `json:"minimumConfidence"`
	ConflictGap       float64 `json:"conflictGap"`
	MaxCandidates     int     `json:"maxCandidates"`
}

type ResolvedBelief struct {
	Key          string     `json:"key"`
	Value        string     `json:"value,omitempty"`
	Confidence   float64    `json:"confidence"`
	Source       SourceKind `json:"source,omitempty"`
	ObservedAt   time.Time  `json:"observedAt,omitempty"`
	Conflicted   bool       `json:"conflicted"`
	Unknown      bool       `json:"unknown"`
	CandidateIDs []string   `json:"candidateIds,omitempty"`
}

type ResolveInput struct {
	Key        string         `json:"key"`
	Candidates []Candidate    `json:"candidates,omitempty"`
	Policy     ResolverPolicy `json:"policy"`
	Now        time.Time      `json:"now"`
}

type ResolveResult struct {
	Version  EngineVersion  `json:"version"`
	Belief   ResolvedBelief `json:"belief"`
	Policy   ResolverPolicy `json:"policy"`
	Rejected []string       `json:"rejected,omitempty"`
	Audit    ResolverAudit  `json:"audit"`
}

type ResolverAudit struct {
	FormulaVersion string   `json:"formulaVersion"`
	Diagnostics    []string `json:"diagnostics,omitempty"`
}
