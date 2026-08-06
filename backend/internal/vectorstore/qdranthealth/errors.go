// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import "errors"

var (
	ErrTargetRequired          = errors.New("qdranthealth: target is required")
	ErrTargetAddressRequired   = errors.New("qdranthealth: target address is required")
	ErrProberRequired          = errors.New("qdranthealth: prober is required")
	ErrIdentityRequired        = errors.New("qdranthealth: identity is required")
	ErrIdentityTitleMismatch   = errors.New("qdranthealth: identity title mismatch")
	ErrIdentityVersionRequired = errors.New("qdranthealth: identity version is required")
	ErrProbeFailed             = errors.New("qdranthealth: probe request failed")
	ErrProbeTimeout            = errors.New("qdranthealth: probe timed out")
	ErrUnexpectedStatusCode    = errors.New("qdranthealth: unexpected status code")
	ErrStartupTimeout          = errors.New("qdranthealth: startup deadline exceeded")
	ErrProcessNotStarted       = errors.New("qdranthealth: process not started")
	ErrProcessExited           = errors.New("qdranthealth: process exited during startup")
	ErrProcessStopFailed       = errors.New("qdranthealth: failed to stop process after ready failure")
	ErrLeaseReleaseFailed      = errors.New("qdranthealth: failed to release lease after ready failure")
	ErrWaiterStopped           = errors.New("qdranthealth: waiter has been stopped")
	ErrInvalidPolicy           = errors.New("qdranthealth: invalid startup policy")
	ErrGuardRequired           = errors.New("qdranthealth: process guard is required")
)
