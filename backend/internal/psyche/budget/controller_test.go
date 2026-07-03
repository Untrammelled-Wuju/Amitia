package budget

import (
	"math"
	"testing"
)

func TestBudgetControllerAllocatesAllWithinBudget(t *testing.T) {
	bc := NewBudgetController(1.0)
	candidates := []CandidateDelta{
		{Module: "affect", Delta: 0.2, Priority: 1},
		{Module: "need", Delta: 0.3, Priority: 2},
		{Module: "belief", Delta: 0.1, Priority: 3},
	}
	result := bc.Allocate(1.0, candidates)
	if len(result.FinalDeltas) != 3 {
		t.Errorf("expected 3 final deltas, got %d", len(result.FinalDeltas))
	}
	if len(result.Rejected) != 0 {
		t.Errorf("expected 0 rejected, got %d", len(result.Rejected))
	}
	expectedTotal := 0.2 + 0.3 + 0.1
	if math.Abs(result.TotalAllocated-expectedTotal) > 1e-9 {
		t.Errorf("expected total %f, got %f", expectedTotal, result.TotalAllocated)
	}
}

func TestBudgetControllerRejectsLowPriorityWhenExhausted(t *testing.T) {
	bc := NewBudgetController(1.0)
	candidates := []CandidateDelta{
		{Module: "affect", Delta: 0.8, Priority: 1},
		{Module: "need", Delta: 0.5, Priority: 2},
	}
	result := bc.Allocate(0.5, candidates)
	if len(result.FinalDeltas) != 1 {
		t.Fatalf("expected 1 final delta, got %d", len(result.FinalDeltas))
	}
	if len(result.Rejected) != 1 {
		t.Errorf("expected 1 rejected, got %d", len(result.Rejected))
	}
	if result.Rejected[0].Module != "need" {
		t.Errorf("expected need rejected, got %s", result.Rejected[0].Module)
	}
}

func TestBudgetControllerScalesPartialFit(t *testing.T) {
	bc := NewBudgetController(1.0)
	candidates := []CandidateDelta{
		{Module: "affect", Delta: 0.8, Priority: 1},
		{Module: "need", Delta: 0.5, Priority: 2},
	}
	result := bc.Allocate(1.0, candidates)
	if len(result.FinalDeltas) != 2 {
		t.Fatalf("expected 2 final deltas, got %d", len(result.FinalDeltas))
	}
	scaled := result.FinalDeltas[1]
	if math.Abs(math.Abs(scaled.Delta)-0.2) > 1e-9 {
		t.Errorf("expected scaled delta 0.2, got %f", scaled.Delta)
	}
}

func TestBudgetControllerZeroSeverity(t *testing.T) {
	bc := NewBudgetController(1.0)
	candidates := []CandidateDelta{
		{Module: "affect", Delta: 0.5, Priority: 1},
	}
	result := bc.Allocate(0, candidates)
	if result.TotalAllocated <= 0 {
		t.Error("expected minimum allocation > 0")
	}
}

func TestBudgetControllerNegativeSeverity(t *testing.T) {
	bc := NewBudgetController(1.0)
	candidates := []CandidateDelta{
		{Module: "affect", Delta: 0.1, Priority: 1},
	}
	result := bc.Allocate(-0.5, candidates)
	if result.TotalAllocated <= 0 {
		t.Error("expected minimum allocation > 0 for negative severity")
	}
}

func TestBudgetControllerSortsByPriority(t *testing.T) {
	bc := NewBudgetController(1.0)
	candidates := []CandidateDelta{
		{Module: "low", Delta: 0.1, Priority: 10},
		{Module: "high", Delta: 0.1, Priority: 1},
		{Module: "mid", Delta: 0.1, Priority: 5},
	}
	result := bc.Allocate(1.0, candidates)
	if result.FinalDeltas[0].Module != "high" {
		t.Errorf("expected high first, got %s", result.FinalDeltas[0].Module)
	}
	if result.FinalDeltas[1].Module != "mid" {
		t.Errorf("expected mid second, got %s", result.FinalDeltas[1].Module)
	}
	if result.FinalDeltas[2].Module != "low" {
		t.Errorf("expected low third, got %s", result.FinalDeltas[2].Module)
	}
}

func TestComputeEventSeverity(t *testing.T) {
	severity := ComputeEventSeverity(0.8, 0.6, 0.4, 0.2)
	if severity < 0 || severity > 1 {
		t.Errorf("expected severity in [0,1], got %f", severity)
	}
}

func TestComputeEventSeverityClampsTo1(t *testing.T) {
	severity := ComputeEventSeverity(2.0, 2.0, 2.0, 2.0)
	if severity != 1.0 {
		t.Errorf("expected 1.0, got %f", severity)
	}
}

func TestComputeEventSeverityClampsTo0(t *testing.T) {
	severity := ComputeEventSeverity(-1, -1, -1, -1)
	if severity != 0 {
		t.Errorf("expected 0, got %f", severity)
	}
}
