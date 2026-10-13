package business

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRerankUsesOccurredAtAndBoundsBoost(t *testing.T) {
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	metadata := map[string]MemoryTemporalMetadata{}
	occurred := time.Date(2025, 7, 18, 9, 0, 0, 0, time.UTC)
	metadata["memory-old"] = MemoryTemporalMetadata{MemoryID: "memory-old", OccurredAtUTC: &occurred, TemporalPrecision: "exact"}
	results, err := RerankMemoryScores(context.Background(), "去年今天发生了什么", []MemoryScoreCandidate{{MemoryID: "memory-old", BaseScore: 1, CreatedAt: now.Format(time.RFC3339), MemoryType: "worldbook"}}, metadata, now)
	if err != nil {
		t.Fatal(err)
	}
	result := results["memory-old"]
	if result.ReferenceSource != "occurred_at_utc" {
		t.Fatalf("wrong reference %#v", result)
	}
	if result.TemporalBoost > .18 {
		t.Fatalf("boost exceeded bound %#v", result)
	}
	if result.FinalScore >= 1.18 || result.FinalScore <= 0 {
		t.Fatalf("event-time decay not applied %#v", result)
	}
}

func TestMemoryRerankAppliesValidityPenalty(t *testing.T) {
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	metadata := map[string]MemoryTemporalMetadata{}
	expired := now.Add(-time.Hour)
	metadata["memory-expired"] = MemoryTemporalMetadata{MemoryID: "memory-expired", ValidToUTC: &expired}
	results, err := RerankMemoryScores(context.Background(), "这个事实什么时候失效", []MemoryScoreCandidate{{MemoryID: "memory-expired", BaseScore: 1, CreatedAt: now.Format(time.RFC3339), MemoryType: "fact"}}, metadata, now)
	if err != nil {
		t.Fatal(err)
	}
	if results["memory-expired"].ValidityPenalty != .65 {
		t.Fatalf("validity penalty missing %#v", results["memory-expired"])
	}
}

func TestMemoryRerankLeavesNonTemporalQueryUnchanged(t *testing.T) {
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	metadata := map[string]MemoryTemporalMetadata{}
	results, err := RerankMemoryScores(context.Background(), "她喜欢什么颜色", []MemoryScoreCandidate{{MemoryID: "memory-old", BaseScore: .83, CreatedAt: "2020-01-01T00:00:00Z", MemoryType: "episodic"}}, metadata, now)
	if err != nil {
		t.Fatal(err)
	}
	result := results["memory-old"]
	if result.FinalScore != .83 || result.TemporalBoost != 0 || result.ValidityPenalty != 1 || result.ReferenceSource != "none" {
		t.Fatalf("non-temporal query changed score %#v", result)
	}
}
