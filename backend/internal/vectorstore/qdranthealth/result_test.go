// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"testing"
	"time"
)

func TestProbeStatusConstants(t *testing.T) {
	tests := []struct {
		s    ProbeStatus
		want string
	}{
		{StatusOK, "ok"},
		{StatusUnreachable, "unreachable"},
		{StatusUnexpectedStatus, "unexpected_status"},
		{StatusProbeError, "probe_error"},
	}
	for _, tt := range tests {
		if string(tt.s) != tt.want {
			t.Errorf("ProbeStatus = %q, want %q", tt.s, tt.want)
		}
	}
}

func TestProbeResultIsOK(t *testing.T) {
	ok := ProbeResult{Status: StatusOK}
	if !ok.IsOK() {
		t.Error("StatusOK result should be OK")
	}

	withErr := ProbeResult{Status: StatusOK, Err: ErrProbeFailed}
	if withErr.IsOK() {
		t.Error("result with error should not be OK")
	}

	bad := ProbeResult{Status: StatusUnreachable}
	if bad.IsOK() {
		t.Error("StatusUnreachable should not be OK")
	}
}

func TestProbeResultError(t *testing.T) {
	r := ProbeResult{
		Status: StatusUnreachable,
		Err:    ErrProbeTimeout,
	}
	if r.Error() != ErrProbeTimeout.Error() {
		t.Errorf("Error() = %q", r.Error())
	}

	noErr := ProbeResult{Status: StatusOK}
	if noErr.Error() != "ok" {
		t.Errorf("Error() = %q, want 'ok'", noErr.Error())
	}
}

func TestProbeResultIsTimeout(t *testing.T) {
	timeout := ProbeResult{Err: ErrProbeTimeout}
	if !timeout.IsTimeout() {
		t.Error("expected timeout result to be timeout")
	}
}

func TestProbeResultWithTimestamp(t *testing.T) {
	r := ProbeResult{}
	now := time.Now()
	r = r.WithTimestamp(now)
	if !r.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", r.Timestamp, now)
	}
}
