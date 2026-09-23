package search

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeProviderForService struct {
	results []SearchResult
	err     *Error
	enabled bool
	calls   int
}

func (p *fakeProviderForService) ID() string { return "fake" }
func (p *fakeProviderForService) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{GeneralWeb: true, MaxResults: 20, LanguageFilter: true, CountryFilter: true, SafeSearch: true, TimeRangeFilter: true, DomainFilter: true}
}
func (p *fakeProviderForService) Search(ctx context.Context, req SearchRequest) (ProviderSearchResponse, error) {
	p.calls++
	select {
	case <-ctx.Done():
		return ProviderSearchResponse{}, NewError(SEARCH_CANCELLED, "fake", false, ctx.Err())
	default:
	}
	if p.err != nil {
		return ProviderSearchResponse{}, p.err
	}
	limit := req.Limit
	if limit <= 0 || limit > len(p.results) {
		limit = len(p.results)
	}
	out := make([]SearchResult, 0, limit)
	for i := 0; i < limit && i < len(p.results); i++ {
		out = append(out, p.results[i])
	}
	return ProviderSearchResponse{Results: out, HasMore: len(p.results) > limit}, nil
}
func (p *fakeProviderForService) Health(_ context.Context) ProviderHealth {
	if !p.enabled {
		return ProviderHealthDisabled
	}
	return ProviderHealthReady
}
func (p *fakeProviderForService) SetCredential(string) {}

func newTestService(provider *fakeProviderForService) *Service {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Providers = map[string]ProviderConfig{
		"fake": {Type: "fake", Enabled: true},
	}
	set := NewProviderSet("fake")
	set.Register("fake", provider)
	return NewService(cfg, set)
}

func TestService_Search_Basic(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{
		{Title: "Result1", URL: "https://a.com/", Source: SearchSourceMetadata{Provider: "fake"}},
		{Title: "Result2", URL: "https://b.com/", Source: SearchSourceMetadata{Provider: "fake"}},
	}}
	svc := newTestService(provider)
	resp, err := svc.Search(context.Background(), GeneralSearchRequest{Query: "test"}, "inv-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Provider != "fake" {
		t.Fatalf("provider mismatch: %s", resp.Provider)
	}
	if resp.Returned != 2 {
		t.Fatalf("expected 2 results, got %d", resp.Returned)
	}
	if resp.Results[0].Rank != 1 || resp.Results[1].Rank != 2 {
		t.Fatal("ranks not assigned")
	}
}

func TestService_Search_Disabled(t *testing.T) {
	provider := &fakeProviderForService{enabled: true}
	cfg := DefaultConfig()
	cfg.Enabled = false
	set := NewProviderSet("fake")
	set.Register("fake", provider)
	svc := NewService(cfg, set)
	_, err := svc.Search(context.Background(), GeneralSearchRequest{Query: "test"}, "")
	if err == nil || err.Code != SEARCH_DISABLED {
		t.Fatalf("expected SEARCH_DISABLED, got %v", err)
	}
}

func TestService_Search_EmptyQuery(t *testing.T) {
	provider := &fakeProviderForService{enabled: true}
	svc := newTestService(provider)
	_, err := svc.Search(context.Background(), GeneralSearchRequest{Query: ""}, "")
	if err == nil || err.Code != SEARCH_INVALID_QUERY {
		t.Fatalf("expected SEARCH_INVALID_QUERY, got %v", err)
	}
}

func TestService_Search_TooLongQuery(t *testing.T) {
	provider := &fakeProviderForService{enabled: true}
	svc := newTestService(provider)
	longQuery := ""
	for i := 0; i < MaxQueryRunes+1; i++ {
		longQuery += "q"
	}
	_, err := svc.Search(context.Background(), GeneralSearchRequest{Query: longQuery}, "")
	if err == nil || err.Code != SEARCH_INVALID_QUERY {
		t.Fatalf("expected SEARCH_INVALID_QUERY for too long, got %v", err)
	}
}

