package search

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type CredentialResolver func(ctx context.Context, providerID, invocation, credentialRef string) (credential string, release func(), err error)

type Service struct {
	providers          *ProviderSet
	config             Config
	normalizer         *Normalizer
	credentialResolver CredentialResolver
	citationBuilder    *CitationBuilder
	cacheMu            sync.RWMutex
	cache              map[string]searchCacheEntry
	circuitMu          sync.Mutex
	circuits           map[string]providerCircuitState
}

type providerCircuitState struct {
	ConsecutiveFailures int
	OpenUntil           time.Time
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
		searchErr := s.normalizeProviderError(provider.ID(), perr)
		s.recordProviderOutcome(resolvedProviderID, searchErr)
		return nil, searchErr
	}
	s.recordProviderOutcome(resolvedProviderID, nil)
	results, _ := s.normalizer.NormalizeResults(raw.Results, provider.ID())
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
	ferr := ProviderSupportsFilter(provider.Capabilities(), kind, req.Language != "", req.Country != "", req.SafeSearch != "", req.TimeRange != nil, len(req.Domains) > 0)
	if ferr != nil {
		return nil, ferr
	}
	cacheKey := s.searchCacheKey(resolvedProviderID, req)
	if cached, ok := s.getCachedSearch(cacheKey); ok {
		cached.CacheHit = true
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
	start := time.Now()
	searchCtx, cancel := context.WithTimeout(ctx, s.config.EffectiveTimeout())
	defer cancel()
	raw, perr := provider.Search(searchCtx, req)
	if perr != nil {
		searchErr := s.normalizeProviderError(provider.ID(), perr)
		s.recordProviderOutcome(resolvedProviderID, searchErr)
		return nil, searchErr
	}
	s.recordProviderOutcome(resolvedProviderID, nil)
	results, _ := s.normalizer.NormalizeResults(raw.Results, provider.ID())
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
	ids := s.providers.CandidateIDs(NormalizeKind(kind))
	if len(ids) == 0 {
		return ids
	}
	now := time.Now()
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if s.providerCircuitAllows(id, now) {
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
		return searchErr
	}
	return NewError(SEARCH_PROVIDER_UNAVAILABLE, providerID, true, err)
}

func (s *Service) providerCircuitAllows(providerID string, now time.Time) bool {
	if strings.TrimSpace(providerID) == "" {
		return true
	}
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	state, ok := s.circuits[providerID]
	if !ok || state.OpenUntil.IsZero() {
		return true
	}
	if !now.Before(state.OpenUntil) {
		state.OpenUntil = time.Time{}
		state.ConsecutiveFailures = 0
		s.circuits[providerID] = state
		return true
	}
	return false
}

func (s *Service) recordProviderOutcome(providerID string, searchErr *Error) {
	if strings.TrimSpace(providerID) == "" {
		return
	}
	s.circuitMu.Lock()
	defer s.circuitMu.Unlock()
	state := s.circuits[providerID]
	if searchErr == nil {
		state.ConsecutiveFailures = 0
		state.OpenUntil = time.Time{}
		s.circuits[providerID] = state
		return
	}
	if !searchErr.Retryable || searchErr.Code == SEARCH_CANCELLED {
		return
	}
	state.ConsecutiveFailures++
	if state.ConsecutiveFailures >= s.config.EffectiveCircuitFailures() {
		state.OpenUntil = time.Now().Add(s.config.EffectiveCircuitOpen())
	}
	s.circuits[providerID] = state
}

func (s *Service) effectiveProviderHealth(providerID string, base ProviderHealth) ProviderHealth {
	if base != ProviderHealthReady {
		return base
	}
	if !s.providerCircuitAllows(providerID, time.Now()) {
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
	return s.SearchAdvancedWithProvider(ctx, req, invocation, providerID)
}

func (s *Service) contextWithCredential(ctx context.Context, providerID, invocation string) (context.Context, func(), *Error) {
	credRef := s.config.ProviderCredentialRef(providerID)
	if s.credentialResolver == nil || credRef == "" {
		return ctx, func() {}, nil
	}
	credential, release, err := s.credentialResolver(ctx, providerID, invocation, credRef)
	if err != nil {
		return ctx, func() {}, NewError(SEARCH_PROVIDER_AUTH_FAILED, providerID, false, err)
	}
	if release == nil {
		release = func() {}
	}
	return ContextWithProviderCredential(ctx, credential), release, nil
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
	req.Query = strings.Join(strings.Fields(strings.TrimSpace(req.Query)), " ")
	req.Kind = NormalizeKind(req.Kind)
	req.Language = strings.ToLower(strings.TrimSpace(req.Language))
	req.Country = strings.ToUpper(strings.TrimSpace(req.Country))
	req.Domains = canonicalStringSet(req.Domains)
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
		return SearchResponse{}, false
	}
	if !entry.expiresAt.After(now) {
		delete(s.cache, key)
		return SearchResponse{}, false
	}
	entry.touchedAt = now
	s.cache[key] = entry
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
		}
	}
	response.CacheHit = false
	s.cache[key] = searchCacheEntry{
		expiresAt: now.Add(s.config.EffectiveCacheTTL()),
		touchedAt: now,
		response:  cloneSearchResponse(response),
	}
}

func cloneSearchResponse(in SearchResponse) SearchResponse {
	out := in
	out.Results = append([]SearchResult(nil), in.Results...)
	for i := range out.Results {
		out.Results[i].Metadata.Authors = append([]string(nil), in.Results[i].Metadata.Authors...)
	}
	out.CitationSet.Citations = append([]Citation(nil), in.CitationSet.Citations...)
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
