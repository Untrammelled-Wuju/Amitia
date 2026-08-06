// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import "time"

const (
	DefaultStartupTimeout            = 60 * time.Second
	MobileCompactStartupTimeout      = 180 * time.Second
	MobileBalancedStartupTimeout     = 120 * time.Second
	MobilePerformanceStartupTimeout  = 90 * time.Second

	DefaultInitialDelay = 200 * time.Millisecond
	DefaultMaxDelay     = 5 * time.Second
	DefaultMultiplier   = 2.0

	DefaultMaxAttempts = 0
)

type Policy struct {
	StartupTimeout    time.Duration
	InitialDelay      time.Duration
	MaxDelay          time.Duration
	Multiplier        float64
	MaxAttempts       int
	RequireIdentity   bool
	StrictLiveCheck   bool
	AllowFallbackLive bool
}

func NewPolicy() Policy {
	return Policy{
		StartupTimeout:    DefaultStartupTimeout,
		InitialDelay:      DefaultInitialDelay,
		MaxDelay:          DefaultMaxDelay,
		Multiplier:        DefaultMultiplier,
		MaxAttempts:       DefaultMaxAttempts,
		RequireIdentity:   true,
		StrictLiveCheck:   true,
		AllowFallbackLive: false,
	}
}

func DesktopPolicy() Policy {
	return Policy{
		StartupTimeout:    DefaultStartupTimeout,
		InitialDelay:      DefaultInitialDelay,
		MaxDelay:          DefaultMaxDelay,
		Multiplier:        DefaultMultiplier,
		MaxAttempts:       DefaultMaxAttempts,
		RequireIdentity:   true,
		StrictLiveCheck:   true,
		AllowFallbackLive: false,
	}
}

func MobileCompactPolicy() Policy {
	return Policy{
		StartupTimeout:    MobileCompactStartupTimeout,
		InitialDelay:      DefaultInitialDelay,
		MaxDelay:          DefaultMaxDelay,
		Multiplier:        DefaultMultiplier,
		MaxAttempts:       DefaultMaxAttempts,
		RequireIdentity:   true,
		StrictLiveCheck:   true,
		AllowFallbackLive: true,
	}
}

func MobileBalancedPolicy() Policy {
	return Policy{
		StartupTimeout:    MobileBalancedStartupTimeout,
		InitialDelay:      DefaultInitialDelay,
		MaxDelay:          DefaultMaxDelay,
		Multiplier:        DefaultMultiplier,
		MaxAttempts:       DefaultMaxAttempts,
		RequireIdentity:   true,
		StrictLiveCheck:   true,
		AllowFallbackLive: false,
	}
}

func MobilePerformancePolicy() Policy {
	return Policy{
		StartupTimeout:    MobilePerformanceStartupTimeout,
		InitialDelay:      DefaultInitialDelay,
		MaxDelay:          DefaultMaxDelay,
		Multiplier:        DefaultMultiplier,
		MaxAttempts:       DefaultMaxAttempts,
		RequireIdentity:   true,
		StrictLiveCheck:   true,
		AllowFallbackLive: false,
	}
}

func (p Policy) Validate() error {
	if p.StartupTimeout <= 0 {
		return ErrInvalidPolicy
	}
	if p.InitialDelay <= 0 {
		p.InitialDelay = DefaultInitialDelay
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = DefaultMaxDelay
	}
	if p.Multiplier < 1.0 {
		p.Multiplier = DefaultMultiplier
	}
	return nil
}

func (p Policy) WithStartupTimeout(d time.Duration) Policy {
	p.StartupTimeout = d
	return p
}

func (p Policy) WithMaxAttempts(n int) Policy {
	p.MaxAttempts = n
	return p
}

func (p Policy) WithIdentityRequired(required bool) Policy {
	p.RequireIdentity = required
	return p
}

func (p Policy) Clone() Policy {
	return Policy{
		StartupTimeout:    p.StartupTimeout,
		InitialDelay:      p.InitialDelay,
		MaxDelay:          p.MaxDelay,
		Multiplier:        p.Multiplier,
		MaxAttempts:       p.MaxAttempts,
		RequireIdentity:   p.RequireIdentity,
		StrictLiveCheck:   p.StrictLiveCheck,
		AllowFallbackLive: p.AllowFallbackLive,
	}
}