func TestService_Search_ProviderError(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, err: NewError(SEARCH_PROVIDER_TIMEOUT, "fake", true, errors.New("timeout"))}
	svc := newTestService(provider)
	_, err := svc.Search(context.Background(), GeneralSearchRequest{Query: "test"}, "")
	if err == nil {
		t.Fatal("expected provider error")
	}
	if err.Code != SEARCH_PROVIDER_TIMEOUT {
		t.Fatalf("expected SEARCH_PROVIDER_TIMEOUT, got %v", err.Code)
	}
}

func TestService_Search_ZeroResults(t *testing.T) {
	provider := &fakeProviderForService{enabled: true}
	svc := newTestService(provider)
	resp, err := svc.Search(context.Background(), GeneralSearchRequest{Query: "no-match-query-xyz"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Returned != 0 {
		t.Fatalf("expected 0 results, got %d", resp.Returned)
	}
}

func TestService_Search_WithCredentialResolver(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{
		{Title: "T", URL: "https://x.com/", Source: SearchSourceMetadata{Provider: "fake"}},
	}}
	svc := newTestService(provider)
	resolverCalls := 0
	svc.WithCredentialResolver(func(ctx context.Context, providerID, invocation, credentialRef string) (string, func(), error) {
		resolverCalls++
		if providerID != "fake" {
			t.Fatalf("wrong providerID: %s", providerID)
		}
		return "fake-api-key", func() {}, nil
	})
	cfg := svc.config
	cfg.Providers["fake"] = ProviderConfig{Type: "fake", Enabled: true, CredentialRef: "secret/brave"}
	svc.config = cfg
	_, err := svc.Search(context.Background(), GeneralSearchRequest{Query: "secret-test"}, "inv-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolverCalls != 1 {
		t.Fatalf("expected 1 resolver call, got %d", resolverCalls)
	}
}

func TestService_Search_CredentialError(t *testing.T) {
	provider := &fakeProviderForService{enabled: true}
	svc := newTestService(provider)
	resolverErr := errors.New("no lease")
	svc.WithCredentialResolver(func(ctx context.Context, providerID, invocation, credentialRef string) (string, func(), error) {
		return "", nil, resolverErr
	})
	cfg := svc.config
	cfg.Providers["fake"] = ProviderConfig{Type: "fake", Enabled: true, CredentialRef: "secret/brave"}
	svc.config = cfg
	_, err := svc.Search(context.Background(), GeneralSearchRequest{Query: "test"}, "")
	if err == nil || err.Code != SEARCH_PROVIDER_AUTH_FAILED {
		t.Fatalf("expected SEARCH_PROVIDER_AUTH_FAILED, got %v", err)
	}
}

func TestService_ExecuteFromJSON_InvalidJSON(t *testing.T) {
	provider := &fakeProviderForService{enabled: true}
	svc := newTestService(provider)
	_, err := svc.ExecuteFromJSON(context.Background(), json.RawMessage("{not json"), "")
	if err == nil || err.Code != SEARCH_INVALID_QUERY {
		t.Fatalf("expected SEARCH_INVALID_QUERY, got %v", err)
	}
}

func TestService_ExecuteFromJSON_Routing(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{
		{Title: "JSON test", URL: "https://json.com/", Source: SearchSourceMetadata{Provider: "fake"}},
	}}
	svc := newTestService(provider)
	input := ToolInput{Query: "json-query", Limit: 5}
	raw, _ := json.Marshal(input)
	resp, err := svc.ExecuteFromJSON(context.Background(), raw, "inv-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Query != "json-query" {
		t.Fatalf("query not passed through: %s", resp.Query)
	}
	if resp.Returned != 1 {
		t.Fatalf("expected 1 result, got %d", resp.Returned)
	}
	if len(resp.Results) > 0 && resp.Results[0].Source.Provider != "fake" {
		t.Fatalf("provider source not tagged: %s", resp.Results[0].Source.Provider)
	}
}

