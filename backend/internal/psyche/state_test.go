package psyche

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestNewPsycheStateDefaults(t *testing.T) {
	state := NewPsycheState("char-001")

	if state.CharacterID != "char-001" {
		t.Fatalf("unexpected character id: %s", state.CharacterID)
	}
	if state.Version != StateVersionV1() {
		t.Fatalf("unexpected version: %s", state.Version)
	}
	if state.Emotion.Valence != 0.5 || state.Emotion.Arousal != 0.5 || state.Emotion.Dominance != 0.5 {
		t.Fatalf("unexpected emotion defaults: %#v", state.Emotion)
	}
	if state.Mood.MoodValence != 0.5 || state.Mood.MoodArousal != 0.5 {
		t.Fatalf("unexpected mood defaults: %#v", state.Mood)
	}
	if state.Stress != 0 {
		t.Fatalf("unexpected stress default: %f", state.Stress)
	}
	if state.Energy != 0.7 {
		t.Fatalf("unexpected energy default: %f", state.Energy)
	}
	if state.CreatedAt.IsZero() || state.UpdatedAt.IsZero() {
		t.Fatalf("timestamps should not be zero")
	}
}

func TestNewPsycheStateIndependent(t *testing.T) {
	a := NewPsycheState("char-a")
	time.Sleep(1 * time.Millisecond)
	b := NewPsycheState("char-b")

	if a.CharacterID != "char-a" || b.CharacterID != "char-b" {
		t.Fatalf("character ids not preserved")
	}
	if a.UpdatedAt.Equal(b.UpdatedAt) {
		t.Fatalf("timestamps should differ across calls")
	}
}

func TestInMemoryStoreSaveAndLoadState(t *testing.T) {
	store := NewInMemoryPsycheStore()
	state := NewPsycheState("char-002")
	state.Stress = 0.3

	if err := store.SaveState(&state); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := store.LoadState("char-002")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.Stress != 0.3 {
		t.Fatalf("stress not preserved: %f", loaded.Stress)
	}
	if loaded.CharacterID != state.CharacterID {
		t.Fatalf("character id not preserved: %s", loaded.CharacterID)
	}
}

func TestInMemoryStoreLoadMissingState(t *testing.T) {
	store := NewInMemoryPsycheStore()
	_, err := store.LoadState("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing state")
	}
}

func TestInMemoryStoreSaveAndLoadSnapshots(t *testing.T) {
	store := NewInMemoryPsycheStore()
	state := NewPsycheState("char-003")

	snap1 := CreateSnapshot(state)
	snap1.EmotionValence = 0.75
	if err := store.SaveSnapshot(&snap1); err != nil {
		t.Fatalf("save snapshot 1 failed: %v", err)
	}

	snap2 := CreateSnapshot(state)
	snap2.EmotionValence = 0.25
	if err := store.SaveSnapshot(&snap2); err != nil {
		t.Fatalf("save snapshot 2 failed: %v", err)
	}

	snaps, err := store.LoadSnapshots("char-003", 0)
	if err != nil {
		t.Fatalf("load snapshots failed: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snaps))
	}
	if snaps[0].EmotionValence != 0.75 {
		t.Fatalf("first snapshot valence: %f", snaps[0].EmotionValence)
	}
	if snaps[1].EmotionValence != 0.25 {
		t.Fatalf("second snapshot valence: %f", snaps[1].EmotionValence)
	}
}

func TestInMemoryStoreLoadEmptySnapshots(t *testing.T) {
	store := NewInMemoryPsycheStore()
	snaps, err := store.LoadSnapshots("unknown", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snaps) != 0 {
		t.Fatalf("expected empty slice, got %d", len(snaps))
	}
}

func TestInMemoryStoreUpdateState(t *testing.T) {
	store := NewInMemoryPsycheStore()
	state := NewPsycheState("char-004")
	if err := store.SaveState(&state); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}

	modified := state
	modified.Stress = 0.75
	if err := store.SaveState(&modified); err != nil {
		t.Fatalf("update save failed: %v", err)
	}

	loaded, _ := store.LoadState("char-004")
	if loaded.Stress != 0.75 {
		t.Fatalf("stress not updated: %f", loaded.Stress)
	}
}

