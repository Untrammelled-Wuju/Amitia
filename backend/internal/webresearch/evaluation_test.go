package webresearch

import "testing"

func TestEvaluateRetrieval(t *testing.T) {
	hits := []SearchHit{
		{URL: "https://noise.example/a"},
		{URL: "https://docs.example/answer?utm_source=test"},
		{URL: "https://other.example/corroboration"},
	}
	gold := map[string]float64{
		"https://docs.example/answer":         3,
		"https://other.example/corroboration": 2,
	}
	metrics := EvaluateRetrieval(hits, gold, 3)
	if metrics.RecallAtK != 1 || metrics.MRR != 0.5 || metrics.NDCGAtK <= 0 || metrics.NDCGAtK >= 1 || metrics.DomainDiversity != 1 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
}

func TestEvaluateCitationIDs(t *testing.T) {
	metrics := EvaluateCitationIDs([]string{"e1", "e2", "bad"}, []string{"e1", "e2", "e3"})
	if metrics.Precision != 2.0/3.0 || metrics.Recall != 2.0/3.0 || metrics.InvalidRate != 1.0/3.0 {
		t.Fatalf("unexpected citation metrics: %+v", metrics)
	}
}

func TestEvaluateDeepResearch(t *testing.T) {
	summary := &ResearchSummary{
		Plan: &ResearchPlan{Questions: []ResearchQuestion{
			{ID: "q1", Required: true}, {ID: "q2", Required: true},
		}},
		Findings: []ResearchFinding{
			{QuestionID: "q1", Status: "corroborated", SourceRefs: []string{"r1", "r2"}},
			{QuestionID: "q2", Status: "insufficient"},
		},
	}
	graph := &EvidenceGraphSummary{ConflictCount: 1, Conflicts: []EvidenceConflict{{ResolutionHint: "prefer newer primary source"}}}
	metrics := EvaluateDeepResearch(summary, graph, []string{"r1", "r3"})
	if metrics.QuestionCoverage != 0.5 || metrics.SourceCoverage != 0.5 || metrics.ConflictHandling != 1 {
		t.Fatalf("unexpected deep research metrics: %+v", metrics)
	}
}
