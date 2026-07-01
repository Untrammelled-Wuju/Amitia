package mindruntime

import (
	"testing"
	"time"
)

func TestResolveBeliefs_EmptyAdjustments(t *testing.T) {
	config := DefaultBeliefResolverConfig()
	candidate := ReflectionCandidate{
		CharacterID:   "char-1",
		Confidence:    0.8,
		BeliefAdjustments: nil,
	}
	applied := ResolveBeliefs(candidate, config)
	if len(applied) != 0 {
		t.Errorf("expected 0 applied, got %d", len(applied))
	}
}

func TestResolveBeliefs_LowConfidence(t *testing.T) {
	config := DefaultBeliefResolverConfig()
	config.MinConfidence = 0.7
	candidate := ReflectionCandidate{
		CharacterID: "char-1",
		Confidence:  0.3,
		BeliefAdjustments: []BeliefAdjustment{
			{BeliefKey: "belief/test", OldStrength: 0, NewStrength: 0.8, Reason: "测试"},
		},
	}
	applied := ResolveBeliefs(candidate, config)
	if len(applied) != 0 {
		t.Errorf("expected 0 applied for low confidence, got %d", len(applied))
	}
}

func TestResolveBeliefs_AppliesAdjustment(t *testing.T) {
	config := DefaultBeliefResolverConfig()
	candidate := ReflectionCandidate{
		ID:          "ref-001",
		CharacterID: "char-1",
		Confidence:  0.8,
		BeliefAdjustments: []BeliefAdjustment{
			{BeliefKey: "belief/music", OldStrength: 0.2, NewStrength: 0.8, Reason: "多次音乐交互"},
		},
	}
	applied := ResolveBeliefs(candidate, config)
	if len(applied) != 1 {
		t.Fatalf("expected 1 applied, got %d", len(applied))
	}
	a := applied[0]
	if a.BeliefKey != "belief/music" {
		t.Errorf("expected belief/music, got %s", a.BeliefKey)
	}
	if a.OldStrength != 0.2 {
		t.Errorf("expected old 0.2, got %f", a.OldStrength)
	}
	expectedNew := 0.2 + (0.8-0.2)*0.3
	if a.NewStrength != expectedNew {
		t.Errorf("expected smoothed new %f, got %f", expectedNew, a.NewStrength)
	}
	if a.CharacterID != "char-1" {
		t.Errorf("expected char-1, got %s", a.CharacterID)
	}
}

func TestResolveBeliefs_MaxAdjustPerRun(t *testing.T) {
	config := DefaultBeliefResolverConfig()
	config.MaxAdjustPerRun = 2
	candidate := ReflectionCandidate{
		ID:          "ref-001",
		CharacterID: "char-1",
		Confidence:  0.9,
		BeliefAdjustments: []BeliefAdjustment{
			{BeliefKey: "belief/a", OldStrength: 0, NewStrength: 0.5},
			{BeliefKey: "belief/b", OldStrength: 0, NewStrength: 0.6},
			{BeliefKey: "belief/c", OldStrength: 0, NewStrength: 0.7},
		},
	}
	applied := ResolveBeliefs(candidate, config)
	if len(applied) != 2 {
		t.Errorf("expected 2 applied (max), got %d", len(applied))
	}
}

func TestResolveRelationNarratives_Empty(t *testing.T) {
	resolved := ResolveRelationNarratives(nil, "char-1")
	if len(resolved) != 0 {
		t.Errorf("expected 0 resolved, got %d", len(resolved))
	}
}

