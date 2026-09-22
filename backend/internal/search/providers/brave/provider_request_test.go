package brave

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/search"
)

func TestBuildRequestUsesScopedCredentialAndFilters(t *testing.T) {
	provider := NewProvider("", "secret://brave", "", true)
	from := time.Now().UTC().Add(-24 * time.Hour)
	req, err := provider.buildRequest(context.Background(), search.SearchRequest{
		Query:      "amitia runtime",
		Limit:      7,
		Language:   "en",
		Country:    "US",
		SafeSearch: search.SafeSearchStrict,
		Domains:    []string{"github.com"},
		TimeRange:  &search.TimeRangeFilter{From: &from},
	}, "scoped-token")
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got := req.Header.Get("X-Subscription-Token"); got != "scoped-token" {
		t.Fatalf("credential header=%q", got)
	}
	q := req.URL.Query()
	if q.Get("count") != "7" || q.Get("search_lang") != "en" || q.Get("country") != "US" {
		t.Fatalf("missing request filters: %s", req.URL.RawQuery)
	}
	if q.Get("safesearch") != "strict" || q.Get("freshness") == "" {
		t.Fatalf("missing safety/freshness: %s", req.URL.RawQuery)
	}
	if !strings.Contains(q.Get("q"), "site:github.com") {
		t.Fatalf("domain filter missing: %q", q.Get("q"))
	}
}
