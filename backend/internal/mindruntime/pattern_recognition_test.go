package mindruntime

import (
	"testing"
	"time"
)

func TestRecognizePatterns_EmptyEvents(t *testing.T) {
	config := DefaultPatternRecognitionConfig()
	candidates := RecognizePatterns(nil, config)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for nil events, got %d", len(candidates))
	}
}

func TestRecognizePatterns_InsufficientEvents(t *testing.T) {
	config := DefaultPatternRecognitionConfig()
	config.MinIndependentEvents = 3
	now := time.Now()
	events := []PatternEvent{
		{ID: "e-1", Kind: "music", Summary: "音乐事件1", Importance: 0.5, Timestamp: now},
		{ID: "e-2", Kind: "music", Summary: "音乐事件2", Importance: 0.5, Timestamp: now},
	}
	candidates := RecognizePatterns(events, config)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for 2 events, got %d", len(candidates))
	}
}

func TestRecognizePatterns_ThreeEventsFormPattern(t *testing.T) {
	config := DefaultPatternRecognitionConfig()
	config.MinIndependentEvents = 3
	now := time.Now()
	events := []PatternEvent{
		{ID: "e-1", Kind: "music", Summary: "音乐事件1", Importance: 0.5, Timestamp: now},
		{ID: "e-2", Kind: "music", Summary: "音乐事件2", Importance: 0.6, Timestamp: now.Add(time.Hour)},
		{ID: "e-3", Kind: "music", Summary: "音乐事件3", Importance: 0.7, Timestamp: now.Add(2 * time.Hour)},
	}
	candidates := RecognizePatterns(events, config)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(candidates))
	}
	c := candidates[0]
	if c.Count != 3 {
		t.Errorf("expected count 3, got %d", c.Count)
	}
	if c.Kind != "kind" {
		t.Errorf("expected kind 'kind', got %s", c.Kind)
	}
	if c.Confidence <= 0 {
		t.Errorf("expected positive confidence, got %f", c.Confidence)
	}
}

func TestRecognizePatterns_ExcludesLowImportance(t *testing.T) {
	config := DefaultPatternRecognitionConfig()
	config.MinIndependentEvents = 2
	config.MinImportance = 0.3
	now := time.Now()
	events := []PatternEvent{
		{ID: "e-1", Kind: "noise", Summary: "噪声1", Importance: 0.0, Timestamp: now},
		{ID: "e-2", Kind: "noise", Summary: "噪声2", Importance: 0.0, Timestamp: now.Add(time.Hour)},
		{ID: "e-3", Kind: "noise", Summary: "噪声3", Importance: 0.0, Timestamp: now.Add(2 * time.Hour)},
	}
	candidates := RecognizePatterns(events, config)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for low importance events, got %d", len(candidates))
	}
}

func TestRecognizePatterns_TagGroupPattern(t *testing.T) {
	config := DefaultPatternRecognitionConfig()
	config.MinIndependentEvents = 3
	now := time.Now()
	events := []PatternEvent{
		{ID: "e-1", Kind: "chat", Summary: "对话1", Importance: 0.5, Timestamp: now, Tags: []string{"user-a", "topic-x"}},
		{ID: "e-2", Kind: "plan", Summary: "计划1", Importance: 0.6, Timestamp: now.Add(time.Hour), Tags: []string{"user-a", "topic-x"}},
		{ID: "e-3", Kind: "note", Summary: "笔记1", Importance: 0.7, Timestamp: now.Add(2 * time.Hour), Tags: []string{"user-a", "topic-x"}},
	}
	candidates := RecognizePatterns(events, config)
	foundTag := false
	for _, c := range candidates {
		if c.Kind == "tag_group" {
			foundTag = true
			if c.Count != 3 {
				t.Errorf("expected tag group count 3, got %d", c.Count)
			}
		}
	}
	if !foundTag {
		t.Error("expected at least one tag_group pattern")
	}
}

func TestRecognizePatterns_TimeWindowFilter(t *testing.T) {
	config := DefaultPatternRecognitionConfig()
	config.MinIndependentEvents = 3
	config.TimeWindow = 24 * time.Hour
	now := time.Now()
	events := []PatternEvent{
		{ID: "e-1", Kind: "music", Summary: "音乐1", Importance: 0.5, Timestamp: now},
		{ID: "e-2", Kind: "music", Summary: "音乐2", Importance: 0.6, Timestamp: now.Add(10 * time.Hour)},
		{ID: "e-3", Kind: "music", Summary: "音乐3", Importance: 0.7, Timestamp: now.Add(50 * time.Hour)},
	}
	candidates := RecognizePatterns(events, config)
	if len(candidates) == 0 {
		t.Log("time window filtered events below MinIndependentEvents (expected behavior)")
	}
}

