package webresearch

import (
	"context"
	"strings"
	"sync"
)

// Observation is the low-cardinality telemetry record emitted by the web
// research runtime. It intentionally excludes full queries, URLs, cookies,
// authorization headers, secrets, and fetched page content.
type Observation struct {
	Name            string  `json:"name"`
	InvocationID    string  `json:"invocation_id,omitempty"`
	Operation       string  `json:"operation,omitempty"`
	Mode            Mode    `json:"mode,omitempty"`
	Provider        string  `json:"provider,omitempty"`
	Domain          string  `json:"domain,omitempty"`
	PolicyDecision  string  `json:"policy_decision,omitempty"`
	ErrorCode       string  `json:"error_code,omitempty"`
	DurationMs      int64   `json:"duration_ms,omitempty"`
	SearchCalls     int     `json:"search_calls,omitempty"`
	FetchCalls      int     `json:"fetch_calls,omitempty"`
	BrowserCalls    int     `json:"browser_calls,omitempty"`
	CacheHits       int     `json:"cache_hits,omitempty"`
	PageCacheHits   int     `json:"page_cache_hits,omitempty"`
	SourceCount     int     `json:"source_count,omitempty"`
	EvidenceCount   int     `json:"evidence_count,omitempty"`
	ProviderCostUSD float64 `json:"provider_cost_usd,omitempty"`
	ProviderCredits float64 `json:"provider_credits,omitempty"`
	Partial         bool    `json:"partial,omitempty"`
}

// Observer can bridge runtime observations to the application's logger,
// metrics registry, OpenTelemetry, or an audit sink. Implementations must be
// concurrency-safe because research mode may search providers in parallel.
type Observer interface {
	Observe(ctx context.Context, observation Observation)
}

type ObserverFunc func(ctx context.Context, observation Observation)

func (f ObserverFunc) Observe(ctx context.Context, observation Observation) {
	if f != nil {
		f(ctx, observation)
	}
}

func (r *Runtime) WithObserver(observer Observer) *Runtime {
	if r != nil {
		r.observer = observer
	}
	return r
}

func (r *Runtime) observe(ctx context.Context, observation Observation) {
	if r == nil {
		return
	}
	observation.Name = strings.TrimSpace(observation.Name)
	if observation.Name == "" {
		return
	}
	if r.metrics != nil {
		r.metrics.Observe(ctx, observation)
	}
	if r.observer != nil {
		r.observer.Observe(ctx, observation)
	}
}

func (r *Runtime) MetricsSnapshot() MetricsSnapshot {
	if r == nil || r.metrics == nil {
		return MetricsSnapshot{}
	}
	return r.metrics.Snapshot()
}

type MetricsSnapshot struct {
	WebRuns             int64            `json:"web_runs"`
	SearchRequests      int64            `json:"search_requests"`
	FetchRequests       int64            `json:"fetch_requests"`
	BrowserOperations   int64            `json:"browser_operations"`
	Errors              int64            `json:"errors"`
	CacheHits           int64            `json:"cache_hits"`
	PageCacheHits       int64            `json:"page_cache_hits"`
	Sources             int64            `json:"sources"`
	Evidence            int64            `json:"evidence"`
	ExecutionDurationMs int64            `json:"execution_duration_ms"`
	SearchDurationMs    int64            `json:"search_duration_ms"`
	FetchDurationMs     int64            `json:"fetch_duration_ms"`
	BrowserDurationMs   int64            `json:"browser_duration_ms"`
	ProviderCostUSD     float64          `json:"provider_cost_usd"`
	ProviderCredits     float64          `json:"provider_credits"`
	ProviderFailures    map[string]int64 `json:"provider_failures,omitempty"`
	OperationFailures   map[string]int64 `json:"operation_failures,omitempty"`
}

type MetricsObserver struct {
	mu       sync.Mutex
	snapshot MetricsSnapshot
}

func NewMetricsObserver() *MetricsObserver {
	return &MetricsObserver{snapshot: MetricsSnapshot{ProviderFailures: make(map[string]int64), OperationFailures: make(map[string]int64)}}
}

func (m *MetricsObserver) Observe(_ context.Context, observation Observation) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch observation.Name {
	case "web.run":
		m.snapshot.WebRuns++
		m.snapshot.ExecutionDurationMs += observation.DurationMs
		m.snapshot.PageCacheHits += int64(observation.PageCacheHits)
		m.snapshot.Evidence += int64(observation.EvidenceCount)
	case "web.search":
		m.snapshot.SearchRequests++
		m.snapshot.SearchDurationMs += observation.DurationMs
		m.snapshot.CacheHits += int64(observation.CacheHits)
		m.snapshot.Sources += int64(observation.SourceCount)
		m.snapshot.ProviderCostUSD += observation.ProviderCostUSD
		m.snapshot.ProviderCredits += observation.ProviderCredits
	case "web.fetch":
		m.snapshot.FetchRequests++
		m.snapshot.FetchDurationMs += observation.DurationMs
	case "web.browser":
		m.snapshot.BrowserOperations++
		m.snapshot.BrowserDurationMs += observation.DurationMs
	}
	if observation.ErrorCode != "" {
		m.snapshot.Errors++
		operation := strings.TrimSpace(observation.Name)
		if operation != "" {
			m.snapshot.OperationFailures[operation]++
		}
		provider := strings.TrimSpace(observation.Provider)
		if provider != "" {
			m.snapshot.ProviderFailures[provider]++
		}
	}
}

func (m *MetricsObserver) Snapshot() MetricsSnapshot {
	if m == nil {
		return MetricsSnapshot{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.snapshot
	out.ProviderFailures = cloneInt64Map(m.snapshot.ProviderFailures)
	out.OperationFailures = cloneInt64Map(m.snapshot.OperationFailures)
	return out
}

func cloneInt64Map(input map[string]int64) map[string]int64 {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]int64, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
