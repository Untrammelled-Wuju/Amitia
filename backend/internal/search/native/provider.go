package native

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/search"
)

type Provider struct {
	registry      *Registry
	fetcher       *Fetcher
	enabled       bool
	maxBytes      int64
	maxEngines    int
	failureLimit  int
	openDuration  time.Duration
	runtimeConfig ProviderRuntimeConfig

	circuitMu sync.Mutex
	circuits  map[string]engineCircuit

	rateMu sync.Mutex
	rates  map[string]engineRateState
}

type engineCircuit struct {
	Failures         int
	OpenUntil        time.Time
	LastFailure      time.Time
	LastCode         string
	RecoveryInFlight bool
}

type engineRateState struct {
	Tokens     float64
	LastRefill time.Time
}

type EngineKindHealth struct {
	Kind      search.SearchKind     `json:"kind"`
	Health    search.ProviderHealth `json:"health"`
	Available int                   `json:"available"`
	Degraded  int                   `json:"degraded"`
}

func NewProvider(registry *Registry, enabled bool, maxBytes int64) *Provider {
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024
	}
	runtimeConfig := DefaultProviderRuntimeConfig()
	return &Provider{
		registry:      registry,
		fetcher:       NewFetcher(maxBytes),
		enabled:       enabled,
		maxBytes:      maxBytes,
		maxEngines:    runtimeConfig.MaxEngines,
		failureLimit:  3,
		openDuration:  30 * time.Second,
		runtimeConfig: runtimeConfig,
		circuits:      make(map[string]engineCircuit),
		rates:         make(map[string]engineRateState),
	}
}

func (p *Provider) Configure(config ProviderRuntimeConfig) *Provider {
	if p == nil {
		return p
	}
	defaults := DefaultProviderRuntimeConfig()
	if config.MaxEngines <= 0 {
		config.MaxEngines = defaults.MaxEngines
	}
	if config.DefaultEngineTimeout <= 0 {
		config.DefaultEngineTimeout = defaults.DefaultEngineTimeout
	}
	if config.DefaultRatePerMinute <= 0 {
		config.DefaultRatePerMinute = defaults.DefaultRatePerMinute
	}
	if config.DefaultBurst <= 0 {
		config.DefaultBurst = defaults.DefaultBurst
	}
	if config.Engines == nil {
		config.Engines = map[string]EngineRuntimeConfig{}
	}
	p.runtimeConfig = config
	p.maxEngines = config.MaxEngines
	return p
}

func (p *Provider) ID() string { return ProviderID }

func (p *Provider) SetMaxResponseBytes(maxBytes int64) {
	if maxBytes > 0 {
		p.maxBytes = maxBytes
		p.fetcher.SetMaxBytes(maxBytes)
	}
}

func (p *Provider) SetMaxEngines(max int) {
	if max > 0 {
		p.maxEngines = max
		p.runtimeConfig.MaxEngines = max
	}
}

func (p *Provider) Fetcher() *Fetcher { return p.fetcher }

func (p *Provider) Capabilities() search.ProviderCapabilities {
	result := search.ProviderCapabilities{MaxResults: 20}
	if p == nil || p.registry == nil {
		return result
	}
	for _, engine := range p.registry.All() {
		descriptor := normalizeDescriptor(engine.Descriptor())
		if !engineEnabled(p.runtimeConfig, descriptor) {
			continue
		}
		caps := descriptor.Capabilities
		result.GeneralWeb = result.GeneralWeb || caps.GeneralWeb
		result.LanguageFilter = result.LanguageFilter || caps.LanguageFilter
		result.CountryFilter = result.CountryFilter || caps.CountryFilter
		result.SafeSearch = result.SafeSearch || caps.SafeSearch
		result.Pagination = result.Pagination || caps.Pagination
		result.TimeRangeFilter = result.TimeRangeFilter || caps.TimeRangeFilter
		result.DomainFilter = result.DomainFilter || caps.DomainFilter
		result.ExcludeDomainFilter = result.ExcludeDomainFilter || caps.ExcludeDomainFilter
		result.SearchKinds = mergeKinds(result.SearchKinds, caps.SearchKinds)
		if caps.MaxResults > result.MaxResults {
			result.MaxResults = caps.MaxResults
		}
	}
	return result
}

