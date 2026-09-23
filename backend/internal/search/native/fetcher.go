package native

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
)

type Fetcher struct {
	transport *search.SecureTransport
	client    *http.Client
	maxBytes  int64
}

func NewFetcher(maxBytes int64) *Fetcher {
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024
	}
	transport := search.NewConfiguredTransport(false, false, true)
	return &Fetcher{
		transport: transport,
		client:    transport.NewHTTPClient(12 * time.Second),
		maxBytes:  maxBytes,
	}
}

func (f *Fetcher) SetMaxBytes(maxBytes int64) {
	if maxBytes > 0 {
		f.maxBytes = maxBytes
	}
}

func (f *Fetcher) Get(ctx context.Context, engineID, rawURL string, headers http.Header) ([]byte, int, error) {
	return f.do(ctx, engineID, http.MethodGet, rawURL, headers, nil)
}

func (f *Fetcher) PostJSON(ctx context.Context, engineID, rawURL string, headers http.Header, payload any) ([]byte, int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, search.NewError(search.SEARCH_PROVIDER_REQUEST_FAILED, engineID, false, err)
	}
	return f.do(ctx, engineID, http.MethodPost, rawURL, headers, body)
}

func (f *Fetcher) do(ctx context.Context, engineID, method, rawURL string, headers http.Header, payload []byte) ([]byte, int, error) {
	if f == nil {
		return nil, 0, search.NewError(search.SEARCH_PROVIDER_UNAVAILABLE, engineID, true, nil)
	}
	rawURL = strings.TrimSpace(rawURL)
	if _, err := f.transport.ValidateEndpoint(ctx, rawURL); err != nil {
		return nil, 0, search.NewError(search.SEARCH_BLOCKED_BY_NETWORK, engineID, false, err)
	}
	var bodyReader io.Reader
	if len(payload) > 0 {
		bodyReader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, 0, search.NewError(search.SEARCH_PROVIDER_REQUEST_FAILED, engineID, false, err)
	}
	request.Header.Set("Accept", "application/json, application/atom+xml, application/rss+xml, application/xml, text/html;q=0.9, */*;q=0.8")
	request.Header.Set("User-Agent", "AmitiaSearch/1.0")
	if len(payload) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	response, err := f.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, search.NewError(search.SEARCH_CANCELLED, engineID, false, ctx.Err())
		}
		return nil, 0, search.NewError(search.SEARCH_PROVIDER_TIMEOUT, engineID, true, err)
	}
	defer response.Body.Close()
	body, err := readLimited(response.Body, f.maxBytes)
	if err != nil {
		return nil, response.StatusCode, search.NewError(search.SEARCH_PROVIDER_RESPONSE_TOO_LARGE, engineID, false, err)
	}
	if response.StatusCode >= 400 {
		return nil, response.StatusCode, mapHTTPError(engineID, response.StatusCode, response.Header, time.Now())
	}
	return body, response.StatusCode, nil
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024
	}
	body, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	return body, nil
}

func mapHTTPError(engineID string, status int, headers http.Header, now time.Time) error {
	if search.IsAuthError(status) {
		return search.WrapHTTPError(search.SEARCH_PROVIDER_AUTH_FAILED, engineID, status, nil)
	}
	if search.IsRateLimited(status) {
		err := search.WrapHTTPError(search.SEARCH_PROVIDER_RATE_LIMITED, engineID, status, nil)
		if headers != nil {
			err.RetryAfter = search.ParseRetryAfter(headers.Get("Retry-After"), now)
		}
		return err
	}
	if search.IsServerError(status) {
		return search.WrapHTTPError(search.SEARCH_PROVIDER_REQUEST_FAILED, engineID, status, nil)
	}
	return search.WrapHTTPError(search.SEARCH_PROVIDER_REQUEST_REJECTED, engineID, status, nil)
}