func TestApplyEventInteractionIncreasesPositiveEmotion(t *testing.T) {
	state := NewPsycheState("char-005")
	now := time.Now().UTC()
	event := PsycheEvent{
		ID:             "evt-001",
		CharacterID:    "char-005",
		Type:           EventTypeInteraction,
		Source:         "user_message",
		ValenceDelta:   0.15,
		ArousalDelta:   0.1,
		DominanceDelta: 0.05,
		StressDelta:    0,
		EnergyDelta:    -0.05,
		Timestamp:      now,
	}

	result := ApplyEvent(state, event)

	if result.Emotion.Valence <= state.Emotion.Valence {
		t.Fatalf("valence did not increase: before=%f after=%f", state.Emotion.Valence, result.Emotion.Valence)
	}
	if result.Emotion.Arousal <= state.Emotion.Arousal {
		t.Fatalf("arousal did not increase")
	}
	if result.Emotion.Dominance <= state.Emotion.Dominance {
		t.Fatalf("dominance did not increase")
	}
	if result.Energy >= state.Energy {
		t.Fatalf("energy did not decrease")
	}
	if result.Emotion.Valence > 1 || result.Emotion.Arousal > 1 {
		t.Fatalf("emotion out of bounds: %#v", result.Emotion)
	}
}

func TestApplyEventNegativeReducesValenceAndIncreasesStress(t *testing.T) {
	state := NewPsycheState("char-006")
	now := time.Now().UTC()
	event := PsycheEvent{
		ID:             "evt-002",
		CharacterID:    "char-006",
		Type:           EventTypeAppraisal,
		Source:         "rejection",
		ValenceDelta:   -0.2,
		ArousalDelta:   0.15,
		DominanceDelta: -0.1,
		StressDelta:    0.2,
		EnergyDelta:    -0.1,
		Timestamp:      now,
	}

	result := ApplyEvent(state, event)

	if result.Emotion.Valence >= state.Emotion.Valence {
		t.Fatalf("valence did not decrease")
	}
	if result.Emotion.Dominance >= state.Emotion.Dominance {
		t.Fatalf("dominance did not decrease")
	}
	if result.Stress <= state.Stress {
		t.Fatalf("stress did not increase")
	}
	if result.Energy >= state.Energy {
		t.Fatalf("energy did not decrease")
	}
	if result.Emotion.Valence < 0 || result.Stress < 0 {
		t.Fatalf("out of bounds: %#v", result)
	}
}

func TestApplyEventRecoveryReducesStressIncreasesEnergy(t *testing.T) {
	state := NewPsycheState("char-007")
	state.Stress = 0.6
	state.Energy = 0.3
	state.Emotion.Valence = 0.3

	now := time.Now().UTC()
	event := PsycheEvent{
		ID:             "evt-003",
		CharacterID:    "char-007",
		Type:           EventTypeRecovery,
		Source:         "rest",
		ValenceDelta:   0.1,
		ArousalDelta:   -0.1,
		DominanceDelta: 0.05,
		StressDelta:    -0.2,
		EnergyDelta:    0.15,
		Timestamp:      now,
	}

	result := ApplyEvent(state, event)

	if result.Stress >= state.Stress {
		t.Fatalf("stress did not decrease")
	}
	if result.Energy <= state.Energy {
		t.Fatalf("energy did not increase")
	}
	if result.Emotion.Arousal >= state.Emotion.Arousal {
		t.Fatalf("arousal did not decrease during recovery")
	}
}

func TestApplyEventInternal(t *testing.T) {
	state := NewPsycheState("char-008")
	now := time.Now().UTC()
	event := PsycheEvent{
		ID:             "evt-004",
		CharacterID:    "char-008",
		Type:           EventTypeInternal,
		Source:         "rumination",
		ValenceDelta:   -0.05,
		ArousalDelta:   0.05,
		DominanceDelta: -0.02,
		StressDelta:    0.05,
		EnergyDelta:    0,
		Timestamp:      now,
	}

	result := ApplyEvent(state, event)

	if result.UpdatedAt != now {
		t.Fatalf("updated at not set to event timestamp")
	}
	if result.Emotion.Valence >= state.Emotion.Valence {
		t.Fatalf("internal rumination should reduce valence")
	}
}