func TestResolveRelationNarratives_Updates(t *testing.T) {
	updates := []RelationNarrative{
		{RelationID: "rel-1", OldNarrative: "陌生人", NewNarrative: "熟人", EvidenceCount: 5},
		{RelationID: "rel-2", OldNarrative: "同事", NewNarrative: "好友", EvidenceCount: 10},
	}
	resolved := ResolveRelationNarratives(updates, "char-1")
	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved, got %d", len(resolved))
	}
	if resolved[0].RelationID != "rel-1" {
		t.Errorf("expected rel-1, got %s", resolved[0].RelationID)
	}
	if resolved[0].OldNarrative != "陌生人" {
		t.Errorf("expected 陌生人, got %s", resolved[0].OldNarrative)
	}
	if resolved[0].NewNarrative != "熟人" {
		t.Errorf("expected 熟人, got %s", resolved[0].NewNarrative)
	}
	if resolved[0].EvidenceCount != 5 {
		t.Errorf("expected 5, got %d", resolved[0].EvidenceCount)
	}
	if resolved[1].RelationID != "rel-2" {
		t.Errorf("expected rel-2, got %s", resolved[1].RelationID)
	}
}

func TestRevertBeliefAdjustment(t *testing.T) {
	applied := AppliedBeliefAdjustment{
		ID: "app-001", BeliefKey: "belief/music",
		OldStrength: 0.2, NewStrength: 0.8,
		CharacterID: "char-1",
	}
	reverted := RevertBeliefAdjustment(applied, "测试回滚")
	if reverted.OldStrength != 0.8 {
		t.Errorf("expected reverted old 0.8, got %f", reverted.OldStrength)
	}
	if reverted.NewStrength != 0.2 {
		t.Errorf("expected reverted new 0.2, got %f", reverted.NewStrength)
	}
	if reverted.BeliefKey != "belief/music" {
		t.Errorf("expected belief/music, got %s", reverted.BeliefKey)
	}
}

func TestIsBeliefAdjustmentApplied(t *testing.T) {
	applied := AppliedBeliefAdjustment{
		ID: "app-001", AppliedAt: time.Now(),
	}
	if !IsBeliefAdjustmentApplied(applied) {
		t.Error("expected applied adjustment to be identified")
	}
	empty := AppliedBeliefAdjustment{}
	if IsBeliefAdjustmentApplied(empty) {
		t.Error("expected empty adjustment to not be identified as applied")
	}
}

func TestMergeAppliedAdjustments(t *testing.T) {
	now := time.Now()
	adjustments := []AppliedBeliefAdjustment{
		{ID: "a1", BeliefKey: "belief/music", OldStrength: 0, NewStrength: 0.5, AppliedAt: now, CharacterID: "char-1"},
		{ID: "a2", BeliefKey: "belief/music", OldStrength: 0.5, NewStrength: 0.8, AppliedAt: now.Add(time.Hour), CharacterID: "char-1"},
		{ID: "a3", BeliefKey: "belief/art", OldStrength: 0, NewStrength: 0.3, AppliedAt: now, CharacterID: "char-1"},
	}
	merged := MergeAppliedAdjustments(adjustments)
	if len(merged) != 2 {
		t.Errorf("expected 2 merged, got %d", len(merged))
	}
	for _, m := range merged {
		if m.BeliefKey == "belief/music" {
			if m.NewStrength != 0.8 {
				t.Errorf("expected music new 0.8, got %f", m.NewStrength)
			}
		}
	}
}

func TestMergeAppliedAdjustments_Single(t *testing.T) {
	now := time.Now()
	adjustments := []AppliedBeliefAdjustment{
		{ID: "a1", BeliefKey: "belief/music", OldStrength: 0, NewStrength: 0.5, AppliedAt: now, CharacterID: "char-1"},
	}
	merged := MergeAppliedAdjustments(adjustments)
	if len(merged) != 1 {
		t.Errorf("expected 1 merged, got %d", len(merged))
	}
	if merged[0].ID != "a1" {
		t.Errorf("expected a1, got %s", merged[0].ID)
	}
}

func TestDefaultBeliefResolverConfig(t *testing.T) {
	config := DefaultBeliefResolverConfig()
	if config.MinConfidence != 0.5 {
		t.Errorf("expected 0.5, got %f", config.MinConfidence)
	}
	if config.MaxAdjustPerRun != 5 {
		t.Errorf("expected 5, got %d", config.MaxAdjustPerRun)
	}
	if config.StrengthSmooth != 0.3 {
		t.Errorf("expected 0.3, got %f", config.StrengthSmooth)
	}
	if !config.RequireApproval {
		t.Error("expected RequireApproval true")
	}
}
