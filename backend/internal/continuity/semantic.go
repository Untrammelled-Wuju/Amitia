package continuity

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Embedder interface {
	Embed(string) ([]float32, error)
}

type BatchEmbedder interface {
	BatchEmbed([]string) ([][]float32, error)
}

type SemanticMatcher interface {
	Scores(context.Context, string, []Thread) (map[string]float64, error)
}

type ThreadDisambiguator interface {
	ChooseThread(context.Context, string, []Thread) (string, float64, error)
}

type EmbeddingSemanticMatcher struct {
	embedder Embedder
	mu       sync.RWMutex
	cache    map[string][]float32
}

func NewEmbeddingSemanticMatcher(embedder Embedder) *EmbeddingSemanticMatcher {
	return &EmbeddingSemanticMatcher{embedder: embedder, cache: map[string][]float32{}}
}

func (m *EmbeddingSemanticMatcher) Scores(_ context.Context, message string, threads []Thread) (map[string]float64, error) {
	out := map[string]float64{}
	if m == nil || m.embedder == nil || strings.TrimSpace(message) == "" || len(threads) == 0 {
		return out, nil
	}

	vectors := make(map[string][]float32, len(threads))
	missing := make([]Thread, 0, len(threads))
	for i := range threads {
		key := semanticCacheKey(threads[i])
		m.mu.RLock()
		vec := m.cache[key]
		m.mu.RUnlock()
		if len(vec) > 0 {
			vectors[threads[i].ID] = vec
		} else {
			missing = append(missing, threads[i])
		}
	}

	var query []float32
	if batch, ok := m.embedder.(BatchEmbedder); ok && len(missing) > 0 {
		texts := make([]string, 0, len(missing)+1)
		texts = append(texts, message)
		for i := range missing {
			texts = append(texts, threadSemanticText(missing[i]))
		}
		if embedded, err := batch.BatchEmbed(texts); err == nil && len(embedded) == len(texts) && len(embedded[0]) > 0 {
			query = embedded[0]
			for i := range missing {
				vec := embedded[i+1]
				if len(vec) == 0 {
					continue
				}
				vectors[missing[i].ID] = vec
				m.putCachedVector(semanticCacheKey(missing[i]), vec)
			}
		}
	}
	if len(query) == 0 {
		var err error
		query, err = m.embedder.Embed(message)
		if err != nil || len(query) == 0 {
			return out, err
		}
	}

	for i := range missing {
		if len(vectors[missing[i].ID]) > 0 {
			continue
		}
		vec, err := m.embedder.Embed(threadSemanticText(missing[i]))
		if err != nil || len(vec) == 0 {
			continue
		}
		vectors[missing[i].ID] = vec
		m.putCachedVector(semanticCacheKey(missing[i]), vec)
	}
	for i := range threads {
		if vec := vectors[threads[i].ID]; len(vec) > 0 {
			out[threads[i].ID] = cosineSimilarity(query, vec)
		}
	}
	return out, nil
}

func semanticCacheKey(thread Thread) string {
	return thread.ID + ":" + strconv.FormatInt(thread.Revision, 10)
}

func (m *EmbeddingSemanticMatcher) putCachedVector(key string, vector []float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.cache) > 512 {
		m.cache = map[string][]float32{}
	}
	m.cache[key] = vector
}

func threadSemanticText(thread Thread) string {
	return strings.Join([]string{thread.Title, thread.Goal, thread.Summary, thread.CurrentState, thread.NextAction}, "\n")
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		av, bv := float64(a[i]), float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

type LLMThreadDisambiguator struct {
	generator WorkshopJSONGenerator
}

func NewLLMThreadDisambiguator(generator WorkshopJSONGenerator) *LLMThreadDisambiguator {
	return &LLMThreadDisambiguator{generator: generator}
}

func (d *LLMThreadDisambiguator) ChooseThread(ctx context.Context, message string, candidates []Thread) (string, float64, error) {
	if d == nil || d.generator == nil || len(candidates) == 0 {
		return "", 0, nil
	}
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	items := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, map[string]any{"id": candidate.ID, "title": candidate.Title, "goal": candidate.Goal, "currentState": candidate.CurrentState, "nextAction": candidate.NextAction})
	}
	body, _ := json.Marshal(map[string]any{"message": message, "candidates": items})
	system := `Select which existing persistent thread the user's message refers to. Return JSON only: {"threadId":"candidate id or empty","confidence":0.0}. Never invent an id. If ambiguous, unrelated, or insufficient evidence, return an empty threadId. A vague word like "继续" is not enough to choose among multiple unrelated candidates without supporting context.`
	raw, _, _, err := d.generator.GenerateWorkshopJSON(ctx, system, string(body))
	if err != nil {
		return "", 0, err
	}
	var result struct {
		ThreadID   string  `json:"threadId"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(raw)), &result); err != nil {
		return "", 0, err
	}
	allowed := map[string]struct{}{}
	for _, candidate := range candidates {
		allowed[candidate.ID] = struct{}{}
	}
	if _, ok := allowed[result.ThreadID]; !ok {
		return "", 0, nil
	}
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 1 {
		result.Confidence = 1
	}
	return result.ThreadID, result.Confidence, nil
}

type threadCandidateScore struct {
	thread Thread
	score  float64
}

func rankThreadCandidates(message string, items []Thread, semantic map[string]float64) []threadCandidateScore {
	out := make([]threadCandidateScore, 0, len(items))
	for _, item := range items {
		lexical := scoreForThread(message, item)
		semanticScore := semantic[item.ID]
		score := math.Max(lexical, semanticScore*0.92)
		if lexical > 0 && semanticScore > 0 {
			score = math.Max(score, lexical*0.55+semanticScore*0.45)
		}
		out = append(out, threadCandidateScore{thread: item, score: score})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}
