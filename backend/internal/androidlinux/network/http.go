//go:build linux && !android

package network

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

var reservedHeaders = map[string]bool{
	"Host":                true,
	"Connection":          true,
	"Transfer-Encoding":   true,
	"Content-Length":      true,
	"Proxy-Authorization": true,
}

func performHTTPRequest(ctx context.Context, req HTTPRequest, policy Policy) (HTTPResponse, error) {
	timeout := coarseTimeout(req, policy)
	method := strings.ToUpper(req.Method)
	if !isValidHTTPMethod(method) {
		return HTTPResponse{}, ErrHTTPDenied("invalid method: " + method)
	}

	validator := NewEndpointValidator(policy)
	sec, err := validator.ResolveAndClassify(ctx, req.URL)
	if err != nil {
		return HTTPResponse{}, err
	}

	body, contentLength, err := decodeHTTPBody(req)
	if err != nil {
		return HTTPResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, sec.URL.String(), body)
	if err != nil {
		return HTTPResponse{}, ErrHTTPDenied(err.Error())
	}
	httpReq.ContentLength = contentLength

	if err := applyHTTPHeaders(httpReq, req.Headers); err != nil {
		return HTTPResponse{}, err
	}

	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", policy.UserAgent)
	}
	if httpReq.Header.Get("Accept-Encoding") == "" {
		httpReq.Header.Set("Accept-Encoding", "gzip, deflate")
	}

	client := buildPinnedHTTPClient(sec, policy, timeout)

	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return HTTPResponse{}, ErrHTTPDenied(err.Error())
	}
	defer resp.Body.Close()

	return readHTTPResponse(resp, req, policy, sec.URL.String(), start)
}

func coarseTimeout(req HTTPRequest, policy Policy) time.Duration {
	if req.TimeoutMS > 0 {
		d := time.Duration(req.TimeoutMS) * time.Millisecond
		if d > policy.MaxTimeout {
			return policy.MaxTimeout
		}
		return d
	}
	return policy.DefaultTimeout
}

func isValidHTTPMethod(method string) bool {
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
		return true
	}
	return false
}

func decodeHTTPBody(req HTTPRequest) (io.Reader, int64, error) {
	if req.BodyBase64 != "" {
		data, err := base64.StdEncoding.DecodeString(req.BodyBase64)
		if err != nil {
			return nil, 0, ErrHTTPDenied("invalid base64 body: " + err.Error())
		}
		if int64(len(data)) > 4*1024*1024 {
			return nil, 0, ErrHTTPDenied("base64 body exceeds 4 MiB")
		}
		return bytes.NewReader(data), int64(len(data)), nil
	}
	if req.Body != "" {
		if int64(len(req.Body)) > 4*1024*1024 {
			return nil, 0, ErrHTTPDenied("body exceeds 4 MiB")
		}
		return strings.NewReader(req.Body), int64(len(req.Body)), nil
	}
	return nil, 0, nil
}

func applyHTTPHeaders(req *http.Request, headers map[string]string) error {
	for key, value := range headers {
		if reservedHeaders[key] {
			continue
		}
		if strings.ContainsAny(key, "\r\n") {
			return ErrCRLFInjection("header key")
		}
		if strings.ContainsAny(value, "\r\n") {
			return ErrCRLFInjection("header value")
		}
		if len(key) > 1024 {
			return ErrHTTPDenied("header key too long")
		}
		if len(value) > 8192 {
			return ErrHTTPDenied("header value too long")
		}
		req.Header.Set(key, value)
	}
	return nil
}

func buildPinnedHTTPClient(sec EndpointSecurity, policy Policy, timeout time.Duration) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if !strings.EqualFold(strings.TrimSuffix(host, "."), strings.TrimSuffix(sec.Host, ".")) {
				return nil, ErrHTTPDenied("host changed during dial")
			}
			dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
			var lastErr error
			for _, ip := range sec.Addresses {
				conn, derr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if derr == nil {
					return conn, nil
				}
				lastErr = derr
			}
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, ErrHTTPDenied("no approved addresses")
		},
		MaxIdleConns:       policy.MaxConcurrentHTTP,
		MaxConnsPerHost:    policy.MaxConcurrentHTTP,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: false,
	}

	maxRedirects := policy.MaxRedirects
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return ErrTooManyRedirects(maxRedirects)
			}
			return nil
		},
	}
}

func readHTTPResponse(resp *http.Response, req HTTPRequest, policy Policy, startURL string, start time.Time) (HTTPResponse, error) {
	maxBytes := policy.MaxHTTPResponseBytes
	if req.MaxResponseBytes > 0 && req.MaxResponseBytes < maxBytes {
		maxBytes = req.MaxResponseBytes
	}

	bodyReader := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(bodyReader)
	if err != nil {
		return HTTPResponse{}, ErrHTTPDenied(err.Error())
	}

	truncated := int64(len(data)) > maxBytes
	if truncated {
		data = data[:maxBytes]
	}

	result := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header.Clone(),
		BytesRead:  int64(len(data)),
		Truncated:  truncated,
		FinalURL:   resp.Request.URL.String(),
		DurationMS: time.Since(start).Milliseconds(),
	}

	peek := data
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, gzErr := gzip.NewReader(bytes.NewReader(data))
		if gzErr == nil {
			defer gzReader.Close()
			uncompressed, readErr := io.ReadAll(io.LimitReader(gzReader, maxBytes+1))
			if readErr == nil && int64(len(uncompressed)) <= maxBytes {
				peek = uncompressed
			}
		}
	}

	if !utf8.Valid(peek) {
		result.BodyBase64 = base64.StdEncoding.EncodeToString(data)
	} else {
		result.Body = string(data)
		mimeType := resp.Header.Get("Content-Type")
		if mimeType != "" {
			parts := strings.Split(mimeType, ";")
			result.MIMEType = strings.TrimSpace(parts[0])
			for _, p := range parts[1:] {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(strings.ToLower(p), "charset=") {
					result.Charset = strings.TrimPrefix(strings.ToLower(p), "charset=")
				}
			}
		}
	}

	return result, nil
}
