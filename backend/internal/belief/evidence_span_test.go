package belief

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEvidenceSpanSerialization(t *testing.T) {
	span := EvidenceSpan{SourceMsgID: "msg-001", SourceStart: 42, SourceEnd: 78}
	data, err := json.Marshal(span)
	if err != nil {
		t.Fatal(err)
	}
	var decoded EvidenceSpan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SourceMsgID != "msg-001" || decoded.SourceStart != 42 || decoded.SourceEnd != 78 {
		t.Fatalf("roundtrip failed: %+v", decoded)
	}
}

func TestEvidenceSpanZeroValues(t *testing.T) {
	span := EvidenceSpan{}
	if span.SourceMsgID != "" || span.SourceStart != 0 || span.SourceEnd != 0 {
		t.Fatal("expected zero values")
	}
}

func TestMemoryCandidateWithEvidence(t *testing.T) {
	candidate := MemoryCandidate{
		ID: "cand-1",
		Key: "hobby",
		Value: "likes hiking",
		Evidence: EvidenceSpan{SourceMsgID: "msg-005", SourceStart: 10, SourceEnd: 25},
		Confidence: 0.85,
		ObservedAt: time.Now(),
		Source: SourceKindUser,
	}
	if candidate.Evidence.SourceMsgID != "msg-005" {
		t.Fatal("expected evidence source msg id")
	}
}

func TestConvertMemoryCandidatesWithEvidence(t *testing.T) {
	now := time.Now()
	mems := []MemoryCandidate{
		{ID: "m1", Key: "pref", Value: "sweet", Confidence: 0.9, ObservedAt: now, Source: SourceKindUser, Evidence: EvidenceSpan{SourceMsgID: "msg-010", SourceStart: 5, SourceEnd: 12}},
	}
	evidence := EvidenceSpan{SourceMsgID: "override-msg"}
	result := ConvertMemoryCandidates("pref", mems, now, evidence)
	if len(result) != 1 {
		t.Fatal("expected 1 candidate")
	}
	if result[0].ID != "m1" {
		t.Fatal("expected ID to be preserved")
	}
}
