package native

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/search"
)

type testEngine struct {
	descriptor EngineDescriptor
	results    []search.SearchResult
	err        error
	calls      int
}

func (e *testEngine) Descriptor() EngineDescriptor {
	return e.descriptor
}

func (e *testEngine) Search(context.Context, search.SearchRequest) (search.ProviderSearchResponse, error) {
	e.calls++
	if e.err != nil {
		return search.ProviderSearchResponse{}, e.err
	}
	return search.ProviderSearchResponse{Results: withProviderResults(e.results, e.descriptor.ID), HTTPStatus: 200}, nil
}

func TestProviderOpensEngineCircuitAfterRetryableFailure(t *testing.T) {
	engine := &testEngine{
		descriptor: EngineDescriptor{
			ID: "unstable",
			Capabilities: search.ProviderCapabilities{
				GeneralWeb: true,
				MaxResults: 10,
			},
		},
		err: search.NewError(search.SEARCH_PROVIDER_TIMEOUT, "unstable", true, errors.New("timeout")),
	}
	provider := NewProvider(NewRegistry(engine), true, 1024)
	provider.failureLimit = 1
	provider.openDuration = time.Minute

	_, err := provider.Search(context.Background(), search.SearchRequest{Query: "test"})
	require.Error(t, err)
	_, err = provider.Search(context.Background(), search.SearchRequest{Query: "test"})
	require.Error(t, err)
	require.Equal(t, 1, engine.calls)
	require.Equal(t, search.ProviderHealthDegraded, provider.Health(context.Background()))
}

func withProviderResults(results []search.SearchResult, provider string) []search.SearchResult {
	out := make([]search.SearchResult, len(results))
	copy(out, results)
	for index := range out {
		out[index].Rank = index + 1
		out[index].Source.Provider = provider
		out[index].Source.ProviderRank = index + 1
		out[index].Source.OriginalURL = out[index].URL
	}
	return out
}

func TestProviderFusesResultsAcrossEngines(t *testing.T) {
	first := &testEngine{
		descriptor: EngineDescriptor{
			ID:       "first",
			Priority: 10,
			Capabilities: search.ProviderCapabilities{
				GeneralWeb: true,
				MaxResults: 10,
			},
		},
		results: []search.SearchResult{
			{Title: "A", URL: "https://example.com/a"},
			{Title: "B", URL: "https://example.com/b"},
		},
	}
	second := &testEngine{
		descriptor: EngineDescriptor{
			ID:       "second",
			Priority: 9,
			Capabilities: search.ProviderCapabilities{
				GeneralWeb: true,
				MaxResults: 10,
			},
		},
		results: []search.SearchResult{
			{Title: "A duplicate", URL: "https://example.com/a?utm_source=test"},
			{Title: "C", URL: "https://example.com/c"},
		},
	}
	provider := NewProvider(NewRegistry(first, second), true, 1024)
	require.Equal(t, ProviderID, provider.ID())
	require.True(t, provider.Capabilities().GeneralWeb)

	response, err := provider.Search(context.Background(), search.SearchRequest{Query: "test", Limit: 3})
	require.NoError(t, err)
	require.Len(t, response.Results, 3)
	require.Equal(t, "https://example.com/a", response.Results[0].URL)
	require.Equal(t, ProviderID, response.Results[0].Source.Provider)
	require.ElementsMatch(t, []string{"first", "second"}, response.Results[0].Source.Engines)
}

func TestProviderReturnsSuccessfulEngineWhenPeerFails(t *testing.T) {
	healthy := &testEngine{
		descriptor: EngineDescriptor{
			ID: "healthy",
			Capabilities: search.ProviderCapabilities{
				GeneralWeb: true,
				MaxResults: 10,
			},
		},
		results: []search.SearchResult{{Title: "A", URL: "https://example.com/a"}},
	}
	failing := &testEngine{
		descriptor: EngineDescriptor{
			ID: "failing",
			Capabilities: search.ProviderCapabilities{
				GeneralWeb: true,
				MaxResults: 10,
			},
		},
		err: errors.New("upstream failed"),
	}
	provider := NewProvider(NewRegistry(healthy, failing), true, 1024)
	response, err := provider.Search(context.Background(), search.SearchRequest{Query: "test", Limit: 3})
	require.NoError(t, err)
	require.Len(t, response.Results, 1)
}

