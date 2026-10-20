package knowledge

import (
	"context"
	"testing"
)

func f64(value float64) *float64 { return &value }

func TestBenchmarkNoAnswerAccuracyOnlyUsesNoAnswerCases(t *testing.T) {
	calls := 0
	search := func(_ context.Context, _ string, _ SearchRequest) (SearchResponse, error) {
		calls++
		if calls == 1 {
			return SearchResponse{
				RetrievalLevel: RetrievalLevelFast,
				AnswerState:    AnswerStateSufficient,
				Sufficient:     true,
				Evidence:       []Evidence{{ID: "e-positive", SourceID: "s-positive", SourceRevision: 1}},
			}, nil
		}
		return SearchResponse{RetrievalLevel: RetrievalLevelFast, AnswerState: AnswerStateInsufficient, Sufficient: false}, nil
	}
	resp, err := runRetrievalBenchmark(context.Background(), "tenant-1", BenchmarkRequest{
		K: 1,
		Cases: []BenchmarkCase{
			{ID: "positive", Query: "known fact", Relevant: []BenchmarkRelevantEvidence{{EvidenceID: "e-positive", Grade: 3}}},
			{ID: "no-answer", Query: "unknown fact", ExpectedNoAnswer: true},
		},
	}, defaultRuntimeSettings(), search)
	if err != nil {
		t.Fatalf("run benchmark: %v", err)
	}
	if resp.Summary.PositiveCases != 1 || resp.Summary.NoAnswerCases != 1 {
		t.Fatalf("unexpected case counters: %+v", resp.Summary)
	}
	if resp.Summary.NoAnswerAccuracy != 1 {
		t.Fatalf("no-answer accuracy must only use no-answer cases, got %v", resp.Summary.NoAnswerAccuracy)
	}
}

func TestBenchmarkCaseAndSuiteGatesAreMachineReadable(t *testing.T) {
	search := func(_ context.Context, _ string, _ SearchRequest) (SearchResponse, error) {
		return SearchResponse{
			RetrievalLevel: RetrievalLevelFast,
			AnswerState:    AnswerStateSufficient,
			Sufficient:     true,
			Evidence:       []Evidence{{ID: "wrong", SourceID: "wrong-source", SourceRevision: 1}},
		}, nil
	}
	resp, err := runRetrievalBenchmark(context.Background(), "tenant-1", BenchmarkRequest{
		K: 1,
		Gate: BenchmarkGate{
			MinRecallAtK: f64(1),
			MaxErrorRate: f64(0),
		},
		Cases: []BenchmarkCase{{
			ID:       "cross-document-comparison",
			Scenario: "cross_document_comparison",
			Query:    "compare both sources",
			Relevant: []BenchmarkRelevantEvidence{{EvidenceID: "expected", Grade: 3}},
			Expect: BenchmarkCaseExpectation{
				ExpectedRetrievalLevel: RetrievalLevelDeep,
				MinRecallAtK:           f64(1),
			},
		}},
	}, defaultRuntimeSettings(), search)
	if err != nil {
		t.Fatalf("run benchmark: %v", err)
	}
	if resp.Summary.Passed {
		t.Fatalf("suite must fail quality gates: %+v", resp.Summary)
	}
	if resp.Summary.ScenarioCounts["cross_document_comparison"] != 1 {
		t.Fatalf("scenario count missing: %+v", resp.Summary.ScenarioCounts)
	}
	if len(resp.Results) != 1 || resp.Results[0].Passed {
		t.Fatalf("case expectation must fail: %+v", resp.Results)
	}
	if len(resp.Results[0].Violations) < 2 {
		t.Fatalf("expected retrieval + recall violations, got %+v", resp.Results[0].Violations)
	}
}

func TestValidateBenchmarkGateRejectsImpossibleThresholds(t *testing.T) {
	tooHigh := 1.01
	negative := int64(-1)
	if err := validateBenchmarkRequest(BenchmarkRequest{
		Gate:  BenchmarkGate{MinRecallAtK: &tooHigh},
		Cases: []BenchmarkCase{{Query: "q", ExpectedNoAnswer: true}},
	}); err == nil {
		t.Fatal("expected invalid aggregate gate to be rejected")
	}
	if err := validateBenchmarkRequest(BenchmarkRequest{
		Cases: []BenchmarkCase{{Query: "q", ExpectedNoAnswer: true, Expect: BenchmarkCaseExpectation{MaxLatencyMs: negative}}},
	}); err == nil {
		t.Fatal("expected invalid case gate to be rejected")
	}
}

func TestBenchmarkScenarioDefaultsWithoutChangingOldClients(t *testing.T) {
	search := func(_ context.Context, _ string, _ SearchRequest) (SearchResponse, error) {
		return SearchResponse{RetrievalLevel: RetrievalLevelFast, AnswerState: AnswerStateInsufficient}, nil
	}
	resp, err := runRetrievalBenchmark(context.Background(), "tenant-1", BenchmarkRequest{
		Cases: []BenchmarkCase{{Query: "unknown", ExpectedNoAnswer: true}},
	}, defaultRuntimeSettings(), search)
	if err != nil {
		t.Fatalf("run benchmark: %v", err)
	}
	if resp.Results[0].Scenario != "default" || resp.Summary.ScenarioCounts["default"] != 1 {
		t.Fatalf("legacy benchmark request lost default scenario: result=%+v summary=%+v", resp.Results[0], resp.Summary)
	}
}
