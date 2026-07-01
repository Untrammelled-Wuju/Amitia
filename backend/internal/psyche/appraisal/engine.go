package appraisal

import "fmt"

type Engine struct {
	config AppraisalConfig
}

func NewEngine(config AppraisalConfig) *Engine {
	return &Engine{config: config}
}

func (e *Engine) Evaluate(input AppraisalInput) Appraisal {
	a := Appraisal{
		EventType: input.EventType,
		Version:   "appraisal-v1",
	}

	if input.RelatesToGoal {
		a.GoalRelevance = 0.80
	} else {
		a.GoalRelevance = 0.30
	}
	if input.GoalCongruent && input.RelatesToGoal {
		a.GoalCongruence = 0.85
	} else if !input.GoalCongruent && input.RelatesToGoal {
		a.GoalCongruence = 0.20
	} else {
		a.GoalCongruence = 0.50
	}
	a.Expectedness = clamp01(input.IsExpected)
	a.Novelty = clamp01(1.0 - input.IsExpected)
	if input.Controllable {
		a.Controllability = 0.70
	} else {
		a.Controllability = 0.25
	}
	a.Responsibility = clamp01(input.Responsibility)
	a.CausalUncertainty = clamp01(input.Uncertainty)
	if input.InvolvesRelation {
		a.RelationshipRelevance = 0.80
	} else {
		a.RelationshipRelevance = 0.15
	}
	if input.NormViolated {
		a.NormViolation = 0.75
	} else {
		a.NormViolation = 0.15
	}
	if input.BoundaryViolated {
		a.BoundaryViolation = 0.85
	} else {
		a.BoundaryViolation = 0.10
	}
	switch {
	case input.SimilarPastEvents >= 10:
		a.MemoryResonance = 0.90
	case input.SimilarPastEvents >= 5:
		a.MemoryResonance = 0.70
	case input.SimilarPastEvents >= 2:
		a.MemoryResonance = 0.40
	default:
		a.MemoryResonance = 0.10
	}
	if input.HasAlternativeExplanation {
		a.AlternativeExplanation = 0.70
	} else {
		a.AlternativeExplanation = 0.10
	}

	a.OverallSeverity = e.ComputeSeverity(a)

	return a
}

func (e *Engine) ModulateAppraisal(a Appraisal, rejectionSens, relRelevanceSens, expectGapSens, uncertaintySens, boundarySens float64) Appraisal {
	a.GoalRelevance = clamp01(a.GoalRelevance * (1.0 + relRelevanceSens*0.3))
	a.GoalCongruence = clamp01(a.GoalCongruence * (1.0 + expectGapSens*0.2))
	a.Expectedness = clamp01(a.Expectedness * (1.0 - rejectionSens*0.15))
	a.Novelty = clamp01(a.Novelty * (1.0 + rejectionSens*0.2))
	a.NormViolation = clamp01(a.NormViolation * (1.0 + boundarySens*0.3))
	a.BoundaryViolation = clamp01(a.BoundaryViolation * (1.0 + boundarySens*0.35))
	a.RelationshipRelevance = clamp01(a.RelationshipRelevance * (1.0 + relRelevanceSens*0.25))
	a.CausalUncertainty = clamp01(a.CausalUncertainty * (1.0 + uncertaintySens*0.3))
	a.Controllability = clamp01(a.Controllability * (1.0 - uncertaintySens*0.2))
	a.OverallSeverity = e.ComputeSeverity(a)
	a.Modulated = true
	return a
}

func (e *Engine) ComputeSeverity(a Appraisal) float64 {
	raw := a.GoalRelevance*e.config.GoalRelevanceWeight +
		a.GoalCongruence*e.config.GoalCongruenceWeight +
		a.Expectedness*e.config.ExpectednessWeight +
		a.Novelty*e.config.NoveltyWeight +
		a.Controllability*e.config.ControllabilityWeight +
		a.Responsibility*e.config.ResponsibilityWeight +
		a.CausalUncertainty*e.config.CausalUncertaintyWeight +
		a.RelationshipRelevance*e.config.RelationshipRelevanceWeight +
		a.NormViolation*e.config.NormViolationWeight +
		a.BoundaryViolation*e.config.BoundaryViolationWeight +
		a.MemoryResonance*e.config.MemoryResonanceWeight +
		a.AlternativeExplanation*e.config.AlternativeExplanationWeight
	totalWeight := e.config.GoalRelevanceWeight +
		e.config.GoalCongruenceWeight +
		e.config.ExpectednessWeight +
		e.config.NoveltyWeight +
		e.config.ControllabilityWeight +
		e.config.ResponsibilityWeight +
		e.config.CausalUncertaintyWeight +
		e.config.RelationshipRelevanceWeight +
		e.config.NormViolationWeight +
		e.config.BoundaryViolationWeight +
		e.config.MemoryResonanceWeight +
		e.config.AlternativeExplanationWeight
	if totalWeight == 0 {
		return 0
	}
	return clamp01(raw / totalWeight)
}

func (e *Engine) String(a Appraisal) string {
	return fmt.Sprintf("Appraisal{severity=%.2f, goalRel=%.2f, goalCong=%.2f, expect=%.2f, novel=%.2f, control=%.2f, resp=%.2f, uncert=%.2f, relRel=%.2f, normViol=%.2f, boundViol=%.2f, memRes=%.2f, altExpl=%.2f}",
		a.OverallSeverity, a.GoalRelevance, a.GoalCongruence, a.Expectedness, a.Novelty,
		a.Controllability, a.Responsibility, a.CausalUncertainty, a.RelationshipRelevance,
		a.NormViolation, a.BoundaryViolation, a.MemoryResonance, a.AlternativeExplanation)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
