package search

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

type concurrentLoadProvider struct {
	calls atomic.Int64
}

func (p *concurrentLoadProvider) ID() string { return "load" }
func (p *concurrentLoadProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{GeneralWeb: true, MaxResults: 20}
}
func (p *concurrentLoadProvider) Search(_ context.Context, req SearchRequest) (ProviderSearchResponse, error) {
	p.calls.Add(1)
	return ProviderSearchResponse{Results: []SearchResult{{Title: req.Query, URL: "https://example.com/" + req.Query}}}, nil
}
func (p *concurrentLoadProvider) Health(context.Context) ProviderHealth { return ProviderHealthReady }

func TestServiceConcurrentLoad100Users(t *testing.T) {
	provider := &concurrentLoadProvider{}
	set := NewProviderSet("load_primary")
	set.RegisterWithPriority("load_primary", provider, 100)
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.DefaultProvider = "load_primary"
	cfg.CacheTTL = -1
	cfg.Providers = map[string]ProviderConfig{"load_primary": {Type: "load", Enabled: true}}
	svc := NewService(cfg, set)

	const users = 100
	var wg sync.WaitGroup
	errs := make(chan error, users)
	for i := 0; i < users; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			query := fmt.Sprintf("load-%03d", i)
			resp, err := svc.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: query, Kind: SearchKindWeb, Limit: 1}, "load-test", "load_primary")
			if err != nil {
				errs <- err
				return
			}
			if resp == nil || resp.Returned != 1 || resp.Results[0].Title != query {
				errs <- fmt.Errorf("unexpected response for %s: %#v", query, resp)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if got := provider.calls.Load(); got != users {
		t.Fatalf("provider calls=%d, want %d", got, users)
	}
}
