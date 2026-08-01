// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package device

import (
	"context"
	"errors"
	"time"
)

var (
	ErrDeviceNotFound   = errors.New("device: not found")
	ErrDeviceNotOwned   = errors.New("device: not owned by user")
	ErrDeviceInactive   = errors.New("device: inactive")
	ErrRuntimeMismatch  = errors.New("device: runtime mismatch")
	ErrDeviceResolution = errors.New("device: resolution failed")
)

type DeviceContext struct {
	UserID    string
	DeviceID  string
	RuntimeID string
	Source    string
}

func (d DeviceContext) IsValid() bool {
	return d.UserID != "" && d.DeviceID != ""
}

type RuntimeResolver interface {
	ResolveDeviceForRuntime(runtimeID string) (deviceID string, userID string, err error)
	VerifyRuntimeDeviceMapping(runtimeID, deviceID string) error
}

type RuntimeClient struct {
	RuntimeID string
	DeviceID  string
	UserID    string
}

type DeviceResolver interface {
	ResolveCurrentDevice(ctx context.Context, userID string, req RequestContext) (*DeviceContext, error)
}

type RequestContext struct {
	DeviceIDHeader string
	RuntimeID     string
	Platform      string
	AppVersion    string
	AuthToken     string
}

func (r RequestContext) HasExplicitDevice() bool {
	return r.DeviceIDHeader != "" || r.RuntimeID != ""
}

type VerificationPolicy struct {
	RequireDeviceOwnership bool
	RequireRuntimeMatch     bool
	AllowFallbackToFirst    bool
	StaleThreshold         time.Duration
}

func DefaultVerificationPolicy() VerificationPolicy {
	return VerificationPolicy{
		RequireDeviceOwnership: true,
		RequireRuntimeMatch:     true,
		AllowFallbackToFirst:    false,
		StaleThreshold:         24 * time.Hour,
	}
}