func (p *Provider) CapabilitiesForContext(ctx context.Context) search.ProviderCapabilities {
	result := search.ProviderCapabilities{MaxResults: 20}
	if p == nil || p.registry == nil {
		return result
	}
	now := time.Now()
	for _, engine := range p.registry.All() {
		descriptor := normalizeDescriptor(engine.Descriptor())
		if !engineEnabled(p.runtimeConfig, descriptor) || !p.engineCircuitAllows(descriptor.ID, now) {
			continue
		}
		available := true
		for _, credentialID := range descriptor.CredentialIDs {
			if !search.EngineCredentialAvailable(ctx, credentialID) {
				available = false
				break
			}
		}
		if !available {
			continue
		}
		caps := descriptor.Capabilities
		result.GeneralWeb = result.GeneralWeb || caps.GeneralWeb
		result.LanguageFilter = result.LanguageFilter || caps.LanguageFilter
		result.CountryFilter = result.CountryFilter || caps.CountryFilter
		result.SafeSearch = result.SafeSearch || caps.SafeSearch
		result.Pagination = result.Pagination || caps.Pagination
		result.TimeRangeFilter = result.TimeRangeFilter || caps.TimeRangeFilter
		result.DomainFilter = result.DomainFilter || caps.DomainFilter
		result.ExcludeDomainFilter = result.ExcludeDomainFilter || caps.ExcludeDomainFilter
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
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
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
	plan := p.plan(ctx, kind, request)
	if len(plan.Engines) == 0 {
		return search.ProviderSearchResponse{}, p.planError(kind, plan)
	}

	type engineResult struct {
		response   search.ProviderSearchResponse
		err        error
		descriptor EngineDescriptor
	}
	results := make(chan engineResult, len(plan.Engines))
	var wg sync.WaitGroup
	for _, planned := range plan.Engines {
		planned := planned
		wg.Add(1)
		go func() {
			defer wg.Done()
			descriptor := planned.Descriptor
			if !p.engineCircuitAcquire(descriptor.ID, time.Now()) {
				results <- engineResult{err: search.NewError(search.SEARCH_PROVIDER_UNAVAILABLE, descriptor.ID, true, fmt.Errorf("native engine circuit is open or recovering")), descriptor: descriptor}
				return
			}
			if rateErr := p.acquireEngineRate(descriptor, time.Now()); rateErr != nil {
				// This is the host-side token bucket, not an upstream failure. Do not
				// poison engine health/circuit state; future plans will skip the engine
				// until the local bucket has refilled.
				results <- engineResult{err: rateErr, descriptor: descriptor}
				return
			}
			engineCtx := ctx
			releases := make([]func(), 0, len(descriptor.CredentialIDs))
			releaseAll := func() {
				for i := len(releases) - 1; i >= 0; i-- {
					if releases[i] != nil {
						releases[i]()
					}
				}
			}
			for _, credentialID := range descriptor.CredentialIDs {
				credential, release, err := search.ResolveEngineCredential(engineCtx, credentialID)
				if err != nil {
					releaseAll()
					searchErr := search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, descriptor.ID, false, err)
					p.recordEngineOutcome(descriptor.ID, searchErr)
					results <- engineResult{err: searchErr, descriptor: descriptor}
					return
				}
				if strings.TrimSpace(credential) == "" {
					releaseAll()
					searchErr := search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, descriptor.ID, false, nil)
					p.recordEngineOutcome(descriptor.ID, searchErr)
					results <- engineResult{err: searchErr, descriptor: descriptor}
					return
				}
				if release != nil {
					releases = append(releases, release)
				}
				engineCtx = search.ContextWithEngineCredential(engineCtx, credentialID, credential)
			}
			timeout := planned.Timeout
			if timeout <= 0 {
				timeout = 8 * time.Second
			}
			callCtx, cancel := context.WithTimeout(engineCtx, timeout)
			response, err := planned.Engine.Search(callCtx, request)
			callErr := callCtx.Err()
			cancel()
			releaseAll()
			err = normalizeNativeEngineError(ctx, descriptor.ID, err, callErr)
			if err == nil && response.Usage == (search.ProviderUsage{}) && descriptor.EstimatedUsage != (search.ProviderUsage{}) {
				response.Usage = descriptor.EstimatedUsage
			}
			p.recordEngineOutcome(descriptor.ID, err)
			results <- engineResult{response: response, err: err, descriptor: descriptor}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	collected := make([]engineResult, 0, len(plan.Engines))
	for item := range results {
		collected = append(collected, item)
	}
	sort.SliceStable(collected, func(i, j int) bool {
		if collected[i].descriptor.Priority != collected[j].descriptor.Priority {
			return collected[i].descriptor.Priority > collected[j].descriptor.Priority
		}
		return collected[i].descriptor.ID < collected[j].descriptor.ID
	})

	all := make([]search.SearchResult, 0)
	weights := make(map[string]float64, len(plan.Engines))
	failures := make([]error, 0, len(collected))
	successes := 0
	hasMore := false
	status := 200
	rawBytes := 0
	usage := search.ProviderUsage{}
	for _, item := range collected {
		descriptor := item.descriptor
		weights[descriptor.ID] = descriptor.Weight
		if item.err != nil {
			failures = append(failures, item.err)
			continue
		}
		successes++
		all = append(all, item.response.Results...)
		hasMore = hasMore || item.response.HasMore
		if item.response.HTTPStatus > 0 {
			status = item.response.HTTPStatus
		}
		rawBytes += item.response.RawBytes
		usage = usage.Add(item.response.Usage)
	}
	if successes == 0 {
		return search.ProviderSearchResponse{}, aggregateNativeEngineErrors(failures)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	fused := fuseResults(all, limit, weights)
	return search.ProviderSearchResponse{
		Results: fused, HasMore: hasMore || len(all) > len(fused), HTTPStatus: status, RawBytes: rawBytes, Usage: usage,
	}, nil
}

func normalizeNativeEngineError(parent context.Context, engineID string, err error, callErr error) error {
	if err == nil {
		return nil
	}
	var searchErr *search.Error
	if errors.As(err, &searchErr) && searchErr != nil {
		return err
	}
	if parent != nil && errors.Is(parent.Err(), context.Canceled) {
		return search.NewError(search.SEARCH_CANCELLED, engineID, false, err)
	}
	if errors.Is(callErr, context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return search.NewError(search.SEARCH_PROVIDER_TIMEOUT, engineID, true, err)
	}
	if errors.Is(callErr, context.Canceled) || errors.Is(err, context.Canceled) {
		return search.NewError(search.SEARCH_CANCELLED, engineID, false, err)
	}
	return err
}

func aggregateNativeEngineErrors(failures []error) error {
	if len(failures) == 0 {
		return search.NewError(search.SEARCH_PROVIDER_UNAVAILABLE, ProviderID, true, nil)
	}
	allAuth := true
	allRateLimited := true
	allCancelled := true
	maxRetryAfter := search.DurationMs(0)
	var first error
	for _, failure := range failures {
		if failure == nil {
			continue
		}
		if first == nil {
			first = failure
		}
		var searchErr *search.Error
		if !errors.As(failure, &searchErr) || searchErr == nil {
			allAuth = false
			allRateLimited = false
			allCancelled = false
			continue
		}
		if searchErr.Code != search.SEARCH_PROVIDER_AUTH_FAILED {
			allAuth = false
		}
		if searchErr.Code != search.SEARCH_PROVIDER_RATE_LIMITED {
			allRateLimited = false
		} else if searchErr.RetryAfter > maxRetryAfter {
			maxRetryAfter = searchErr.RetryAfter
		}
		if searchErr.Code != search.SEARCH_CANCELLED {
			allCancelled = false
		}
	}
	switch {
	case allAuth:
		return search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, ProviderID, false, fmt.Errorf("all planned native engines rejected their credentials: %w", first))
	case allRateLimited:
		err := search.NewError(search.SEARCH_PROVIDER_RATE_LIMITED, ProviderID, true, fmt.Errorf("all planned native engines are rate limited: %w", first))
		err.RetryAfter = maxRetryAfter
		return err
	case allCancelled:
		return search.NewError(search.SEARCH_CANCELLED, ProviderID, false, first)
	default:
		return search.NewError(search.SEARCH_PROVIDER_UNAVAILABLE, ProviderID, true, fmt.Errorf("all planned native search engines failed: %w", first))
	}
}

func planHasReason(plan EnginePlan, reason string) bool {
	for _, item := range plan.Skipped {
		if item.Reason == reason {
			return true
		}
	}
	return false
}

// ValidateSearchRequest lets the outer search service ask the native metasearch
// planner whether the concrete request is executable right now. Static provider
// capabilities are intentionally insufficient here because engine credentials,
// filter combinations, rate limits and circuit state are dynamic.
func (p *Provider) ValidateSearchRequest(ctx context.Context, request search.SearchRequest) *search.Error {
	if p == nil || !p.enabled {
		return search.NewError(search.SEARCH_DISABLED, ProviderID, false, nil)
	}
	if p.registry == nil || p.registry.Count() == 0 {
		return search.NewError(search.SEARCH_PROVIDER_NOT_CONFIGURED, ProviderID, false, nil)
	}
	kind := search.NormalizeKind(request.Kind)
	plan := p.plan(ctx, kind, request)
	if len(plan.Engines) > 0 {
		return nil
	}
	return p.planError(kind, plan)
}

func (p *Provider) planError(kind search.SearchKind, plan EnginePlan) *search.Error {
	if !p.kindSupportedByConfiguredEngine(kind) {
		return search.NewError(search.SEARCH_KIND_UNSUPPORTED, ProviderID, false, nil)
	}
	// These reasons only occur after an engine has already passed kind/filter
	// compatibility, so they are stronger diagnostics than a generic filter
	// error.
	if planHasReason(plan, "rate_limited") {
		return search.NewError(search.SEARCH_PROVIDER_RATE_LIMITED, ProviderID, true, fmt.Errorf("compatible native engines are rate limited"))
	}
	if planHasReason(plan, "circuit_open") {
		return search.NewError(search.SEARCH_PROVIDER_UNAVAILABLE, ProviderID, true, fmt.Errorf("compatible native engines are temporarily unavailable"))
	}
	if planHasReason(plan, "credential_missing") {
		return search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, ProviderID, false, fmt.Errorf("no configured native engine credential can satisfy this request"))
	}
	return search.NewError(search.SEARCH_FILTER_UNSUPPORTED, ProviderID, false, nil)
}

