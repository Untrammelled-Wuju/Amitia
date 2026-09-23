package search

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type CredentialResolver func(ctx context.Context, providerID, invocation, credentialRef string) (credential string, release func(), err error)
type EngineCredentialResolver func(ctx context.Context, providerID, invocation string) (credentials map[string]string, release func(), err error)
type EngineCredentialSourceFactory func(ctx context.Context, providerID, invocation string) (source EngineCredentialSource, release func(), err error)

type CacheMetrics struct {
	Hit         int64 `json:"hit"`
	Miss        int64 `json:"miss"`
	Expired     int64 `json:"expired"`
	Evicted     int64 `json:"evicted"`
	NegativeHit int64 `json:"negative_hit"`
}

type cacheMetricCounters struct {
	hit         atomic.Int64
	miss        atomic.Int64
	expired     atomic.Int64
	evicted     atomic.Int64
	negativeHit atomic.Int64
}

type Service struct {
	providers                     *ProviderSet
	config                        Config
	normalizer                    *Normalizer
	credentialResolver            CredentialResolver
	engineCredentialResolver      EngineCredentialResolver
	engineCredentialSourceFactory EngineCredentialSourceFactory
	citationBuilder               *CitationBuilder
	cacheMu                       sync.RWMutex
	cache                         map[string]searchCacheEntry
	circuitMu                     sync.Mutex
	circuits                      map[string]providerCircuitState
	cacheMetrics                  cacheMetricCounters
}

type circuitPhase string

const (
	circuitClosed   circuitPhase = "closed"
	circuitOpen     circuitPhase = "open"
	circuitHalfOpen circuitPhase = "half_open"
)

type providerCircuitState struct {
	Phase               circuitPhase
	ConsecutiveFailures int
	OpenUntil           time.Time
	OpenedAt            time.Time
	RecoveryInFlight    bool
	RecoveryAttempts    int
	RecentFailures      int
	RecentTotal         int
	RecentOutcomes      []bool
	LastFailure         time.Time
}

