package relationship

import (
	"testing"
	"time"
)

func TestDefaultUnresolvedConfig(t *testing.T) {
	cfg := DefaultUnresolvedConfig()
	if cfg.BaseEscalationHours <= 0 {
		t.Fatalf("expected positive base escalation hours, got %v", cfg.BaseEscalationHours)
	}
	if cfg.MaxEscalationLevel <= 0 {
		t.Fatalf("expected positive max escalation level, got %d", cfg.MaxEscalationLevel)
	}
	if cfg.NaturalDecayPerHour <= 0 {
		t.Fatalf("expected positive decay rate, got %v", cfg.NaturalDecayPerHour)
	}
}

func TestCreateUnresolvedThread(t *testing.T) {
	thread := CreateUnresolvedThread("", "", 0.6, []string{"ev-001", "ev-002"})
	if thread.Severity != 0.6 {
		t.Fatalf("expected severity 0.6, got %v", thread.Severity)
	}
	if len(thread.EventRefs) != 2 {
		t.Fatalf("expected 2 event refs, got %d", len(thread.EventRefs))
	}
	if thread.ResolvedAt != nil {
		t.Fatalf("expected non-resolved thread")
	}
	if thread.EscalationLevel != 0 {
		t.Fatalf("expected escalation level 0, got %d", thread.EscalationLevel)
	}
}

func TestCreateUnresolvedThreadClampsSeverity(t *testing.T) {
	thread := CreateUnresolvedThread("", "", 1.5, nil)
	if thread.Severity != 1.0 {
		t.Fatalf("expected severity clamped to 1.0, got %v", thread.Severity)
	}
}

func TestEscalateUnresolvedWhenTimeElapsed(t *testing.T) {
	cfg := DefaultUnresolvedConfig()
	cfg.BaseEscalationHours = 1
	thread := CreateUnresolvedThread("", "", 0.4, nil)

	future := thread.CreatedAt.Add(2 * time.Hour)
	escalated := EscalateUnresolved(&thread, cfg, future)
	if !escalated {
		t.Fatalf("expected escalation after time elapsed")
	}
	if thread.EscalationLevel != 1 {
		t.Fatalf("expected escalation level 1, got %d", thread.EscalationLevel)
	}
	if thread.Severity <= 0.4 {
		t.Fatalf("expected severity to increase, got %v", thread.Severity)
	}
}

func TestEscalateUnresolvedMaxLevel(t *testing.T) {
	cfg := DefaultUnresolvedConfig()
	cfg.MaxEscalationLevel = 2
	thread := CreateUnresolvedThread("", "", 0.4, nil)
	thread.EscalationLevel = 2

	future := thread.CreatedAt.Add(100 * time.Hour)
	escalated := EscalateUnresolved(&thread, cfg, future)
	if escalated {
		t.Fatalf("expected no escalation at max level")
	}
}

func TestEscalateUnresolvedResolvedThread(t *testing.T) {
	cfg := DefaultUnresolvedConfig()
	thread := CreateUnresolvedThread("", "", 0.4, nil)
	ResolveUnresolved(&thread)

	future := thread.CreatedAt.Add(100 * time.Hour)
	escalated := EscalateUnresolved(&thread, cfg, future)
	if escalated {
		t.Fatalf("expected no escalation for resolved thread")
	}
}

func TestDecayUnresolvedReducesSeverity(t *testing.T) {
	thread := CreateUnresolvedThread("", "", 0.6, nil)
	thread.DecayPerHour = 0.1

	before := thread.Severity
	DecayUnresolved(&thread, 5)
	after := thread.Severity

	if after >= before {
		t.Fatalf("expected decay to reduce severity, before=%v after=%v", before, after)
	}
}

func TestDecayUnresolvedResolvedThread(t *testing.T) {
	thread := CreateUnresolvedThread("", "", 0.6, nil)
	ResolveUnresolved(&thread)

	before := thread.Severity
	DecayUnresolved(&thread, 10)
	if thread.Severity != before {
		t.Fatalf("expected no decay for resolved thread, got %v", thread.Severity)
	}
}

func TestResolveUnresolvedSetsResolvedAt(t *testing.T) {
	thread := CreateUnresolvedThread("", "", 0.6, nil)
	ok := ResolveUnresolved(&thread)
	if !ok {
		t.Fatalf("expected resolve to succeed")
	}
	if thread.ResolvedAt == nil {
		t.Fatalf("expected ResolvedAt to be set")
	}
	if thread.RelationImpact != 0 {
		t.Fatalf("expected relation impact 0 after resolve, got %v", thread.RelationImpact)
	}
}

func TestApplyUnresolvedImpactToDimensions(t *testing.T) {
	dims := DefaultDimensions()
	threads := []UnresolvedThread{
		{Severity: 0.5, EscalationLevel: 2, RelationImpact: 0.15, ResolvedAt: nil},
		{Severity: 0.7, EscalationLevel: 3, RelationImpact: 0.315, ResolvedAt: nil},
	}

	impacts := ApplyUnresolvedImpact(&dims, threads)
	if len(impacts) == 0 {
		t.Fatalf("expected impacts from unresolved threads")
	}
	if dims.Conflict.Value <= 15 {
		t.Fatalf("expected conflict to increase from unresolved threads, got %v", dims.Conflict.Value)
	}
	if dims.Trust.Value >= 50 {
		t.Fatalf("expected trust to decrease from unresolved threads, got %v", dims.Trust.Value)
	}
}

func TestApplyUnresolvedImpactResolvedThreadsIgnored(t *testing.T) {
	dims := DefaultDimensions()
	now := time.Now()
	threads := []UnresolvedThread{
		{Severity: 0.5, EscalationLevel: 2, RelationImpact: 0.15, ResolvedAt: &now},
	}

	impacts := ApplyUnresolvedImpact(&dims, threads)
	if len(impacts) > 0 {
		t.Fatalf("expected no impacts from resolved threads, got %d", len(impacts))
	}
}

func TestProcessUnresolvedThreadsWithResolution(t *testing.T) {
	dims := DefaultDimensions()
	cfg := DefaultUnresolvedConfig()
	cfg.ResolutionThreshold = 0.5
	cfg.BaseEscalationHours = 100

	thread := CreateUnresolvedThread("", "", 0.1, nil)
	thread.Severity = 0.05
	thread.EscalationLevel = 0

	now := time.Now()
	processed, impacts := ProcessUnresolvedThreads([]UnresolvedThread{thread}, cfg, &dims, now)

	if len(processed) != 1 {
		t.Fatalf("expected 1 processed thread, got %d", len(processed))
	}
	if processed[0].ResolvedAt == nil {
		t.Fatalf("expected low severity thread to be auto-resolved")
	}
	if len(impacts) > 0 {
		t.Fatalf("expected no impacts after auto-resolution")
	}
}
