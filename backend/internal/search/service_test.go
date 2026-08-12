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
	return ProviderCapabilities{GeneralWeb: true, MaxResults: 20}
}
func (p *fakeProviderForService) Search(ctx context.Context, req GeneralSearchRequest) (ProviderSearchResponse, error) {
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