func (p *Provider) kindSupportedByConfiguredEngine(kind search.SearchKind) bool {
	if p == nil || p.registry == nil {
		return false
	}
	for _, engine := range p.registry.All() {
		descriptor := normalizeDescriptor(engine.Descriptor())
		if engineEnabled(p.runtimeConfig, descriptor) && search.SupportsKind(descriptor.Capabilities, kind) {
			return true
		}
	}
	return false
}

func (p *Provider) Health(ctx context.Context) search.ProviderHealth {
	return p.KindHealth(ctx, search.SearchKindWeb).Health
}

func (p *Provider) KindHealth(ctx context.Context, kind search.SearchKind) EngineKindHealth {
	health := EngineKindHealth{Kind: kind, Health: search.ProviderHealthMisconfigured}
	if p == nil || !p.enabled {
		health.Health = search.ProviderHealthDisabled
		return health
	}
	if p.registry == nil || p.registry.Count() == 0 {
		return health
	}
	now := time.Now()
	supported := 0
	for _, engine := range p.registry.All() {
		descriptor := normalizeDescriptor(engine.Descriptor())
		if !engineEnabled(p.runtimeConfig, descriptor) || !search.SupportsKind(descriptor.Capabilities, kind) {
			continue
		}
		// Credentialed engines are optional for health unless this context can
		// prove the credential is available. Missing optional keys must not mark
		// the entire native provider degraded.
		missingCredential := false
		for _, credentialID := range descriptor.CredentialIDs {
			if !search.EngineCredentialAvailable(ctx, credentialID) {
				missingCredential = true
				break
			}
		}
		if missingCredential {
			continue
		}
		supported++
		if p.engineCircuitAllows(descriptor.ID, now) {
			health.Available++
		} else {
			health.Degraded++
		}
	}
	switch {
	case health.Available > 0:
		health.Health = search.ProviderHealthReady
	case supported > 0 && health.Degraded > 0:
		health.Health = search.ProviderHealthDegraded
	default:
		health.Health = search.ProviderHealthMisconfigured
	}
	return health
}

