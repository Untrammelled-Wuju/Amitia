package knowledge

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// RerankCandidate is deliberately text-only. A reranker never receives tenant
// credentials, storage keys, raw files or repository handles.
type RerankCandidate struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Reranker is an optional finite-candidate relevance refinement stage. A host
// can plug in a cross-encoder or late-interaction model without changing the
// retrieval/runtime contract. Failures never replace canonical retrieval: the
// runtime falls back to the deterministic local reranker.
type Reranker interface {
	Score(ctx context.Context, query string, candidates []RerankCandidate) ([]float64, error)
}

func (r *RetrievalRuntime) WithReranker(reranker Reranker) *RetrievalRuntime {
	if r != nil {
		r.reranker = reranker
	}
	return r
}

func (r *RetrievalRuntime) WithRerankTimeout(timeout time.Duration) *RetrievalRuntime {
	if r != nil && timeout > 0 {
		r.rerankTimeout = timeout
	}
	return r
}

// rerankEvidenceCandidates refines only a bounded prefiltered candidate set.
// Fusion/RRF remains the recall ordering signal, rerank is relevance, and MMR
// remains a later diversity step. These scores are intentionally separate.
func (r *RetrievalRuntime) rerankEvidenceCandidates(ctx context.Context, query string, candidates []evidenceCandidate) ([]evidenceCandidate, string) {
	if len(candidates) == 0 {
		return []evidenceCandidate{}, ""
	}
	out := append([]evidenceCandidate(nil), candidates...)
	limit := len(out)
	candidateLimit := defaultRuntimeSettings().RerankCandidateLimit
	if r != nil && r.settings.RerankCandidateLimit > 0 {
		candidateLimit = r.settings.RerankCandidateLimit
	}
	if limit > candidateLimit {
		limit = candidateLimit
	}

	// Always compute a deterministic local refinement. This gives an explicit
	// rerank score even when no cross-encoder is configured and is also the
	// timeout/error fallback path.
	for i := range out {
		local := localRerankRelevance(query, out[i])
		out[i].rerankScore = local
		out[i].relevance = blendRelevance(out[i].relevance, local, 0.20)
	}

	warning := ""
	if r != nil && r.reranker != nil && limit >= 2 && strings.TrimSpace(query) != "" {
		timeout := r.rerankTimeout
		if timeout <= 0 {
			timeout = defaultRuntimeSettings().RerankTimeout
		}
		rerankCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		payload := make([]RerankCandidate, limit)
		for i := 0; i < limit; i++ {
			maxTitle, maxContent := 512, 6000
			if r.settings.RerankMaxTitleRunes > 0 {
				maxTitle = r.settings.RerankMaxTitleRunes
			}
			if r.settings.RerankMaxContentRunes > 0 {
				maxContent = r.settings.RerankMaxContentRunes
			}
			payload[i] = RerankCandidate{ID: out[i].readID, Title: truncateRerankText(out[i].read.Title, maxTitle), Content: truncateRerankText(out[i].read.Content, maxContent)}
		}
		scores, err := r.reranker.Score(rerankCtx, query, payload)
		if err != nil {
			warning = "Reranker 不可用，已回退本地重排: " + err.Error()
		} else if len(scores) != limit {
			warning = fmt.Sprintf("Reranker 返回 %d 个分数，期望 %d；已回退本地重排", len(scores), limit)
		} else {
			for i := 0; i < limit; i++ {
				score := clamp01(scores[i])
				out[i].rerankScore = score
				// Cross-encoder refines but never erases hybrid recall evidence.
				out[i].relevance = blendRelevance(out[i].relevance, score, 0.35)
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].relevance != out[j].relevance {
			return out[i].relevance > out[j].relevance
		}
		if out[i].state != nil && out[j].state != nil && out[i].state.fusion != out[j].state.fusion {
			return out[i].state.fusion > out[j].state.fusion
		}
		return out[i].readID < out[j].readID
	})
	return out, warning
}

func localRerankRelevance(query string, candidate evidenceCandidate) float64 {
	query = normalizeContentForFingerprint(query)
	if query == "" {
		return clamp01(candidate.relevance)
	}
	text := normalizeContentForFingerprint(candidate.read.Title + "\n" + candidate.read.Content)
	if text == "" {
		return clamp01(candidate.relevance)
	}
	qFeatures := similarityFeatures(query)
	cFeatures := candidate.features
	if len(cFeatures) == 0 {
		cFeatures = similarityFeatures(text)
	}
	overlap := featureJaccard(qFeatures, cFeatures)
	exactBonus := 0.0
	if strings.Contains(text, query) {
		exactBonus = 1
	}
	// Local fallback intentionally remains conservative: it is a finite-set
	// relevance refinement, not another retrieval source.
	return clamp01(0.72*clamp01(candidate.relevance) + 0.23*overlap + 0.05*exactBonus)
}

func blendRelevance(base, refinement, weight float64) float64 {
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	return clamp01((1-weight)*clamp01(base) + weight*clamp01(refinement))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func truncateRerankText(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
