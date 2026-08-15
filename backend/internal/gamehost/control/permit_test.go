package control

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestOutputPermit_CurrentEpochNotStale(t *testing.T) {
	now := time.Now().UTC()
	p := NewOutputPermit("out-1", "rt-1", "svc-1", "plugin-1", 1, 1, KindCustomRPC, domain.ControlModePluginControl, DefaultPermitTTL, now)
	if !p.IsCurrent(1) {
		t.Fatal("permit should be current for epoch 1")
	}
	if p.IsCurrent(2) {
		t.Fatal("permit should be stale for epoch 2")
	}
}

func TestOutputPermit_TTLEnforced(t *testing.T) {
	now := time.Now().UTC()
	p := NewOutputPermit("out-1", "rt-1", "svc-1", "plugin-1", 1, 1, KindCustomRPC, domain.ControlModePluginControl, 1*time.Millisecond, now)
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
	p := NewOutputPermit("out-1", "rt-1", "svc-1", "plugin-1", 1, 1, KindCustomRPC, domain.ControlModePluginControl, DefaultPermitTTL, now)

	if err := p.Validate("rt-1", "svc-1", 1, 1, now.Add(1*time.Millisecond)); err != nil {
		t.Fatalf("valid case got error: %v", err)
	}
	if err := p.Validate("rt-other", "svc-1", 1, 1, now); err == nil {
		t.Fatal("expected error for runtime mismatch")
	}
	if err := p.Validate("rt-1", "svc-other", 1, 1, now); err == nil {
		t.Fatal("expected error for service mismatch")
	}
	if err := p.Validate("rt-1", "svc-1", 2, 1, now); err == nil {
		t.Fatal("expected error for epoch mismatch")
	}
	if err := p.Validate("rt-1", "svc-1", 1, 2, now); err == nil {
		t.Fatal("expected error for generation mismatch")
	}
}

func TestOutputPermit_CrossEpochNotUsable(t *testing.T) {
	now := time.Now().UTC()
	p := NewOutputPermit("out-1", "rt-1", "svc-1", "plugin-1", 1, 1, KindCustomRPC, domain.ControlModePluginControl, DefaultPermitTTL, now)
	if p.IsCurrent(2) {
		t.Fatal("permit should not be usable across epochs")
	}
}

func TestOutputPermit_PermitIDIsUnique(t *testing.T) {
	now := time.Now().UTC()
	p1 := NewOutputPermit("out-1", "rt-1", "svc-1", "plugin-1", 1, 1, KindCustomRPC, domain.ControlModePluginControl, DefaultPermitTTL, now)
	p2 := NewOutputPermit("out-2", "rt-1", "svc-1", "plugin-1", 1, 1, KindCustomRPC, domain.ControlModePluginControl, DefaultPermitTTL, now)
	if p1.PermitID == p2.PermitID {
		t.Fatal("permit IDs should be unique")
	}
}
