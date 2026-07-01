package appraisal

import (
	"testing"
)

func TestEvaluateBasicAppraisal(t *testing.T) {
	eng := NewEngine(DefaultAppraisalConfig())
	input := AppraisalInput{
		EventType:               "interaction",
		RelatesToGoal:           true,
		GoalCongruent:           false,
		IsExpected:              0.2,
		InvolvesRelation:        true,
		NormViolated:            true,
		BoundaryViolated:        true,
		HasAlternativeExplanation: false,
		SimilarPastEvents:       1,
		Controllable:            false,
		Responsibility:          0.8,
		Uncertainty:             0.7,
	}
	a := eng.Evaluate(input)
	if a.GoalRelevance < 0.7 || a.NormViolation < 0.6 || a.BoundaryViolation < 0.7 {
		t.Fatalf("expected high severity appraisal, got severity=%.2f", a.OverallSeverity)
	}
	if a.OverallSeverity < 0.4 {
		t.Fatalf("expected severity >= 0.4 for this scenario, got %.2f", a.OverallSeverity)
	}
}

func TestEvaluateLowSeverityAppraisal(t *testing.T) {
	eng := NewEngine(DefaultAppraisalConfig())
	input := AppraisalInput{
		EventType:               "interaction",
		RelatesToGoal:           false,
		GoalCongruent:           true,
		IsExpected:              0.9,
		InvolvesRelation:        false,
		NormViolated:            false,
		BoundaryViolated:        false,
		HasAlternativeExplanation: true,
		SimilarPastEvents:       15,
		Controllable:            true,
		Responsibility:          0.2,
		Uncertainty:             0.1,
	}
	a := eng.Evaluate(input)
	if a.OverallSeverity > 0.4 {
		t.Fatalf("expected low severity appraisal, got severity=%.2f", a.OverallSeverity)
	}
}

func TestModulateAppraisal(t *testing.T) {
	eng := NewEngine(DefaultAppraisalConfig())
	input := AppraisalInput{
		EventType:        "conflict",
		RelatesToGoal:    true,
		GoalCongruent:    false,
		IsExpected:       0.1,
		InvolvesRelation: true,
		NormViolated:     true,
		BoundaryViolated: true,
		SimilarPastEvents: 3,
		Uncertainty:      0.5,
	}
	base := eng.Evaluate(input)
	modulated := eng.ModulateAppraisal(base, 0.8, 0.7, 0.6, 0.9, 0.5)
	if !modulated.Modulated {
		t.Fatal("expected modulated=true after modulation")
	}
	if modulated.NormViolation < base.NormViolation || modulated.BoundaryViolation < base.BoundaryViolation {
		t.Fatal("expected modulated values >= base for this high-sensitivity scenario")
	}
}

func TestComputeSeverity(t *testing.T) {
	eng := NewEngine(DefaultAppraisalConfig())
	a := Appraisal{
		GoalRelevance:         1.0,
		GoalCongruence:        1.0,
		Expectedness:          1.0,
		Novelty:               0.0,
		Controllability:       0.0,
		Responsibility:        0.0,
		CausalUncertainty:     0.0,
		RelationshipRelevance: 1.0,
		NormViolation:         0.0,
		BoundaryViolation:     0.0,
		MemoryResonance:       1.0,
		AlternativeExplanation: 1.0,
	}
	s := eng.ComputeSeverity(a)
	if s < 0.3 || s > 0.7 {
		t.Fatalf("expected moderate severity for mixed input, got %.2f", s)
	}
}