type CircuitStatus struct {
	Phase               string     `json:"phase"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	FailureRate         float64    `json:"failureRate"`
	RecentFailures      int        `json:"recentFailures"`
	RecentTotal         int        `json:"recentTotal"`
	LastFailure         *time.Time `json:"lastFailure,omitempty"`
	OpenedAt            *time.Time `json:"openedAt,omitempty"`
	OpenUntil           *time.Time `json:"openUntil,omitempty"`
	RecoveryAttempts    int        `json:"recoveryAttempts"`
	RecoveryInFlight    bool       `json:"recoveryInFlight"`
}

type searchCacheEntry struct {
	expiresAt time.Time
	touchedAt time.Time
	response  SearchResponse
}

func NewService(config Config, providers *ProviderSet) *Service {
	return &Service{
		providers:          providers,
		config:             config,
		normalizer:         NewNormalizer(),
		credentialResolver: noopResolver,
		citationBuilder:    NewCitationBuilder(),
		cache:              make(map[string]searchCacheEntry),
		circuits:           make(map[string]providerCircuitState),
	}
}

func (s *Service) WithCredentialResolver(r CredentialResolver) *Service {
	if r != nil {
		s.credentialResolver = r
	}
	return s
}

func (s *Service) WithEngineCredentialResolver(r EngineCredentialResolver) *Service {
	if r != nil {
		s.engineCredentialResolver = r
	}
	return s
}

func (s *Service) WithEngineCredentialSourceFactory(factory EngineCredentialSourceFactory) *Service {
	if factory != nil {
		s.engineCredentialSourceFactory = factory
	}
	return s
}

func noopResolver(ctx context.Context, providerID, invocation, credentialRef string) (string, func(), error) {
	return "", func() {}, nil
}

func (s *Service) Search(ctx context.Context, req GeneralSearchRequest, invocation string) (*GeneralSearchResponse, *Error) {
	if !s.config.Enabled {
		return nil, NewError(SEARCH_DISABLED, "", false, nil)
	}
	if _, verr := sanitizeQuery(req.Query); verr != nil {
		return nil, NewError(SEARCH_INVALID_QUERY, "", false, verr)
	}
	provider, resolvedProviderID, serr := s.resolveProvider()
	if serr != nil {
		return nil, serr
	}
	if !s.providerCircuitAllows(resolvedProviderID, time.Now()) {
		return nil, NewError(SEARCH_PROVIDER_UNAVAILABLE, resolvedProviderID, true, fmt.Errorf("provider circuit is open"))
	}
	ctx, releaseCred, rerr := s.contextWithCredential(ctx, resolvedProviderID, invocation)
	if rerr != nil {
		return nil, rerr
	}
	defer releaseCred()
	start := time.Now()
	searchCtx, cancel := context.WithTimeout(ctx, s.config.EffectiveTimeout())
	defer cancel()
	raw, perr := provider.Search(searchCtx, SearchRequest{
		Query:      req.Query,
		Kind:       SearchKindWeb,
		Limit:      req.Limit,
		Offset:     req.Offset,
		Language:   req.Language,
		Country:    req.Country,
		SafeSearch: req.SafeSearch,
	})
	if perr != nil {
		searchErr := s.normalizeProviderError(resolvedProviderID, perr)
		s.recordProviderOutcome(resolvedProviderID, searchErr)
		return nil, searchErr
	}
	s.recordProviderOutcome(resolvedProviderID, nil)
	results, _ := s.normalizer.NormalizeResults(raw.Results, resolvedProviderID)
	s.normalizer.AssignRanks(results)
	resp := &GeneralSearchResponse{
		Query:       req.Query,
		Provider:    resolvedProviderID,
		Results:     results,
		Returned:    len(results),
		Offset:      req.Offset,
		HasMore:     raw.HasMore,
		RetrievedAt: time.Now().UTC(),
		DurationMs:  time.Since(start).Milliseconds(),
		Usage:       raw.Usage,
	}
	return resp, nil
}

func (s *Service) SearchAdvanced(ctx context.Context, req SearchRequest, invocation string) (*SearchResponse, *Error) {
	return s.SearchAdvancedWithProvider(ctx, req, invocation, "")
}

func (s *Service) SearchAdvancedWithProvider(ctx context.Context, req SearchRequest, invocation, providerID string) (*SearchResponse, *Error) {
	if !s.config.Enabled {
		return nil, NewError(SEARCH_DISABLED, "", false, nil)
	}
	if _, verr := sanitizeQuery(req.Query); verr != nil {
		return nil, NewError(SEARCH_INVALID_QUERY, "", false, verr)
	}
	kind := NormalizeKind(req.Kind)
	provider, resolvedProviderID, serr := s.resolveProviderForKindWithID(kind, providerID)
	if serr != nil {
		return nil, serr
	}
	cacheKey := s.searchCacheKey(resolvedProviderID, req)
	if cached, ok := s.getCachedSearch(cacheKey); ok {
		cached.CacheHit = true
		cached.Usage = ProviderUsage{}
		return &cached, nil
	}
	if !s.providerCircuitAllows(resolvedProviderID, time.Now()) {
		return nil, NewError(SEARCH_PROVIDER_UNAVAILABLE, resolvedProviderID, true, fmt.Errorf("provider circuit is open"))
	}
	ctx, releaseCred, rerr := s.contextWithCredential(ctx, resolvedProviderID, invocation)
	if rerr != nil {
		return nil, rerr
	}
	defer releaseCred()
	if validator, ok := provider.(RequestCapabilityProvider); ok {
		if ferr := validator.ValidateSearchRequest(ctx, req); ferr != nil {
			return nil, ferr
		}
	} else {
		caps := provider.Capabilities()
		if contextual, ok := provider.(ContextCapabilitiesProvider); ok {
			caps = contextual.CapabilitiesForContext(ctx)
		}
		if req.Offset > 0 && !caps.Pagination {
			return nil, NewError(SEARCH_FILTER_UNSUPPORTED, resolvedProviderID, false, fmt.Errorf("provider does not support pagination"))
		}
		ferr := ProviderSupportsFilter(caps, kind, req.Language != "", req.Country != "", req.SafeSearch != "", req.TimeRange != nil, len(req.Domains) > 0, len(req.ExcludeDomains) > 0)
		if ferr != nil {
			ferr.Provider = resolvedProviderID
			return nil, ferr
		}
	}
	start := time.Now()
	searchCtx, cancel := context.WithTimeout(ctx, s.config.EffectiveTimeout())
	defer cancel()
	raw, perr := provider.Search(searchCtx, req)
	if perr != nil {
		searchErr := s.normalizeProviderError(resolvedProviderID, perr)
		s.recordProviderOutcome(resolvedProviderID, searchErr)
		return nil, searchErr
	}
	s.recordProviderOutcome(resolvedProviderID, nil)
	results, _ := s.normalizer.NormalizeResults(raw.Results, resolvedProviderID)
	s.normalizer.AssignRanks(results)
	citationSet := s.citationBuilder.Build(results, kind)
	s.citationBuilder.AssignCitations(results, citationSet)
	resp := &SearchResponse{
		Query:       req.Query,
		Kind:        kind,
		Provider:    resolvedProviderID,
		Results:     results,
		Returned:    len(results),
		Offset:      req.Offset,
		HasMore:     raw.HasMore,
		RetrievedAt: time.Now().UTC(),
		DurationMs:  time.Since(start).Milliseconds(),
		Usage:       raw.Usage,
		CitationSet: citationSet,
	}
	s.putCachedSearch(cacheKey, *resp)
	return resp, nil
}

func (s *Service) resolveProvider() (Provider, string, *Error) {
	provider, ok := s.providers.Default()
	if ok {
		return provider, s.providers.DefaultID(), nil
	}
	if s.config.DefaultProvider != "" {
		provider, ok = s.providers.Get(s.config.DefaultProvider)
		if ok {
			return provider, s.config.DefaultProvider, nil
		}
		return nil, "", NewError(SEARCH_PROVIDER_NOT_CONFIGURED, s.config.DefaultProvider, false, nil)
	}
	return nil, "", NewError(SEARCH_PROVIDER_NOT_CONFIGURED, "", false, nil)
}

func (s *Service) resolveProviderForKind(kind SearchKind) (Provider, string, *Error) {
	return s.resolveProviderForKindWithID(kind, "")
}

func (s *Service) resolveProviderForKindWithID(kind SearchKind, providerID string) (Provider, string, *Error) {
	if providerID != "" && providerID != "default" {
		provider, ok := s.providers.Get(providerID)
		if !ok || !s.config.IsProviderEnabled(providerID) {
			return nil, "", NewError(SEARCH_PROVIDER_NOT_CONFIGURED, providerID, false, nil)
		}
		if !SupportsKind(provider.Capabilities(), kind) {
			return nil, "", NewError(SEARCH_KIND_UNSUPPORTED, providerID, false, nil)
		}
		return provider, providerID, nil
	}
	provider, ok := s.providers.Default()
	if ok && SupportsKind(provider.Capabilities(), kind) {
		return provider, s.providers.DefaultID(), nil
	}
	candidateIDs := s.providers.CandidateIDs(kind)
	if len(candidateIDs) > 0 {
		provider, _ = s.providers.Get(candidateIDs[0])
		return provider, candidateIDs[0], nil
	}
	return nil, "", NewError(SEARCH_KIND_UNSUPPORTED, "", false, nil)
}

func (s *Service) CandidateProviderIDs(kind SearchKind) []string {
	kind = NormalizeKind(kind)
	ids := s.providers.CandidateIDs(kind)
	if len(ids) == 0 {
		return ids
	}

	candidateSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		candidateSet[id] = struct{}{}
	}
	ordered := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	appendID := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := candidateSet[id]; !ok {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ordered = append(ordered, id)
	}

	routeKeys := []string{string(kind)}
	if kind != SearchKindWeb {
		routeKeys = append(routeKeys, "general")
	} else {
		routeKeys = []string{"general", string(kind)}
	}
	for _, key := range routeKeys {
		route, ok := s.config.Routes[key]
		if !ok {
			continue
		}
		for _, id := range route.Preferred {
			appendID(id)
		}
		for _, id := range route.Fallback {
			appendID(id)
		}
		break
	}
	for _, id := range ids {
		appendID(id)
	}

	now := time.Now()
	filtered := make([]string, 0, len(ordered))
	for _, id := range ordered {
		if s.providerCircuitCandidateAllowed(id, now) {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

func (s *Service) Health(ctx context.Context) map[string]ProviderHealth {
	result := make(map[string]ProviderHealth)
	for id, p := range s.providers.All() {
		result[id] = s.effectiveProviderHealth(id, p.Health(ctx))
	}
	return result
}

func (s *Service) DefaultProviderHealth(ctx context.Context) (string, ProviderHealth) {
	provider, ok := s.providers.Default()
	if !ok {
		return "", ProviderHealthMisconfigured
	}
	id := s.providers.DefaultID()
	return id, s.effectiveProviderHealth(id, provider.Health(ctx))
}

func (s *Service) normalizeProviderError(providerID string, err error) *Error {
	if searchErr, ok := err.(*Error); ok {
		return &Error{
			Code:       searchErr.Code,
			Provider:   providerID,
			HTTPStatus: searchErr.HTTPStatus,
			RetryAfter: searchErr.RetryAfter,
			Retryable:  searchErr.Retryable,
			Cause:      searchErr.Cause,
		}
	}
	return NewError(SEARCH_PROVIDER_UNAVAILABLE, providerID, true, err)
}

func (s *Service) providerCircuitCandidateAllowed(providerID string, now time.Time) bool {
	if strings.TrimSpace(providerID) == "" {
		return true
	}
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	state, ok := s.circuits[providerID]
	if !ok || state.Phase == "" || state.Phase == circuitClosed {
		return true
	}
	if state.Phase == circuitOpen {
		return !now.Before(state.OpenUntil)
	}
	if state.Phase == circuitHalfOpen {
		return !state.RecoveryInFlight
	}
	return true
}

func (s *Service) providerCircuitAllows(providerID string, now time.Time) bool {
	if strings.TrimSpace(providerID) == "" {
		return true
	}
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	state := s.circuits[providerID]
	if state.Phase == "" {
		state.Phase = circuitClosed
	}
	switch state.Phase {
	case circuitClosed:
		s.circuits[providerID] = state
		return true
	case circuitOpen:
		if now.Before(state.OpenUntil) {
			return false
		}
		state.Phase = circuitHalfOpen
		state.RecoveryInFlight = true
		state.RecoveryAttempts++
		s.circuits[providerID] = state
		return true
	case circuitHalfOpen:
		if state.RecoveryInFlight {
			return false
		}
		state.RecoveryInFlight = true
		s.circuits[providerID] = state
		return true
	default:
		state.Phase = circuitClosed
		s.circuits[providerID] = state
		return true
	}
}

func (s *Service) recordProviderOutcome(providerID string, searchErr *Error) {
	if strings.TrimSpace(providerID) == "" {
		return
	}
	now := time.Now()
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	state := s.circuits[providerID]
	if state.Phase == "" {
		state.Phase = circuitClosed
	}
	if searchErr != nil && searchErr.Code == SEARCH_CANCELLED {
		state.RecoveryInFlight = false
		s.circuits[providerID] = state
		return
	}
	retryableFailure := searchErr != nil && searchErr.Retryable
	state = appendCircuitOutcome(state, retryableFailure)
	if searchErr == nil {
		state.Phase = circuitClosed
		state.ConsecutiveFailures = 0
		state.OpenUntil = time.Time{}
		state.OpenedAt = time.Time{}
		state.RecoveryInFlight = false
		s.circuits[providerID] = state
		return
	}
	if !searchErr.Retryable {
		if state.Phase == circuitHalfOpen {
			state.Phase = circuitClosed
			state.OpenUntil = time.Time{}
			state.OpenedAt = time.Time{}
		}
		state.RecoveryInFlight = false
		s.circuits[providerID] = state
		return
	}
	state.LastFailure = now
	state.ConsecutiveFailures++
	failureRate := circuitFailureRate(state)
	shouldOpen := state.Phase == circuitHalfOpen ||
		state.ConsecutiveFailures >= s.config.EffectiveCircuitFailures() ||
		(state.RecentTotal >= 5 && failureRate >= 0.60)
	if shouldOpen {
		openFor := s.config.EffectiveCircuitOpen()
		if searchErr.RetryAfter > 0 {
			retryAfter := time.Duration(searchErr.RetryAfter) * time.Millisecond
			if retryAfter > openFor {
				openFor = retryAfter
			}
		}
		state.Phase = circuitOpen
		state.OpenedAt = now
		state.OpenUntil = now.Add(openFor)
		state.RecoveryInFlight = false
	}
	s.circuits[providerID] = state
}

func appendCircuitOutcome(state providerCircuitState, failure bool) providerCircuitState {
	const window = 20
	state.RecentOutcomes = append(state.RecentOutcomes, failure)
	if len(state.RecentOutcomes) > window {
		state.RecentOutcomes = append([]bool(nil), state.RecentOutcomes[len(state.RecentOutcomes)-window:]...)
	}
	state.RecentTotal = len(state.RecentOutcomes)
	state.RecentFailures = 0
	for _, failed := range state.RecentOutcomes {
		if failed {
			state.RecentFailures++
		}
	}
	return state
}

func circuitFailureRate(state providerCircuitState) float64 {
	if state.RecentTotal <= 0 {
		return 0
	}
	return float64(state.RecentFailures) / float64(state.RecentTotal)
}

func (s *Service) CircuitStatus(providerID string) CircuitStatus {
	if s == nil || strings.TrimSpace(providerID) == "" {
		return CircuitStatus{Phase: string(circuitClosed)}
	}
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	state, ok := s.circuits[strings.TrimSpace(providerID)]
	if !ok || state.Phase == "" {
		state.Phase = circuitClosed
	}
	return circuitStatusFromState(state)
}

func (s *Service) CircuitStatuses() map[string]CircuitStatus {
	result := make(map[string]CircuitStatus)
	if s == nil || s.providers == nil {
		return result
	}
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	for _, id := range s.providers.CandidateIDs(SearchKindWeb) {
		state := s.circuits[id]
		if state.Phase == "" {
			state.Phase = circuitClosed
		}
		result[id] = circuitStatusFromState(state)
	}
	for id := range s.providers.All() {
		if _, exists := result[id]; exists {
			continue
		}
		state := s.circuits[id]
		if state.Phase == "" {
			state.Phase = circuitClosed
		}
		result[id] = circuitStatusFromState(state)
	}
	return result
}

func circuitStatusFromState(state providerCircuitState) CircuitStatus {
	status := CircuitStatus{
		Phase:               string(state.Phase),
		ConsecutiveFailures: state.ConsecutiveFailures,
		FailureRate:         circuitFailureRate(state),
		RecentFailures:      state.RecentFailures,
		RecentTotal:         state.RecentTotal,
		RecoveryAttempts:    state.RecoveryAttempts,
		RecoveryInFlight:    state.RecoveryInFlight,
	}
	if !state.LastFailure.IsZero() {
		value := state.LastFailure.UTC()
		status.LastFailure = &value
	}
	if !state.OpenedAt.IsZero() {
		value := state.OpenedAt.UTC()
		status.OpenedAt = &value
	}
	if !state.OpenUntil.IsZero() {
		value := state.OpenUntil.UTC()
		status.OpenUntil = &value
	}
	return status
}

func (s *Service) providerCircuitDegraded(providerID string, now time.Time) bool {
	if strings.TrimSpace(providerID) == "" {
		return false
	}
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	state, ok := s.circuits[providerID]
	if !ok || state.Phase == "" || state.Phase == circuitClosed {
		return false
	}
	if state.Phase == circuitOpen && !now.Before(state.OpenUntil) {
		return true
	}
	return state.Phase == circuitOpen || state.Phase == circuitHalfOpen
}

func (s *Service) effectiveProviderHealth(providerID string, base ProviderHealth) ProviderHealth {
	if base != ProviderHealthReady {
		return base
	}
	if s.providerCircuitDegraded(providerID, time.Now()) {
		return ProviderHealthDegraded
	}
	return base
}

func (s *Service) ExecuteFromJSON(ctx context.Context, input json.RawMessage, invocation string) (*SearchResponse, *Error) {
	return s.ExecuteFromJSONWithProvider(ctx, input, invocation, "")
}

func (s *Service) ExecuteFromJSONWithProvider(ctx context.Context, input json.RawMessage, invocation, providerID string) (*SearchResponse, *Error) {
	var toolInput ToolInput
	if err := json.Unmarshal(input, &toolInput); err != nil {
		return nil, NewError(SEARCH_INVALID_QUERY, "", false, err)
	}
	validated, verr := validateAndNormalize(&toolInput)
	if verr != nil {
		return nil, verr
	}
	req := toolInput.ToRequest()
	req.Query = validated.query
	req.Limit = validated.limit
	req.Offset = validated.offset
	req.Language = validated.language
	req.Country = validated.country
	req.SafeSearch = validated.safeSearch
	req.Kind = validated.kind
	req.Domains = validated.domains
	req.ExcludeDomains = validated.excludeDomains
	return s.SearchAdvancedWithProvider(ctx, req, invocation, providerID)
}

type referenceEngineCredentialSource struct {
	providerID string
	invocation string
	refs       map[string]string
	resolver   CredentialResolver
}

func (s *referenceEngineCredentialSource) Available(_ context.Context, engineID string) bool {
	if s == nil || s.resolver == nil {
		return false
	}
	engineID = normalizeEngineCredentialID(engineID)
	if engineID == "google_cse" {
		// google_cse may be stored as one composite JSON secret or as two
		// separate secret references (API key + CX). A primary reference is
		// required in either form.
		return strings.TrimSpace(s.refs["google_cse"]) != ""
	}
	return strings.TrimSpace(s.refs[engineID]) != ""
}

func (s *referenceEngineCredentialSource) Resolve(ctx context.Context, engineID string) (string, func(), error) {
	if s == nil || s.resolver == nil {
		return "", func() {}, nil
	}
	engineID = normalizeEngineCredentialID(engineID)
	ref := strings.TrimSpace(s.refs[engineID])
	if ref == "" {
		return "", func() {}, nil
	}
	primary, primaryRelease, err := s.resolver(ctx, s.providerID, s.invocation, ref)
	if err != nil {
		return "", primaryRelease, err
	}
	if engineID != "google_cse" {
		return primary, primaryRelease, nil
	}
	// A single reference is allowed to contain the already-composed JSON
	// credential. If a separate CX reference is configured, compose a JSON
	// payload compatible with googleCSEEngine and release both leases together.
	cxRef := strings.TrimSpace(s.refs["google_cse_cx"])
	if cxRef == "" {
		return primary, primaryRelease, nil
	}
	cx, cxRelease, err := s.resolver(ctx, s.providerID, s.invocation, cxRef)
	if err != nil {
		if primaryRelease != nil {
			primaryRelease()
		}
		return "", cxRelease, err
	}
	payload, err := json.Marshal(map[string]string{"apiKey": primary, "cx": cx})
	if err != nil {
		if cxRelease != nil {
			cxRelease()
		}
		if primaryRelease != nil {
			primaryRelease()
		}
		return "", func() {}, err
	}
	release := func() {
		if cxRelease != nil {
			cxRelease()
		}
		if primaryRelease != nil {
			primaryRelease()
		}
	}
	return string(payload), release, nil
}

type chainedEngineCredentialSource struct {
	sources []EngineCredentialSource
}

func (s *chainedEngineCredentialSource) Available(ctx context.Context, engineID string) bool {
	if s == nil {
		return false
	}
	for _, source := range s.sources {
		if source != nil && source.Available(ctx, engineID) {
			return true
		}
	}
	return false
}

func (s *chainedEngineCredentialSource) Resolve(ctx context.Context, engineID string) (string, func(), error) {
	if s == nil {
		return "", func() {}, nil
	}
	for _, source := range s.sources {
		if source == nil || !source.Available(ctx, engineID) {
			continue
		}
		return source.Resolve(ctx, engineID)
	}
	return "", func() {}, nil
}

func chainEngineCredentialSources(sources ...EngineCredentialSource) EngineCredentialSource {
	filtered := make([]EngineCredentialSource, 0, len(sources))
	for _, source := range sources {
		if source != nil {
			filtered = append(filtered, source)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return &chainedEngineCredentialSource{sources: filtered}
}

func (s *Service) contextWithCredential(ctx context.Context, providerID, invocation string) (context.Context, func(), *Error) {
	credRef := s.config.ProviderCredentialRef(providerID)
	engineRefs := s.config.ProviderEngineCredentials(providerID)
	releases := make([]func(), 0, 2)
	releaseAll := func() {
		for index := len(releases) - 1; index >= 0; index-- {
			if releases[index] != nil {
				releases[index]()
			}
		}
	}
	if credRef != "" {
		if s.credentialResolver != nil {
			credential, release, err := s.credentialResolver(ctx, providerID, invocation, credRef)
			if err != nil {
				return ctx, releaseAll, NewError(SEARCH_PROVIDER_AUTH_FAILED, providerID, false, err)
			}
			if release != nil {
				releases = append(releases, release)
			}
			ctx = ContextWithProviderCredential(ctx, credential)
		}
	}
	var configuredSource EngineCredentialSource
	if len(engineRefs) > 0 && s.credentialResolver != nil {
		normalized := make(map[string]string, len(engineRefs))
		for engineID, ref := range engineRefs {
			engineID = normalizeEngineCredentialID(engineID)
			ref = strings.TrimSpace(ref)
			if engineID != "" && ref != "" {
				normalized[engineID] = ref
			}
		}
		if len(normalized) > 0 {
			configuredSource = &referenceEngineCredentialSource{providerID: providerID, invocation: invocation, refs: normalized, resolver: s.credentialResolver}
		}
	}
	var storedSource EngineCredentialSource
	if s.engineCredentialSourceFactory != nil && s.providerUsesEngineCredentials(providerID) {
		source, release, err := s.engineCredentialSourceFactory(ctx, providerID, invocation)
		if err != nil {
			return ctx, releaseAll, NewError(SEARCH_PROVIDER_AUTH_FAILED, providerID, false, err)
		}
		if release != nil {
			releases = append(releases, release)
		}
		storedSource = source
	}
	if source := chainEngineCredentialSources(configuredSource, storedSource); source != nil {
		ctx = ContextWithEngineCredentialSource(ctx, source)
	} else if s.engineCredentialResolver != nil {
		// Legacy compatibility path for older integrations/tests. Production
		// wiring uses the lazy source factory above so native search never loads
		// every engine secret into one request context.
		credentials, release, err := s.engineCredentialResolver(ctx, providerID, invocation)
		if err != nil {
			return ctx, releaseAll, NewError(SEARCH_PROVIDER_AUTH_FAILED, providerID, false, err)
		}
		if release != nil {
			releases = append(releases, release)
		}
		for engineID, credential := range credentials {
			ctx = ContextWithEngineCredential(ctx, engineID, credential)
		}
	}
	return ctx, releaseAll, nil
}

func (s *Service) providerUsesEngineCredentials(providerID string) bool {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return false
	}
	if provider, ok := s.config.Providers[providerID]; ok {
		if strings.EqualFold(strings.TrimSpace(provider.Type), ProviderNative) {
			return true
		}
		// Custom/plugin providers may explicitly declare engine credentials. Keep
		// that extension point without making every ordinary top-level provider
		// query the native credential store.
		return len(provider.EngineCredentials) > 0
	}
	return strings.EqualFold(providerID, ProviderNative)
}

func (s *Service) searchCacheKey(providerID string, req SearchRequest) string {
	canonical := canonicalSearchRequest(req)
	payload, _ := json.Marshal(struct {
		Provider string        `json:"provider"`
		Request  SearchRequest `json:"request"`
	}{Provider: strings.TrimSpace(providerID), Request: canonical})
	return string(payload)
}

func canonicalSearchRequest(req SearchRequest) SearchRequest {
	req.Query = strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(req.Query)), " "))
	req.Kind = NormalizeKind(req.Kind)
	req.Language = strings.ToLower(strings.TrimSpace(req.Language))
	req.Country = strings.ToUpper(strings.TrimSpace(req.Country))
	req.Domains = canonicalStringSet(req.Domains)
	req.ExcludeDomains = canonicalStringSet(req.ExcludeDomains)
	if req.TimeRange != nil {
		clone := *req.TimeRange
		clone.From = canonicalCacheTime(clone.From)
		clone.To = canonicalCacheTime(clone.To)
		req.TimeRange = &clone
	}
	return req
}

func canonicalStringSet(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func canonicalCacheTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	// Search freshness is coarse-grained at provider level. Hour-level
	// normalization prevents recency-based requests created seconds apart from
	// defeating the cache while still keeping time-sensitive searches fresh.
	normalized := value.UTC().Truncate(time.Hour)
	return &normalized
}

func (s *Service) getCachedSearch(key string) (SearchResponse, bool) {
	if s == nil || key == "" || s.config.EffectiveCacheTTL() <= 0 {
		return SearchResponse{}, false
	}
	now := time.Now().UTC()
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	entry, ok := s.cache[key]
	if !ok {
		s.cacheMetrics.miss.Add(1)
		return SearchResponse{}, false
	}
	if !entry.expiresAt.After(now) {
		delete(s.cache, key)
		s.cacheMetrics.expired.Add(1)
		s.cacheMetrics.miss.Add(1)
		return SearchResponse{}, false
	}
	entry.touchedAt = now
	s.cache[key] = entry
	if entry.response.Returned == 0 {
		s.cacheMetrics.negativeHit.Add(1)
	} else {
		s.cacheMetrics.hit.Add(1)
	}
	return cloneSearchResponse(entry.response), true
}

func (s *Service) putCachedSearch(key string, response SearchResponse) {
	if s == nil || key == "" || s.config.EffectiveCacheTTL() <= 0 {
		return
	}
	now := time.Now().UTC()
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	for existingKey, entry := range s.cache {
		if !entry.expiresAt.After(now) {
			delete(s.cache, existingKey)
			s.cacheMetrics.expired.Add(1)
		}
	}
	maxEntries := s.config.EffectiveCacheMaxEntries()
	if _, replacing := s.cache[key]; !replacing && len(s.cache) >= maxEntries {
		var oldestKey string
		var oldestTime time.Time
		for existingKey, entry := range s.cache {
			stamp := entry.touchedAt
			if stamp.IsZero() {
				stamp = entry.expiresAt.Add(-s.config.EffectiveCacheTTL())
			}
			if oldestKey == "" || stamp.Before(oldestTime) {
				oldestKey = existingKey
				oldestTime = stamp
			}
		}
		if oldestKey != "" {
			delete(s.cache, oldestKey)
			s.cacheMetrics.evicted.Add(1)
		}
	}
	response.CacheHit = false
	ttl := s.config.EffectiveCacheTTL()
	if response.Returned == 0 {
		negativeTTL := s.config.EffectiveNegativeCacheTTL()
		if negativeTTL <= 0 {
			return
		}
		if negativeTTL < ttl {
			ttl = negativeTTL
		}
	}
	s.cache[key] = searchCacheEntry{
		expiresAt: now.Add(ttl),
		touchedAt: now,
		response:  cloneSearchResponse(response),
	}
}

func cloneSearchResponse(in SearchResponse) SearchResponse {
	out := in
	out.Results = append([]SearchResult(nil), in.Results...)
	for i := range out.Results {
		out.Results[i].Metadata.Authors = append([]string(nil), in.Results[i].Metadata.Authors...)
		out.Results[i].Source.Engines = append([]string(nil), in.Results[i].Source.Engines...)
	}
	out.CitationSet.Citations = append([]Citation(nil), in.CitationSet.Citations...)
	for i := range out.CitationSet.Citations {
		out.CitationSet.Citations[i].Engines = append([]string(nil), in.CitationSet.Citations[i].Engines...)
		out.CitationSet.Citations[i].Metadata.Authors = append([]string(nil), in.CitationSet.Citations[i].Metadata.Authors...)
	}
	return out
}

func (s *Service) ProviderHealth(ctx context.Context, providerID string) ProviderHealth {
	if s == nil || s.providers == nil {
		return ProviderHealthMisconfigured
	}
	providerID = strings.TrimSpace(providerID)
	if providerID != "" && providerID != "default" {
		provider, ok := s.providers.Get(providerID)
		if !ok {
			return ProviderHealthMisconfigured
		}
		return s.effectiveProviderHealth(providerID, provider.Health(ctx))
	}
	if provider, ok := s.providers.Default(); ok {
		id := s.providers.DefaultID()
		return s.effectiveProviderHealth(id, provider.Health(ctx))
	}
	ids := s.providers.CandidateIDs(SearchKindWeb)
	if len(ids) == 0 {
		return ProviderHealthMisconfigured
	}
	provider, ok := s.providers.Get(ids[0])
	if !ok {
		return ProviderHealthMisconfigured
	}
	return s.effectiveProviderHealth(ids[0], provider.Health(ctx))
}

func (s *Service) DebugCacheSize() int {
	if s == nil {
		return 0
	}
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return len(s.cache)
}

func (s *Service) CacheMetrics() CacheMetrics {
	if s == nil {
		return CacheMetrics{}
	}
	return CacheMetrics{
		Hit:         s.cacheMetrics.hit.Load(),
		Miss:        s.cacheMetrics.miss.Load(),
		Expired:     s.cacheMetrics.expired.Load(),
		Evicted:     s.cacheMetrics.evicted.Load(),
		NegativeHit: s.cacheMetrics.negativeHit.Load(),
	}
}

func (s *Service) ProviderManifests() []ProviderManifest {
	if s == nil || s.providers == nil {
		return nil
	}
	return s.providers.Manifests()
}
