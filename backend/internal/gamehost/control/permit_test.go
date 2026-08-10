package control

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestOutputPermit_CurrentEpochNotStale(t *testing.T) {
	now := time.Now().UTC()
	p := NewOutputPermit("rt-1", "svc-1", "plugin-1", 10, KindCustomRPC, domain.ControlModePluginControl, DefaultPermitTTL, now)
	if !p.IsCurrent(10) {
		t.Fatal("permit should be current for epoch 10")
	}
	if p.IsCurrent(9) {
		t.Fatal("permit should be stale for epoch 9")
	}
	if p.IsCurrent(11) {
		t.Fatal("permit should be invalid for future epoch 11")
	}
}

func TestOutputPermit_TTLEnforced(t *testing.T) {
	now := time.Now().UTC()
	p := NewOutputPermit("rt-1", "svc-1", "plugin-1", 10, KindCustomRPC, domain.ControlModePluginControl, 1*time.Millisecond, now)
	if p.IsExpired(now) {
		t.Fatal("permit should not be expired immediately at issue time")
	}
	later := now.Add(2 * time.Millisecond)
	if !p.IsExpired(later) {
		t.Fatal("permit should be expired after TTL")
	}
}

func TestOutputPermit_ValidateBinding(t *testing.T) {
	now := time.Now().UTC()
	p := NewOutputPermit("rt-1", "svc-1", "plugin-1", 10, KindCustomRPC, domain.ControlModePluginControl, DefaultPermitTTL, now)

	if err := p.Validate("rt-1", "svc-1", 10, now.Add(1*time.Millisecond)); err != nil {
		t.Fatalf("valid case got error: %v", err)
	}
	if err := p.Validate("rt-other", "svc-1", 10, now); err == nil {
		t.Fatal("expected error for runtime mismatch")
	}
	if err := p.Validate("rt-1", "svc-other", 10, now); err == nil {
		t.Fatal("expected error for service mismatch")
	}
	if err := p.Validate("rt-1", "svc-1", 11, now); err == nil {
		t.Fatal("expected error for epoch mismatch")
	}
}

func TestOutputPermit_CrossEpochNotUsable(t *testing.T) {
	now := time.Now().UTC()
	p := NewOutputPermit("rt-1", "svc-1", "plugin-1", 5, KindCustomRPC, domain.ControlModePluginControl, DefaultPermitTTL, now)
	if p.IsCurrent(6) {
		t.Fatal("permit should not be usable across epochs")
	}
}

func TestOutputPermit_PermitIDIsUnique(t *testing.T) {
	now := time.Now().UTC()
	p1 := NewOutputPermit("rt-1", "svc-1", "plugin-1", 1, KindCustomRPC, domain.ControlModePluginControl, DefaultPermitTTL, now)
	p2 := NewOutputPermit("rt-1", "svc-1", "plugin-1", 1, KindCustomRPC, domain.ControlModePluginControl, DefaultPermitTTL, now)
	if p1.PermitID == p2.PermitID {
		t.Fatal("permit IDs should be unique")
	}
}
