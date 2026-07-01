package mindruntime

import (
	"testing"
	"time"
)

func TestBuildMemorySummaryRecordSetsIDAndFields(t *testing.T) {

	record := BuildMemorySummaryRecord("爱好", "char-1", "req-100", 3,
		[]string{"mem-1", "mem-2", "mem-3"},
		[]string{"颜色", "音乐"},
		time.Hour,
	)

	if record.Topic != "爱好" {
		t.Fatalf("unexpected topic: %s", record.Topic)
	}
	if record.CharacterID != "char-1" {
		t.Fatalf("unexpected characterID: %s", record.CharacterID)
	}
	if record.TotalEntries != 3 {
		t.Fatalf("unexpected totalEntries: %d", record.TotalEntries)
	}
	if record.SourceRequestID != "req-100" {
		t.Fatalf("unexpected sourceRequestID: %s", record.SourceRequestID)
	}
	if len(record.MemoryIDs) != 3 {
		t.Fatalf("expected 3 memory ids, got %d", len(record.MemoryIDs))
	}
	if len(record.SummarizedKeys) != 2 {
		t.Fatalf("expected 2 summarized keys, got %d", len(record.SummarizedKeys))
	}
	if record.ExpiresAt.IsZero() {
		t.Fatal("expected non-zero expiresAt")
	}
	if record.ID == "" {
		t.Fatal("expected non-empty id")
	}
	if !stringsHasPrefix(record.ID, "mem-summary-") {
		t.Fatalf("unexpected id prefix: %s", record.ID)
	}
}

func TestMemorySummaryRecordExpired(t *testing.T) {
	record := BuildMemorySummaryRecord("test", "char-1", "", 1, nil, nil, time.Hour)
	base := record.CreatedAt
	if record.Expired(base.Add(59 * time.Minute)) {
		t.Fatal("record expired too early")
	}
	if !record.Expired(base.Add(61 * time.Minute)) {
		t.Fatal("record should be expired after retention")
	}
	noRetention := BuildMemorySummaryRecord("test", "char-1", "", 1, nil, nil, 0)
	if noRetention.Expired(base.Add(90 * 24 * time.Hour)) {
		t.Fatal("record without retention should never expire")
	}
}

func TestMemorySummaryRecordCoversTopicAndCharacter(t *testing.T) {
	record := BuildMemorySummaryRecord(" 爱好 ", " char-1 ", "", 2, nil, nil, 0)
	if !record.CoversTopic("爱好") {
		t.Fatal("should cover topic")
	}
	if !record.CoversTopic(" 爱好 ") {
		t.Fatal("should cover topic with whitespace")
	}
	if record.CoversTopic("工作") {
		t.Fatal("should not cover other topic")
	}
	if !record.CoversCharacter("char-1") {
		t.Fatal("should cover character char-1")
	}
	if record.CoversCharacter("char-2") {
		t.Fatal("should not cover other character")
	}
}

func TestMemorySummaryRecordNormalizesIDs(t *testing.T) {
	record := BuildMemorySummaryRecord("test", "char-1", "", 1,
		[]string{"mem-1", "", "mem-1", "mem-2"},
		[]string{"a", "a", "b"},
		0,
	)
	if len(record.MemoryIDs) != 2 {
		t.Fatalf("expected 2 unique memory ids, got %d", len(record.MemoryIDs))
	}
	if len(record.SummarizedKeys) != 2 {
		t.Fatalf("expected 2 unique keys, got %d", len(record.SummarizedKeys))
	}
}

func TestDefaultSummarizerConfig(t *testing.T) {
	cfg := DefaultSummarizerConfig()
	if cfg.DefaultRetention != 24*time.Hour {
		t.Fatalf("unexpected default retention: %v", cfg.DefaultRetention)
	}
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
