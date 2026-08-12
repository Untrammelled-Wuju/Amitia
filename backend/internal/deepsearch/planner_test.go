package deepsearch

import (
	"testing"
)

func TestDeterministic_PlanInitial_OnlyQuery(t *testing.T) {
	p := NewDeterministicRoundPlanner(DefaultDeepSearchPolicy())
	req := DeepSearchRequest{Query: "golang web framework"}
	plans := p.PlanInitial(req)
	if len(plans) < 1 {
		t.Fatal("expected at least 1 plan")
	}
	if plans[0].Query != "golang web framework" {
		t.Fatalf("expected seed query, got %q", plans[0].Query)
	}
	if plans[0].Reason != "seed" {
		t.Fatalf("expected reason=seed, got %q", plans[0].Reason)
	}
}

func TestDeterministic_PlanInitial_WithFocusAreas(t *testing.T) {
	p := NewDeterministicRoundPlanner(DefaultDeepSearchPolicy())
	req := DeepSearchRequest{
		Query:      "golang web framework",
		FocusAreas: []string{"performance", "security"},
	}
	plans := p.PlanInitial(req)
	if len(plans) != 3 {
		t.Fatalf("expected 3 plans (seed + 2 focus), got %d", len(plans))
	}
	if plans[1].Query != "golang web framework performance" {
		t.Fatalf("unexpected plan query: %q", plans[1].Query)
	}
	if plans[1].FocusArea != "performance" {
		t.Fatalf("expected focus area 'performance', got %q", plans[1].FocusArea)
	}
}

func TestDeterministic_PlanInitial_DedupeDuplicateFocus(t *testing.T) {
	p := NewDeterministicRoundPlanner(DefaultDeepSearchPolicy())
	req := DeepSearchRequest{
		Query:      "golang web framework",
		FocusAreas: []string{"performance", "performance"},
	}
	plans := p.PlanInitial(req)
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans (seed + 1 deduped focus), got %d", len(plans))
	}
}

func TestDeterministic_PlanNext_NoFocusAreas(t *testing.T) {
	p := NewDeterministicRoundPlanner(DefaultDeepSearchPolicy())
	req := DeepSearchRequest{Query: "test"}
	state := NewSearchState()
	state.CurrentRound = 1
	plans := p.PlanNext(req, state)
	if len(plans) != 1 {
		t.Fatalf("expected 1 clarification plan, got %d", len(plans))
	}
	if plans[0].Reason != "coverage_gap" {
		t.Fatalf("expected coverage_gap reason, got %q", plans[0].Reason)
	}
}

func TestDeterministic_PlanNext_CoveredFocusSkips(t *testing.T) {
	p := NewDeterministicRoundPlanner(DefaultDeepSearchPolicy())
	req := DeepSearchRequest{
		Query:      "test",
		FocusAreas: []string{"architecture", "security"},
	}
	state := NewSearchState()
	state.CurrentRound = 1
	state.Coverage["architecture"] = 5
	state.Coverage["security"] = 0
	plans := p.PlanNext(req, state)
	for _, pl := range plans {
		if pl.FocusArea == "architecture" {
			t.Fatal("should not generate plan for already-covered focus area")
		}
	}
}

func TestDeterministic_PlanNext_MaxRoundsReached(t *testing.T) {
	policy := DefaultDeepSearchPolicy()
	p := NewDeterministicRoundPlanner(policy)
	req := DeepSearchRequest{Query: "test"}
	state := NewSearchState()
	state.CurrentRound = policy.MaxRounds - 1
	plans := p.PlanNext(req, state)
	if len(plans) != 0 {
		t.Fatalf("expected 0 plans at max rounds, got %d", len(plans))
	}
}

func TestNormalizeQuery(t *testing.T) {
	cases := map[string]string{
		"  Hello   World  ": "hello world",
		"Golang Web":        "golang web",
		"Test":              "test",
	}
	for input, expected := range cases {
		got := normalize(input)
		if got != expected {
			t.Fatalf("normalize(%q) = %q, want %q", input, got, expected)
		}
	}
}