func TestApplyEventClampsAtBounds(t *testing.T) {
	state := NewPsycheState("char-009")
	state.Emotion.Valence = 0.95
	state.Stress = 0.95
	state.Energy = 0.05

	now := time.Now().UTC()
	event := PsycheEvent{
		ID:             "evt-005",
		CharacterID:    "char-009",
		Type:           EventTypeInteraction,
		Source:         "overflow_test",
		ValenceDelta:   0.2,
		ArousalDelta:   1.5,
		DominanceDelta: 2.0,
		StressDelta:    0.2,
		EnergyDelta:    -0.2,
		Timestamp:      now,
	}

	result := ApplyEvent(state, event)

	if result.Emotion.Valence > 1 {
		t.Fatalf("valence exceeded 1: %f", result.Emotion.Valence)
	}
	if result.Emotion.Arousal > 1 {
		t.Fatalf("arousal exceeded 1: %f", result.Emotion.Arousal)
	}
	if result.Emotion.Dominance > 1 {
		t.Fatalf("dominance exceeded 1: %f", result.Emotion.Dominance)
	}
	if result.Stress > 1 {
		t.Fatalf("stress exceeded 1: %f", result.Stress)
	}
	if result.Energy < 0 {
		t.Fatalf("energy below 0: %f", result.Energy)
	}
}

func TestApplyEventClampsAtFloor(t *testing.T) {
	state := NewPsycheState("char-010")
	state.Emotion.Valence = 0.05
	state.Emotion.Dominance = 0.02
	state.Stress = 0.05

	now := time.Now().UTC()
	event := PsycheEvent{
		ID:             "evt-006",
		CharacterID:    "char-010",
		Type:           EventTypeAppraisal,
		Source:         "underflow_test",
		ValenceDelta:   -0.2,
		DominanceDelta: -0.5,
		StressDelta:    -0.2,
		Timestamp:      now,
	}

	result := ApplyEvent(state, event)

	if result.Emotion.Valence < 0 {
		t.Fatalf("valence below 0: %f", result.Emotion.Valence)
	}
	if result.Emotion.Dominance < 0 {
		t.Fatalf("dominance below 0: %f", result.Emotion.Dominance)
	}
	if result.Stress < 0 {
		t.Fatalf("stress below 0: %f", result.Stress)
	}
}

func TestApplyEventMoodTransfer(t *testing.T) {
	state := NewPsycheState("char-011")
	state.Emotion.Valence = 0.5
	state.Mood.MoodValence = 0.5

	now := time.Now().UTC()
	event := PsycheEvent{
		ID:           "evt-007",
		CharacterID:  "char-011",
		Type:         EventTypeInteraction,
		Source:       "mood_test",
		ValenceDelta: 0.3,
		ArousalDelta: 0.2,
		Timestamp:    now,
	}

	result := ApplyEvent(state, event)

	if result.Mood.MoodValence <= state.Mood.MoodValence {
		t.Fatalf("mood valence not affected by positive event")
	}
	if result.Mood.MoodArousal <= state.Mood.MoodArousal {
		t.Fatalf("mood arousal not affected by arousal event")
	}

	moodDelta := result.Mood.MoodValence - state.Mood.MoodValence
	if moodDelta > 0.031 {
		t.Fatalf("mood transfer too large: %f", moodDelta)
	}
}

