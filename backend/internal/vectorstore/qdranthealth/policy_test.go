// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"testing"
	"time"
)

func TestDefaultPolicyConstants(t *testing.T) {
	if DefaultStartupTimeout != 60*time.Second {
		t.Errorf("DefaultStartupTimeout = %v", DefaultStartupTimeout)
	}
	if MobileCompactStartupTimeout != 180*time.Second {
		t.Errorf("MobileCompactStartupTimeout = %v", MobileCompactStartupTimeout)
	}
	if MobileBalancedStartupTimeout != 120*time.Second {
		t.Errorf("MobileBalancedStartupTimeout = %v", MobileBalancedStartupTimeout)
	}
	if MobilePerformanceStartupTimeout != 90*time.Second {
		t.Errorf("MobilePerformanceStartupTimeout = %v", MobilePerformanceStartupTimeout)
	}
}

func TestNewPolicy(t *testing.T) {
	p := NewPolicy()
	if p.StartupTimeout != DefaultStartupTimeout {
		t.Errorf("StartupTimeout = %v", p.StartupTimeout)
	}
	if p.InitialDelay != DefaultInitialDelay {
		t.Errorf("InitialDelay = %v", p.InitialDelay)
	}
	if p.MaxDelay != DefaultMaxDelay {
		t.Errorf("MaxDelay = %v", p.MaxDelay)
	}
	if p.Multiplier != DefaultMultiplier {
		t.Errorf("Multiplier = %v", p.Multiplier)
	}
	if !p.RequireIdentity {
		t.Error("RequireIdentity should be true")
	}
}

func TestDesktopPolicy(t *testing.T) {
	p := DesktopPolicy()
	if p.StartupTimeout != DefaultStartupTimeout {
		t.Errorf("Desktop StartupTimeout = %v", p.StartupTimeout)
	}
	if p.AllowFallbackLive {
		t.Error("Desktop should not allow fallback live")
	}
}

func TestMobilePolicies(t *testing.T) {
	compact := MobileCompactPolicy()
	if compact.StartupTimeout != MobileCompactStartupTimeout {
		t.Errorf("Compact StartupTimeout = %v", compact.StartupTimeout)
	}
	if !compact.AllowFallbackLive {
		t.Error("Compact should allow fallback live")
	}

	balanced := MobileBalancedPolicy()
	if balanced.StartupTimeout != MobileBalancedStartupTimeout {
		t.Errorf("Balanced StartupTimeout = %v", balanced.StartupTimeout)
	}

	perf := MobilePerformancePolicy()
	if perf.StartupTimeout != MobilePerformanceStartupTimeout {
		t.Errorf("Performance StartupTimeout = %v", perf.StartupTimeout)
	}
}

func TestPolicyValidate(t *testing.T) {
	p := Policy{}
	if err := p.Validate(); err != ErrInvalidPolicy {
		t.Errorf("Validate() with zero timeout should return ErrInvalidPolicy, got %v", err)
	}

	p.StartupTimeout = 30 * time.Second
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() returned error: %v", err)
	}
}

func TestPolicyValidateDefaults(t *testing.T) {
	p := Policy{StartupTimeout: 30 * time.Second}
	p = p.WithMaxAttempts(5)
	if p.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", p.MaxAttempts)
	}
}

func TestPolicyClone(t *testing.T) {
	p := DesktopPolicy()
	clone := p.Clone()
	if clone.StartupTimeout != p.StartupTimeout {
		t.Errorf("Clone StartupTimeout = %v", clone.StartupTimeout)
	}
	if clone.MaxAttempts != p.MaxAttempts {
		t.Errorf("Clone MaxAttempts = %d", clone.MaxAttempts)
	}
}
