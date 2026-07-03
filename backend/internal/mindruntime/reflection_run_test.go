package mindruntime

import (
	"testing"
	"time"
)

func TestRunReflection_EmptyEvidence(t *testing.T) {
	config := DefaultReflectionRunConfig()
	evidence := ReflectionEvidence{}
	candidate := RunReflection("char-1", evidence, config, time.Now())
	if candidate.CharacterID != "char-1" {
		t.Errorf("expected char-1, got %s", candidate.CharacterID)
	}
	if len(candidate.BeliefAdjustments) > 0 {
		t.Error("expected no belief adjustments with empty evidence")
	}
	if candidate.Confidence != 0 {
		t.Errorf("expected 0 confidence, got %f", candidate.Confidence)
	}
}

func TestRunReflection_BeliefAdjustments(t *testing.T) {
	config := DefaultReflectionRunConfig()
	config.MinEvidenceForAdjustment = 1
	now := time.Now()
	evidence := ReflectionEvidence{
		Events: []VerifiedEvent{
			{ID: "evt-1", Kind: "user_liked_topic", Summary: "用户喜欢音乐", Importance: 0.8, Timestamp: now},
			{ID: "evt-2", Kind: "user_liked_topic", Summary: "用户喜欢阅读", Importance: 0.6, Timestamp: now},
		},
	}
	candidate := RunReflection("char-1", evidence, config, now)
	if len(candidate.BeliefAdjustments) == 0 {
		t.Error("expected belief adjustments")
	}
	if len(candidate.EvidenceRefs) != 2 {
		t.Errorf("expected 2 evidence refs, got %d", len(candidate.EvidenceRefs))
	}
}

func TestRunReflection_NoLowImportance(t *testing.T) {
	config := DefaultReflectionRunConfig()
	now := time.Now()
	evidence := ReflectionEvidence{
		Events: []VerifiedEvent{
			{ID: "evt-1", Kind: "trivial", Summary: "无关紧要", Importance: 0, Timestamp: now},
		},
	}
	candidate := RunReflection("char-1", evidence, config, now)
	if len(candidate.BeliefAdjustments) > 0 {
		t.Error("expected no adjustments for zero-importance events")
	}
}

func TestRunReflection_RelationUpdates(t *testing.T) {
	config := DefaultReflectionRunConfig()
	config.MinEvidenceForAdjustment = 2
	now := time.Now()
	evidence := ReflectionEvidence{
		Events: []VerifiedEvent{
			{ID: "evt-1", Kind: "chat", Summary: "聊天", Importance: 0.5, Timestamp: now, Tags: []string{"char-x"}},
			{ID: "evt-2", Kind: "chat", Summary: "聊天2", Importance: 0.6, Timestamp: now, Tags: []string{"char-x"}},
		},
		Relations: []VerifiedRelation{
			{ID: "rel-1", CharacterID: "char-x", Kind: "friend", Strength: 0.7, LastUpdated: now},
		},
	}
	candidate := RunReflection("char-1", evidence, config, now)
	if len(candidate.RelationUpdates) != 1 {
		t.Errorf("expected 1 relation update, got %d", len(candidate.RelationUpdates))
	}
	if candidate.RelationUpdates[0].RelationID != "rel-1" {
		t.Errorf("expected rel-1, got %s", candidate.RelationUpdates[0].RelationID)
	}
	if candidate.RelationUpdates[0].EvidenceCount != 2 {
		t.Errorf("expected 2 evidence count, got %d", candidate.RelationUpdates[0].EvidenceCount)
	}
}

func TestRunReflection_MemoryAbstractions(t *testing.T) {
	config := DefaultReflectionRunConfig()
	config.MinEvidenceForAdjustment = 2
	now := time.Now()
	evidence := ReflectionEvidence{
		Memories: []VerifiedMemory{
			{ID: "mem-1", Topic: "爱好", Content: "喜欢音乐", Importance: 0.5, CreatedAt: now},
			{ID: "mem-2", Topic: "爱好", Content: "喜欢绘画", Importance: 0.6, CreatedAt: now},
			{ID: "mem-3", Topic: "工作", Content: "程序员", Importance: 0.8, CreatedAt: now},
		},
	}
	candidate := RunReflection("char-1", evidence, config, now)
	if len(candidate.MemoryAbstractions) == 0 {
		t.Error("expected memory abstractions")
	}
	foundHobby := false
	for _, a := range candidate.MemoryAbstractions {
		if a.Topic == "爱好" {
			foundHobby = true
			if len(a.SourceIDs) != 2 {
				t.Errorf("expected 2 source IDs for hobby, got %d", len(a.SourceIDs))
			}
		}
	}
	if !foundHobby {
		t.Error("expected hobby topic abstraction")
	}
}

