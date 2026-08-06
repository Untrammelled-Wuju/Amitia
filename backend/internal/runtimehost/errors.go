// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import "errors"

var (
	ErrHostCapabilityUnsupported = errors.New("runtimehost: capability unsupported")
	ErrHostProcessUnsupported    = errors.New("runtimehost: process execution unsupported on this host")
	ErrHostUnknownDescriptor     = errors.New("runtimehost: unknown runtime descriptor")
	ErrPortInUse                 = errors.New("runtimehost: port already in use")
	ErrDuplicateProcessID        = errors.New("runtimehost: duplicate process ID")
	ErrProcessNotFound           = errors.New("runtimehost: process not found")
	ErrInvalidProcessSpec        = errors.New("runtimehost: invalid process spec")
	ErrProcessNotRunning         = errors.New("runtimehost: process not running")
	ErrProcessAlreadyRunning     = errors.New("runtimehost: process already running")
	ErrMaxRestartsReached        = errors.New("runtimehost: max restarts reached")
	ErrHostStopped               = errors.New("runtimehost: host already stopped")
)

type hostCapabilityError struct {
	providerID string
	capability HostCapabilityID
	required   CapabilitySupport
	actual     CapabilitySupport
}

func (e *hostCapabilityError) Error() string {
	return ErrHostCapabilityUnsupported.Error() +
		" provider=" + e.providerID +
		" capability=" + string(e.capability) +
		" required=" + supportString(e.required) +
		" actual=" + supportString(e.actual)
}

func supportString(s CapabilitySupport) string {
	switch s {
	case SupportSupported:
		return "supported"
	case SupportLimited:
		return "limited"
	default:
		return "unsupported"
	}
}

func (s CapabilitySupport) String() string {
	return supportString(s)
}

type CapabilityRequirementUnsatisfiedError struct {
	ProviderID string
	Capability HostCapabilityID
	Required   CapabilitySupport
	Actual     CapabilitySupport
}

func (e *CapabilityRequirementUnsatisfiedError) Error() string {
	return "runtimehost: capability requirement unsatisfied: " +
		"provider=" + e.ProviderID +
		" capability=" + string(e.Capability) +
		" required=" + e.Required.String() +
		" actual=" + e.Actual.String()
}

func (e *CapabilityRequirementUnsatisfiedError) Is(target error) bool {
	return target == ErrHostCapabilityUnsupported
}
