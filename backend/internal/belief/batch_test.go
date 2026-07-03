package belief

import (
	"testing"
	"time"
)

func TestResolveBatchSingleEntry(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	result := ResolveBatch(BatchInput{
		Beliefs: []ResolveInput{
			{
				Key: "city",
				Now: now,
				Candidates: []Candidate{
					{Value: "上海", Source: SourceKindFact, Confidence: 0.85},
				},
			},
		},
	})
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].Belief.Value != "上海" {
		t.Fatalf("unexpected belief: %#v", result.Results[0].Belief)
	}
}

func TestConvertMemoryCandidates(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	mems := []MemoryCandidate{
		{Key: "city", Value: "上海", Confidence: 0.8, ObservedAt: now.Add(-1 * time.Hour)},
		{Key: "city", Value: "北京", Confidence: 0.6, ObservedAt: now.Add(-2 * time.Hour)},
	}
	cands := ConvertMemoryCandidates("city", mems, now, EvidenceSpan{})
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(cands))
	}
	if cands[0].Source != SourceKindMemory {
		t.Fatalf("expected source memory: %#v", cands[0])
	}
	if cands[0].ExpiresAt.IsZero() {
		t.Fatalf("expected non-zero expiresAt: %#v", cands[0])
	}
}