func TestRunReflection_InsufficientEvidenceForAbstraction(t *testing.T) {
	config := DefaultReflectionRunConfig()
	config.MinEvidenceForAdjustment = 3
	now := time.Now()
	evidence := ReflectionEvidence{
		Memories: []VerifiedMemory{
			{ID: "mem-1", Topic: "爱好", Content: "喜欢音乐", Importance: 0.5, CreatedAt: now},
			{ID: "mem-2", Topic: "爱好", Content: "喜欢绘画", Importance: 0.6, CreatedAt: now},
		},
	}
	candidate := RunReflection("char-1", evidence, config, now)
	if len(candidate.MemoryAbstractions) > 0 {
		t.Error("expected no abstractions with insufficient evidence")
	}
}

func TestIsReflectionCandidateSignificant(t *testing.T) {
	config := DefaultReflectionRunConfig()
	empty := ReflectionCandidate{}
	if IsReflectionCandidateSignificant(empty, config) {
		t.Error("expected empty candidate to be insignificant")
	}
	withBelief := ReflectionCandidate{
		BeliefAdjustments: []BeliefAdjustment{{BeliefKey: "test"}},
	}
	if !IsReflectionCandidateSignificant(withBelief, config) {
		t.Error("expected candidate with belief to be significant")
	}
}

func TestMergeReflectionCandidates(t *testing.T) {
	now := time.Now()
	c1 := ReflectionCandidate{
		ID: "ref-1", CharacterID: "char-1",
		BeliefAdjustments: []BeliefAdjustment{{BeliefKey: "k1"}},
		Confidence:        0.5, CreatedAt: now,
	}
	c2 := ReflectionCandidate{
		ID: "ref-2", CharacterID: "char-1",
		BeliefAdjustments: []BeliefAdjustment{{BeliefKey: "k2"}},
		Confidence:        0.7, CreatedAt: now,
	}
	merged := MergeReflectionCandidates([]ReflectionCandidate{c1, c2})
	if len(merged.BeliefAdjustments) != 2 {
		t.Errorf("expected 2 merged adjustments, got %d", len(merged.BeliefAdjustments))
	}
	if merged.Confidence != 0.6 {
		t.Errorf("expected 0.6 avg confidence, got %f", merged.Confidence)
	}
}

func TestMergeReflectionCandidates_DeDupRefs(t *testing.T) {
	now := time.Now()
	c1 := ReflectionCandidate{
		ID: "ref-1", CharacterID: "char-1",
		EvidenceRefs: []string{"event:1", "event:2"},
		CreatedAt:    now,
	}
	c2 := ReflectionCandidate{
		ID: "ref-2", CharacterID: "char-1",
		EvidenceRefs: []string{"event:2", "event:3"},
		CreatedAt:    now,
	}
	merged := MergeReflectionCandidates([]ReflectionCandidate{c1, c2})
	if len(merged.EvidenceRefs) != 3 {
		t.Errorf("expected 3 unique refs, got %d", len(merged.EvidenceRefs))
	}
}

func TestDefaultReflectionRunConfig(t *testing.T) {
	config := DefaultReflectionRunConfig()
	if config.MinEvidenceForAdjustment != 2 {
		t.Errorf("expected 2, got %d", config.MinEvidenceForAdjustment)
	}
	if config.MinConfidenceForAdopt != 0.5 {
		t.Errorf("expected 0.5, got %f", config.MinConfidenceForAdopt)
	}
	if config.MaxAbstractionsPerRun != 5 {
		t.Errorf("expected 5, got %d", config.MaxAbstractionsPerRun)
	}
}

func TestClampStrength(t *testing.T) {
	if clampStrength(-0.5) != 0 {
		t.Error("expected clamp of -0.5 to be 0")
	}
	if clampStrength(1.5) != 1 {
		t.Error("expected clamp of 1.5 to be 1")
	}
	if clampStrength(0.5) != 0.5 {
		t.Error("expected clamp of 0.5 to be 0.5")
	}
}
