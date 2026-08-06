// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Prober interface {
	Probe(ctx context.Context, target ProbeTarget) ProbeResult
}

type HTTPProber struct {
	client *http.Client
}

func NewHTTPProber() *HTTPProber {
	return &HTTPProber{
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:          4,
				MaxIdleConnsPerHost:   2,
				IdleConnTimeout:       30 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				DisableCompression:    true,
			},
		},
	}
}

func (p *HTTPProber) Probe(ctx context.Context, target ProbeTarget) ProbeResult {
	if err := ValidateTarget(target); err != nil {
		return ProbeResult{
			Endpoint: endpointFromPath(target.Path),
			Status:   StatusProbeError,
			Err:      err,
		}
	}

	timeout := target.Timeout
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, target.Method, target.URL(), nil)
	if err != nil {
		return ProbeResult{
			Endpoint: endpointFromPath(target.Path),
			Status:   StatusProbeError,
			Err:      fmt.Errorf("%w: %v", ErrProbeFailed, err),
		}
	}

	start := time.Now()
	resp, err := p.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		status := StatusUnreachable
		if probeCtx.Err() == context.DeadlineExceeded {
			status = StatusUnreachable
			err = ErrProbeTimeout
		}
		return ProbeResult{
			Endpoint: endpointFromPath(target.Path),
			Status:   status,
			Err:      err,
		}
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8192))
	resp.Body.Close()

	if readErr != nil {
		return ProbeResult{
			Endpoint: endpointFromPath(target.Path),
			Status:   StatusProbeError,
			Latency:  latency,
			Err:      fmt.Errorf("%w: read body: %v", ErrProbeFailed, readErr),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ProbeResult{
			Endpoint:   endpointFromPath(target.Path),
			Status:     StatusUnexpectedStatus,
			HTTPStatus: resp.StatusCode,
			Latency:    latency,
			Body:       body,
			Err:        fmt.Errorf("%w: %d", ErrUnexpectedStatusCode, resp.StatusCode),
		}
	}

	return ProbeResult{
		Endpoint:   endpointFromPath(target.Path),
		Status:     StatusOK,
		HTTPStatus: resp.StatusCode,
		Latency:    latency,
		Body:       body,
	}
}

func endpointFromPath(path string) Endpoint {
	switch path {
	case EndpointPathRoot:
		return EndpointRoot
	case EndpointPathHealthz:
		return EndpointHealthz
	case EndpointPathLivez:
		return EndpointLivez
	case EndpointPathReadyz:
		return EndpointReadyz
	}
	return EndpointRoot
}

func NewProber() Prober {
	return NewHTTPProber()
}
