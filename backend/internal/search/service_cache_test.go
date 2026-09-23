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
		GeneralWeb:          true,
		SearchKinds:         []SearchKind{SearchKindWeb},
		LanguageFilter:      true,
		CountryFilter:       true,
		SafeSearch:          true,
		Pagination:          true,
		TimeRangeFilter:     true,
		DomainFilter:        true,
		ExcludeDomainFilter: true,
		MaxResults:          20,
	}
}

func (p *cacheTestProvider) Search(_ context.Context, req SearchRequest) (ProviderSearchResponse, error) {
	p.calls.Add(1)
	return ProviderSearchResponse{
		Results: []SearchResult{{
			Title:   "cached result",
			URL:     "https://example.com/result",
			Snippet: req.Query,
		}},
		Usage: ProviderUsage{CostUSD: 0.0125, Credits: 1},
	}, nil
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
	if first.Usage.CostUSD != 0.0125 || first.Usage.Credits != 1 {
		t.Fatalf("first usage=%#v, want provider usage", first.Usage)
	}
	second, err := service.SearchAdvancedWithProvider(context.Background(), req, "invoke-2", "primary")
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if !second.CacheHit {
		t.Fatal("second search should be served from cache")
	}
	if second.Usage.CostUSD != 0 || second.Usage.Credits != 0 {
		t.Fatalf("cache hit must not charge provider usage: %#v", second.Usage)
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

func TestCandidateProviderIDsUsesConfiguredRouteOrder(t *testing.T) {
	primary := &cacheTestProvider{}
	backup := &cacheTestProvider{}
	providers := NewProviderSet("local_backup")
	providers.RegisterWithPriority("local_backup", primary, 100)
	providers.RegisterWithPriority("brave_primary", backup, 90)
	service := NewService(Config{
		Enabled:         true,
		DefaultProvider: "local_backup",
		Providers: map[string]ProviderConfig{
			"local_backup":  {Enabled: true},
			"brave_primary": {Enabled: true},
		},
		Routes: map[string]ProviderRouteConfig{
			"general": {Preferred: []string{"brave_primary"}, Fallback: []string{"local_backup"}},
		},
	}, providers)

	got := service.CandidateProviderIDs(SearchKindWeb)
	if len(got) != 2 || got[0] != "brave_primary" || got[1] != "local_backup" {
		t.Fatalf("unexpected route order: %#v", got)
	}
}

func TestSearchAdvancedTagsConfiguredProviderInstance(t *testing.T) {
	provider := &cacheTestProvider{}
	providers := NewProviderSet("brave_primary")
	providers.RegisterWithPriority("brave_primary", provider, 100)
	service := NewService(Config{
		Enabled:         true,
		DefaultProvider: "brave_primary",
		Providers: map[string]ProviderConfig{
			"brave_primary": {Enabled: true},
		},
	}, providers)

	resp, err := service.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: "instance identity", Kind: SearchKindWeb}, "invoke", "brave_primary")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if resp.Provider != "brave_primary" {
		t.Fatalf("response provider=%q, want brave_primary", resp.Provider)
	}
	if len(resp.Results) != 1 || resp.Results[0].Source.Provider != "brave_primary" {
		t.Fatalf("result provider identity not normalized to configured instance: %#v", resp.Results)
	}
}

func TestServiceNegativeCacheUsesShortTTLAndMetrics(t *testing.T) {
	providers := NewProviderSet("empty")
	empty := &fakeProviderForService{enabled: true}
	providers.RegisterWithPriority("empty", empty, 100)
	service := NewService(Config{
		Enabled:          true,
		DefaultProvider:  "empty",
		CacheTTL:         time.Minute,
		NegativeCacheTTL: 20 * time.Millisecond,
		Providers: map[string]ProviderConfig{
			"empty": {Enabled: true},
		},
	}, providers)

	req := SearchRequest{Query: "no results", Kind: SearchKindWeb}
	first, err := service.SearchAdvancedWithProvider(context.Background(), req, "invoke", "empty")
	if err != nil {
		t.Fatalf("first search: %v", err)
	}
	if first.Returned != 0 || first.CacheHit {
		t.Fatalf("unexpected first response: %#v", first)
	}
	second, err := service.SearchAdvancedWithProvider(context.Background(), req, "invoke", "empty")
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if !second.CacheHit {
		t.Fatal("empty response should be negative-cache hit")
	}
	metrics := service.CacheMetrics()
	if metrics.NegativeHit != 1 || metrics.Miss < 1 {
		t.Fatalf("unexpected cache metrics: %#v", metrics)
	}
	time.Sleep(30 * time.Millisecond)
	third, err := service.SearchAdvancedWithProvider(context.Background(), req, "invoke", "empty")
	if err != nil {
		t.Fatalf("third search: %v", err)
	}
	if third.CacheHit {
		t.Fatal("negative cache entry should expire using the shorter TTL")
	}
}

func TestSearchCacheKeySeparatesExcludedDomains(t *testing.T) {
	service := NewService(Config{Enabled: true, CacheTTL: time.Minute}, NewProviderSet(""))
	base := SearchRequest{Query: "same query", Kind: SearchKindWeb, Domains: []string{"docs.example.com"}}
	first := base
	first.ExcludeDomains = []string{"spam.example"}
	second := base
	second.ExcludeDomains = []string{"ads.example"}
	if service.searchCacheKey("provider", first) == service.searchCacheKey("provider", second) {
		t.Fatal("different excludeDomains must not share the same cache key")
	}

	third := base
	third.ExcludeDomains = []string{"SPAM.EXAMPLE", " spam.example "}
	if service.searchCacheKey("provider", first) != service.searchCacheKey("provider", third) {
		t.Fatal("equivalent excludeDomains should canonicalize to the same cache key")
	}
}

func TestSearchCacheKeyCanonicalizesQueryCaseAndWhitespace(t *testing.T) {
	service := NewService(Config{Enabled: true, CacheTTL: time.Minute}, NewProviderSet(""))
	first := SearchRequest{Query: "  Codex   Search  ", Kind: SearchKindWeb}
	second := SearchRequest{Query: "codex search", Kind: SearchKindWeb}
	if service.searchCacheKey("provider", first) != service.searchCacheKey("provider", second) {
		t.Fatal("query case and repeated whitespace should canonicalize to the same cache key")
	}
}
