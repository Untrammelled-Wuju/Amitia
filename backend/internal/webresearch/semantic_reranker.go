package webresearch

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// SemanticRerankCandidate is intentionally text-only: rerankers do not receive
// cookies, credentials, private workspace data, or full fetched page bodies.
type SemanticRerankCandidate struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

// SemanticReranker is an optional host capability. Implementations may use a
// local embedding model or a configured embedding service. The web runtime
// never falls back to an LLM rerank for every page.
type SemanticReranker interface {
	Score(ctx context.Context, query string, candidates []SemanticRerankCandidate) ([]float64, error)
}

func (r *Runtime) WithSemanticReranker(reranker SemanticReranker) *Runtime {
	if r != nil {
		r.reranker = reranker
	}
	return r
}

func (r *Runtime) semanticRerank(ctx context.Context, query string, items []fusedResult) (bool, error) {
	if r == nil || r.reranker == nil || len(items) < 2 || strings.TrimSpace(query) == "" {
		return false, nil
	}
	candidates := make([]SemanticRerankCandidate, len(items))
	for i, item := range items {
		candidates[i] = SemanticRerankCandidate{URL: item.result.URL, Title: item.result.Title, Snippet: item.result.Snippet}
	}
	scores, err := r.reranker.Score(ctx, query, candidates)
	if err != nil {
		return false, err
	}
	if len(scores) != len(items) {
		return false, fmt.Errorf("semantic reranker returned %d scores for %d candidates", len(scores), len(items))
	}
	for i := range items {
		score := scores[i]
		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}
		// Semantic relevance refines rather than replaces provider/lexical/
		// freshness/authority signals, so a single embedding provider cannot
		// completely dominate ranking.
		items[i].score = items[i].score*0.80 + score*0.20
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		if items[i].seenCount != items[j].seenCount {
			return items[i].seenCount > items[j].seenCount
		}
		return fusedCanonical(items[i]) < fusedCanonical(items[j])
	})
	return true, nil
}

func rerankQuery(commands []SearchQueryCommand) string {
	parts := make([]string, 0, len(commands))
	for _, command := range commands {
		if value := strings.TrimSpace(command.Q); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(dedupeStrings(parts), " ; ")
}

func (r *Runtime) buildPageEvidence(ctx context.Context, page Page, query string, fallbackBlocks []string) []Evidence {
	if r == nil {
		return buildEvidenceForPage(page, query, fallbackBlocks, 8, 3500)
	}
	candidateLimit := r.config.MaxEvidencePerPage
	if candidateLimit <= 0 {
		candidateLimit = 8
	}
	if r.reranker != nil && strings.TrimSpace(query) != "" && candidateLimit < 20 {
		candidateLimit = 20
	}
	items := buildEvidenceForPage(page, query, fallbackBlocks, candidateLimit, r.config.MaxEvidenceChars)
	if r.reranker == nil || strings.TrimSpace(query) == "" || len(items) < 2 {
		return trimEvidence(items, r.config.MaxEvidencePerPage)
	}
	candidates := make([]SemanticRerankCandidate, len(items))
	for i, item := range items {
		candidates[i] = SemanticRerankCandidate{URL: page.URL, Title: page.Title, Snippet: item.Text}
	}
	scores, err := r.reranker.Score(ctx, query, candidates)
	if err != nil || len(scores) != len(items) {
		return trimEvidence(items, r.config.MaxEvidencePerPage)
	}
	for i := range items {
		score := scores[i]
		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}
		// Keep lexical/BM25-style relevance dominant and use semantic similarity
		// only as a refinement over a small prefiltered candidate set.
		items[i].Relevance = items[i].Relevance*0.75 + score*0.25
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Relevance != items[j].Relevance {
			return items[i].Relevance > items[j].Relevance
		}
		if items[i].Locator.Page != items[j].Locator.Page {
			return items[i].Locator.Page < items[j].Locator.Page
		}
		return items[i].Locator.BlockIndex < items[j].Locator.BlockIndex
	})
	return trimEvidence(items, r.config.MaxEvidencePerPage)
}

func trimEvidence(items []Evidence, limit int) []Evidence {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
