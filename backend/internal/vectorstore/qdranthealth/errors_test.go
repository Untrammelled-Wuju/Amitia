// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrTargetRequired", ErrTargetRequired, "target is required"},
		{"ErrTargetAddressRequired", ErrTargetAddressRequired, "target address is required"},
		{"ErrProberRequired", ErrProberRequired, "prober is required"},
		{"ErrIdentityRequired", ErrIdentityRequired, "identity is required"},
		{"ErrIdentityTitleMismatch", ErrIdentityTitleMismatch, "identity title mismatch"},
		{"ErrIdentityVersionRequired", ErrIdentityVersionRequired, "identity version is required"},
		{"ErrProbeFailed", ErrProbeFailed, "probe request failed"},
		{"ErrProbeTimeout", ErrProbeTimeout, "probe timed out"},
		{"ErrUnexpectedStatusCode", ErrUnexpectedStatusCode, "unexpected status code"},
		{"ErrStartupTimeout", ErrStartupTimeout, "startup deadline exceeded"},
		{"ErrProcessNotStarted", ErrProcessNotStarted, "process not started"},
		{"ErrProcessExited", ErrProcessExited, "process exited during startup"},
		{"ErrWaiterStopped", ErrWaiterStopped, "waiter has been stopped"},
		{"ErrInvalidPolicy", ErrInvalidPolicy, "invalid startup policy"},
		{"ErrGuardRequired", ErrGuardRequired, "process guard is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("expected non-nil error")
			}
			if tt.err.Error() == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

func TestErrConstants(t *testing.T) {
	if errors.Is(ErrTargetRequired, ErrTargetAddressRequired) {
		t.Error("ErrTargetRequired should not be ErrTargetAddressRequired")
	}
	if !errors.Is(ErrStartupTimeout, ErrStartupTimeout) {
		t.Error("ErrStartupTimeout should be itself")
	}
}
