// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import "time"

const (
	StatusOK             ProbeStatus = "ok"
	StatusUnreachable    ProbeStatus = "unreachable"
	StatusUnexpectedStatus ProbeStatus = "unexpected_status"
	StatusProbeError     ProbeStatus = "probe_error"
)

type ProbeStatus string

type ProbeResult struct {
	Endpoint   Endpoint
	Status     ProbeStatus
	HTTPStatus int
	Latency    time.Duration
	Body       []byte
	Err        error
	Timestamp  time.Time
}

func (r ProbeResult) IsOK() bool {
	return r.Status == StatusOK && r.Err == nil
}

func (r ProbeResult) Unwrap() error {
	return r.Err
}

func (r ProbeResult) Error() string {
	if r.Err != nil {
		return r.Err.Error()
	}
	return string(r.Status)
}

func (r ProbeResult) WithTimestamp(t time.Time) ProbeResult {
	r.Timestamp = t
	return r
}

func (r ProbeResult) IsTimeout() bool {
	return r.Err == ErrProbeTimeout || r.Status == StatusUnreachable
}