func TestService_ExecuteFromJSON_DefaultLimit(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{
		{Title: "A", URL: "https://a.com/", Source: SearchSourceMetadata{Provider: "fake"}},
		{Title: "B", URL: "https://b.com/", Source: SearchSourceMetadata{Provider: "fake"}},
		{Title: "C", URL: "https://c.com/", Source: SearchSourceMetadata{Provider: "fake"}},
	}}
	svc := newTestService(provider)
	input := ToolInput{Query: "limit-test"}
	raw, _ := json.Marshal(input)
	resp, err := svc.ExecuteFromJSON(context.Background(), raw, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Returned != 3 {
		t.Fatalf("expected 3 results since all fit, got %d", resp.Returned)
	}
}

func TestService_ExecuteFromJSON_DropsBadURLs(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{
		{Title: "Good", URL: "https://good.com/", Source: SearchSourceMetadata{Provider: "fake"}},
		{Title: "Bad", URL: "javascript:alert(1)", Source: SearchSourceMetadata{Provider: "fake"}},
	}}
	svc := newTestService(provider)
	raw, _ := json.Marshal(ToolInput{Query: "clean-me"})
	resp, err := svc.ExecuteFromJSON(context.Background(), raw, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Returned != 1 {
		t.Fatalf("expected 1 after dropping bad URL, got %d", resp.Returned)
	}
}

func TestService_ExecuteFromHTMLStripped(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{
		{Title: "<b>Bold</b>", URL: "https://bold.com/", Snippet: "<em>hi</em>", Source: SearchSourceMetadata{Provider: "fake"}},
	}}
	svc := newTestService(provider)
	raw, _ := json.Marshal(ToolInput{Query: "html-test"})
	resp, err := svc.ExecuteFromJSON(context.Background(), raw, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Results[0].Title != "Bold" {
		t.Fatalf("title not stripped: %q", resp.Results[0].Title)
	}
	if resp.Results[0].Snippet != "hi" {
		t.Fatalf("snippet not stripped: %q", resp.Results[0].Snippet)
	}
}

func TestService_DefaultProvider_NotConfigured(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.DefaultProvider = "nonexistent"
	set := NewProviderSet("")
	svc := NewService(cfg, set)
	_, err := svc.Search(context.Background(), GeneralSearchRequest{Query: "test"}, "")
	if err == nil || err.Code != SEARCH_PROVIDER_NOT_CONFIGURED {
		t.Fatalf("expected SEARCH_PROVIDER_NOT_CONFIGURED, got %v", err)
	}
}

func TestService_Search_DurationMs(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{
		{Title: "A", URL: "https://a.com/", Source: SearchSourceMetadata{Provider: "fake"}},
	}}
	svc := newTestService(provider)
	resp, err := svc.Search(context.Background(), GeneralSearchRequest{Query: "duration"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DurationMs < 0 {
		t.Fatal("DurationMs should be non-negative")
	}
	_ = time.Now
}

func TestService_ExecuteFromJSON_SafeSearchDefault(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{
		{Title: "S", URL: "https://s.com/", Source: SearchSourceMetadata{Provider: "fake"}},
	}}
	svc := newTestService(provider)
	raw, _ := json.Marshal(ToolInput{Query: "safesearch"})
	resp, err := svc.ExecuteFromJSON(context.Background(), raw, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
}

func TestService_CircuitBreakerOpensAfterRetryableFailures(t *testing.T) {
	provider := &fakeProviderForService{
		enabled: true,
		err:     NewError(SEARCH_PROVIDER_TIMEOUT, "fake", true, errors.New("timeout")),
	}
	svc := newTestService(provider)
	svc.config.CircuitFailures = 2
	svc.config.CircuitOpen = time.Minute

	for i := 0; i < 2; i++ {
		_, err := svc.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: "circuit-test", Kind: SearchKindWeb}, "", "fake")
		if err == nil || err.Code != SEARCH_PROVIDER_TIMEOUT {
			t.Fatalf("attempt %d: expected provider timeout, got %v", i+1, err)
		}
	}
	if provider.calls != 2 {
		t.Fatalf("expected provider to be called twice, got %d", provider.calls)
	}

	_, err := svc.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: "circuit-test-3", Kind: SearchKindWeb}, "", "fake")
	if err == nil || err.Code != SEARCH_PROVIDER_UNAVAILABLE {
		t.Fatalf("expected open circuit to return provider unavailable, got %v", err)
	}
	if provider.calls != 2 {
		t.Fatalf("open circuit should skip provider call, got %d calls", provider.calls)
	}
	if ids := svc.CandidateProviderIDs(SearchKindWeb); len(ids) != 0 {
		t.Fatalf("open circuit provider must be omitted from candidates: %v", ids)
	}
	if health := svc.Health(context.Background())["fake"]; health != ProviderHealthDegraded {
		t.Fatalf("expected degraded health while circuit open, got %s", health)
	}
}

