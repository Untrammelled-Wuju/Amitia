package fake

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/search"
)

const providerID = "fake"

type Provider struct {
	enabled   bool
	provider  string
	results   []search.SearchResult
	err       *search.Error
	roundTrip time.Duration
	calls     int
}

func NewProvider() *Provider {
	return &Provider{
		enabled:  true,
		provider: providerID,
	}
}

func (p *Provider) WithError(err *search.Error) *Provider {
	p.err = err
	return p
}

func (p *Provider) WithResults(results ...search.SearchResult) *Provider {
	p.results = results
	return p
}

func (p *Provider) WithEnabled(enabled bool) *Provider {
	p.enabled = enabled
	return p
}

func (p *Provider) WithDelay(d time.Duration) *Provider {
	p.roundTrip = d
	return p
}

func (p *Provider) CallCount() int {
	return p.calls
}

func (p *Provider) ID() string {
	return p.provider
}

func (p *Provider) Capabilities() search.ProviderCapabilities {
	return search.ProviderCapabilities{
		GeneralWeb:     true,
		LanguageFilter: true,
		CountryFilter:  true,
		SafeSearch:     true,
		Pagination:     true,
		MaxResults:     20,
	}
}

func (p *Provider) Search(ctx context.Context, req search.GeneralSearchRequest) (search.ProviderSearchResponse, error) {
	p.calls++
	select {
	case <-ctx.Done():
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_CANCELLED, p.provider, false, ctx.Err())
	default:
	}
	if p.roundTrip > 0 {
		timer := time.NewTimer(p.roundTrip)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_CANCELLED, p.provider, false, ctx.Err())
		case <-timer.C:
		}
	}
	if p.err != nil {
		return search.ProviderSearchResponse{}, p.err
	}
	limit := req.Limit
	if limit <= 0 || limit > len(p.results) {
		limit = len(p.results)
	}
	results := make([]search.SearchResult, 0, limit)
	for i := 0; i < limit && i < len(p.results); i++ {
		results = append(results, p.results[i])
	}
	return search.ProviderSearchResponse{
		Results: results,
		HasMore: len(p.results) > limit,
	}, nil
}

func (p *Provider) Health(ctx context.Context) search.ProviderHealth {
	if !p.enabled {
		return search.ProviderHealthDisabled
	}
	return search.ProviderHealthReady
}
