package native

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/search"
)

type Provider struct {
	registry     *Registry
	fetcher      *Fetcher
	enabled      bool
	maxBytes     int64
	maxEngines   int
	failureLimit int
	openDuration time.Duration
	circuitMu    sync.Mutex
	circuits     map[string]engineCircuit
}

type engineCircuit struct {
	Failures  int
	OpenUntil time.Time
}

func NewProvider(registry *Registry, enabled bool, maxBytes int64) *Provider {
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024
	}
	return &Provider{
		registry:     registry,
		fetcher:      NewFetcher(maxBytes),
		enabled:      enabled,
		maxBytes:     maxBytes,
		maxEngines:   6,
		failureLimit: 3,
		openDuration: 30 * time.Second,
		circuits:     make(map[string]engineCircuit),
	}
}

func (p *Provider) ID() string {
	return ProviderID
}

func (p *Provider) SetMaxResponseBytes(maxBytes int64) {
	if maxBytes > 0 {
		p.maxBytes = maxBytes
		p.fetcher.SetMaxBytes(maxBytes)
	}
}

func (p *Provider) SetMaxEngines(max int) {
	if max > 0 {
		p.maxEngines = max
	}
}

func (p *Provider) Fetcher() *Fetcher {
	return p.fetcher
}

func (p *Provider) Capabilities() search.ProviderCapabilities {
	result := search.ProviderCapabilities{MaxResults: 20}
	for _, engine := range p.registry.All() {
		caps := engine.Descriptor().Capabilities
		result.GeneralWeb = result.GeneralWeb || caps.GeneralWeb
		result.LanguageFilter = result.LanguageFilter || caps.LanguageFilter
		result.CountryFilter = result.CountryFilter || caps.CountryFilter
		result.SafeSearch = result.SafeSearch || caps.SafeSearch
		result.Pagination = result.Pagination || caps.Pagination
		result.TimeRangeFilter = result.TimeRangeFilter || caps.TimeRangeFilter
		result.DomainFilter = result.DomainFilter || caps.DomainFilter
		result.SearchKinds = mergeKinds(result.SearchKinds, caps.SearchKinds)
		if caps.MaxResults > result.MaxResults {
			result.MaxResults = caps.MaxResults
		}
	}
	return result
}

func mergeKinds(existing, incoming []search.SearchKind) []search.SearchKind {
	seen := make(map[search.SearchKind]struct{}, len(existing)+len(incoming))
	for _, kind := range existing {
		seen[kind] = struct{}{}
	}
	for _, kind := range incoming {
		seen[kind] = struct{}{}
	}
	result := make([]search.SearchKind, 0, len(seen))
	for kind := range seen {
		result = append(result, kind)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

func (p *Provider) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	if !p.enabled {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_DISABLED, ProviderID, false, nil)
	}
	if p.registry == nil || p.registry.Count() == 0 {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_NOT_CONFIGURED, ProviderID, false, nil)
	}
	kind := search.NormalizeKind(request.Kind)
	engines := p.candidates(kind, request)
	if len(engines) == 0 {
		if !search.SupportsKind(p.Capabilities(), kind) {
			return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_KIND_UNSUPPORTED, ProviderID, false, nil)
		}
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_FILTER_UNSUPPORTED, ProviderID, false, nil)
	}
	if p.maxEngines > 0 && len(engines) > p.maxEngines {
		engines = engines[:p.maxEngines]
	}
	type engineResult struct {
		response search.ProviderSearchResponse
		err      error
		priority int
		engineID string
	}
	results := make(chan engineResult, len(engines))
	var wg sync.WaitGroup
	for _, engine := range engines {
		engine := engine
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := engine.Search(ctx, request)
			descriptor := engine.Descriptor()
			p.recordEngineOutcome(descriptor.ID, err)
			results <- engineResult{response: response, err: err, priority: descriptor.Priority, engineID: descriptor.ID}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	all := make([]search.SearchResult, 0)
	weights := make(map[string]float64, len(engines))
	var firstErr error
	successes := 0
	hasMore := false
	status := 200
	rawBytes := 0
	collected := make([]engineResult, 0, len(engines))
	for item := range results {
		collected = append(collected, item)
	}
	sort.SliceStable(collected, func(i, j int) bool {
		if collected[i].priority != collected[j].priority {
			return collected[i].priority > collected[j].priority
		}
		return collected[i].engineID < collected[j].engineID
	})
	for _, item := range collected {
		if item.err != nil {
			if firstErr == nil {
				firstErr = item.err
			}
			continue
		}
		successes++
		all = append(all, item.response.Results...)
		hasMore = hasMore || item.response.HasMore
		if item.response.HTTPStatus > 0 {
			status = item.response.HTTPStatus
		}
		rawBytes += item.response.RawBytes
	}
	if successes == 0 {
		if firstErr != nil {
			return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_UNAVAILABLE, ProviderID, true, fmt.Errorf("all native search engines failed: %w", firstErr))
		}
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_UNAVAILABLE, ProviderID, true, nil)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	for _, engine := range engines {
		descriptor := engine.Descriptor()
		weights[strings.ToLower(descriptor.ID)] = descriptor.Weight
	}
	fused := fuseResults(all, limit, weights)
	return search.ProviderSearchResponse{
		Results:    fused,
		HasMore:    hasMore || len(all) > len(fused),
		HTTPStatus: status,
		RawBytes:   rawBytes,
	}, nil
}

