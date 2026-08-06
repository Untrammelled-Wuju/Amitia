// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import "time"

type Target struct {
	Host    string
	Port    int
	BaseURL string
	Timeout time.Duration
}

func NewTarget(host string, port int) Target {
	return Target{
		Host:    host,
		Port:    port,
		BaseURL: BuildBaseURL(host, port),
		Timeout: DefaultProbeTimeout,
	}
}

func NewTargetFromURL(baseURL string) Target {
	return Target{
		BaseURL: baseURL,
		Timeout: DefaultProbeTimeout,
	}
}

func (t Target) Probe(endpoint Endpoint) ProbeTarget {
	return NewProbeTarget(t.BaseURL, endpoint, t.Timeout)
}

func (t Target) WithTimeout(d time.Duration) Target {
	t.Timeout = d
	return t
}

func (t Target) IdentityProbe() ProbeTarget {
	return NewProbeTarget(t.BaseURL, EndpointRoot, DefaultIdentityTimeout)
}

func (t Target) LiveProbe() ProbeTarget {
	return NewProbeTarget(t.BaseURL, EndpointLivez, t.Timeout)
}

func (t Target) ReadyProbe() ProbeTarget {
	return NewProbeTarget(t.BaseURL, EndpointReadyz, t.Timeout)
}

func (t Target) HealthProbe() ProbeTarget {
	return NewProbeTarget(t.BaseURL, EndpointHealthz, t.Timeout)
}

func (t Target) Validate() error {
	if t.BaseURL == "" {
		return ErrTargetAddressRequired
	}
	return nil
}
