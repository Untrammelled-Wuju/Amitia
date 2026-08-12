package deepsearch

import (
	"testing"
)

func makeHit(url, title string, rank int) ChildSearchResult {
	return ChildSearchResult{
		URL:          url,
		Domain:       domainOf(url),
		Title:        title,
		Snippet:      "Snippet text",
		Rank:         rank,
		CanonicalURL: url,
	}
}

func domainOf(url string) string {
	url = trimPrefix(url, "https://")
	url = trimPrefix(url, "http://")
	if i := indexOfSlash(url); i >= 0 {
		return url[:i]
	}
	return url
}

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

func indexOfSlash(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

func TestAggregator_ExactURLDedup(t *testing.T) {
	agg := NewAggregator(DefaultDeepSearchPolicy())
	n := agg.AddResults([]ChildSearchResult{
		makeHit("https://example.com/a", "Article A", 1),
		makeHit("https://example.com/a", "Article A2", 2),
	}, 1, 0, "")
	if n != 1 {
		t.Fatalf("expected 1 added (deduped), got %d", n)
	}
	if agg.SourceCount() != 1 {
		t.Fatalf("expected 1 source (deduped), got %d", agg.SourceCount())
	}
}

func TestAggregator_NearDuplicateTitle(t *testing.T) {
	agg := NewAggregator(DefaultDeepSearchPolicy())
	n := agg.AddResults([]ChildSearchResult{
		makeHit("https://example.com/a", "Golang Web Frameworks : A Comparison", 1),
		makeHit("https://example.com/b", "Golang Web Frameworks : A Comparison", 2),
	}, 1, 0, "")
	if n != 1 {
		t.Fatalf("expected 1 added (near-duplicate), got %d", n)
	}
}

func TestAggregator_DifferentTitles_Kept(t *testing.T) {
	agg := NewAggregator(DefaultDeepSearchPolicy())
	n := agg.AddResults([]ChildSearchResult{
		makeHit("https://example.com/a", "Gin vs Echo Comparison", 1),
		makeHit("https://example.com/b", "Fiber Framework Tutorial", 2),
	}, 1, 0, "")
	if n != 2 {
		t.Fatalf("expected 2 added, got %d", n)
	}
}

func TestAggregator_SeenCount(t *testing.T) {
	agg := NewAggregator(DefaultDeepSearchPolicy())
	agg.AddResults([]ChildSearchResult{
		makeHit("https://example.com/a", "Article A", 1),
	}, 1, 0, "")
	agg.AddResults([]ChildSearchResult{
		makeHit("https://example.com/a", "Article A", 1),
	}, 2, 0, "")
	agg.AddResults([]ChildSearchResult{
		makeHit("https://example.com/a", "Article A", 1),
	}, 3, 0, "")
	res := agg.BuildResult("test", nil, 3, 3, StopReasonMaxRounds, 1700000000)
	if len(res.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(res.Sources))
	}
	if res.Sources[0].SeenCount < 3 {
		t.Fatalf("expected SeenCount >= 3, got %d", res.Sources[0].SeenCount)
	}
}

func TestAggregator_RankScore(t *testing.T) {
	agg := NewAggregator(DefaultDeepSearchPolicy())
	agg.AddResults([]ChildSearchResult{
		makeHit("https://a.com/page", "First Result", 1),
		makeHit("https://b.com/page", "Tenth Result", 10),
	}, 1, 0, "")
	res := agg.BuildResult("test", nil, 1, 1, StopReasonMaxRounds, 1700000000)
	if len(res.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(res.Sources))
	}
	if res.Sources[0].Score < res.Sources[1].Score {
		t.Fatalf("expected higher score first (rank 1 > rank 10): %f vs %f",
			res.Sources[0].Score, res.Sources[1].Score)
	}
}

func TestAggregator_DomainDiversity(t *testing.T) {
	policy := DefaultDeepSearchPolicy()
	policy.MaxPerDomain = 2
	agg := NewAggregator(policy)
	agg.AddResults([]ChildSearchResult{
		makeHit("https://example.com/a1", "A1", 1),
		makeHit("https://example.com/a2", "A2", 2),
		makeHit("https://example.com/a3", "A3", 3),
		makeHit("https://other.com/b", "B", 4),
	}, 1, 0, "")
	res := agg.BuildResult("test", nil, 1, 1, StopReasonMaxRounds, 1700000000)
	seen := 0
	for _, r := range res.Sources {
		if r.Domain == "example.com" {
			seen++
		}
	}
	if seen != policy.MaxPerDomain {
		t.Fatalf("expected max %d per domain, got %d from example.com", policy.MaxPerDomain, seen)
	}
}

func TestAggregator_BuildResult_Coverage(t *testing.T) {
	agg := NewAggregator(DefaultDeepSearchPolicy())
	agg.AddResults([]ChildSearchResult{
		makeHit("https://example.com/a", "A1", 1),
		makeHit("https://example.com/b", "A2", 2),
	}, 1, 0, "security")
	res := agg.BuildResult("test", []string{"security"}, 1, 1, StopReasonMaxRounds, 1700000000)
	if len(res.Coverage.FocusAreas) != 1 {
		t.Fatalf("expected 1 focus area in coverage, got %d", len(res.Coverage.FocusAreas))
	}
	if !res.Coverage.FocusAreas[0].Covered {
		t.Fatalf("expected security focus area to be covered (hitCount=%d >= threshold=%d)",
			res.Coverage.FocusAreas[0].HitCount, DefaultDeepSearchPolicy().FocusHitThreshold)
	}
	if res.Coverage.UniqueSources != 2 {
		t.Fatalf("expected 2 unique sources, got %d", res.Coverage.UniqueSources)
	}
	if res.StoppedReason != StopReasonMaxRounds {
		t.Fatalf("expected stop reason %q, got %q", StopReasonMaxRounds, res.StoppedReason)
	}
}

func TestNormalizeTitle(t *testing.T) {
	cases := map[string]string{
		"  Hello   World  ":      "hello world",
		"Golang Web Frameworks!": "golang web frameworks",
		"Test 123 (Updated)":     "test 123 updated",
	}
	for input, expected := range cases {
		got := normalizeTitle(input)
		if got != expected {
			t.Fatalf("normalizeTitle(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestTitleSimilarity(t *testing.T) {
	a := "golang web frameworks comparison"
	b := "golang web frameworks comparison"
	if sim := titleSimilarity(a, b); sim < 0.9 {
		t.Fatalf("identical titles should have high similarity, got %f", sim)
	}
	c := "completely different title xyz"
	if sim := titleSimilarity(a, c); sim >= 0.9 {
		t.Fatalf("different titles should have low similarity, got %f", sim)
	}
	empty := ""
	if sim := titleSimilarity(a, empty); sim != 0.0 {
		t.Fatalf("empty title should have 0 similarity, got %f", sim)
	}
}
