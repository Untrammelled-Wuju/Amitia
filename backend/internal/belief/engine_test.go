package belief

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResolveBeliefUsesHighestPriorityStableCandidate(t *testing.T) {
	now := time.Date(2026, 7, 1, 18, 0, 0, 0, time.UTC)
	result := ResolveBelief(ResolveInput{
		Key: "city",
		Now: now,
		Candidates: []Candidate{
			{ID: "mem-1", Key: "city", Value: "上海", Source: SourceKindMemory, Confidence: 0.85, ObservedAt: now.Add(-2 * time.Hour)},
			{ID: "fact-1", Key: "city", Value: "上海", Source: SourceKindFact, Confidence: 0.88, ObservedAt: now.Add(-1 * time.Hour)},
			{ID: "user-1", Key: "city", Value: "北京", Source: SourceKindUser, Confidence: 0.7, ObservedAt: now.Add(-30 * time.Minute)},
		},
	})
	if result.Belief.Value != "上海" || result.Belief.Source != SourceKindFact {
		t.Fatalf("expected fact-backed belief, got %#v", result.Belief)
	}
}

func TestResolveBeliefSkipsExpiredAndLowConfidenceCandidates(t *testing.T) {
	now := time.Date(2026, 7, 1, 18, 30, 0, 0, time.UTC)
	result := ResolveBelief(ResolveInput{
		Key: "status",
		Now: now,
		Candidates: []Candidate{
			{ID: "expired", Key: "status", Value: "offline", Source: SourceKindFact, Confidence: 0.95, ExpiresAt: now.Add(-time.Minute)},
			{ID: "low", Key: "status", Value: "away", Source: SourceKindUser, Confidence: 0.2},
			{ID: "valid", Key: "status", Value: "online", Source: SourceKindUser, Confidence: 0.8},
		},
	})
	if result.Belief.Value != "online" {
		t.Fatalf("expected surviving candidate, got %#v", result.Belief)
	}
	if len(result.Rejected) != 2 {
		t.Fatalf("expected rejected candidates, got %#v", result.Rejected)
	}
}

func TestResolveBeliefMarksConflictWhenScoresClose(t *testing.T) {
	now := time.Date(2026, 7, 1, 19, 0, 0, 0, time.UTC)
	result := ResolveBelief(ResolveInput{
		Key: "mood",
		Now: now,
		Policy: ResolverPolicy{
			MinimumConfidence: 0.3,
			ConflictGap:       0.12,
			MaxCandidates:     10,
		},
		Candidates: []Candidate{
			{ID: "a", Key: "mood", Value: "calm", Source: SourceKindUser, Confidence: 0.71},
			{ID: "b", Key: "mood", Value: "anxious", Source: SourceKindMemory, Confidence: 0.76},
		},
	})
	if !result.Belief.Conflicted {
		t.Fatalf("expected conflict mark, got %#v", result.Belief)
	}
}

func TestResolveBeliefReturnsUnknownWhenNoUsableCandidate(t *testing.T) {
	now := time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC)
	result := ResolveBelief(ResolveInput{
		Key: "nickname",
		Now: now,
		Candidates: []Candidate{
			{ID: "x", Key: "other", Value: "阿米", Source: SourceKindUser, Confidence: 0.9},
		},
	})
	if !result.Belief.Unknown {
		t.Fatalf("expected unknown belief, got %#v", result.Belief)
	}
}

func TestResolveBeliefIsStableForRepeatedInput(t *testing.T) {
	now := time.Date(2026, 7, 1, 21, 0, 0, 0, time.UTC)
	input := ResolveInput{
		Key: "favorite_food",
		Now: now,
		Candidates: []Candidate{
			{ID: "1", Key: "favorite_food", Value: "面", Source: SourceKindInference, Confidence: 0.66, ObservedAt: now.Add(-2 * time.Hour)},
			{ID: "2", Key: "favorite_food", Value: "面", Source: SourceKindMemory, Confidence: 0.74, ObservedAt: now.Add(-3 * time.Hour)},
			{ID: "3", Key: "favorite_food", Value: "饭", Source: SourceKindUser, Confidence: 0.63, ObservedAt: now.Add(-90 * time.Minute)},
		},
	}
	first := ResolveBelief(input)
	second := ResolveBelief(input)
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("expected stable output\nfirst=%s\nsecond=%s", firstJSON, secondJSON)
	}
}
