package knowledge

import (
	"context"
	"errors"
	"testing"
)

func TestEvaluateBenchmarkRankingUsesUniqueLabelsAndGrades(t *testing.T) {
	observed := []benchmarkObservedEvidence{
		{EvidenceID: "e2", SourceID: "s2", SourceRevision: 1},
		{EvidenceID: "noise", SourceID: "noise", SourceRevision: 1},
		{EvidenceID: "e1", SourceID: "s1", SourceRevision: 1},
	}
	labels := []BenchmarkRelevantEvidence{
		{EvidenceID: "e1", Grade: 3},
		{EvidenceID: "e2", Grade: 2},
	}
	got := evaluateBenchmarkRanking(observed, labels, 3)
	if got.RecallAtK != 1 || got.ContextRecall != 1 {
		t.Fatalf("expected full recall, got %+v", got)
	}
	if got.ContextPrecision != 2.0/3.0 {
		t.Fatalf("unexpected context precision: %v", got.ContextPrecision)
	}
	if got.MRR != 1 {
		t.Fatalf("first result is relevant, mrr=%v", got.MRR)
	}
	if got.NDCGAtK <= 0 || got.NDCGAtK >= 1 {
		t.Fatalf("graded order is intentionally suboptimal, ndcg=%v", got.NDCGAtK)
	}
}

func TestEvaluateBenchmarkCitationMetricsRejectUnretrievedCitation(t *testing.T) {
	observed := []benchmarkObservedEvidence{{EvidenceID: "e1", SourceID: "s1", SourceRevision: 2}}
	labels := []BenchmarkRelevantEvidence{{EvidenceID: "e1", Grade: 3}}
	precision, recall, grounded, ok := evaluateBenchmarkCitations([]string{"e1", "fabricated"}, observed, labels)
	if !ok || precision != 0.5 || recall != 1 || grounded != 0.5 {
		t.Fatalf("unexpected citation metrics: precision=%v recall=%v grounded=%v ok=%v", precision, recall, grounded, ok)
	}
}

func TestRunRetrievalBenchmarkDoesNotRequireTraceAndKeepsPartialErrors(t *testing.T) {
	settings := defaultRuntimeSettings()
	settings.ConfigHash = "cfg-test"
	calls := 0
	fake := func(ctx context.Context, tenantID string, req SearchRequest) (SearchResponse, error) {
		calls++
		if !req.SkipTrace {
			t.Fatalf("benchmark retrieval must not pollute production traces")
		}
		if req.Query == "boom" {
			return SearchResponse{}, errors.New("boom")
		}
		return SearchResponse{
			Evidence:       []Evidence{{ID: "e1", SourceID: "s1", SourceRevision: 1}},
			Sufficient:     true,
			AnswerState:    AnswerStateSufficient,
			RetrievalLevel: RetrievalLevelFast,
		}, nil
	}
	resp, err := runRetrievalBenchmark(context.Background(), "tenant-1", BenchmarkRequest{K: 1, Cases: []BenchmarkCase{
		{ID: "ok", Query: "q", Relevant: []BenchmarkRelevantEvidence{{EvidenceID: "e1", Grade: 3}}, CitationEvidenceIDs: []string{"e1"}},
		{ID: "err", Query: "boom", Relevant: []BenchmarkRelevantEvidence{{EvidenceID: "e2", Grade: 1}}},
	}}, settings, fake)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || resp.Summary.SuccessfulCases != 1 || resp.Summary.ErrorCases != 1 {
		t.Fatalf("unexpected benchmark summary: %+v", resp.Summary)
	}
	if resp.Summary.ConfigHash != "cfg-test" || resp.Summary.RecallAtK != 1 || resp.Summary.Groundedness != 1 {
		t.Fatalf("unexpected benchmark metrics: %+v", resp.Summary)
	}
}

func TestValidateBenchmarkRequestRequiresGroundTruthForAnswerableCase(t *testing.T) {
	if err := validateBenchmarkRequest(BenchmarkRequest{Cases: []BenchmarkCase{{Query: "answerable"}}}); err == nil {
		t.Fatal("expected validation error")
	}
	if err := validateBenchmarkRequest(BenchmarkRequest{Cases: []BenchmarkCase{{Query: "no answer", ExpectedNoAnswer: true}}}); err != nil {
		t.Fatalf("no-answer case should not require relevance labels: %v", err)
	}
}
