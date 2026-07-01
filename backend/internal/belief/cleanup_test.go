package belief

import (
	"testing"
	"time"
)

func TestCleanupExpiredBeliefsRemovesExpiredCandidates(t *testing.T) {
	now := time.Date(2026, 7, 1, 18, 0, 0, 0, time.UTC)
	result := CleanupExpiredBeliefs(CleanupInput{
		Now: now,
		Candidates: []Candidate{
			{ID: "c1", Key: "status", Value: "online", Source: SourceKindUser, Confidence: 0.8, ExpiresAt: now.Add(1 * time.Hour)},
			{ID: "c2", Key: "status", Value: "offline", Source: SourceKindFact, Confidence: 0.95, ExpiresAt: now.Add(-1 * time.Minute)},
			{ID: "c3", Key: "status", Value: "away", Source: SourceKindMemory, Confidence: 0.6, ExpiresAt: now.Add(-2 * time.Hour)},
			{ID: "c4", Key: "status", Value: "busy", Source: SourceKindUser, Confidence: 0.7},
		},
	})
	if result.Removed != 2 {
		t.Fatalf("expected 2 removed, got %d (expired=%v)", result.Removed, result.Expired)
	}
	if result.Alive != 2 {
		t.Fatalf("expected 2 alive, got %d", result.Alive)
	}
	if len(result.Expired) != 2 {
		t.Fatalf("expected 2 expired IDs, got %v", result.Expired)
	}
}

func TestCleanupExpiredBeliefsKeepsAllWhenNoExpiry(t *testing.T) {
	now := time.Date(2026, 7, 1, 19, 0, 0, 0, time.UTC)
	result := CleanupExpiredBeliefs(CleanupInput{
		Now: now,
		Candidates: []Candidate{
			{ID: "a", Key: "color", Value: "blue", Source: SourceKindUser, Confidence: 0.8},
			{ID: "b", Key: "color", Value: "red", Source: SourceKindMemory, Confidence: 0.6},
		},
	})
	if result.Removed != 0 {
		t.Fatalf("expected 0 removed, got %d", result.Removed)
	}
	if result.Alive != 2 {
		t.Fatalf("expected 2 alive, got %d", result.Alive)
	}
}

func TestCleanupExpiredBeliefsHandlesEmptyInput(t *testing.T) {
	now := time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC)
	result := CleanupExpiredBeliefs(CleanupInput{Now: now})
	if result.Removed != 0 || result.Alive != 0 {
		t.Fatalf("expected zero counts for empty input: %#v", result)
	}
}

func TestCleanupExpiredBeliefsUsesDefaultTimeWhenZero(t *testing.T) {
	result := CleanupExpiredBeliefs(CleanupInput{
		Candidates: []Candidate{
			{ID: "exp", Key: "test", Value: "gone", Source: SourceKindFact, Confidence: 1, ExpiresAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
	})
	if result.Removed != 1 {
		t.Fatalf("expected 1 removed with zero Now (uses UTC now), got %d", result.Removed)
	}
}

func TestCleanupExpiredBeliefsIsIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 1, 21, 0, 0, 0, time.UTC)
	input := CleanupInput{
		Now: now,
		Candidates: []Candidate{
			{ID: "keep", Key: "mood", Value: "happy", Source: SourceKindUser, Confidence: 0.8, ExpiresAt: now.Add(2 * time.Hour)},
			{ID: "gone", Key: "mood", Value: "sad", Source: SourceKindMemory, Confidence: 0.5, ExpiresAt: now.Add(-1 * time.Hour)},
		},
	}
	first := CleanupExpiredBeliefs(input)
	second := CleanupExpiredBeliefs(input)
	if first.Removed != second.Removed || first.Alive != second.Alive {
		t.Fatalf("expected idempotent: first=%#v second=%#v", first, second)
	}
}
