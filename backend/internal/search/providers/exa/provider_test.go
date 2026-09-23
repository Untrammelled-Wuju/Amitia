package exa

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/search"
)

func TestBuildRequestMapsFilters(t *testing.T) {
	provider := NewProvider("key", "", "", true)
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	req, err := provider.buildRequest(context.Background(), search.SearchRequest{
		Query:          "codex search",
		Limit:          12,
		SafeSearch:     search.SafeSearchStrict,
		Domains:        []string{"openai.com", "openai.com"},
		ExcludeDomains: []string{"example.com"},
		TimeRange:      &search.TimeRangeFilter{From: &from, To: &to},
	}, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost || req.Header.Get("x-api-key") != "secret" {
		t.Fatalf("unexpected request: method=%s auth=%q", req.Method, req.Header.Get("x-api-key"))
	}
	var payload requestPayload
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.NumResults != 12 || !payload.Moderation || len(payload.IncludeDomains) != 1 || len(payload.ExcludeDomains) != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.StartPublishedDate == "" || payload.EndPublishedDate == "" {
		t.Fatalf("missing time range: %+v", payload)
	}
}

func TestMapSuccessPrefersSummaryAndAuthor(t *testing.T) {
	provider := NewProvider("key", "", "", true)
	body := []byte(`{"results":[{"title":"Doc","url":"https://example.com/a","publishedDate":"2026-09-22T10:00:00Z","author":"A","text":"long text","highlights":["h"],"summary":"summary"}],"costDollars":{"total":0.007}}`)
	resp, err := provider.handleResponse(http.StatusOK, body, search.SearchRequest{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Snippet != "summary" || len(resp.Results[0].Metadata.Authors) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Results[0].PublishedAt == nil {
		t.Fatal("publishedAt not parsed")
	}
	if resp.Usage.CostUSD != 0.007 {
		t.Fatalf("unexpected provider cost: %+v", resp.Usage)
	}
}

func TestMapHTTPError(t *testing.T) {
	provider := NewProvider("key", "", "", true)
	_, err := provider.handleResponse(http.StatusTooManyRequests, []byte(`{"error":"limited"}`), search.SearchRequest{})
	searchErr, ok := err.(*search.Error)
	if !ok || searchErr.Code != search.SEARCH_PROVIDER_RATE_LIMITED || !searchErr.Retryable {
		t.Fatalf("unexpected error: %#v", err)
	}
}