func TestApplyEventDeterministic(t *testing.T) {
	state := NewPsycheState("char-012")
	state.Stress = 0.3
	state.Emotion.Valence = 0.45

	now := time.Now().UTC()
	event := PsycheEvent{
		ID:             "evt-008",
		CharacterID:    "char-012",
		Type:           EventTypeInteraction,
		Source:         "test",
		ValenceDelta:   0.12,
		ArousalDelta:   -0.08,
		DominanceDelta: 0.03,
		StressDelta:    -0.04,
		EnergyDelta:    -0.02,
		Timestamp:      now,
	}

	first := ApplyEvent(state, event)
	second := ApplyEvent(state, event)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("ApplyEvent is not deterministic\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestCreateSnapshotPreservesAllDimensions(t *testing.T) {
	state := NewPsycheState("char-013")
	state.Emotion.Valence = 0.72
	state.Emotion.Arousal = 0.38
	state.Emotion.Dominance = 0.61
	state.Mood.MoodValence = 0.55
	state.Mood.MoodArousal = 0.42
	state.Stress = 0.28
	state.Energy = 0.85

	snap := CreateSnapshot(state)

	if snap.CharacterID != "char-013" {
		t.Fatalf("character id mismatch")
	}
	if snap.Version != StateVersionV1() {
		t.Fatalf("version mismatch")
	}
	if snap.EmotionValence != 0.72 {
		t.Fatalf("emotion valence mismatch: %f", snap.EmotionValence)
	}
	if snap.EmotionArousal != 0.38 {
		t.Fatalf("emotion arousal mismatch: %f", snap.EmotionArousal)
	}
	if snap.EmotionDominance != 0.61 {
		t.Fatalf("emotion dominance mismatch: %f", snap.EmotionDominance)
	}
	if snap.MoodValence != 0.55 {
		t.Fatalf("mood valence mismatch: %f", snap.MoodValence)
	}
	if snap.MoodArousal != 0.42 {
		t.Fatalf("mood arousal mismatch: %f", snap.MoodArousal)
	}
	if snap.Stress != 0.28 {
		t.Fatalf("stress mismatch: %f", snap.Stress)
	}
	if snap.Energy != 0.85 {
		t.Fatalf("energy mismatch: %f", snap.Energy)
	}
	if snap.ID == "" {
		t.Fatalf("snapshot id is empty")
	}
	if snap.Timestamp.IsZero() {
		t.Fatalf("snapshot timestamp is zero")
	}
}

func TestCreateSnapshotUniqueIDs(t *testing.T) {
	state := NewPsycheState("char-014")
	a := CreateSnapshot(state)
	b := CreateSnapshot(state)

	if a.ID == b.ID {
		t.Fatalf("snapshot ids should be unique: %s", a.ID)
	}
}

func TestEventTypesCoverAllConstants(t *testing.T) {
	types := []EventType{EventTypeInteraction, EventTypeAppraisal, EventTypeInternal, EventTypeRecovery}
	for i, typ := range types {
		if typ == "" {
			t.Fatalf("event type at index %d is empty", i)
		}
	}
}

func TestJSONRoundTripPsycheState(t *testing.T) {
	state := NewPsycheState("char-015")
	state.Emotion.Valence = 0.66
	state.Stress = 0.15

	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var restored PsycheState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if restored.CharacterID != "char-015" {
		t.Fatalf("character id mismatch after round trip: %s", restored.CharacterID)
	}
	if restored.Emotion.Valence != 0.66 {
		t.Fatalf("valence mismatch: %f", restored.Emotion.Valence)
	}
	if restored.Stress != 0.15 {
		t.Fatalf("stress mismatch: %f", restored.Stress)
	}
}

func TestJSONRoundTripPsycheEvent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	event := PsycheEvent{
		ID:           "evt-json-001",
		CharacterID:  "char-016",
		Type:         EventTypeInteraction,
		Source:       "test",
		ValenceDelta: 0.1,
		Timestamp:    now,
	}

	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var restored PsycheEvent
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if restored.ID != "evt-json-001" {
		t.Fatalf("id mismatch: %s", restored.ID)
	}
	if restored.Type != EventTypeInteraction {
		t.Fatalf("type mismatch: %s", restored.Type)
	}
	if restored.ValenceDelta != 0.1 {
		t.Fatalf("valence delta mismatch: %f", restored.ValenceDelta)
	}
}

func TestJSONRoundTripPsycheSnapshot(t *testing.T) {
	state := NewPsycheState("char-017")
	snap := CreateSnapshot(state)
	snap.EmotionValence = 0.42

	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var restored PsycheSnapshot
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if restored.CharacterID != "char-017" {
		t.Fatalf("character id mismatch")
	}
	if restored.EmotionValence != 0.42 {
		t.Fatalf("emotion valence mismatch: %f", restored.EmotionValence)
	}
	if restored.ID != snap.ID {
		t.Fatalf("id mismatch")
	}
}