func (p *Provider) recordEngineOutcome(engineID string, err error) {
	if p == nil || strings.TrimSpace(engineID) == "" {
		return
	}
	now := time.Now()
	p.circuitMu.Lock()
	defer p.circuitMu.Unlock()
	state := p.circuits[engineID]
	if err == nil {
		delete(p.circuits, engineID)
		return
	}
	var searchErr *search.Error
	if candidate, ok := err.(*search.Error); ok {
		searchErr = candidate
	}
	if searchErr != nil {
		state.LastCode = searchErr.Code
		state.LastFailure = now
		switch searchErr.Code {
		case search.SEARCH_CANCELLED, search.SEARCH_INVALID_QUERY, search.SEARCH_INVALID_LANGUAGE, search.SEARCH_FILTER_UNSUPPORTED:
			// Request-specific/cancellation failures do not count against engine
			// health. If this request owned the half-open probe, release it so a
			// later request can probe again.
			if state.RecoveryInFlight {
				state.RecoveryInFlight = false
				p.circuits[engineID] = state
			}
			return
		case search.SEARCH_PROVIDER_AUTH_FAILED:
			// Invalid/expired credentials otherwise get retried on every query.
			state.Failures = p.failureLimit
			state.RecoveryInFlight = false
			state.OpenUntil = now.Add(5 * time.Minute)
			p.circuits[engineID] = state
			return
		case search.SEARCH_PROVIDER_RATE_LIMITED:
			openFor := p.openDuration
			if searchErr.RetryAfter > 0 {
				candidate := time.Duration(searchErr.RetryAfter) * time.Millisecond
				if candidate > openFor {
					openFor = candidate
				}
			}
			state.Failures = p.failureLimit
			state.RecoveryInFlight = false
			state.OpenUntil = now.Add(openFor)
			p.circuits[engineID] = state
			return
		}
		if !searchErr.Retryable {
			// A non-retryable request/provider response is not evidence that the
			// transport is unhealthy. A half-open probe reaching such a response
			// proves the engine is reachable, so close the old circuit.
			if state.RecoveryInFlight {
				delete(p.circuits, engineID)
			}
			return
		}
	}
	state.RecoveryInFlight = false
	state.Failures++
	state.LastFailure = now
	if state.Failures >= p.failureLimit {
		state.OpenUntil = now.Add(p.openDuration)
	}
	p.circuits[engineID] = state
}