func TestProviderRejectsUnsupportedFilter(t *testing.T) {
	engine := &testEngine{
		descriptor: EngineDescriptor{
			ID: "web",
			Capabilities: search.ProviderCapabilities{
				GeneralWeb: true,
				MaxResults: 10,
			},
		},
	}
	provider := NewProvider(NewRegistry(engine), true, 1024)
	_, err := provider.Search(context.Background(), search.SearchRequest{
		Query:    "test",
		Language: "zh",
	})
	require.Error(t, err)
	var searchErr *search.Error
	require.ErrorAs(t, err, &searchErr)
	require.Equal(t, search.SEARCH_FILTER_UNSUPPORTED, searchErr.Code)
}

type staticCredentialSource struct {
	values map[string]string
}

func (s staticCredentialSource) Available(_ context.Context, engineID string) bool {
	return s.values[engineID] != ""
}

func (s staticCredentialSource) Resolve(_ context.Context, engineID string) (string, func(), error) {
	return s.values[engineID], func() {}, nil
}

func TestProviderSkipsCredentialedEngineBeforeScheduling(t *testing.T) {
	credentialed := &testEngine{descriptor: EngineDescriptor{
		ID: "serper", Priority: 100,
		Capabilities: search.ProviderCapabilities{GeneralWeb: true, MaxResults: 10},
	}, results: []search.SearchResult{{Title: "paid", URL: "https://paid.example/a"}}}
	free := &testEngine{descriptor: EngineDescriptor{
		ID: "free", Priority: 10,
		Capabilities: search.ProviderCapabilities{GeneralWeb: true, MaxResults: 10},
	}, results: []search.SearchResult{{Title: "free", URL: "https://free.example/a"}}}
	provider := NewProvider(NewRegistry(credentialed, free), true, 1024)
	response, err := provider.Search(context.Background(), search.SearchRequest{Query: "test", Kind: search.SearchKindWeb})
	require.NoError(t, err)
	require.Equal(t, 0, credentialed.calls)
	require.Equal(t, 1, free.calls)
	require.Len(t, response.Results, 1)
}

func TestProviderSchedulesCredentialedEngineWhenCredentialAvailable(t *testing.T) {
	credentialed := &testEngine{descriptor: EngineDescriptor{
		ID: "serper", Priority: 100,
		Capabilities: search.ProviderCapabilities{GeneralWeb: true, MaxResults: 10},
	}, results: []search.SearchResult{{Title: "paid", URL: "https://paid.example/a"}}}
	free := &testEngine{descriptor: EngineDescriptor{
		ID: "free", Priority: 10,
		Capabilities: search.ProviderCapabilities{GeneralWeb: true, MaxResults: 10},
	}, results: []search.SearchResult{{Title: "free", URL: "https://free.example/a"}}}
	provider := NewProvider(NewRegistry(credentialed, free), true, 1024).Configure(ProviderRuntimeConfig{
		MaxEngines: 1, DefaultEngineTimeout: time.Second, DefaultRatePerMinute: 60, DefaultBurst: 6,
	})
	ctx := search.ContextWithEngineCredentialSource(context.Background(), staticCredentialSource{values: map[string]string{"serper": "key"}})
	response, err := provider.Search(ctx, search.SearchRequest{Query: "test", Kind: search.SearchKindWeb})
	require.NoError(t, err)
	require.Equal(t, 1, credentialed.calls)
	require.Equal(t, 0, free.calls)
	require.Len(t, response.Results, 1)
}

func TestProviderHealthIgnoresMissingOptionalCredentials(t *testing.T) {
	credentialed := &testEngine{descriptor: EngineDescriptor{
		ID: "serper", Priority: 100,
		Capabilities: search.ProviderCapabilities{GeneralWeb: true, MaxResults: 10},
	}}
	free := &testEngine{descriptor: EngineDescriptor{
		ID: "free", Priority: 10,
		Capabilities: search.ProviderCapabilities{GeneralWeb: true, MaxResults: 10},
	}}
	provider := NewProvider(NewRegistry(credentialed, free), true, 1024)
	require.Equal(t, search.ProviderHealthReady, provider.Health(context.Background()))
}

