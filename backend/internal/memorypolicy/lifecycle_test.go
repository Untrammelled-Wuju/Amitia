package memorypolicy

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLifecycleProjectionPreservesDecayAnchor(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	document := map[string]json.RawMessage{}
	initial := NewLifecycle(3, false, 5, now)
	WriteLifecycle(document, initial)
	raw, _ := json.Marshal(document)
	later := now.Add(90 * 24 * time.Hour)
	first := ProjectLifecycle(raw, later)
	second := ProjectLifecycle(first, later)
	if string(first) != string(second) {
		t.Fatal("repeated reads applied decay twice")
	}
	if err := json.Unmarshal(second, &document); err != nil {
		t.Fatal(err)
	}
	if stored := ReadLifecycle(document, later); stored != initial {
		t.Fatalf("projection changed persistent lifecycle: %+v", stored)
	}
	var strength float64
	if err := json.Unmarshal(document["effectiveStrength"], &strength); err != nil {
		t.Fatal(err)
	}
	if strength != initial.Effective(later).MemoryStrength || strength >= initial.MemoryStrength {
		t.Fatal("projection did not expose decayed strength")
	}
}

func TestLifecycleDecayProtectionAndReinforcement(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for level := 1; level <= 5; level++ {
		state := NewLifecycle(level, false, 5, now)
		if err := state.Validate(); err != nil {
			t.Fatal(err)
		}
		later := state.Effective(now.Add(365 * 24 * time.Hour))
		if level == 1 && later.MemoryStrength != 1 {
			t.Fatal("L1 lost protection")
		}
		if level != 1 && later.MemoryStrength >= state.MemoryStrength {
			t.Fatal("memory did not decay")
		}
	}
	pinned := NewLifecycle(5, true, 1, now)
	if pinned.Effective(now.Add(3650*24*time.Hour)).MemoryStrength != 1 || pinned.RetentionLevel != 1 {
		t.Fatal("pinned memory decayed")
	}
	weak := NewLifecycle(5, false, 1, now)
	if weak.Effective(now.Add(30*24*time.Hour)).DecayState != "archived" {
		t.Fatal("weak memory was not archived")
	}
	strong := NewLifecycle(3, false, 7, now)
	for count := 0; count < 5; count++ {
		strong = strong.Reinforce(now.Add(time.Duration(count) * time.Hour))
	}
	if strong.RetentionLevel != 1 || strong.ReinforceCount < 2 || strong.MemoryStrength <= .8 {
		t.Fatalf("reinforcement=%+v", strong)
	}
}
