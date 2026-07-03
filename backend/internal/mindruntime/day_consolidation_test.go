package mindruntime

import (
	"strings"
	"testing"
	"time"
)

func TestRunDayConsolidation_PreservesImportant(t *testing.T) {
	config := DefaultConsolidationConfig()
	now := time.Now()
	dayEvents := DayEvents{
		Date: "2026-07-01",
		Events: []DayEvent{
			{ID: "evt-1", Kind: "chat", Summary: "普通对话", Importance: 0.3, Timestamp: now},
			{ID: "evt-2", Kind: "chat", Summary: "重要讨论", Importance: 0.8, Timestamp: now},
			{ID: "evt-3", Kind: "plan", Summary: "计划任务", Importance: 0.4, IsPlan: true, Timestamp: now},
		},
	}
	result := RunDayConsolidation(dayEvents, "char-1", config)
	if len(result.PreservedEvents) != 2 {
		t.Errorf("expected 2 preserved events, got %d", len(result.PreservedEvents))
	}
	if len(result.CompressedEvents) != 1 {
		t.Errorf("expected 1 compressed event, got %d", len(result.CompressedEvents))
	}
	if result.Date != "2026-07-01" {
		t.Errorf("expected 2026-07-01, got %s", result.Date)
	}
}

func TestRunDayConsolidation_PreservesThread(t *testing.T) {
	config := DefaultConsolidationConfig()
	now := time.Now()
	dayEvents := DayEvents{
		Date: "2026-07-01",
		Events: []DayEvent{
			{ID: "evt-1", Kind: "chat", Summary: "对话", Importance: 0.2, Timestamp: now},
			{ID: "evt-2", Kind: "thread", Summary: "线程事件", Importance: 0.1, IsThread: true, Timestamp: now},
		},
	}
	result := RunDayConsolidation(dayEvents, "char-1", config)
	if len(result.PreservedEvents) != 1 {
		t.Errorf("expected 1 preserved thread event, got %d", len(result.PreservedEvents))
	}
	if result.PreservedEvents[0].ID != "evt-2" {
		t.Errorf("expected evt-2 to be preserved, got %s", result.PreservedEvents[0].ID)
	}
}

func TestRunDayConsolidation_CompressLimit(t *testing.T) {
	config := DefaultConsolidationConfig()
	config.MaxCompressedEvents = 2
	now := time.Now()
	events := make([]DayEvent, 5)
	for i := 0; i < 5; i++ {
		events[i] = DayEvent{
			ID:   "evt-" + string(rune('a'+i)),
			Kind: "chat", Summary: "对话", Importance: 0.1, Timestamp: now,
		}
	}
	dayEvents := DayEvents{Date: "2026-07-01", Events: events}
	result := RunDayConsolidation(dayEvents, "char-1", config)
	if len(result.CompressedEvents) != 2 {
		t.Errorf("expected 2 compressed events due to limit, got %d", len(result.CompressedEvents))
	}
}

func TestRunDayConsolidation_GeneratesSummaries(t *testing.T) {
	config := DefaultConsolidationConfig()
	now := time.Now()
	dayEvents := DayEvents{
		Date: "2026-07-01",
		Events: []DayEvent{
			{ID: "evt-1", Kind: "chat", Summary: "对话1", Importance: 0.3, Timestamp: now},
			{ID: "evt-2", Kind: "chat", Summary: "对话2", Importance: 0.2, Timestamp: now},
			{ID: "evt-3", Kind: "plan", Summary: "计划", Importance: 0.6, Timestamp: now},
		},
	}
	result := RunDayConsolidation(dayEvents, "char-1", config)
	if len(result.GeneratedSummaries) == 0 {
		t.Error("expected generated summaries")
	}
	for _, s := range result.GeneratedSummaries {
		if s.CharacterID != "char-1" {
			t.Errorf("expected char-1, got %s", s.CharacterID)
		}
	}
}

func TestRunDayConsolidation_BuildsAbstractions(t *testing.T) {
	config := DefaultConsolidationConfig()
	now := time.Now()
	dayEvents := DayEvents{
		Date: "2026-07-01",
		Events: []DayEvent{
			{ID: "evt-1", Kind: "music", Summary: "音乐", Importance: 0.3, Timestamp: now},
			{ID: "evt-2", Kind: "music", Summary: "音乐2", Importance: 0.2, Timestamp: now},
			{ID: "evt-3", Kind: "music", Summary: "音乐3", Importance: 0.4, Timestamp: now},
		},
	}
	result := RunDayConsolidation(dayEvents, "char-1", config)
	if len(result.MemoryAbstractions) == 0 {
		t.Error("expected at least 1 memory abstraction")
	}
	foundMusic := false
	for _, a := range result.MemoryAbstractions {
		if a.Topic == "music" {
			foundMusic = true
			if len(a.SourceIDs) != 3 {
				t.Errorf("expected 3 source IDs, got %d", len(a.SourceIDs))
			}
		}
	}
	if !foundMusic {
		t.Error("expected music topic abstraction")
	}
}

func TestRunDayConsolidation_EmptyDay(t *testing.T) {
	config := DefaultConsolidationConfig()
	dayEvents := DayEvents{Date: "2026-07-01"}
	result := RunDayConsolidation(dayEvents, "char-1", config)
	if len(result.PreservedEvents) > 0 {
		t.Error("expected no preserved events for empty day")
	}
	if len(result.CompressedEvents) > 0 {
		t.Error("expected no compressed events for empty day")
	}
}

func TestBuildAbstractSummary(t *testing.T) {
	result := buildAbstractSummary("2026-07-01", "music", 3)
	if !strings.Contains(result, "2026-07-01") {
		t.Errorf("expected date in summary, got %s", result)
	}
	if !strings.Contains(result, "music") {
		t.Errorf("expected kind in summary, got %s", result)
	}
}

func TestDefaultConsolidationConfig(t *testing.T) {
	config := DefaultConsolidationConfig()
	if config.ImportanceThreshold != 0.5 {
		t.Errorf("expected 0.5, got %f", config.ImportanceThreshold)
	}
	if config.MaxCompressedEvents != 100 {
		t.Errorf("expected 100, got %d", config.MaxCompressedEvents)
	}
	if config.Retention != 30*24*time.Hour {
		t.Errorf("expected 30 days, got %v", config.Retention)
	}
}