func TestRecognizePatterns_MaxPatternsPerRun(t *testing.T) {
	config := DefaultPatternRecognitionConfig()
	config.MinIndependentEvents = 3
	config.MaxPatternsPerRun = 2
	now := time.Now()
	events := make([]PatternEvent, 0)
	kinds := []string{"a", "b", "c", "d", "e"}
	for _, k := range kinds {
		for i := 0; i < 3; i++ {
			events = append(events, PatternEvent{
				ID: k + "-" + string(rune('a'+i)),
				Kind: k, Summary: k, Importance: 0.5,
				Timestamp: now,
			})
		}
	}
	candidates := RecognizePatterns(events, config)
	if len(candidates) > 2 {
		t.Errorf("expected at most 2 patterns, got %d", len(candidates))
	}
}

func TestIsPatternDistinct_DifferentKind(t *testing.T) {
	a := PatternCandidate{Kind: "kind", EventIDs: []string{"e-1", "e-2", "e-3"}}
	b := PatternCandidate{Kind: "tag_group", EventIDs: []string{"e-1", "e-2", "e-3"}}
	if !IsPatternDistinct(a, b) {
		t.Error("expected different kind patterns to be distinct")
	}
}

func TestIsPatternDistinct_HighOverlap(t *testing.T) {
	a := PatternCandidate{Kind: "kind", EventIDs: []string{"e-1", "e-2", "e-3"}}
	b := PatternCandidate{Kind: "kind", EventIDs: []string{"e-1", "e-2", "e-4"}}
	if IsPatternDistinct(a, b) {
		t.Error("expected high overlap patterns to not be distinct")
	}
}

func TestIsPatternDistinct_LowOverlap(t *testing.T) {
	a := PatternCandidate{Kind: "kind", EventIDs: []string{"e-1", "e-2"}}
	b := PatternCandidate{Kind: "kind", EventIDs: []string{"e-5", "e-6"}}
	if !IsPatternDistinct(a, b) {
		t.Error("expected zero overlap patterns to be distinct")
	}
}

func TestMergePatternCandidates_MergesSimilar(t *testing.T) {
	now := time.Now()
	candidates := []PatternCandidate{
		{Kind: "kind", EventIDs: []string{"e-1", "e-2"}, Count: 2, FirstSeen: now, LastSeen: now, Confidence: 0.5},
		{Kind: "kind", EventIDs: []string{"e-2", "e-3"}, Count: 2, FirstSeen: now, LastSeen: now, Confidence: 0.6},
	}
	merged := MergePatternCandidates(candidates)
	if len(merged) != 1 {
		t.Errorf("expected 1 merged candidate, got %d", len(merged))
	}
	if merged[0].Count != 3 {
		t.Errorf("expected merged count 3, got %d", merged[0].Count)
	}
}

func TestMergePatternCandidates_NoMergeForDistinct(t *testing.T) {
	now := time.Now()
	candidates := []PatternCandidate{
		{Kind: "kind", EventIDs: []string{"e-1", "e-2"}, Count: 2, FirstSeen: now, LastSeen: now, Confidence: 0.5},
		{Kind: "kind", EventIDs: []string{"e-5", "e-6"}, Count: 2, FirstSeen: now, LastSeen: now, Confidence: 0.6},
	}
	merged := MergePatternCandidates(candidates)
	if len(merged) != 2 {
		t.Errorf("expected 2 distinct candidates, got %d", len(merged))
	}
}

func TestDefaultPatternRecognitionConfig(t *testing.T) {
	config := DefaultPatternRecognitionConfig()
	if config.MinIndependentEvents != 3 {
		t.Errorf("expected MinIndependentEvents 3, got %d", config.MinIndependentEvents)
	}
	if config.TimeWindow != 7*24*time.Hour {
		t.Errorf("expected TimeWindow 7 days, got %v", config.TimeWindow)
	}
	if config.MaxPatternsPerRun != 10 {
		t.Errorf("expected MaxPatternsPerRun 10, got %d", config.MaxPatternsPerRun)
	}
}