func TestService_CircuitBreakerResetsAfterOpenWindow(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{{Title: "ok", URL: "https://ok.example/", Source: SearchSourceMetadata{Provider: "fake"}}}}
	svc := newTestService(provider)
	svc.circuits["fake"] = providerCircuitState{Phase: circuitOpen, ConsecutiveFailures: 3, OpenUntil: time.Now().Add(-time.Second)}

	ids := svc.CandidateProviderIDs(SearchKindWeb)
	if len(ids) != 1 || ids[0] != "fake" {
		t.Fatalf("expired circuit should allow provider again: %v", ids)
	}
	resp, err := svc.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: "recovered", Kind: SearchKindWeb}, "", "fake")
	if err != nil || resp == nil || resp.Returned != 1 {
		t.Fatalf("expected provider recovery, resp=%v err=%v", resp, err)
	}
}

func TestService_CircuitBreakerHalfOpenAllowsSingleProbe(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{{Title: "ok", URL: "https://ok.example/"}}}
	svc := newTestService(provider)
	svc.circuits["fake"] = providerCircuitState{Phase: circuitOpen, ConsecutiveFailures: 3, OpenUntil: time.Now().Add(-time.Second)}

	if !svc.providerCircuitAllows("fake", time.Now()) {
		t.Fatal("first half-open probe should be allowed")
	}
	if svc.providerCircuitAllows("fake", time.Now()) {
		t.Fatal("second concurrent half-open probe should be rejected")
	}
	svc.recordProviderOutcome("fake", nil)
	if !svc.providerCircuitAllows("fake", time.Now()) {
		t.Fatal("successful half-open probe should close the circuit")
	}
}

func TestService_CircuitBreakerHonorsRetryAfter(t *testing.T) {
	svc := newTestService(&fakeProviderForService{enabled: true})
	svc.config.CircuitFailures = 1
	svc.config.CircuitOpen = time.Second
	svc.recordProviderOutcome("fake", &Error{Code: SEARCH_PROVIDER_RATE_LIMITED, Retryable: true, RetryAfter: DurationMs(5000)})

	state := svc.circuits["fake"]
	if state.Phase != circuitOpen {
		t.Fatalf("phase=%s, want open", state.Phase)
	}
	if time.Until(state.OpenUntil) < 4*time.Second {
		t.Fatalf("Retry-After was not honored, open until %s", state.OpenUntil)
	}
}

func TestServiceProviderErrorUsesConfiguredInstanceID(t *testing.T) {
	providers := NewProviderSet("brave_primary")
	provider := &fakeProviderForService{enabled: true, err: NewError(SEARCH_PROVIDER_RATE_LIMITED, "brave", true, errors.New("limited"))}
	providers.RegisterWithPriority("brave_primary", provider, 100)
	svc := NewService(Config{
		Enabled: true, DefaultProvider: "brave_primary",
		Providers: map[string]ProviderConfig{"brave_primary": {Enabled: true}},
	}, providers)

	_, got := svc.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: "test", Kind: SearchKindWeb}, "invoke", "brave_primary")
	if got == nil {
		t.Fatal("expected provider error")
	}
	if got.Provider != "brave_primary" {
		t.Fatalf("provider error identity=%q, want brave_primary", got.Provider)
	}
	if got.Code != SEARCH_PROVIDER_RATE_LIMITED || !got.Retryable {
		t.Fatalf("provider error metadata lost: %#v", got)
	}
}