// engineCircuitAllows is a read-only scheduling check. Once an open window has
// elapsed it reports the engine as probe-eligible, but it does not itself claim
// the probe. engineCircuitAcquire performs that atomic claim immediately before
// network execution.
func (p *Provider) engineCircuitAllows(engineID string, now time.Time) bool {
	p.circuitMu.Lock()
	defer p.circuitMu.Unlock()
	state, ok := p.circuits[engineID]
	if !ok || state.OpenUntil.IsZero() {
		return true
	}
	if now.Before(state.OpenUntil) {
		return false
	}
	return !state.RecoveryInFlight
}

func (p *Provider) engineCircuitAcquire(engineID string, now time.Time) bool {
	p.circuitMu.Lock()
	defer p.circuitMu.Unlock()
	state, ok := p.circuits[engineID]
	if !ok || state.OpenUntil.IsZero() {
		return true
	}
	if now.Before(state.OpenUntil) || state.RecoveryInFlight {
		return false
	}
	state.RecoveryInFlight = true
	p.circuits[engineID] = state
	return true
}

func (p *Provider) engineRateCandidateAllowed(engineID string, now time.Time) bool {
	p.rateMu.Lock()
	defer p.rateMu.Unlock()
	state, ok := p.rates[engineID]
	if !ok {
		return true
	}
	descriptor := EngineDescriptor{ID: engineID}
	if p.registry != nil {
		for _, engine := range p.registry.All() {
			current := normalizeDescriptor(engine.Descriptor())
			if current.ID == engineID {
				descriptor = current
				break
			}
		}
	}
	rate, burst := effectiveEngineRate(p.runtimeConfig, descriptor)
	refillRate := float64(rate) / 60.0
	elapsed := now.Sub(state.LastRefill).Seconds()
	tokens := state.Tokens + elapsed*refillRate
	if tokens > float64(burst) {
		tokens = float64(burst)
	}
	return tokens >= 1
}

func (p *Provider) acquireEngineRate(descriptor EngineDescriptor, now time.Time) error {
	rate, burst := effectiveEngineRate(p.runtimeConfig, descriptor)
	if rate <= 0 || burst <= 0 {
		return nil
	}
	p.rateMu.Lock()
	defer p.rateMu.Unlock()
	state, ok := p.rates[descriptor.ID]
	if !ok || state.LastRefill.IsZero() {
		state = engineRateState{Tokens: float64(burst), LastRefill: now}
	} else {
		refillRate := float64(rate) / 60.0
		state.Tokens += now.Sub(state.LastRefill).Seconds() * refillRate
		if state.Tokens > float64(burst) {
			state.Tokens = float64(burst)
		}
		state.LastRefill = now
	}
	if state.Tokens < 1 {
		retry := time.Second
		if rate > 0 {
			retry = time.Duration(float64(time.Minute) / float64(rate))
		}
		p.rates[descriptor.ID] = state
		err := search.NewError(search.SEARCH_PROVIDER_RATE_LIMITED, descriptor.ID, true, fmt.Errorf("native engine rate limit exceeded"))
		err.RetryAfter = search.DurationMs(retry.Milliseconds())
		return err
	}
	state.Tokens -= 1
	state.LastRefill = now
	p.rates[descriptor.ID] = state
	return nil
}

var (
	_ search.Provider                    = (*Provider)(nil)
	_ search.ContextCapabilitiesProvider = (*Provider)(nil)
	_ search.RequestCapabilityProvider   = (*Provider)(nil)
)