func TestProviderFusionPreservesEngineProvenance(t *testing.T) {
	first := &testEngine{descriptor: EngineDescriptor{ID: "one", Priority: 10, Capabilities: search.ProviderCapabilities{GeneralWeb: true}}, results: []search.SearchResult{{Title: "A", URL: "https://example.com/a"}}}
	second := &testEngine{descriptor: EngineDescriptor{ID: "two", Priority: 9, Capabilities: search.ProviderCapabilities{GeneralWeb: true}}, results: []search.SearchResult{{Title: "A2", URL: "https://example.com/a?utm_source=x"}}}
	provider := NewProvider(NewRegistry(first, second), true, 1024)
	response, err := provider.Search(context.Background(), search.SearchRequest{Query: "test", Kind: search.SearchKindWeb})
	require.NoError(t, err)
	require.Len(t, response.Results, 1)
	require.Equal(t, ProviderID, response.Results[0].Source.Provider)
	require.ElementsMatch(t, []string{"one", "two"}, response.Results[0].Source.Engines)
}

func TestProviderDefaultModerateSafeSearchDoesNotDisableCodeVerticals(t *testing.T) {
	code := &testEngine{descriptor: EngineDescriptor{
		ID:       "code",
		Priority: 10,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode},
			MaxResults:  10,
		},
	}, results: []search.SearchResult{{Title: "repo", URL: "https://code.example/repo"}}}
	provider := NewProvider(NewRegistry(code), true, 1024)
	response, err := provider.Search(context.Background(), search.SearchRequest{
		Query:      "agent runtime",
		Kind:       search.SearchKindCode,
		SafeSearch: search.SafeSearchModerate,
	})
	require.NoError(t, err)
	require.Equal(t, 1, code.calls)
	require.Len(t, response.Results, 1)
}

func TestProviderModerateSafeSearchFallsBackWhenEngineHasNoControl(t *testing.T) {
	unfiltered := &testEngine{descriptor: EngineDescriptor{
		ID:       "unfiltered",
		Priority: 10,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb: true,
			MaxResults: 10,
		},
	}, results: []search.SearchResult{{Title: "page", URL: "https://web.example/page"}}}
	provider := NewProvider(NewRegistry(unfiltered), true, 1024)
	response, err := provider.Search(context.Background(), search.SearchRequest{
		Query:      "agent runtime",
		Kind:       search.SearchKindWeb,
		SafeSearch: search.SafeSearchModerate,
	})
	require.NoError(t, err)
	require.Equal(t, 1, unfiltered.calls)
	require.Len(t, response.Results, 1)
}

func TestProviderStrictSafeSearchRequiresControlForGeneralWeb(t *testing.T) {
	unfiltered := &testEngine{descriptor: EngineDescriptor{
		ID:       "unfiltered",
		Priority: 10,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb: true,
			MaxResults: 10,
		},
	}, results: []search.SearchResult{{Title: "page", URL: "https://web.example/page"}}}
	provider := NewProvider(NewRegistry(unfiltered), true, 1024)
	_, err := provider.Search(context.Background(), search.SearchRequest{
		Query:      "agent runtime",
		Kind:       search.SearchKindWeb,
		SafeSearch: search.SafeSearchStrict,
	})
	require.Error(t, err)
	require.Equal(t, 0, unfiltered.calls)
}

func TestProviderModerateSafeSearchPrefersCapableEngine(t *testing.T) {
	unsafe := &testEngine{descriptor: EngineDescriptor{
		ID: "unsafe", Priority: 100, Capabilities: search.ProviderCapabilities{GeneralWeb: true, MaxResults: 10},
	}, results: []search.SearchResult{{Title: "unsafe", URL: "https://unsafe.example/page"}}}
	safe := &testEngine{descriptor: EngineDescriptor{
		ID: "safe", Priority: 1, Capabilities: search.ProviderCapabilities{GeneralWeb: true, SafeSearch: true, MaxResults: 10},
	}, results: []search.SearchResult{{Title: "safe", URL: "https://safe.example/page"}}}
	provider := NewProvider(NewRegistry(unsafe, safe), true, 1024).Configure(ProviderRuntimeConfig{
		MaxEngines: 1, DefaultEngineTimeout: time.Second, DefaultRatePerMinute: 60, DefaultBurst: 6,
	})
	response, err := provider.Search(context.Background(), search.SearchRequest{
		Query: "agent runtime", Kind: search.SearchKindWeb, SafeSearch: search.SafeSearchModerate,
	})
	require.NoError(t, err)
	require.Equal(t, 0, unsafe.calls)
	require.Equal(t, 1, safe.calls)
	require.Len(t, response.Results, 1)
}

