package knowledge

import "testing"

func TestBenchmarkLabelsDeduplicateByIdentityAndKeepHighestGrade(t *testing.T) {
	labels := normalizedBenchmarkLabels([]BenchmarkRelevantEvidence{
		{EvidenceID: "e1", Grade: 1},
		{EvidenceID: "e1", Grade: 3},
		{SourceID: "s1", SourceRevision: 2, Grade: 2},
	})
	if len(labels) != 2 {
		t.Fatalf("expected 2 normalized labels, got %d", len(labels))
	}
	if labels[0].Grade != 3 {
		t.Fatalf("duplicate label must keep strongest grade, got %d", labels[0].Grade)
	}
}

func TestBenchmarkValidationBoundsCasesAndLabels(t *testing.T) {
	tooMany := make([]BenchmarkCase, maxBenchmarkCases+1)
	for i := range tooMany {
		tooMany[i] = BenchmarkCase{Query: "q", ExpectedNoAnswer: true}
	}
	if err := validateBenchmarkRequest(BenchmarkRequest{Cases: tooMany}); err == nil {
		t.Fatal("expected case limit error")
	}
	labels := make([]BenchmarkRelevantEvidence, maxBenchmarkLabelsPerCase+1)
	for i := range labels {
		labels[i] = BenchmarkRelevantEvidence{EvidenceID: "e"}
	}
	if err := validateBenchmarkRequest(BenchmarkRequest{Cases: []BenchmarkCase{{Query: "q", Relevant: labels}}}); err == nil {
		t.Fatal("expected label limit error")
	}
}
