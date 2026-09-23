package webresearch

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/search"
)

type semanticRerankerTestFunc func(context.Context, string, []SemanticRerankCandidate) ([]float64, error)

func (f semanticRerankerTestFunc) Score(ctx context.Context, query string, candidates []SemanticRerankCandidate) ([]float64, error) {
	return f(ctx, query, candidates)
}

func TestSemanticRerankRefinesExistingRanking(t *testing.T) {
	runtime := (&Runtime{}).WithSemanticReranker(semanticRerankerTestFunc(func(_ context.Context, query string, candidates []SemanticRerankCandidate) ([]float64, error) {
		if query != "provider routing" || len(candidates) != 2 {
			t.Fatalf("unexpected rerank input query=%q candidates=%d", query, len(candidates))
		}
		return []float64{0, 1}, nil
	}))
	items := []fusedResult{
		{result: search.SearchResult{URL: "https://a.example", Title: "A"}, score: 0.7},
		{result: search.SearchResult{URL: "https://b.example", Title: "B"}, score: 0.65},
	}
	used, err := runtime.semanticRerank(context.Background(), "provider routing", items)
	if err != nil || !used {
		t.Fatalf("used=%v err=%v", used, err)
	}
	if items[0].result.Title != "B" {
		t.Fatalf("semantic ranking not applied: %#v", items)
	}
}

func TestSemanticRerankFailureDoesNotMutateRanking(t *testing.T) {
	runtime := (&Runtime{}).WithSemanticReranker(semanticRerankerTestFunc(func(context.Context, string, []SemanticRerankCandidate) ([]float64, error) {
		return nil, errors.New("embedding unavailable")
	}))
	items := []fusedResult{
		{result: search.SearchResult{URL: "https://a.example", Title: "A"}, score: 0.7},
		{result: search.SearchResult{URL: "https://b.example", Title: "B"}, score: 0.65},
	}
	used, err := runtime.semanticRerank(context.Background(), "query", items)
	if err == nil || used {
		t.Fatalf("used=%v err=%v", used, err)
	}
	if items[0].result.Title != "A" {
		t.Fatalf("ranking changed on rerank failure: %#v", items)
	}
}

func TestSemanticPageEvidenceRerankUsesPrefilteredChunks(t *testing.T) {
	called := 0
	runtime := (&Runtime{config: DefaultConfig()}).WithSemanticReranker(semanticRerankerTestFunc(func(_ context.Context, query string, candidates []SemanticRerankCandidate) ([]float64, error) {
		called++
		if query != "semantic target" {
			t.Fatalf("query=%q", query)
		}
		if len(candidates) > 20 {
			t.Fatalf("semantic page rerank received %d candidates, want <=20", len(candidates))
		}
		scores := make([]float64, len(candidates))
		for i := range scores {
			if candidates[i].Snippet == "semantic-only target evidence" {
				scores[i] = 1
			}
		}
		return scores, nil
	}))
	runtime.config.MaxEvidencePerPage = 4
	blocks := []string{
		"semantic target lexical match one",
		"semantic target lexical match two",
		"semantic target lexical match three",
		"semantic target lexical match four",
		"semantic-only target evidence",
	}
	page := Page{RefID: "p", ConversationID: "c", URL: "https://example.com", Title: "Doc", ContentHash: "h"}
	items := runtime.buildPageEvidence(context.Background(), page, "semantic target", blocks)
	if called != 1 {
		t.Fatalf("reranker calls=%d, want 1", called)
	}
	if len(items) != 4 {
		t.Fatalf("items=%d, want 4", len(items))
	}
	found := false
	for _, item := range items {
		if item.Text == "semantic-only target evidence" {
			found = true
		}
	}
	if !found {
		t.Fatalf("semantic evidence was not promoted: %#v", items)
	}
}
