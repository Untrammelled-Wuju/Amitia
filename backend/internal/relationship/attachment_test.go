package relationship

import (
	"testing"
)

func TestDefaultAttachmentProfileIsSecure(t *testing.T) {
	p := DefaultAttachmentProfile()
	if p.Style != AttachmentSecure {
		t.Fatalf("expected secure attachment, got %s", p.Style)
	}
	if p.ConflictSensitivity <= 0 {
		t.Fatalf("expected positive conflict sensitivity, got %v", p.ConflictSensitivity)
	}
}

func TestComputeSecurityScoreHighTrust(t *testing.T) {
	dims := DefaultDimensions()
	dims.Trust.Value = 80
	dims.Intimacy.Value = 70
	dims.Dependency.Value = 50
	dims.Conflict.Value = 10
	dims.Repair.Value = 60

	score := ComputeSecurityScore(dims)
	if score < 0.3 {
		t.Fatalf("expected reasonable security score for high trust, got %v", score)
	}
	if score > 1 {
		t.Fatalf("expected security score <= 1, got %v", score)
	}
}

func TestComputeSecurityScoreHighConflictLowersScore(t *testing.T) {
	dims := DefaultDimensions()
	dims.Trust.Value = 80
	dims.Intimacy.Value = 70
	dims.Conflict.Value = 80
	dims.Repair.Value = 20

	score := ComputeSecurityScore(dims)
	if score >= 0.42 {
		t.Fatalf("expected security around 0.38, got %v", score)
	}
}

func TestComputeSecurityFromState(t *testing.T) {
	state := DefaultState()
	state.Trust = 0.8
	state.Familiarity = 0.7
	state.Security = 0.5
	state.Tension = 0.1
	state.RepairConfidence = 0.6

	score := ComputeSecurityFromState(state)
	if score < 0.4 {
		t.Fatalf("expected reasonable security score, got %v", score)
	}
	if score > 1 {
		t.Fatalf("expected score <= 1, got %v", score)
	}
}

func TestAttachmentRecoveryMultiplierSecure(t *testing.T) {
	p := AttachmentProfile{Style: AttachmentSecure, RecoverySpeed: 0.8}
	m := AttachmentRecoveryMultiplier(p)
	if m != 1.0 {
		t.Fatalf("expected secure multiplier 1.0, got %v", m)
	}
}

func TestAttachmentRecoveryMultiplierAnxious(t *testing.T) {
	p := AttachmentProfile{Style: AttachmentAnxious, RecoverySpeed: 0.5}
	m := AttachmentRecoveryMultiplier(p)
	if m != 0.5 {
		t.Fatalf("expected anxious multiplier 0.5, got %v", m)
	}
}

func TestAttachmentConflictModifierAnxious(t *testing.T) {
	p := AttachmentProfile{Style: AttachmentAnxious, ConflictSensitivity: 0.9}
	m := AttachmentConflictModifier(p)
	if m != 0.9 {
		t.Fatalf("expected anxious conflict modifier 0.9, got %v", m)
	}
}

func TestAttachmentConflictModifierDefaultAnxious(t *testing.T) {
	p := AttachmentProfile{Style: AttachmentAnxious}
	m := AttachmentConflictModifier(p)
	if m != 1.3 {
		t.Fatalf("expected default anxious modifier 1.3, got %v", m)
	}
}

func TestAttachmentProtestBehaviorSecureNeverProtests(t *testing.T) {
	p := AttachmentProfile{Style: AttachmentSecure}
	if AttachmentProtestBehavior(p, 0.2) {
		t.Fatalf("expected secure never protests")
	}
}

func TestAttachmentProtestBehaviorAnxious(t *testing.T) {
	p := AttachmentProfile{Style: AttachmentAnxious, ProtestIntensity: 0.8}
	if !AttachmentProtestBehavior(p, 0.3) {
		t.Fatalf("expected anxious to protest at low security")
	}
	if AttachmentProtestBehavior(p, 0.7) {
		t.Fatalf("expected anxious not to protest at high security")
	}
}

func TestAdjustTensionDecayForSecure(t *testing.T) {
	p := AttachmentProfile{Style: AttachmentSecure, RecoverySpeed: 1.0}
	adjusted := AdjustTensionDecayForAttachment(p, 0.04)
	if adjusted != 0.04 {
		t.Fatalf("expected secure to use base decay, got %v", adjusted)
	}
}

func TestAdjustTensionDecayForAnxious(t *testing.T) {
	p := AttachmentProfile{Style: AttachmentAnxious, RecoverySpeed: 0.5}
	adjusted := AdjustTensionDecayForAttachment(p, 0.04)
	if adjusted != 0.02 {
		t.Fatalf("expected anxious to halve base decay, got %v", adjusted)
	}
}
