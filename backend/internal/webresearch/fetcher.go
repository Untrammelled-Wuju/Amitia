package webresearch

import (
	"bytes"
	"compress/gzip"
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
	pdfParser    PDFParser
}

func NewFetcher(config Config) *Fetcher {
	config = config.normalize()
	return &Fetcher{
		transport:    search.NewConfiguredTransport(true, false, false),
		timeout:      config.FetchTimeout,
		maxBytes:     config.MaxFetchBytes,
		maxChars:     config.MaxPageChars,
		maxRedirects: config.MaxRedirects,
		pdfParser:    newPopplerPDFParser(config),
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
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/pdf,text/plain,text/markdown,application/json;q=0.8,*/*;q=0.1")
		// Ask servers not to compress so byte limits apply before decompression.
		// If a server ignores this header, readResponse still enforces both raw
		// and decoded limits for gzip payloads.
		req.Header.Set("Accept-Encoding", "identity")
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
		page, ferr := f.readResponse(ctx, resp, current, query)
		resp.Body.Close()
		return page, ferr
	}
	return fetchedPage{}, newError(ErrFetchFailed, "redirect limit exceeded", false, nil)
}

func (f *Fetcher) readResponse(ctx context.Context, resp *http.Response, current, query string) (fetchedPage, *Error) {
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType, _, _ := mime.ParseMediaType(contentType)
	mediaType = strings.ToLower(mediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	switch mediaType {
	case "text/html", "application/xhtml+xml", "application/pdf", "text/plain", "text/markdown", "application/json", "application/ld+json":
	default:
		return fetchedPage{}, newError(ErrUnsupportedContent, mediaType, false, nil)
	}
	if resp.ContentLength > f.maxBytes && resp.ContentLength >= 0 {
		return fetchedPage{}, newError(ErrFetchFailed, "response exceeds byte limit", false, nil)
	}
	body, err := readBoundedHTTPBody(resp.Body, resp.Header.Get("Content-Encoding"), f.maxBytes)
	if err != nil {
		return fetchedPage{}, err
	}
	canonical := canonicalizeURL(current)
	if mediaType == "application/pdf" {
		if f.pdfParser == nil || !f.pdfParser.Available(ctx) {
			return fetchedPage{}, newError(ErrUnsupportedContent, "application/pdf parser unavailable", false, nil)
		}
		doc, parseErr := f.pdfParser.Parse(ctx, body)
		if parseErr != nil {
			return fetchedPage{}, parseErr
		}
		content, truncated := formatPDFDocument(doc, f.maxChars)
		return fetchedPage{URL: current, CanonicalURL: canonical, Title: doc.Title, ContentType: mediaType, Content: content, Truncated: truncated, Hash: hashBytes(body)}, nil
	}
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

func readBoundedHTTPBody(body io.Reader, contentEncoding string, maxBytes int64) ([]byte, *Error) {
	if maxBytes <= 0 {
		maxBytes = 6 * 1024 * 1024
	}
	raw, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, newError(ErrFetchFailed, "read response failed", true, err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, newError(ErrFetchFailed, "raw response exceeds byte limit", false, nil)
	}
	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	if encoding == "" || encoding == "identity" {
		return raw, nil
	}
	if encoding != "gzip" {
		return nil, newError(ErrUnsupportedContent, "unsupported content encoding: "+encoding, false, nil)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, newError(ErrFetchFailed, "invalid gzip response", false, err)
	}
	defer zr.Close()
	decoded, err := io.ReadAll(io.LimitReader(zr, maxBytes+1))
	if err != nil {
		return nil, newError(ErrFetchFailed, "decompress response failed", false, err)
	}
	if int64(len(decoded)) > maxBytes {
		return nil, newError(ErrFetchFailed, "decoded response exceeds byte limit", false, nil)
	}
	if len(raw) > 0 && len(decoded) > len(raw)*100 {
		return nil, newError(ErrFetchFailed, "response decompression ratio exceeds limit", false, nil)
	}
	return decoded, nil
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
	if err != nil || u.Scheme == "" || u.Host == "" {
		return strings.TrimSpace(raw)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	hostname := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	if strings.Contains(hostname, ":") {
		if port != "" {
			u.Host = "[" + hostname + "]:" + port
		} else {
			u.Host = "[" + hostname + "]"
		}
	} else if port != "" {
		u.Host = hostname + ":" + port
	} else {
		u.Host = hostname
	}
	u.Fragment = ""
	query := u.Query()
	for key := range query {
		if isTrackingURLParam(key) {
			query.Del(key)
		}
	}
	u.RawQuery = query.Encode()
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

func isTrackingURLParam(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if strings.HasPrefix(key, "utm_") {
		return true
	}
	switch key {
	case "fbclid", "gclid", "dclid", "msclkid", "mc_cid", "mc_eid", "igshid", "yclid", "vero_id", "_hsenc", "_hsmi":
		return true
	default:
		return false
	}
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
