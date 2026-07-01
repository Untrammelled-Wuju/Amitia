package decision

import (
	"testing"
	"time"
)

func TestBuildBeliefSnapshotPopulatesCategories(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	input := BeliefSnapshotInput{
		Facts: []BeliefEntry{
			{Key: "weather", Value: "晴朗", Confidence: 0.95, Source: BeliefSourceFact},
		},
		UserClaims: []BeliefEntry{
			{Key: "user_name", Value: "Lin", Confidence: 0.85, Source: BeliefSourceUser},
		},
		RoleBeliefs: []BeliefEntry{
			{Key: "role", Value: "companion", Confidence: 0.90, Source: BeliefSourceRole},
		},
		Inferences: []BeliefEntry{
			{Key: "mood_inferred", Value: "sad", Confidence: 0.70, Source: BeliefSourceInference},
		},
		Now: now,
	}

	snapshot := BuildBeliefSnapshot(input)

	if snapshot.ID == "" {
		t.Fatal("snapshot ID 不能为空")
	}
	if !snapshot.CapturedAt.Equal(now) {
		t.Fatalf("CapturedAt 不一致, expected %v, got %v", now, snapshot.CapturedAt)
	}
	if len(snapshot.Facts) != 1 || snapshot.Facts[0].Key != "weather" {
		t.Fatalf("Facts 分类错误: %#v", snapshot.Facts)
	}
	if len(snapshot.UserClaims) != 1 || snapshot.UserClaims[0].Key != "user_name" {
		t.Fatalf("UserClaims 分类错误: %#v", snapshot.UserClaims)
	}
	if len(snapshot.RoleBeliefs) != 1 || snapshot.RoleBeliefs[0].Key != "role" {
		t.Fatalf("RoleBeliefs 分类错误: %#v", snapshot.RoleBeliefs)
	}
	if len(snapshot.Inferences) != 1 || snapshot.Inferences[0].Key != "mood_inferred" {
		t.Fatalf("Inferences 分类错误: %#v", snapshot.Inferences)
	}
}

func TestBuildBeliefSnapshotDeterministicID(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	input := BeliefSnapshotInput{
		Facts: []BeliefEntry{
			{Key: "a", Value: "1", Confidence: 0.9, Source: BeliefSourceFact},
		},
		Now: now,
	}

	a := BuildBeliefSnapshot(input)
	b := BuildBeliefSnapshot(input)

	if a.ID != b.ID {
		t.Fatalf("相同输入的 ID 应一致: %s vs %s", a.ID, b.ID)
	}
}

func TestBeliefSnapshotAllEntries(t *testing.T) {
	snapshot := BeliefSnapshot{
		ID:          "test-1",
		Facts:       []BeliefEntry{{Key: "f1", Value: "v1", Confidence: 0.8, Source: BeliefSourceFact}},
		UserClaims:  []BeliefEntry{{Key: "u1", Value: "v2", Confidence: 0.7, Source: BeliefSourceUser}},
		RoleBeliefs: []BeliefEntry{{Key: "r1", Value: "v3", Confidence: 0.9, Source: BeliefSourceRole}},
		Inferences:  []BeliefEntry{{Key: "i1", Value: "v4", Confidence: 0.6, Source: BeliefSourceInference}},
	}

	all := snapshot.AllEntries()
	if len(all) != 4 {
		t.Fatalf("AllEntries 应包含 4 条, 实际 %d", len(all))
	}
}

func TestBeliefSnapshotHighConfidence(t *testing.T) {
	snapshot := BeliefSnapshot{
		Facts: []BeliefEntry{
			{Key: "fa", Value: "va", Confidence: 0.95, Source: BeliefSourceFact},
			{Key: "fb", Value: "vb", Confidence: 0.55, Source: BeliefSourceFact},
		},
	}

	result := snapshot.HighConfidence(0.8)
	if len(result) != 1 || result[0].Key != "fa" {
		t.Fatalf("HighConfidence 过滤错误: %#v", result)
	}
}

func TestBeliefSnapshotBySource(t *testing.T) {
	snapshot := BeliefSnapshot{
		Facts:       []BeliefEntry{{Key: "f1", Value: "v1", Confidence: 0.8, Source: BeliefSourceFact}},
		UserClaims:  []BeliefEntry{{Key: "u1", Value: "v2", Confidence: 0.7, Source: BeliefSourceUser}},
		RoleBeliefs: []BeliefEntry{{Key: "r1", Value: "v3", Confidence: 0.9, Source: BeliefSourceRole}},
		Inferences:  []BeliefEntry{{Key: "i1", Value: "v4", Confidence: 0.6, Source: BeliefSourceInference}},
	}

	if len(snapshot.BySource(BeliefSourceFact)) != 1 {
		t.Fatal("BySource(Fact) 错误")
	}
	if len(snapshot.BySource(BeliefSourceUser)) != 1 {
		t.Fatal("BySource(User) 错误")
	}
	if len(snapshot.BySource(BeliefSourceRole)) != 1 {
		t.Fatal("BySource(Role) 错误")
	}
	if len(snapshot.BySource(BeliefSourceInference)) != 1 {
		t.Fatal("BySource(Inference) 错误")
	}
	if snapshot.BySource("unknown") != nil {
		t.Fatal("未知 source 应返回 nil")
	}
}
