package chat

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPhase46ExplicitEmptyCitationSnapshotClearsPersistedJSON(t *testing.T) {
	old := `[{'marker':'[K1]'}]`
	old = strings.ReplaceAll(old, "'", `"`)
	msg := &Message{CitationSnapshotJSON: old, CitationSnapshot: []CitationSnapshot{}}
	if err := msg.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	if msg.CitationSnapshotJSON != "[]" {
		t.Fatalf("stale persisted citation JSON: %s", msg.CitationSnapshotJSON)
	}
	var got []CitationSnapshot
	if err := json.Unmarshal([]byte(msg.CitationSnapshotJSON), &got); err != nil || len(got) != 0 {
		t.Fatalf("invalid cleared JSON: %v %+v", err, got)
	}
}

func TestPhase46LegacyJSONOnlySavePreservesCitationData(t *testing.T) {
	old := `[{"marker":"[K1]","kind":"knowledge","index":1}]`
	msg := &Message{CitationSnapshotJSON: old}
	if err := msg.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	if msg.CitationSnapshotJSON != old {
		t.Fatalf("legacy JSON-only row was erased: %s", msg.CitationSnapshotJSON)
	}
}

func TestPhase46ProvidedCitationSnapshotPersistsReplacement(t *testing.T) {
	msg := &Message{CitationSnapshotJSON: `[{"marker":"[K1]","kind":"knowledge","index":1}]`, CitationSnapshot: []CitationSnapshot{{Marker: "[K2]", Kind: "knowledge", Index: 2}}}
	if err := msg.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg.CitationSnapshotJSON, "[K1]") || !strings.Contains(msg.CitationSnapshotJSON, "[K2]") {
		t.Fatalf("replacement was not persisted: %s", msg.CitationSnapshotJSON)
	}
}
