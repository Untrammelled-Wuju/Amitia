package relationship

import (
	"testing"
	"time"
)

func TestDefaultConflictStateIsEmpty(t *testing.T) {
	cs := DefaultConflictState()
	if cs.ActiveRepair {
		t.Fatalf("expected active repair to be false")
	}
	if len(cs.ActiveConflicts) != 0 {
		t.Fatalf("expected no active conflicts, got %d", len(cs.ActiveConflicts))
	}
}

func TestStartConflictIncreasesCount(t *testing.T) {
	cs := DefaultConflictState()
	conflict := StartConflict(&cs, "ev-001", 0.7)

	if conflict == nil {
		t.Fatalf("expected non-nil conflict")
	}
	if cs.ConflictCount != 1 {
		t.Fatalf("expected conflict count 1, got %d", cs.ConflictCount)
	}
	if len(cs.ActiveConflicts) != 1 {
		t.Fatalf("expected 1 active conflict, got %d", len(cs.ActiveConflicts))
	}
}

func TestResolveConflictMarksResolved(t *testing.T) {
	cs := DefaultConflictState()
	conflict := StartConflict(&cs, "ev-001", 0.6)

	ok := ResolveConflict(&cs, conflict.ID)
	if !ok {
		t.Fatalf("expected resolve to succeed")
	}
	if cs.ResolvedCount != 1 {
		t.Fatalf("expected resolved count 1, got %d", cs.ResolvedCount)
	}
	if cs.ActiveConflicts[0].ResolvedAt == nil {
		t.Fatalf("expected ResolvedAt to be set")
	}
}

func TestResolveConflictNotFound(t *testing.T) {
	cs := DefaultConflictState()
	ok := ResolveConflict(&cs, "nonexistent")
	if ok {
		t.Fatalf("expected resolve to fail for nonexistent conflict")
	}
}

func TestRecordRepairAttemptAddsToHistory(t *testing.T) {
	cs := DefaultConflictState()
	conflict := StartConflict(&cs, "ev-001", 0.6)
	RecordRepairAttempt(&cs, conflict.ID, true, 0.8)

	if len(cs.RepairAttempts) != 1 {
		t.Fatalf("expected 1 repair attempt, got %d", len(cs.RepairAttempts))
	}
	if !cs.RepairAttempts[0].Effective {
		t.Fatalf("expected effective repair")
	}
}

func TestComputeRepairConfidenceEmpty(t *testing.T) {
	cs := DefaultConflictState()
	rc := ComputeRepairConfidence(cs)
	if rc != 0.35 {
		t.Fatalf("expected baseline 0.35, got %v", rc)
	}
}

func TestComputeRepairConfidenceAfterSuccesses(t *testing.T) {
	cs := DefaultConflictState()
	RecordRepairAttempt(&cs, "c1", true, 0.9)
	RecordRepairAttempt(&cs, "c1", true, 0.8)
	RecordRepairAttempt(&cs, "c1", true, 0.7)

	rc := ComputeRepairConfidence(cs)
	if rc <= 0.35 {
		t.Fatalf("expected above baseline after successes, got %v", rc)
	}
}

func TestComputeRepairConfidenceWithUnresolvedConflicts(t *testing.T) {
	cs := DefaultConflictState()
	RecordRepairAttempt(&cs, "c1", true, 0.9)
	RecordRepairAttempt(&cs, "c1", true, 0.9)
	StartConflict(&cs, "ev-001", 0.6)
	StartConflict(&cs, "ev-002", 0.5)

	rc := ComputeRepairConfidence(cs)
	if rc > 0.9 {
		t.Fatalf("expected penalty from unresolved conflicts, got %v", rc)
	}
}

func TestCheckActiveRepairTriggerNoTriggerWhenEmpty(t *testing.T) {
	cs := DefaultConflictState()
	if CheckActiveRepairTrigger(cs, 0.5) {
		t.Fatalf("expected no trigger with no conflicts")
	}
}

func TestCheckActiveRepairTriggerWhenHighConflictLowRepair(t *testing.T) {
	cs := DefaultConflictState()
	cs.ConflictCount = 4
	StartConflict(&cs, "ev-001", 0.8)
	StartConflict(&cs, "ev-002", 0.7)
	StartConflict(&cs, "ev-003", 0.6)

	RecordRepairAttempt(&cs, "c1", false, 0.3)
	RecordRepairAttempt(&cs, "c1", false, 0.2)

	if !CheckActiveRepairTrigger(cs, 0.5) {
		t.Fatalf("expected repair trigger with high conflict and low repair confidence")
	}
}

func TestTriggerActiveRepair(t *testing.T) {
	cs := DefaultConflictState()
	ok := TriggerActiveRepair(&cs)
	if !ok {
		t.Fatalf("expected trigger to succeed")
	}
	if !cs.ActiveRepair {
		t.Fatalf("expected active repair to be set")
	}
	if cs.RepairTriggeredAt.IsZero() {
		t.Fatalf("expected trigger time to be set")
	}

	ok2 := TriggerActiveRepair(&cs)
	if ok2 {
		t.Fatalf("expected second trigger to fail when already active")
	}
}

func TestClearActiveRepair(t *testing.T) {
	cs := DefaultConflictState()
	TriggerActiveRepair(&cs)
	ClearActiveRepair(&cs)
	if cs.ActiveRepair {
		t.Fatalf("expected active repair to be cleared")
	}
}

func TestComputeConflictTensionEmpty(t *testing.T) {
	tension := ComputeConflictTension(nil)
	if tension != 0 {
		t.Fatalf("expected 0 tension for no conflicts, got %v", tension)
	}
}

func TestComputeConflictTensionWithUnresolved(t *testing.T) {
	conflicts := []ActiveConflict{
		{ID: "c1", Intensity: 0.5, StartedAt: time.Now()},
		{ID: "c2", Intensity: 0.7, StartedAt: time.Now()},
	}
	tension := ComputeConflictTension(conflicts)
	if tension <= 0 {
		t.Fatalf("expected positive tension, got %v", tension)
	}
	if tension > 1 {
		t.Fatalf("expected tension <= 1, got %v", tension)
	}
}

func TestComputeConflictTensionWithEscalated(t *testing.T) {
	conflicts := []ActiveConflict{
		{ID: "c1", Intensity: 0.5, StartedAt: time.Now(), Escalated: true},
	}
	tension := ComputeConflictTension(conflicts)
	if tension <= 0.5*0.9 {
		t.Fatalf("expected elevated tension for escalated conflict, got %v", tension)
	}
}