func (p *Provider) Health(ctx context.Context) search.ProviderHealth {
	if !p.enabled {
		return search.ProviderHealthDisabled
	}
	if p.registry == nil || p.registry.Count() == 0 {
		return search.ProviderHealthMisconfigured
	}
	now := time.Now()
	for _, engine := range p.registry.All() {
		if !p.engineCircuitAllows(engine.Descriptor().ID, now) {
			return search.ProviderHealthDegraded
		}
	}
	return search.ProviderHealthReady
}

func (p *Provider) recordEngineOutcome(engineID string, err error) {
	if p == nil || strings.TrimSpace(engineID) == "" {
		return
	}
	p.circuitMu.Lock()
	defer p.circuitMu.Unlock()
	state := p.circuits[engineID]
	if err == nil {
		delete(p.circuits, engineID)
		return
	}
	if searchErr, ok := err.(*search.Error); ok && !searchErr.Retryable {
		return
	}
	state.Failures++
	if state.Failures >= p.failureLimit {
		state.OpenUntil = time.Now().Add(p.openDuration)
	}
	p.circuits[engineID] = state
}

func (p *Provider) engineCircuitAllows(engineID string, now time.Time) bool {
	p.circuitMu.Lock()
	defer p.circuitMu.Unlock()
	state, ok := p.circuits[engineID]
	if !ok {
		return true
	}
	if state.OpenUntil.IsZero() {
		return true
	}
	if now.Before(state.OpenUntil) {
		return false
	}
	delete(p.circuits, engineID)
	return true
}

func (p *Provider) candidates(kind search.SearchKind, request search.SearchRequest) []Engine {
	type candidate struct {
		engine   Engine
		priority int
		weight   float64
	}
	items := make([]candidate, 0, p.registry.Count())
	now := time.Now()
	for _, engine := range p.registry.All() {
		descriptor := engine.Descriptor()
		if !p.engineCircuitAllows(descriptor.ID, now) {
			continue
		}
		if !search.SupportsKind(descriptor.Capabilities, kind) {
			continue
		}
		if request.Language != "" && !descriptor.Capabilities.LanguageFilter {
			continue
		}
		if request.Country != "" && !descriptor.Capabilities.CountryFilter {
			continue
		}
		if request.SafeSearch != "" && !descriptor.Capabilities.SafeSearch {
			continue
		}
		if request.TimeRange != nil && !descriptor.Capabilities.TimeRangeFilter {
			continue
		}
		if len(request.Domains) > 0 && !descriptor.Capabilities.DomainFilter {
			continue
		}
		items = append(items, candidate{engine: engine, priority: descriptor.Priority, weight: descriptor.Weight})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].priority != items[j].priority {
			return items[i].priority > items[j].priority
		}
		if items[i].weight != items[j].weight {
			return items[i].weight > items[j].weight
		}
		return strings.Compare(items[i].engine.Descriptor().ID, items[j].engine.Descriptor().ID) < 0
	})
	result := make([]Engine, 0, len(items))
	for _, item := range items {
		result = append(result, item.engine)
	}
	return result
}

var _ search.Provider = (*Provider)(nil)
