package mindruntime

import (
	"testing"
	"time"
)

func TestBuildAbstractionHierarchy_Empty(t *testing.T) {
	config := DefaultAbstractionHierarchyConfig()
	result := BuildAbstractionHierarchy(nil, "char-1", config)
	if len(result) != 0 {
		t.Errorf("expected 0 abstractions for empty input, got %d", len(result))
	}
}

func TestBuildAbstractionHierarchy_SpecificLevel(t *testing.T) {
	config := DefaultAbstractionHierarchyConfig()
	now := time.Now()
	memories := []VerifiedMemory{
		{ID: "mem-1", Topic: "音乐", Content: "喜欢古典音乐", Importance: 0.5, CreatedAt: now},
		{ID: "mem-2", Topic: "音乐", Content: "喜欢爵士音乐", Importance: 0.6, CreatedAt: now},
	}
	result := BuildAbstractionHierarchy(memories, "char-1", config)
	if len(result) == 0 {
		t.Fatal("expected at least 1 abstraction")
	}
	foundSpecific := false
	for _, a := range result {
		if a.Level == AbstractionSpecific {
			foundSpecific = true
			if a.Topic != "音乐" {
				t.Errorf("expected topic 音乐, got %s", a.Topic)
			}
			if a.SourceCount != 2 {
				t.Errorf("expected source count 2, got %d", a.SourceCount)
			}
			if a.CharacterID != "char-1" {
				t.Errorf("expected char-1, got %s", a.CharacterID)
			}
		}
	}
	if !foundSpecific {
		t.Error("expected specific level abstraction")
	}
}

func TestBuildAbstractionHierarchy_GeneralLevel(t *testing.T) {
	config := DefaultAbstractionHierarchyConfig()
	config.MinSourcesForGeneral = 3
	now := time.Now()
	memories := []VerifiedMemory{
		{ID: "mem-1", Topic: "阅读", Content: "喜欢小说阅读", Importance: 0.5, CreatedAt: now},
		{ID: "mem-2", Topic: "阅读", Content: "喜欢历史阅读", Importance: 0.6, CreatedAt: now},
		{ID: "mem-3", Topic: "阅读", Content: "喜欢科普阅读", Importance: 0.7, CreatedAt: now},
	}
	result := BuildAbstractionHierarchy(memories, "char-1", config)
	foundGeneral := false
	for _, a := range result {
		if a.Level == AbstractionGeneral {
			foundGeneral = true
			if a.Topic != "阅读" {
				t.Errorf("expected topic 阅读, got %s", a.Topic)
			}
			if a.SourceCount != 3 {
				t.Errorf("expected source count 3, got %d", a.SourceCount)
			}
		}
	}
	if !foundGeneral {
		t.Error("expected general level abstraction with 3 sources")
	}
}

func TestBuildAbstractionHierarchy_HighLevel(t *testing.T) {
	config := DefaultAbstractionHierarchyConfig()
	config.MinSourcesForHighLevel = 5
	now := time.Now()
	memories := make([]VerifiedMemory, 6)
	for i := 0; i < 6; i++ {
		memories[i] = VerifiedMemory{
			ID: "mem-" + string(rune('a'+i)), Topic: "编程",
			Content: "编程相关", Importance: 0.5, CreatedAt: now,
		}
	}
	result := BuildAbstractionHierarchy(memories, "char-1", config)
	foundHigh := false
	for _, a := range result {
		if a.Level == AbstractionHighLevel {
			foundHigh = true
			if a.Topic != "编程" {
				t.Errorf("expected topic 编程, got %s", a.Topic)
			}
		}
	}
	if !foundHigh {
		t.Error("expected high level abstraction with 6 sources")
	}
}

func TestBuildAbstractionHierarchy_NoHighLevelWhenInsufficient(t *testing.T) {
	config := DefaultAbstractionHierarchyConfig()
	config.MinSourcesForHighLevel = 10
	now := time.Now()
	memories := make([]VerifiedMemory, 5)
	for i := 0; i < 5; i++ {
		memories[i] = VerifiedMemory{
			ID: "mem-" + string(rune('a'+i)), Topic: "运动",
			Content: "运动相关", Importance: 0.5, CreatedAt: now,
		}
	}
	result := BuildAbstractionHierarchy(memories, "char-1", config)
	for _, a := range result {
		if a.Level == AbstractionHighLevel {
			t.Error("expected no high level with insufficient sources")
		}
	}
}

