package searxng

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/search"
)

func TestBuildRequestIncludesFiltersAndCategory(t *testing.T) {
	provider := NewProvider("http://127.0.0.1:8080", true, true, true)
	from := time.Now().UTC().Add(-24 * time.Hour)
	req, err := provider.buildRequest(context.Background(), search.SearchRequest{
		Query:      "amitia search",
		Kind:       search.SearchKindNews,
		Language:   "zh-CN",
		SafeSearch: search.SafeSearchStrict,
		Domains:    []string{"github.com", "example.com"},
		TimeRange:  &search.TimeRangeFilter{From: &from},
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	q := req.URL.Query()
	if q.Get("format") != "json" || q.Get("categories") != "news" {
		t.Fatalf("unexpected query: %s", req.URL.RawQuery)
	}
	if q.Get("safesearch") != "2" || q.Get("language") != "zh-CN" {
		t.Fatalf("filters missing: %s", req.URL.RawQuery)
	}
	searchQuery, _ := url.QueryUnescape(q.Get("q"))
	if !strings.Contains(searchQuery, "site:github.com") || !strings.Contains(searchQuery, "site:example.com") {
		t.Fatalf("domain filter missing: %q", searchQuery)
	}
	if q.Get("time_range") == "" {
		t.Fatal("expected time_range")
	}
}
