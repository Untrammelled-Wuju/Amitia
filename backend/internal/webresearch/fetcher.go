package webresearch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
)

type fetchedPage struct {
	URL          string
	CanonicalURL string
	Title        string
	ContentType  string
	Content      string
	Blocks       []string
	Links        []Link
	Truncated    bool
	Dynamic      bool
	Hash         string
}

type Fetcher struct {
	transport    *search.SecureTransport
	timeout      time.Duration
	maxBytes     int64
	maxChars     int
	maxRedirects int
}

func NewFetcher(config Config) *Fetcher {
	config = config.normalize()
	return &Fetcher{
		transport:    search.NewConfiguredTransport(true, false, false),
		timeout:      config.FetchTimeout,
		maxBytes:     config.MaxFetchBytes,
		maxChars:     config.MaxPageChars,
		maxRedirects: config.MaxRedirects,
	}
}

func (f *Fetcher) ValidateURL(ctx context.Context, rawURL string) *Error {
	if f == nil || f.transport == nil {
		return newError(ErrNotConfigured, "fetch runtime unavailable", false, nil)
	}
	if _, err := f.transport.ValidateEndpoint(ctx, strings.TrimSpace(rawURL)); err != nil {
		return newError(ErrFetchBlocked, "URL rejected by network policy", false, err)
	}
	return nil
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL, query string) (fetchedPage, *Error) {
	_ = query // Page cache is query-independent; evidence is generated per query by Runtime.
	current := strings.TrimSpace(rawURL)
	if current == "" {
		return fetchedPage{}, newError(ErrInvalidInput, "empty URL", false, nil)
	}
	for redirect := 0; redirect <= f.maxRedirects; redirect++ {
		if err := ctx.Err(); err != nil {
			return fetchedPage{}, newError(ErrCancelled, "fetch cancelled", false, err)
		}
		validated, err := f.transport.ValidateEndpoint(ctx, current)
		if err != nil {
			return fetchedPage{}, newError(ErrFetchBlocked, "URL rejected by network policy", false, err)
		}
		client := f.transport.PinHTTPClient(validated, f.timeout)
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return fetchedPage{}, newError(ErrInvalidInput, "invalid URL", false, err)
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,application/json;q=0.8,*/*;q=0.1")
		req.Header.Set("User-Agent", "Amitia-WebResearch/1.0")
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return fetchedPage{}, newError(ErrCancelled, "fetch cancelled", false, ctx.Err())
			}
			return fetchedPage{}, newError(ErrFetchFailed, "HTTP request failed", true, err)
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := strings.TrimSpace(resp.Header.Get("Location"))
			resp.Body.Close()
			if location == "" {
				return fetchedPage{}, newError(ErrFetchFailed, "redirect has no location", false, nil)
			}
			next, err := resolveRedirect(current, location)
			if err != nil {
				return fetchedPage{}, newError(ErrFetchBlocked, "invalid redirect target", false, err)
			}
			current = next
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
			return fetchedPage{}, newError(ErrFetchFailed, "unexpected HTTP status", retryable, nil)
		}
		page, ferr := f.readResponse(resp, current, query)
		resp.Body.Close()
		return page, ferr
	}
	return fetchedPage{}, newError(ErrFetchFailed, "redirect limit exceeded", false, nil)
}

func (f *Fetcher) readResponse(resp *http.Response, current, query string) (fetchedPage, *Error) {
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType, _, _ := mime.ParseMediaType(contentType)
	mediaType = strings.ToLower(mediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	switch mediaType {
	case "text/html", "application/xhtml+xml", "text/plain", "application/json", "application/ld+json":
	default:
		return fetchedPage{}, newError(ErrUnsupportedContent, mediaType, false, nil)
	}
	reader := io.LimitReader(resp.Body, f.maxBytes+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return fetchedPage{}, newError(ErrFetchFailed, "read response failed", true, err)
	}
	if int64(len(body)) > f.maxBytes {
		return fetchedPage{}, newError(ErrFetchFailed, "response exceeds byte limit", false, nil)
	}
	canonical := canonicalizeURL(current)
	raw := string(body)
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		doc, err := extractHTML(raw, current, "", f.maxChars)
		if err != nil {
			return fetchedPage{}, newError(ErrFetchFailed, "HTML extraction failed", false, err)
		}
		content := doc.Content
		truncated := len([]rune(content)) >= f.maxChars
		return fetchedPage{URL: current, CanonicalURL: canonical, Title: doc.Title, ContentType: mediaType, Content: content, Blocks: doc.Blocks, Links: doc.Links, Truncated: truncated, Dynamic: doc.Dynamic, Hash: hashBytes(body)}, nil
	}
	content := raw
	truncated := false
	if len([]rune(content)) > f.maxChars {
		content = string([]rune(content)[:f.maxChars])
		truncated = true
	}
	blocks := splitTextBlocks(content)
	return fetchedPage{URL: current, CanonicalURL: canonical, ContentType: mediaType, Content: content, Blocks: blocks, Truncated: truncated, Hash: hashBytes(body)}, nil
}

func resolveRedirect(baseURL, location string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	next, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(next).String(), nil
}

func canonicalizeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	u.Fragment = ""
	query := u.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" || lower == "mc_cid" || lower == "mc_eid" {
			query.Del(key)
		}
	}
	u.RawQuery = query.Encode()
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

func hashBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func splitTextBlocks(value string) []string {
	parts := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = normalizeSpace(part)
		if len([]rune(part)) >= 20 {
			out = append(out, part)
		}
	}
	if len(out) == 0 && strings.TrimSpace(value) != "" {
		out = append(out, truncateRunes(normalizeSpace(value), 4000))
	}
	return out
}