func TestBuildAbstractionHierarchy_MultiTopic(t *testing.T) {
	config := DefaultAbstractionHierarchyConfig()
	now := time.Now()
	memories := []VerifiedMemory{
		{ID: "mem-1", Topic: "音乐", Content: "喜欢古典", Importance: 0.5, CreatedAt: now},
		{ID: "mem-2", Topic: "音乐", Content: "喜欢爵士", Importance: 0.6, CreatedAt: now},
		{ID: "mem-3", Topic: "阅读", Content: "喜欢小说", Importance: 0.7, CreatedAt: now},
		{ID: "mem-4", Topic: "阅读", Content: "喜欢历史", Importance: 0.8, CreatedAt: now},
	}
	result := BuildAbstractionHierarchy(memories, "char-1", config)
	topics := make(map[string]bool)
	for _, a := range result {
		topics[a.Topic] = true
	}
	if !topics["音乐"] {
		t.Error("expected 音乐 topic")
	}
	if !topics["阅读"] {
		t.Error("expected 阅读 topic")
	}
}

func TestGetAbstractionLevels_Sorted(t *testing.T) {
	now := time.Now()
	abstractions := []HierarchicalAbstraction{
		{Topic: "音乐", Level: AbstractionHighLevel, CreatedAt: now},
		{Topic: "音乐", Level: AbstractionSpecific, CreatedAt: now},
		{Topic: "音乐", Level: AbstractionGeneral, CreatedAt: now},
	}
	sorted := GetAbstractionLevels(abstractions)
	if len(sorted) != 3 {
		t.Fatalf("expected 3 sorted, got %d", len(sorted))
	}
	if sorted[0].Level != AbstractionSpecific {
		t.Errorf("expected specific first, got %s", sorted[0].Level)
	}
	if sorted[1].Level != AbstractionGeneral {
		t.Errorf("expected general second, got %s", sorted[1].Level)
	}
	if sorted[2].Level != AbstractionHighLevel {
		t.Errorf("expected high third, got %s", sorted[2].Level)
	}
}

func TestIsAbstractionExpired(t *testing.T) {
	now := time.Now()
	a := HierarchicalAbstraction{
		ExpiresAt: now.Add(time.Hour),
	}
	if IsAbstractionExpired(a, now) {
		t.Error("expected not expired")
	}
	if !IsAbstractionExpired(a, now.Add(2*time.Hour)) {
		t.Error("expected expired after 2 hours")
	}
	noExpiry := HierarchicalAbstraction{}
	if IsAbstractionExpired(noExpiry, now.Add(365*24*time.Hour)) {
		t.Error("expected never expired without expiry")
	}
}

func TestFilterActiveAbstractions(t *testing.T) {
	now := time.Now()
	abstractions := []HierarchicalAbstraction{
		{Topic: "音乐", ExpiresAt: now.Add(time.Hour)},
		{Topic: "阅读", ExpiresAt: now.Add(-time.Hour)},
		{Topic: "运动", ExpiresAt: time.Time{}},
	}
	active := FilterActiveAbstractions(abstractions, now)
	if len(active) != 2 {
		t.Errorf("expected 2 active, got %d", len(active))
	}
	for _, a := range active {
		if a.Topic == "阅读" {
			t.Error("expected 阅读 to be filtered out")
		}
	}
}

func TestExtractKeywords(t *testing.T) {
	now := time.Now()
	mems := []VerifiedMemory{
		{ID: "mem-1", Content: "喜欢古典音乐和爵士音乐", CreatedAt: now},
		{ID: "mem-2", Content: "经常听古典音乐", CreatedAt: now},
		{ID: "mem-3", Content: "也喜欢流行音乐", CreatedAt: now},
	}
	keywords := extractKeywords(mems)
	if len(keywords) == 0 {
		t.Fatal("expected non-empty keywords")
	}
}

func TestDefaultAbstractionHierarchyConfig(t *testing.T) {
	config := DefaultAbstractionHierarchyConfig()
	if config.MinSourcesForGeneral != 3 {
		t.Errorf("expected 3, got %d", config.MinSourcesForGeneral)
	}
	if config.MinSourcesForHighLevel != 9 {
		t.Errorf("expected 9, got %d", config.MinSourcesForHighLevel)
	}
	if config.DefaultRetentionDays != 90 {
		t.Errorf("expected 90, got %d", config.DefaultRetentionDays)
	}
	if config.MaxAbstractionsPerRun != 20 {
		t.Errorf("expected 20, got %d", config.MaxAbstractionsPerRun)
	}
}