func TestCircuitStatusTracksRollingFailuresAndRecovery(t *testing.T) {
	providers := NewProviderSet("brave_primary")
	provider := &fakeProviderForService{enabled: true, err: NewError(SEARCH_PROVIDER_TIMEOUT, "brave", true, errors.New("timeout"))}
	providers.RegisterWithPriority("brave_primary", provider, 100)
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.DefaultProvider = "brave_primary"
	cfg.CircuitFailures = 2
	cfg.CircuitOpen = time.Millisecond
	cfg.Providers = map[string]ProviderConfig{"brave_primary": {Enabled: true}}
	svc := NewService(cfg, providers)

	for i := 0; i < 2; i++ {
		_, _ = svc.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: "circuit", Kind: SearchKindWeb}, "invoke", "brave_primary")
	}
	status := svc.CircuitStatus("brave_primary")
	if status.Phase != string(circuitOpen) || status.ConsecutiveFailures != 2 || status.FailureRate <= 0 {
		t.Fatalf("unexpected open status: %+v", status)
	}
	if status.OpenedAt == nil || status.OpenUntil == nil || status.LastFailure == nil {
		t.Fatalf("missing circuit timestamps: %+v", status)
	}

	time.Sleep(2 * time.Millisecond)
	provider.err = nil
	provider.results = []SearchResult{{Title: "ok", URL: "https://example.com"}}
	_, err := svc.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: "recovery", Kind: SearchKindWeb}, "invoke", "brave_primary")
	if err != nil {
		t.Fatalf("half-open recovery failed: %v", err)
	}
	status = svc.CircuitStatus("brave_primary")
	if status.Phase != string(circuitClosed) || status.RecoveryAttempts < 1 || status.RecoveryInFlight {
		t.Fatalf("unexpected recovered status: %+v", status)
	}
}

type requestValidatingProvider struct {
	fakeProviderForService
	validationErr *Error
	validated     int
}

func (p *requestValidatingProvider) ValidateSearchRequest(ctx context.Context, request SearchRequest) *Error {
	p.validated++
	return p.validationErr
}

func TestService_SearchAdvancedUsesRequestCapabilityProvider(t *testing.T) {
	provider := &requestValidatingProvider{
		fakeProviderForService: fakeProviderForService{enabled: true},
		validationErr:          NewError(SEARCH_FILTER_UNSUPPORTED, "validator", false, nil),
	}
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Providers = map[string]ProviderConfig{"validator": {Type: "probe", Enabled: true}}
	set := NewProviderSet("validator")
	set.Register("validator", provider)
	svc := NewService(cfg, set)
	_, err := svc.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: "test", Kind: SearchKindWeb}, "", "validator")
	if err == nil || err.Code != SEARCH_FILTER_UNSUPPORTED {
		t.Fatalf("expected SEARCH_FILTER_UNSUPPORTED, got %v", err)
	}
	if provider.validated != 1 {
		t.Fatalf("validation calls = %d", provider.validated)
	}
	if provider.calls != 0 {
		t.Fatalf("provider search should not run after request validation failure; calls=%d", provider.calls)
	}
}

func TestService_TopLevelProviderDoesNotLoadNativeCredentialStore(t *testing.T) {
	provider := &fakeProviderForService{enabled: true, results: []SearchResult{{Title: "ok", URL: "https://example.com"}}}
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.DefaultProvider = "brave_primary"
	cfg.Providers = map[string]ProviderConfig{
		"brave_primary": {Type: "brave", Enabled: true},
	}
	set := NewProviderSet("brave_primary")
	set.Register("brave_primary", provider)
	svc := NewService(cfg, set)
	factoryCalls := 0
	svc.WithEngineCredentialSourceFactory(func(ctx context.Context, providerID, invocation string) (EngineCredentialSource, func(), error) {
		factoryCalls++
		return nil, func() {}, nil
	})

	resp, err := svc.SearchAdvancedWithProvider(context.Background(), SearchRequest{Query: "test", Kind: SearchKindWeb}, "invoke", "brave_primary")
	if err != nil || resp == nil {
		t.Fatalf("unexpected search result: resp=%v err=%v", resp, err)
	}
	if factoryCalls != 0 {
		t.Fatalf("top-level provider should not query native engine credential store; calls=%d", factoryCalls)
	}
}
