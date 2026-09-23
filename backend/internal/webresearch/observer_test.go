package webresearch

import (
	"context"
	"testing"
)

func TestMetricsObserverSeparatesDurationsAndCosts(t *testing.T) {
	m := NewMetricsObserver()
	ctx := context.Background()
	m.Observe(ctx, Observation{Name: "web.run", DurationMs: 100, PageCacheHits: 2, EvidenceCount: 3})
	m.Observe(ctx, Observation{Name: "web.search", Provider: "brave_primary", DurationMs: 20, CacheHits: 1, SourceCount: 5, ProviderCostUSD: 0.02})
	m.Observe(ctx, Observation{Name: "web.fetch", DurationMs: 30})
	m.Observe(ctx, Observation{Name: "web.browser", DurationMs: 40, ErrorCode: "browser_failed"})

	snapshot := m.Snapshot()
	if snapshot.WebRuns != 1 || snapshot.SearchRequests != 1 || snapshot.FetchRequests != 1 || snapshot.BrowserOperations != 1 {
		t.Fatalf("unexpected counters: %+v", snapshot)
	}
	if snapshot.ExecutionDurationMs != 100 || snapshot.SearchDurationMs != 20 || snapshot.FetchDurationMs != 30 || snapshot.BrowserDurationMs != 40 {
		t.Fatalf("unexpected durations: %+v", snapshot)
	}
	if snapshot.ProviderCostUSD != 0.02 || snapshot.CacheHits != 1 || snapshot.PageCacheHits != 2 || snapshot.Evidence != 3 {
		t.Fatalf("unexpected accounting: %+v", snapshot)
	}
	if snapshot.OperationFailures["web.browser"] != 1 {
		t.Fatalf("expected browser failure metric: %+v", snapshot.OperationFailures)
	}
}
