package tavily

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/search"
)

func TestBuildRequestMapsNewsFilters(t *testing.T) {
	provider := NewProvider("key", "", "", true)
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	req, err := provider.buildRequest(context.Background(), search.SearchRequest{
		Query:          "latest ai",
		Kind:           search.SearchKindNews,
		Limit:          20,
		Language:       "en",
		SafeSearch:     search.SafeSearchModerate,
		Domains:        []string{"openai.com"},
		ExcludeDomains: []string{"example.com"},
		TimeRange:      &search.TimeRangeFilter{From: &from, To: &to},
	}, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost || req.Header.Get("Authorization") != "Bearer secret" {
		t.Fatalf("unexpected request: method=%s auth=%q", req.Method, req.Header.Get("Authorization"))
	}
	var payload requestPayload
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Topic != "news" || payload.MaxResults != 20 || !payload.SafeSearch || !payload.FilterByLanguage || !payload.IncludeUsage {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.StartDate != "2026-09-01" || payload.EndDate != "2026-09-23" {
		t.Fatalf("unexpected date range: %+v", payload)
	}
}

func TestMapSuccessParsesPublishedDate(t *testing.T) {
	provider := NewProvider("key", "", "", true)
	body := []byte(`{"results":[{"title":"Doc","url":"https://example.com/a","content":"snippet","score":0.9,"published_date":"Tue, 22 Sep 2026 17:00:00 GMT","favicon":"https://example.com/favicon.ico"}],"usage":{"credits":1}}`)
	resp, err := provider.handleResponse(http.StatusOK, body, search.SearchRequest{Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Snippet != "snippet" || resp.Results[0].PublishedAt == nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Usage.Credits != 1 {
		t.Fatalf("unexpected provider credits: %+v", resp.Usage)
	}
}

func TestPlanLimitIsNotRetryable(t *testing.T) {
	provider := NewProvider("key", "", "", true)
	_, err := provider.handleResponse(432, []byte(`{"detail":{"error":"plan limit"}}`), search.SearchRequest{})
	searchErr, ok := err.(*search.Error)
	if !ok || searchErr.Code != search.SEARCH_PROVIDER_REQUEST_REJECTED || searchErr.Retryable {
		t.Fatalf("unexpected error: %#v", err)
	}
}
