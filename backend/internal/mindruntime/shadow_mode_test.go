package mindruntime

import (
	"testing"
	"time"
)

func TestNewShadowState(t *testing.T) {
	state := NewShadowState()

	if state.Status != ShadowModeOff {
		t.Fatalf("expected off status, got %s", state.Status)
	}
	if state.CurrentPhase != ShadowPhaseInteraction {
		t.Fatalf("expected interaction phase, got %s", state.CurrentPhase)
	}
	if len(state.Decisions) != 0 {
		t.Fatalf("expected 0 decisions, got %d", len(state.Decisions))
	}
	if len(state.PhasesCompleted) != 0 {
		t.Fatalf("expected 0 completed phases, got %d", len(state.PhasesCompleted))
	}
}

func TestComputeShadowDecision(t *testing.T) {
	input := RuntimeObservabilityInput{
		Snapshot: RuntimeSnapshot{
			ID:            "snap-001",
			CharacterID:   "char-1",
			InteractionID: "inter-1",
			StateVersion:  5,
			CreatedAt:     time.Now().UTC(),
		},
		RequestID:     "req-001",
		TotalDuration: 150 * time.Millisecond,
		QueueDuration: 20 * time.Millisecond,
		QueueDepth:    5,
		DeliveryStatus: "delivered",
	}

	decision := ComputeShadowDecision(input, ShadowPhaseInteraction)

	if decision.ID == "" {
		t.Fatal("expected decision ID")
	}
	if decision.Phase != ShadowPhaseInteraction {
		t.Fatalf("expected interaction phase, got %s", decision.Phase)
	}
	if decision.SentToAuthority {
		t.Fatal("shadow decisions should not be sent to authority")
	}
	if decision.Metrics.LatencyMs == 0 {
		t.Fatal("expected non-zero latency")
	}
}

func TestCompareShadowResults(t *testing.T) {
	oldMetrics := ShadowMetrics{
		LatencyMs:       200,
		ErrorCount:      3,
		CancelCount:     2,
		QueueDepth:      15,
		DeliveryStatus:  "delivered",
		SafetyScore:     0.90,
		ConsistencyDiffs: 1,
	}

	newMetrics := ShadowMetrics{
		LatencyMs:       150,
		ErrorCount:      1,
		CancelCount:     1,
		QueueDepth:      10,
		DeliveryStatus:  "delivered",
		SafetyScore:     0.95,
		ConsistencyDiffs: 0,
	}

	result := CompareShadowResults(oldMetrics, newMetrics)

	if result.Decision == "" {
		t.Fatal("expected a decision")
	}
	if result.LatencyDiff != -50 {
		t.Fatalf("expected -50 latency diff, got %d", result.LatencyDiff)
	}
	if result.SafetyDiff < 0.049 || result.SafetyDiff > 0.051 {
		t.Fatalf("expected ~0.05 safety diff, got %f", result.SafetyDiff)
	}
	if len(result.OldReplyFeatures) == 0 {
		t.Fatal("expected old features")
	}
	if len(result.NewReplyFeatures) == 0 {
		t.Fatal("expected new features")
	}
}

func TestCheckAutoRollback(t *testing.T) {
	state := NewShadowState()
	state.Status = ShadowModeGray

	thresholds := DefaultAutoRollbackThresholds()

	currentMetrics := ShadowMetrics{
		LatencyMs:        6000,
		ErrorCount:       100,
		CancelCount:      10,
		QueueDepth:       200,
		DeliveryStatus:   "unknown",
		SafetyScore:      0.50,
		ConsistencyDiffs: 20,
		UnknownBacklog:   50,
		DuplicateDeliveries: 10,
		QueueAgeMs:       40000,
	}

	shouldRollback, event := CheckAutoRollback(state, currentMetrics, thresholds)

	if !shouldRollback {
		t.Fatal("expected auto rollback")
	}
	if event.Phase != ShadowPhaseInteraction {
		t.Fatalf("expected interaction phase, got %s", event.Phase)
	}
	if event.ToStatus != ShadowModeShadow {
		t.Fatalf("expected rollback to shadow, got %s", event.ToStatus)
	}
}

func TestCheckAutoRollbackNoTrigger(t *testing.T) {
	state := NewShadowState()
	state.Status = ShadowModeGray

	thresholds := DefaultAutoRollbackThresholds()

	currentMetrics := ShadowMetrics{
		LatencyMs:        100,
		ErrorCount:       0,
		CancelCount:      0,
		QueueDepth:       5,
		DeliveryStatus:   "delivered",
		SafetyScore:      0.95,
		ConsistencyDiffs: 0,
		UnknownBacklog:   0,
		DuplicateDeliveries: 0,
		QueueAgeMs:       100,
	}

	shouldRollback, _ := CheckAutoRollback(state, currentMetrics, thresholds)

	if shouldRollback {
		t.Fatal("expected no auto rollback")
	}
}

func TestAdvanceShadowPhase(t *testing.T) {
	state := NewShadowState()
	state.CurrentPhase = ShadowPhaseInteraction

	newState, advanced := AdvanceShadowPhase(state)

	if !advanced {
		t.Fatal("expected phase advance")
	}
	if newState.CurrentPhase != ShadowPhaseStateVersion {
		t.Fatalf("expected state_version phase, got %s", newState.CurrentPhase)
	}
}

func TestAdvanceShadowPhaseLast(t *testing.T) {
	state := NewShadowState()
	state.CurrentPhase = ShadowPhaseReflection

	newState, advanced := AdvanceShadowPhase(state)

	if advanced {
		t.Fatal("expected no phase advance at end")
	}
	if newState.CurrentPhase != ShadowPhaseReflection {
		t.Fatalf("expected reflection phase, got %s", newState.CurrentPhase)
	}
}

func TestAllShadowPhases(t *testing.T) {
	phases := AllShadowPhases()

	if len(phases) != 9 {
		t.Fatalf("expected 9 phases, got %d", len(phases))
	}

	expected := []ShadowPhase{
		ShadowPhaseInteraction,
		ShadowPhaseStateVersion,
		ShadowPhasePsyche,
		ShadowPhaseBelief,
		ShadowPhaseBDI,
		ShadowPhaseDelivery,
		ShadowPhaseReconciliation,
		ShadowPhaseProactive,
		ShadowPhaseReflection,
	}

	for i, p := range phases {
		if p != expected[i] {
			t.Fatalf("phase %d: expected %s, got %s", i, expected[i], p)
		}
	}
}

func TestDefaultAutoRollbackThresholds(t *testing.T) {
	thresholds := DefaultAutoRollbackThresholds()

	if thresholds.MaxErrorRate <= 0 {
		t.Fatal("expected positive max error rate")
	}
	if thresholds.MaxP95Latency <= 0 {
		t.Fatal("expected positive max P95 latency")
	}
}