func TestProviderOffsetRequiresPaginationCapability(t *testing.T) {
	nonPaged := &testEngine{descriptor: EngineDescriptor{
		ID: "nonpaged", Priority: 100, Capabilities: search.ProviderCapabilities{GeneralWeb: true, MaxResults: 10},
	}, results: []search.SearchResult{{Title: "np", URL: "https://np.example/page"}}}
	paged := &testEngine{descriptor: EngineDescriptor{
		ID: "paged", Priority: 1, Capabilities: search.ProviderCapabilities{GeneralWeb: true, Pagination: true, MaxResults: 10},
	}, results: []search.SearchResult{{Title: "p", URL: "https://p.example/page"}}}
	provider := NewProvider(NewRegistry(nonPaged, paged), true, 1024).Configure(ProviderRuntimeConfig{
		MaxEngines: 1, DefaultEngineTimeout: time.Second, DefaultRatePerMinute: 60, DefaultBurst: 6,
	})
	response, err := provider.Search(context.Background(), search.SearchRequest{
		Query: "agent runtime", Kind: search.SearchKindWeb, Offset: 10, SafeSearch: search.SafeSearchOff,
	})
	require.NoError(t, err)
	require.Equal(t, 0, nonPaged.calls)
	require.Equal(t, 1, paged.calls)
	require.Len(t, response.Results, 1)
}

func TestEngineCircuitAllowsOnlySingleHalfOpenProbe(t *testing.T) {
	provider := NewProvider(NewRegistry(), true, 1024)
	provider.circuits["engine"] = engineCircuit{
		Failures:    provider.failureLimit,
		OpenUntil:   time.Now().Add(-time.Second),
		LastFailure: time.Now().Add(-time.Minute),
	}
	now := time.Now()
	require.True(t, provider.engineCircuitAllows("engine", now))
	require.True(t, provider.engineCircuitAcquire("engine", now))
	require.False(t, provider.engineCircuitAllows("engine", now))
	require.False(t, provider.engineCircuitAcquire("engine", now))

	provider.recordEngineOutcome("engine", nil)
	require.True(t, provider.engineCircuitAcquire("engine", now))
}

func TestProviderPreservesAllAuthFailureClassification(t *testing.T) {
	first := &testEngine{descriptor: EngineDescriptor{ID: "auth_one", Capabilities: search.ProviderCapabilities{GeneralWeb: true}}, err: search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, "auth_one", false, errors.New("invalid key"))}
	second := &testEngine{descriptor: EngineDescriptor{ID: "auth_two", Capabilities: search.ProviderCapabilities{GeneralWeb: true}}, err: search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, "auth_two", false, errors.New("invalid key"))}
	provider := NewProvider(NewRegistry(first, second), true, 1024)

	_, err := provider.Search(context.Background(), search.SearchRequest{Query: "test", Kind: search.SearchKindWeb})
	require.Error(t, err)
	var searchErr *search.Error
	require.ErrorAs(t, err, &searchErr)
	require.Equal(t, search.SEARCH_PROVIDER_AUTH_FAILED, searchErr.Code)
	require.False(t, searchErr.Retryable)
}

