package appraisal

type Appraisal struct {
	Version                string  `json:"version"`
	GoalRelevance          float64 `json:"goalRelevance"`
	GoalCongruence         float64 `json:"goalCongruence"`
	Expectedness           float64 `json:"expectedness"`
	Novelty                float64 `json:"novelty"`
	Controllability        float64 `json:"controllability"`
	Responsibility         float64 `json:"responsibility"`
	CausalUncertainty      float64 `json:"causalUncertainty"`
	RelationshipRelevance  float64 `json:"relationshipRelevance"`
	NormViolation          float64 `json:"normViolation"`
	BoundaryViolation      float64 `json:"boundaryViolation"`
	MemoryResonance        float64 `json:"memoryResonance"`
	AlternativeExplanation float64 `json:"alternativeExplanation"`
	OverallSeverity        float64 `json:"overallSeverity"`
	Explanation            string  `json:"explanation"`
	SourceEvent            string  `json:"sourceEvent"`
	EventType              string  `json:"eventType"`
	Modulated              bool    `json:"modulated"`
}

type AppraisalConfig struct {
	GoalRelevanceWeight          float64 `json:"goalRelevanceWeight"`
	GoalCongruenceWeight         float64 `json:"goalCongruenceWeight"`
	ExpectednessWeight           float64 `json:"expectednessWeight"`
	NoveltyWeight                float64 `json:"noveltyWeight"`
	ControllabilityWeight        float64 `json:"controllabilityWeight"`
	ResponsibilityWeight         float64 `json:"responsibilityWeight"`
	CausalUncertaintyWeight      float64 `json:"causalUncertaintyWeight"`
	RelationshipRelevanceWeight  float64 `json:"relationshipRelevanceWeight"`
	NormViolationWeight          float64 `json:"normViolationWeight"`
	BoundaryViolationWeight      float64 `json:"boundaryViolationWeight"`
	MemoryResonanceWeight        float64 `json:"memoryResonanceWeight"`
	AlternativeExplanationWeight float64 `json:"alternativeExplanationWeight"`
}

func DefaultAppraisalConfig() AppraisalConfig {
	return AppraisalConfig{
		RelationshipRelevanceWeight:  1.5,
		NormViolationWeight:          1.3,
		BoundaryViolationWeight:      1.2,
		GoalRelevanceWeight:          1.0,
		GoalCongruenceWeight:         1.0,
		ExpectednessWeight:           0.8,
		NoveltyWeight:                0.7,
		ControllabilityWeight:        0.6,
		ResponsibilityWeight:         0.5,
		CausalUncertaintyWeight:      0.4,
		MemoryResonanceWeight:        0.3,
		AlternativeExplanationWeight: 0.2,
	}
}

type AppraisalInput struct {
	EventType                 string  `json:"eventType"`
	Source                    string  `json:"source"`
	IsUserInitiated           bool    `json:"isUserInitiated"`
	RelatesToGoal             bool    `json:"relatesToGoal"`
	GoalCongruent             bool    `json:"goalCongruent"`
	IsExpected                float64 `json:"isExpected"`
	InvolvesRelation          bool    `json:"involvesRelation"`
	NormViolated              bool    `json:"normViolated"`
	BoundaryViolated          bool    `json:"boundaryViolated"`
	HasAlternativeExplanation bool    `json:"hasAlternativeExplanation"`
	SimilarPastEvents         int     `json:"similarPastEvents"`
	Controllable              bool    `json:"controllable"`
	Responsibility            float64 `json:"responsibility"`
	Uncertainty               float64 `json:"uncertainty"`
}
