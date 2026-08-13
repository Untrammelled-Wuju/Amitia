package workflow

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSideEffectJournal(t *testing.T) {
	journal := NewSideEffectJournal()

	if journal.Count() != 0 {
		t.Errorf("initial count = %d, want 0", journal.Count())
	}

	journal.Record("a", SideEffectToolCall, "tool-1", json.RawMessage(`{"x":1}`), json.RawMessage(`{"ok":true}`), "", 10*time.Millisecond)
	journal.Record("b", SideEffectHTTPCall, "http://api.example.com", json.RawMessage(`{}`), json.RawMessage(`{}`), "timeout", 20*time.Millisecond)

	if journal.Count() != 2 {
		t.Errorf("count = %d, want 2", journal.Count())
	}

	records := journal.Records()
	if len(records) != 2 {
		t.Errorf("records len = %d, want 2", len(records))
	}

	if records[0].NodeID != "a" {
		t.Errorf("first record node = %s, want a", records[0].NodeID)
	}
	if records[0].Kind != SideEffectToolCall {
		t.Errorf("first record kind = %s, want tool_call", records[0].Kind)
	}
	if records[1].NodeID != "b" {
		t.Errorf("second record node = %s, want b", records[1].NodeID)
	}
	if records[1].Error != "timeout" {
		t.Errorf("second record error = %s, want timeout", records[1].Error)
	}

	toolRecords := journal.ByKind(SideEffectToolCall)
	if len(toolRecords) != 1 {
		t.Errorf("tool records = %d, want 1", len(toolRecords))
	}

	nodeRecords := journal.ByNode("a")
	if len(nodeRecords) != 1 {
		t.Errorf("node a records = %d, want 1", len(nodeRecords))
	}

	nodeRecordsB := journal.ByNode("nonexistent")
	if len(nodeRecordsB) != 0 {
		t.Errorf("nonexistent records = %d, want 0", len(nodeRecordsB))
	}
}

func TestSideEffectJournalNilSafety(t *testing.T) {
	var journal *SideEffectJournal

	journal.Record("a", SideEffectToolCall, "t", nil, nil, "", 0)

	if journal.Count() != 0 {
		t.Error("nil journal count should be 0")
	}
	if journal.Records() != nil {
		t.Error("nil journal records should be nil")
	}
	if journal.ByKind(SideEffectToolCall) != nil {
		t.Error("nil journal by kind should be nil")
	}
}

func TestSideEffectJournalRedaction(t *testing.T) {
	journal := NewSideEffectJournal()

	largeInput := make([]byte, 5000)
	for i := range largeInput {
		largeInput[i] = 'a'
	}

	journal.Record("a", SideEffectToolCall, "t", json.RawMessage(largeInput), nil, "", 0)

	records := journal.Records()
	if len(records) != 1 {
		t.Fatalf("count = %d, want 1", len(records))
	}
	if len(records[0].Input) <= 4096 {
		t.Errorf("expected redacted input to be ~4096 bytes + suffix, got %d", len(records[0].Input))
	}
}
