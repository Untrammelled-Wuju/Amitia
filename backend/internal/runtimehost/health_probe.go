// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type HealthProbe interface {
	Check(ctx context.Context) error
}

type ProcessAliveProbe struct {
	supervisor *defaultProcessSupervisor
	id         ProcessID
}

func (p *ProcessAliveProbe) Check(ctx context.Context) error {
	if p.supervisor == nil {
		return fmt.Errorf("no supervisor")
	}
	snap, ok := p.supervisor.Snapshot(p.id)
	if !ok {
		return fmt.Errorf("process not registered")
	}
	if snap.State == StateStopped || snap.State == StateFailed {
		return fmt.Errorf("process is %s", snap.State)
	}
	if snap.PID > 0 && !p.supervisor.isAlive(snap.PID) {
		return fmt.Errorf("process pid=%d not alive", snap.PID)
	}
	return nil
}

type HTTPHealthProbe struct {
	URL               string
	ExpectedStatusMin int
	ExpectedStatusMax int
	Timeout           time.Duration
	client            *http.Client
}

func NewHTTPHealthProbe(url string, timeout time.Duration) *HTTPHealthProbe {
	return &HTTPHealthProbe{
		URL:               url,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 299,
		Timeout:           timeout,
		client:            &http.Client{Timeout: timeout},
	}
}

func (p *HTTPHealthProbe) Check(ctx context.Context) error {
	if p.client == nil {
		p.client = &http.Client{}
	}
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode < p.ExpectedStatusMin || resp.StatusCode > p.ExpectedStatusMax {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

type TCPHealthProbe struct {
	Address string
	Timeout time.Duration
}

func (p *TCPHealthProbe) Check(ctx context.Context) error {
	dialer := net.Dialer{}
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}
	conn, err := dialer.DialContext(ctx, "tcp", p.Address)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
