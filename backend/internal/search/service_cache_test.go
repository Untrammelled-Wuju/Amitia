package search

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type cacheTestProvider struct {
	calls atomic.Int32
}

func (p *cacheTestProvider) ID() string { return "cache-test" }

func (p *cacheTestProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		GeneralWeb:      true,
		SearchKinds:     []SearchKind{SearchKindWeb},
		LanguageFilter:  true,
		CountryFilter:   true,
		SafeSearch:      true,
		Pagination:      true,
		TimeRangeFilter: true,
		DomainFilter:    true,
		MaxResults:      20,
	}
}

func (p *cacheTestProvider) Search(_ context.Context, req SearchRequest) (ProviderSearchResponse, error) {
	p.calls.Add(1)
	return ProviderSearchResponse{Results: []SearchResult{{
		Title:   "cached result",
		URL:     "https://example.com/result",
		Snippet: req.Query,
	}}}, nil
}

func (p *cacheTestProvider) Health(context.Context) ProviderHealth { return ProviderHealthReady }

func TestServiceSearchAdvancedUsesCache(t *testing.T) {
	provider := &cacheTestProvider{}
	providers := NewProviderSet("primary")
	providers.RegisterWithPriority("primary", provider, 100)
	service := NewService(Config{
		Enabled:         true,
		DefaultProvider: "primary",
		DefaultLimit:    8,
		MaxLimit:        20,
		CacheTTL:        time.Minute,
		Providers: map[string]ProviderConfig{
			"primary": {Enabled: true},
		},
	}, providers)

	req := SearchRequest{Query: "amitia", Kind: SearchKindWeb, Limit: 5}
	first, err := service.SearchAdvancedWithProvider(context.Background(), req, "invoke-1", "primary")
	if err != nil {
		t.Fatalf("first search: %v", err)
	}
	if first.CacheHit {
		t.Fatal("first search must not be a cache hit")
	}
	second, err := service.SearchAdvancedWithProvider(context.Background(), req, "invoke-2", "primary")
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if !second.CacheHit {
		t.Fatal("second search should be served from cache")
	}
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("provider called %d times, want 1", got)
	}
}

func TestServiceSearchCacheCanBeDisabled(t *testing.T) {
	provider := &cacheTestProvider{}
	providers := NewProviderSet("primary")
	providers.RegisterWithPriority("primary", provider, 100)
	service := NewService(Config{
		Enabled:         true,
		DefaultProvider: "primary",
		DefaultLimit:    8,
		MaxLimit:        20,
		CacheTTL:        -1,
		Providers: map[string]ProviderConfig{
			"primary": {Enabled: true},
		},
	}, providers)

	req := SearchRequest{Query: "amitia", Kind: SearchKindWeb, Limit: 5}
	for i := 0; i < 2; i++ {
		resp, err := service.SearchAdvancedWithProvider(context.Background(), req, "invoke", "primary")
		if err != nil {
			t.Fatalf("search %d: %v", i, err)
		}
		if resp.CacheHit {
			t.Fatalf("search %d unexpectedly hit cache", i)
		}
	}
	if got := provider.calls.Load(); got != 2 {
		t.Fatalf("provider called %d times, want 2", got)
	}
}

func TestSearchCacheKeyCanonicalizesRecencyAndDomains(t *testing.T) {
	provider := &cacheTestProvider{}
	providers := NewProviderSet("primary")
	providers.RegisterWithPriority("primary", provider, 100)
	service := NewService(Config{
		Enabled:         true,
		DefaultProvider: "primary",
		DefaultLimit:    8,
		MaxLimit:        20,
		CacheTTL:        time.Minute,
		Providers: map[string]ProviderConfig{
			"primary": {Enabled: true},
		},
	}, providers)

	base := time.Date(2026, 9, 23, 10, 12, 20, 0, time.UTC)
	firstFrom := base
	secondFrom := base.Add(30 * time.Second)
	firstReq := SearchRequest{
		Query:     "  Amitia   web research ",
		Kind:      SearchKindWeb,
		Limit:     5,
		Domains:   []string{"GitHub.COM", "docs.example.com", "github.com"},
		TimeRange: &TimeRangeFilter{From: &firstFrom},
	}
	secondReq := SearchRequest{
		Query:     "Amitia web research",
		Kind:      SearchKindWeb,
		Limit:     5,
		Domains:   []string{"docs.example.com", "github.com"},
		TimeRange: &TimeRangeFilter{From: &secondFrom},
	}
	if _, err := service.SearchAdvancedWithProvider(context.Background(), firstReq, "invoke-1", "primary"); err != nil {
		t.Fatalf("first search: %v", err)
	}
	second, err := service.SearchAdvancedWithProvider(context.Background(), secondReq, "invoke-2", "primary")
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if !second.CacheHit {
		t.Fatal("equivalent recency/domain request should hit cache")
	}
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("provider called %d times, want 1", got)
	}
}

func TestServiceSearchCacheEvictsWhenCapacityReached(t *testing.T) {
	provider := &cacheTestProvider{}
	providers := NewProviderSet("primary")
	providers.RegisterWithPriority("primary", provider, 100)
	service := NewService(Config{
		Enabled:         true,
		DefaultProvider: "primary",
		DefaultLimit:    8,
		MaxLimit:        20,
		CacheTTL:        time.Minute,
		CacheMaxEntries: 2,
		Providers: map[string]ProviderConfig{
			"primary": {Enabled: true},
		},
	}, providers)

	for _, query := range []string{"one", "two", "three"} {
		if _, err := service.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: query, Kind: SearchKindWeb}, "invoke", "primary"); err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
	}
	if got := len(service.cache); got != 2 {
		t.Fatalf("cache size=%d, want 2", got)
	}
	if _, ok := service.cache[service.searchCacheKey("primary", SearchRequest{Query: "one", Kind: SearchKindWeb})]; ok {
		t.Fatal("oldest cache entry should have been evicted")
	}
}
