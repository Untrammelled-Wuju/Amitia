package knowledge

import "testing"

func TestSufficiencyAllowsNoEvidence(t *testing.T) {
	got := evaluateSufficiency("unknown", nil, false)
	if got.Sufficient || got.State != AnswerStateInsufficient || got.Reason != "no_evidence" {
		t.Fatalf("unexpected no-evidence decision: %#v", got)
	}
}

func TestSufficiencyComparisonRequiresDistinctSources(t *testing.T) {
	one := []Evidence{{ID: "a", SourceID: "s1", FusionScore: 0.05}}
	got := evaluateSufficiency("A 和 B 有什么区别", one, false)
	if got.Sufficient || got.Reason != "comparison_source_coverage" {
		t.Fatalf("comparison with one evidence should be insufficient: %#v", got)
	}

	sameSource := append(one, Evidence{ID: "b", SourceID: "s1", FusionScore: 0.04})
	got = evaluateSufficiency("A vs B", sameSource, false)
	if got.Sufficient || got.Reason != "comparison_source_coverage" {
		t.Fatalf("two chunks from one source must not satisfy a comparison: %#v", got)
	}

	twoSources := append(one, Evidence{ID: "b", SourceID: "s2", FusionScore: 0.04})
	got = evaluateSufficiency("A vs B", twoSources, false)
	if !got.Sufficient || got.State != AnswerStateSufficient {
		t.Fatalf("comparison with two canonical sources should pass: %#v", got)
	}
}

func TestSufficiencyRequiresExplicitLocatorCoverage(t *testing.T) {
	items := []Evidence{{
		ID: "a", SourceID: "s1", FusionScore: 0.05,
		CoverageKeys: []string{"page=36"}, Locator: map[string]any{"page": 36},
	}}
	got := evaluateSufficiency("请核对第 37 页原文", items, false)
	if got.Sufficient || got.Reason != "locator_coverage" {
		t.Fatalf("wrong page must not satisfy explicit locator request: %#v", got)
	}

	items[0].CoverageKeys = []string{"pageNumber=37"}
	got = evaluateSufficiency("please check page 37", items, false)
	if !got.Sufficient || got.State != AnswerStateSufficient {
		t.Fatalf("pageNumber alias should satisfy page requirement: %#v", got)
	}

	rows := []Evidence{{
		ID: "row", SourceID: "sheet-source", FusionScore: 0.05,
		Locator: map[string]any{"sheet": "Sheet1", "rowStart": 146, "rowEnd": 146},
	}}
	got = evaluateSufficiency("核对第146行", rows, false)
	if !got.Sufficient {
		t.Fatalf("rowStart/rowEnd aliases should satisfy row locator: %#v", got)
	}
}

func TestSufficiencySurfacesAmbiguousWithoutChoosingWinner(t *testing.T) {
	items := []Evidence{
		{ID: "a", SourceID: "s1", FusionScore: 0.05, ConflictGroup: "launch"},
		{ID: "b", SourceID: "s2", FusionScore: 0.04, ConflictGroup: "launch"},
	}
	got := evaluateSufficiency("launch date", items, true)
	if !got.Sufficient || got.State != AnswerStateAmbiguous || got.Reason != "conflicting_evidence" {
		t.Fatalf("ambiguous evidence should be answerable as ambiguous: %#v", got)
	}
}

func TestSufficiencyRejectsLowQuality(t *testing.T) {
	got := evaluateSufficiency("x", []Evidence{{ID: "a", SourceID: "s", FusionScore: 0.0001}}, false)
	if got.Sufficient || got.Reason != "low_quality" {
		t.Fatalf("low quality evidence must not be forced through: %#v", got)
	}
}
