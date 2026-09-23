package webresearch

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/search"
)

type chaosSearchProvider struct {
	id      string
	err     error
	results []search.SearchResult
	calls   int
}

func (p *chaosSearchProvider) ID() string { return p.id }
func (p *chaosSearchProvider) Capabilities() search.ProviderCapabilities {
	return search.ProviderCapabilities{GeneralWeb: true, MaxResults: 20}
}
func (p *chaosSearchProvider) Search(ctx context.Context, req search.SearchRequest) (search.ProviderSearchResponse, error) {
	p.calls++
	if err := ctx.Err(); err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_CANCELLED, p.id, false, err)
	}
	if p.err != nil {
		return search.ProviderSearchResponse{}, p.err
	}
	return search.ProviderSearchResponse{Results: append([]search.SearchResult(nil), p.results...)}, nil
}
func (p *chaosSearchProvider) Health(context.Context) search.ProviderHealth {
	return search.ProviderHealthReady
}

func newChaosSearchService(primary, backup *chaosSearchProvider) *search.Service {
	set := search.NewProviderSet("primary")
	set.RegisterWithPriority("primary", primary, 100)
	set.RegisterWithPriority("backup", backup, 90)
	cfg := search.DefaultConfig()
	cfg.Enabled = true
	cfg.DefaultProvider = "primary"
	cfg.CacheTTL = -1
	cfg.Providers = map[string]search.ProviderConfig{
		"primary": {Type: "fake", Enabled: true, Priority: 100},
		"backup":  {Type: "fake", Enabled: true, Priority: 90},
	}
	cfg.Routes = map[string]search.ProviderRouteConfig{"general": {Preferred: []string{"primary"}, Fallback: []string{"backup"}}}
	return search.NewService(cfg, set)
}

func TestFastSearchChaosFallsBackAfterPrimaryTimeout(t *testing.T) {
	primary := &chaosSearchProvider{id: "primary", err: search.NewError(search.SEARCH_PROVIDER_TIMEOUT, "primary", true, errors.New("timeout"))}
	backup := &chaosSearchProvider{id: "backup", results: []search.SearchResult{{Title: "answer", URL: "https://example.com/answer"}}}
	r := NewRuntime(DefaultConfig(), newChaosSearchService(primary, backup), nil, nil)
	observations, providers, calls, _, partial, err := r.searchQueriesFast(context.Background(), Scope{InvocationID: "chaos"}, []SearchQueryCommand{{Q: "fallback test"}}, nil)
	if err != nil || partial {
		t.Fatalf("fallback should recover without terminal error: partial=%v err=%v", partial, err)
	}
	if calls != 2 || primary.calls != 1 || backup.calls != 1 || len(observations) != 1 {
		t.Fatalf("unexpected fallback execution: calls=%d primary=%d backup=%d observations=%d", calls, primary.calls, backup.calls, len(observations))
	}
	if len(providers) != 2 || observations[0].provider != "backup" {
		t.Fatalf("unexpected providers/observation: providers=%v observation=%+v", providers, observations[0])
	}
}

func TestFastSearchChaosCancellationDoesNotTryFallback(t *testing.T) {
	primary := &chaosSearchProvider{id: "primary"}
	backup := &chaosSearchProvider{id: "backup"}
	r := NewRuntime(DefaultConfig(), newChaosSearchService(primary, backup), nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, _, _, err := r.searchQueriesFast(ctx, Scope{InvocationID: "cancel"}, []SearchQueryCommand{{Q: "cancelled"}}, nil)
	if err == nil || err.Code != ErrCancelled {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if primary.calls != 0 || backup.calls != 0 {
		t.Fatalf("cancelled execution reached providers: primary=%d backup=%d", primary.calls, backup.calls)
	}
}
