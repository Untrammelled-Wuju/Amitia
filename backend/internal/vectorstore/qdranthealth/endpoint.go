// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"fmt"
	"net/http"
	"time"
)

const (
	EndpointPathRoot    = "/"
	EndpointPathHealthz = "/healthz"
	EndpointPathLivez   = "/livez"
	EndpointPathReadyz  = "/readyz"
)

const (
	DefaultProbeTimeout    = 3 * time.Second
	DefaultIdentityTimeout = 5 * time.Second
)

type Endpoint int

const (
	EndpointRoot    Endpoint = iota
	EndpointHealthz
	EndpointLivez
	EndpointReadyz
)

func (e Endpoint) Path() string {
	switch e {
	case EndpointRoot:
		return EndpointPathRoot
	case EndpointHealthz:
		return EndpointPathHealthz
	case EndpointLivez:
		return EndpointPathLivez
	case EndpointReadyz:
		return EndpointPathReadyz
	}
	return EndpointPathRoot
}

func (e Endpoint) String() string {
	return e.Path()
}

type EndpointMethod int

const (
	MethodGet EndpointMethod = iota
)

func (m EndpointMethod) String() string {
	switch m {
	case MethodGet:
		return http.MethodGet
	}
	return http.MethodGet
}

type ProbeTarget struct {
	BaseURL string
	Path    string
	Method  string
	Timeout time.Duration
}

func NewProbeTarget(baseURL string, endpoint Endpoint, timeout time.Duration) ProbeTarget {
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	return ProbeTarget{
		BaseURL: baseURL,
		Path:    endpoint.Path(),
		Method:  EndpointMethod(MethodGet).String(),
		Timeout: timeout,
	}
}

func (t ProbeTarget) URL() string {
	return t.BaseURL + t.Path
}

func (t ProbeTarget) WithTimeout(d time.Duration) ProbeTarget {
	t.Timeout = d
	return t
}

func ValidateTarget(target ProbeTarget) error {
	if target.BaseURL == "" {
		return ErrTargetAddressRequired
	}
	if target.Path == "" {
		return ErrTargetRequired
	}
	return nil
}

func BuildBaseURL(host string, port int) string {
	return fmt.Sprintf("http://%s:%d", host, port)
}
