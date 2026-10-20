package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

type rerankerFunc func(context.Context, string, []RerankCandidate) ([]float64, error)

func (f rerankerFunc) Score(ctx context.Context, query string, candidates []RerankCandidate) ([]float64, error) {
	return f(ctx, query, candidates)
}

func rerankTestCandidate(id, content string, relevance float64) evidenceCandidate {
	chunk := Chunk{ID: id, SourceID: "source-" + id, SourceRevision: 1, UnitID: "unit-" + id, Content: content}
	return evidenceCandidate{
		state: &fusionState{chunkID: id, fusion: relevance}, matched: chunk, read: chunk, readID: id,
		relevance: relevance, features: similarityFeatures(content),
	}
}

func TestRerankStageIsSeparateFromMMR(t *testing.T) {
	runtime := (&RetrievalRuntime{}).WithReranker(rerankerFunc(func(_ context.Context, query string, candidates []RerankCandidate) ([]float64, error) {
		if query != "database lease" || len(candidates) != 2 {
			t.Fatalf("unexpected rerank input query=%q candidates=%d", query, len(candidates))
		}
		return []float64{0.1, 1.0}, nil
	}))
	items, warning := runtime.rerankEvidenceCandidates(context.Background(), "database lease", []evidenceCandidate{
		rerankTestCandidate("a", "database transaction", 1.0),
		rerankTestCandidate("b", "lease prevents duplicate worker", 0.9),
	})
	if warning != "" {
		t.Fatalf("unexpected warning: %s", warning)
	}
	if len(items) != 2 || items[0].matched.ID != "b" {
		t.Fatalf("cross reranker did not refine order: %#v", items)
	}
	if items[0].rerankScore != 1.0 || items[0].mmrScore != 0 {
		t.Fatalf("rerank/MMR scores were conflated: %#v", items[0])
	}
}

func TestRerankFailureFallsBackToLocalScores(t *testing.T) {
	runtime := (&RetrievalRuntime{}).WithReranker(rerankerFunc(func(context.Context, string, []RerankCandidate) ([]float64, error) {
		return nil, errors.New("provider unavailable")
	}))
	items, warning := runtime.rerankEvidenceCandidates(context.Background(), "knowledge global", []evidenceCandidate{
		rerankTestCandidate("a", "knowledge is global within tenant", 0.8),
		rerankTestCandidate("b", "unrelated weather", 0.7),
	})
	if warning == "" || len(items) != 2 {
		t.Fatalf("expected graceful fallback, warning=%q items=%d", warning, len(items))
	}
	if items[0].rerankScore <= 0 || items[0].relevance <= 0 {
		t.Fatalf("local fallback did not produce relevance: %#v", items[0])
	}
}

func TestRerankTimeoutFallsBack(t *testing.T) {
	runtime := (&RetrievalRuntime{}).
		WithRerankTimeout(5 * time.Millisecond).
		WithReranker(rerankerFunc(func(ctx context.Context, _ string, _ []RerankCandidate) ([]float64, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}))
	items, warning := runtime.rerankEvidenceCandidates(context.Background(), "x", []evidenceCandidate{
		rerankTestCandidate("a", "x one", 0.8), rerankTestCandidate("b", "x two", 0.7),
	})
	if warning == "" || len(items) != 2 {
		t.Fatalf("timeout did not degrade cleanly: warning=%q items=%d", warning, len(items))
	}
}

func TestRerankerPayloadIsBounded(t *testing.T) {
	long := make([]rune, 7000)
	for i := range long {
		long[i] = '数'
	}
	runtime := (&RetrievalRuntime{}).WithReranker(rerankerFunc(func(_ context.Context, _ string, candidates []RerankCandidate) ([]float64, error) {
		if got := len([]rune(candidates[0].Content)); got > 6001 {
			t.Fatalf("reranker content exceeded budget: %d", got)
		}
		return []float64{0.8, 0.7}, nil
	}))
	_, _ = runtime.rerankEvidenceCandidates(context.Background(), "query", []evidenceCandidate{
		rerankTestCandidate("a", string(long), 0.8), rerankTestCandidate("b", "short", 0.7),
	})
}
