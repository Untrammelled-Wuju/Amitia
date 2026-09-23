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
	require.Equal(t, "first", response.Results[0].Source.Provider)
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
