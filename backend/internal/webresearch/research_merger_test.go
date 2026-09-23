package webresearch

import "testing"

func TestMergeResearchFindingsTracksEvidenceAndCorroboration(t *testing.T) {
	plan := ResearchPlan{Questions: []ResearchQuestion{{ID: "q1", Question: "Amitia browser support", Required: true}}}
	citations := []Citation{
		{EvidenceID: "e1", RefID: "r1", Title: "Browser docs", Text: "Amitia browser support includes dynamic page extraction."},
		{EvidenceID: "e2", RefID: "r2", Title: "Release notes", Text: "Dynamic browser support is available in Amitia."},
	}
	findings := mergeResearchFindings(plan, citations)
	if len(findings) != 1 || findings[0].Status != "corroborated" {
		t.Fatalf("findings = %+v", findings)
	}
	if len(findings[0].EvidenceIDs) != 2 || len(findings[0].SourceRefs) != 2 || findings[0].Summary == "" {
		t.Fatalf("finding does not preserve traceability: %+v", findings[0])
	}
}

func TestMergeResearchFindingsLeavesUnsupportedQuestionInsufficient(t *testing.T) {
	plan := ResearchPlan{Questions: []ResearchQuestion{{ID: "q1", Question: "quantum networking", Required: true}}}
	citations := []Citation{{EvidenceID: "e1", RefID: "r1", Text: "This document discusses CSS layout."}}
	findings := mergeResearchFindings(plan, citations)
	if len(findings) != 1 || findings[0].Status != "insufficient" || len(findings[0].EvidenceIDs) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}
