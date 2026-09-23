package native

import (
	"math"
	"net/url"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/search"
)

type rankedResult struct {
	result  search.SearchResult
	score   float64
	sources map[string]struct{}
}

func fuseResults(results []search.SearchResult, limit int, weights ...map[string]float64) []search.SearchResult {
	if limit <= 0 {
		limit = 20
	}
	weightFor := func(provider string) float64 {
		if len(weights) == 0 || weights[0] == nil {
			return 1
		}
		weight := weights[0][strings.ToLower(strings.TrimSpace(provider))]
		if weight <= 0 {
			return 1
		}
		return weight
	}
	byURL := make(map[string]*rankedResult, len(results))
	for _, result := range results {
		key := canonicalResultURL(result.URL)
		if key == "" {
			continue
		}
		rank := result.Rank
		if rank <= 0 {
			rank = 1
		}
		weight := weightFor(result.Source.Provider)
		score := weight / float64(rank)
		source := strings.TrimSpace(result.Source.Provider)
		if existing, ok := byURL[key]; ok {
			existing.score += score * 0.35
			if existing.result.Snippet == "" && result.Snippet != "" {
				existing.result.Snippet = result.Snippet
			}
			if existing.result.PublishedAt == nil && result.PublishedAt != nil {
				existing.result.PublishedAt = result.PublishedAt
			}
			if source != "" {
				if _, exists := existing.sources[source]; !exists {
					existing.sources[source] = struct{}{}
					existing.score += 0.25
				}
			}
			continue
		}
		item := &rankedResult{result: result, score: score, sources: map[string]struct{}{}}
		if source != "" {
			item.sources[source] = struct{}{}
		}
		byURL[key] = item
	}
	items := make([]*rankedResult, 0, len(byURL))
	for _, item := range byURL {
		item.score += float64(len(item.sources)) * 0.2
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if math.Abs(items[i].score-items[j].score) > 1e-9 {
			return items[i].score > items[j].score
		}
		if items[i].result.Rank != items[j].result.Rank {
			return items[i].result.Rank < items[j].result.Rank
		}
		return items[i].result.URL < items[j].result.URL
	})
	if len(items) > limit {
		items = items[:limit]
	}
	fused := make([]search.SearchResult, 0, len(items))
	for index, item := range items {
		item.result.Rank = index + 1
		fused = append(fused, item.result)
	}
	return fused
}

func canonicalResultURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return ""
	}
	parsed.Fragment = ""
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "ref" || lower == "source" {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
