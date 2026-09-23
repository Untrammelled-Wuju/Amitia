package webresearch

import (
	"context"
	"testing"
)

type fakeAdvancedFetchProvider struct {
	id     string
	calls  int
	result AdvancedFetchResult
	err    *Error
}

func (p *fakeAdvancedFetchProvider) ID() string { return p.id }

func (p *fakeAdvancedFetchProvider) Fetch(_ context.Context, _, _ string) (AdvancedFetchResult, *Error) {
	p.calls++
	return p.result, p.err
}

func TestAdvancedFetchProviderPublicResultAndDedup(t *testing.T) {
	r := NewRuntime(DefaultConfig(), nil, nil, nil)
	first := &fakeAdvancedFetchProvider{id: "crawl_primary", result: AdvancedFetchResult{
		Title:       "Example",
		ContentType: "text/markdown",
		Content:     "# Example\n\nadvanced content",
		Blocks:      []string{"Example", "advanced content"},
	}}
	duplicate := &fakeAdvancedFetchProvider{id: "CRAWL_PRIMARY"}
	r.WithFetchProvider(first).WithFetchProvider(duplicate)
	if got := len(r.fetchProviders); got != 1 {
		t.Fatalf("expected duplicate provider IDs to be ignored, got %d providers", got)
	}

	page, used, err := r.advancedFetch(context.Background(), "https://93.184.216.34/article", "example")
	if err != nil {
		t.Fatalf("advancedFetch returned error: %v", err)
	}
	if !used {
		t.Fatal("expected advanced fetch provider to be used")
	}
	if first.calls != 1 || duplicate.calls != 0 {
		t.Fatalf("unexpected provider calls: first=%d duplicate=%d", first.calls, duplicate.calls)
	}
	if page.URL != "https://93.184.216.34/article" || page.Content == "" || page.Hash == "" {
		t.Fatalf("unexpected normalized page: %#v", page)
	}
}

func TestAdvancedFetchValidatesURLBeforePlugin(t *testing.T) {
	r := NewRuntime(DefaultConfig(), nil, nil, nil)
	provider := &fakeAdvancedFetchProvider{id: "crawl_primary", result: AdvancedFetchResult{Content: "should not be returned"}}
	r.WithFetchProvider(provider)

	_, used, err := r.advancedFetch(context.Background(), "http://127.0.0.1/private", "private")
	if err == nil || err.Code != ErrFetchBlocked {
		t.Fatalf("expected ErrFetchBlocked, got used=%v err=%v", used, err)
	}
	if used {
		t.Fatal("blocked URL must not reach advanced provider")
	}
	if provider.calls != 0 {
		t.Fatalf("provider received blocked URL: calls=%d", provider.calls)
	}
}

func TestAdvancedFetchFallsThroughProviderErrors(t *testing.T) {
	r := NewRuntime(DefaultConfig(), nil, nil, nil)
	broken := &fakeAdvancedFetchProvider{id: "broken", err: newError(ErrFetchFailed, "upstream failed", true, nil)}
	working := &fakeAdvancedFetchProvider{id: "working", result: AdvancedFetchResult{Content: "working provider content"}}
	r.WithFetchProvider(broken).WithFetchProvider(working)

	page, used, err := r.advancedFetch(context.Background(), "https://93.184.216.34/", "example")
	if err != nil || !used {
		t.Fatalf("expected fallback provider success, used=%v err=%v", used, err)
	}
	if broken.calls != 1 || working.calls != 1 || page.Content != "working provider content" {
		t.Fatalf("unexpected fallback result: broken=%d working=%d page=%#v", broken.calls, working.calls, page)
	}
}