func TestProviderPreservesAllRateLimitFailureClassification(t *testing.T) {
	firstErr := search.NewError(search.SEARCH_PROVIDER_RATE_LIMITED, "rate_one", true, errors.New("limited"))
	firstErr.RetryAfter = search.DurationMs(1200)
	secondErr := search.NewError(search.SEARCH_PROVIDER_RATE_LIMITED, "rate_two", true, errors.New("limited"))
	secondErr.RetryAfter = search.DurationMs(2400)
	first := &testEngine{descriptor: EngineDescriptor{ID: "rate_one", Capabilities: search.ProviderCapabilities{GeneralWeb: true}}, err: firstErr}
	second := &testEngine{descriptor: EngineDescriptor{ID: "rate_two", Capabilities: search.ProviderCapabilities{GeneralWeb: true}}, err: secondErr}
	provider := NewProvider(NewRegistry(first, second), true, 1024)

	_, err := provider.Search(context.Background(), search.SearchRequest{Query: "test", Kind: search.SearchKindWeb})
	require.Error(t, err)
	var searchErr *search.Error
	require.ErrorAs(t, err, &searchErr)
	require.Equal(t, search.SEARCH_PROVIDER_RATE_LIMITED, searchErr.Code)
	require.True(t, searchErr.Retryable)
	require.Equal(t, search.DurationMs(2400), searchErr.RetryAfter)
}

type blockingEngine struct {
	descriptor EngineDescriptor
	calls      int
}

func (e *blockingEngine) Descriptor() EngineDescriptor { return e.descriptor }
func (e *blockingEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	e.calls++
	<-ctx.Done()
	return search.ProviderSearchResponse{}, ctx.Err()
}

func TestProviderNormalizesPerEngineDeadlineAsTimeout(t *testing.T) {
	engine := &blockingEngine{descriptor: EngineDescriptor{
		ID: "slow", Capabilities: search.ProviderCapabilities{GeneralWeb: true, MaxResults: 10},
	}}
	enabled := true
	provider := NewProvider(NewRegistry(engine), true, 1024).Configure(ProviderRuntimeConfig{
		MaxEngines: 1, DefaultEngineTimeout: time.Second, DefaultRatePerMinute: 60, DefaultBurst: 6,
		Engines: map[string]EngineRuntimeConfig{
			"slow": {Enabled: &enabled, Timeout: 5 * time.Millisecond},
		},
	})
	_, err := provider.Search(context.Background(), search.SearchRequest{Query: "test", Kind: search.SearchKindWeb})
	require.Error(t, err)
	var searchErr *search.Error
	require.ErrorAs(t, err, &searchErr)
	require.Equal(t, search.SEARCH_PROVIDER_UNAVAILABLE, searchErr.Code)
	// A single timed-out engine is aggregated as provider unavailable to the
	// outer router, while the engine-level failure is classified retryable and
	// counted by the circuit breaker rather than as an opaque Go error.
	require.Equal(t, 1, engine.calls)
	provider.circuitMu.Lock()
	state := provider.circuits["slow"]
	provider.circuitMu.Unlock()
	require.Equal(t, 1, state.Failures)
	require.Equal(t, string(search.SEARCH_PROVIDER_TIMEOUT), state.LastCode)
}

func TestProviderEngineRateLimitPreventsImmediateSecondCall(t *testing.T) {
	engine := &testEngine{descriptor: EngineDescriptor{
		ID: "rate_limited_engine", Capabilities: search.ProviderCapabilities{GeneralWeb: true, MaxResults: 10},
	}, results: []search.SearchResult{{Title: "ok", URL: "https://example.com"}}}
	provider := NewProvider(NewRegistry(engine), true, 1024).Configure(ProviderRuntimeConfig{
		MaxEngines: 1, DefaultEngineTimeout: time.Second, DefaultRatePerMinute: 1, DefaultBurst: 1,
	})
	_, err := provider.Search(context.Background(), search.SearchRequest{Query: "first", Kind: search.SearchKindWeb})
	require.NoError(t, err)
	_, err = provider.Search(context.Background(), search.SearchRequest{Query: "second", Kind: search.SearchKindWeb})
	require.Error(t, err)
	var searchErr *search.Error
	require.ErrorAs(t, err, &searchErr)
	require.Equal(t, search.SEARCH_PROVIDER_RATE_LIMITED, searchErr.Code)
	require.Equal(t, 1, engine.calls)
	provider.circuitMu.Lock()
	_, circuitCreated := provider.circuits["rate_limited_engine"]
	provider.circuitMu.Unlock()
	require.False(t, circuitCreated, "local rate limiting must not mark the engine unhealthy")
}
