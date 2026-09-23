package webresearch

import (
	"context"
	"strings"
)

// AdvancedFetchResult is the provider-neutral result returned by an advanced
// crawl/fetch backend. It intentionally contains only web content metadata;
// provider credentials, cookies and transport internals never cross this seam.
type AdvancedFetchResult struct {
	URL          string
	CanonicalURL string
	Title        string
	ContentType  string
	Content      string
	Blocks       []string
	Links        []Link
	Truncated    bool
	Dynamic      bool
}

// AdvancedFetchProvider is the public host/plugin seam for crawl or advanced
// extraction services (for example Firecrawl-like providers). Providers never
// become model-visible tools; they are escalation backends behind web_run.
//
// The runtime validates the target URL with the built-in SSRF policy before a
// provider receives it. Implementations must still apply their own network
// policy because remote fetch infrastructure has a separate trust boundary.
type AdvancedFetchProvider interface {
	ID() string
	Fetch(ctx context.Context, rawURL, query string) (AdvancedFetchResult, *Error)
}

func (r *Runtime) WithFetchProvider(provider AdvancedFetchProvider) *Runtime {
	if r == nil || provider == nil || strings.TrimSpace(provider.ID()) == "" {
		return r
	}
	for _, existing := range r.fetchProviders {
		if existing != nil && strings.EqualFold(strings.TrimSpace(existing.ID()), strings.TrimSpace(provider.ID())) {
			return r
		}
	}
	r.fetchProviders = append(r.fetchProviders, provider)
	return r
}

func (r *Runtime) advancedFetch(ctx context.Context, rawURL, query string) (fetchedPage, bool, *Error) {
	if r == nil || len(r.fetchProviders) == 0 {
		return fetchedPage{}, false, nil
	}
	if r.fetcher == nil {
		return fetchedPage{}, false, newError(ErrNotConfigured, "safe fetch validator unavailable", false, nil)
	}
	if err := r.fetcher.ValidateURL(ctx, rawURL); err != nil {
		return fetchedPage{}, false, err
	}
	var lastErr *Error
	for _, provider := range r.fetchProviders {
		if provider == nil {
			continue
		}
		result, err := provider.Fetch(ctx, rawURL, query)
		if err != nil {
			lastErr = err
			continue
		}
		if strings.TrimSpace(result.Content) == "" {
			lastErr = newError(ErrFetchFailed, "advanced fetch provider returned empty content", true, nil)
			continue
		}
		page := fetchedPage{
			URL:          strings.TrimSpace(result.URL),
			CanonicalURL: strings.TrimSpace(result.CanonicalURL),
			Title:        strings.TrimSpace(result.Title),
			ContentType:  strings.TrimSpace(result.ContentType),
			Content:      result.Content,
			Blocks:       append([]string(nil), result.Blocks...),
			Links:        append([]Link(nil), result.Links...),
			Truncated:    result.Truncated,
			Dynamic:      result.Dynamic,
		}
		if page.URL == "" {
			page.URL = rawURL
		}
		if page.CanonicalURL == "" {
			page.CanonicalURL = canonicalizeURL(page.URL)
		}
		page.Hash = hashBytes([]byte(page.Content))
		return page, true, nil
	}
	return fetchedPage{}, false, lastErr
}
