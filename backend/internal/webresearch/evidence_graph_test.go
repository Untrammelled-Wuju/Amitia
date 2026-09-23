package webresearch

import "testing"

func TestDetectEvidenceConflictsVersionValue(t *testing.T) {
	citations := []Citation{
		{EvidenceID: "e1", RefID: "r1", Text: "The Amitia runtime version 2.0 is released and available now."},
		{EvidenceID: "e2", RefID: "r2", Text: "The Amitia runtime version 3.0 is released and available now."},
	}
	conflicts := detectEvidenceConflicts(citations)
	if len(conflicts) == 0 || conflicts[0].Kind != "value_conflict" {
		t.Fatalf("conflicts = %+v", conflicts)
	}
}

func TestDetectEvidenceConflictsSkipsDifferentPlatformScope(t *testing.T) {
	citations := []Citation{
		{EvidenceID: "e1", RefID: "r1", Text: "Windows supports feature X."},
		{EvidenceID: "e2", RefID: "r2", Text: "Linux does not support feature X."},
	}
	if conflicts := detectEvidenceConflicts(citations); len(conflicts) != 0 {
		t.Fatalf("scope-specific facts should not conflict: %+v", conflicts)
	}
}

func TestBuildEvidenceGraphSummary(t *testing.T) {
	out := &ToolOutput{
		Search: []SearchHit{{RefID: "s1", URL: "https://a.example/x"}},
		Pages:  []Page{{RefID: "p1", SourceRefID: "s1"}},
		Citations: []Citation{
			{EvidenceID: "e1", RefID: "p1", Text: "Feature X is available."},
		},
	}
	graph := buildEvidenceGraphSummary(out)
	if graph == nil || graph.SourceCount != 1 || graph.PageCount != 1 || graph.EvidenceCount != 1 {
		t.Fatalf("graph = %+v", graph)
	}
}

func TestDetectEvidenceConflictsMarksTemporalChange(t *testing.T) {
	citations := []Citation{
		{EvidenceID: "e1", RefID: "r1", Text: "In 2025, feature X is not available for Windows users."},
		{EvidenceID: "e2", RefID: "r2", Text: "In 2026, feature X is available for Windows users."},
	}
	conflicts := detectEvidenceConflicts(citations)
	if len(conflicts) == 0 || conflicts[0].Kind != "temporal_change" {
		t.Fatalf("expected temporal change, got %+v", conflicts)
	}
	if len(conflicts[0].LeftScope.Years) != 1 || len(conflicts[0].RightScope.Years) != 1 {
		t.Fatalf("expected year scopes, got %+v", conflicts[0])
	}
}

func TestDetectEvidenceConflictsSkipsDifferentRegionScope(t *testing.T) {
	citations := []Citation{
		{EvidenceID: "e1", RefID: "r1", Text: "Feature X is available in China."},
		{EvidenceID: "e2", RefID: "r2", Text: "Feature X is not available in Europe."},
	}
	if conflicts := detectEvidenceConflicts(citations); len(conflicts) != 0 {
		t.Fatalf("region-specific facts should not conflict: %+v", conflicts)
	}
}

func TestEvidenceGraphClaimsBindEvidenceToStableCitationIndexes(t *testing.T) {
	out := &ToolOutput{
		Citations: []Citation{
			{Index: 3, EvidenceID: "e1", RefID: "r1", Text: "first evidence"},
			{Index: 7, EvidenceID: "e2", RefID: "r2", Text: "second evidence"},
		},
		Research: &ResearchSummary{Findings: []ResearchFinding{{
			QuestionID: "q1", Question: "Does X work?", Status: "corroborated",
			Summary: "X works according to two sources.", EvidenceIDs: []string{"e2", "e1"},
		}}},
	}
	graph := buildEvidenceGraphSummary(out)
	if graph == nil || graph.ClaimCount != 1 || len(graph.Claims) != 1 {
		t.Fatalf("graph = %+v", graph)
	}
	claim := graph.Claims[0]
	if len(claim.CitationIndexes) != 2 || claim.CitationIndexes[0] != 3 || claim.CitationIndexes[1] != 7 {
		t.Fatalf("claim citation mapping = %+v", claim)
	}
}
