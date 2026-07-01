package interaction

import (
	"encoding/json"
	"testing"
	"time"
)

func TestContextSnapshotDefaultVersion(t *testing.T) {
	s := ContextSnapshot{}
	if s.SnapshotVersion() != "context-snapshot-v1" {
		t.Fatal("bad default version")
	}
}

func TestContextSnapshotCustomVersion(t *testing.T) {
	s := ContextSnapshot{Version: "v2"}
	if s.SnapshotVersion() != "v2" {
		t.Fatal("bad custom version")
	}
}

func TestFieldReady(t *testing.T) {
	f := FieldReady("x", "src", "v1")
	if f.Value != "x" || f.Source != "src" {
		t.Fatal("fieldReady fail")
	}
	if f.Status != LoadStatusReady {
		t.Fatal("status not ready")
	}
}

func TestFieldUnavailable(t *testing.T) {
	f := FieldUnavailable[string]("mem")
	if f.Source != "mem" || f.Status != LoadStatusUnavailable {
		t.Fatal("unavailable fail")
	}
}

func TestFieldError(t *testing.T) {
	f := FieldError[string]("qdrant")
	if f.Status != LoadStatusError {
		t.Fatal("error fail")
	}
}

func TestMarshalUnmarshalRoundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	orig := ContextSnapshot{
		Version:        "tv1",
		RuntimeProfile: FieldReady(RuntimeProfile{PersonalitySource: "p1"}, "char", "v1"),
		Conversation:   FieldReady(ConversationState{ConversationID: "c1", MessageCount: 5}, "chat", "v1"), Psyche: FieldReady(PsycheState{Stress: 0.3, Fatigue: 0.1}, "psyche", "v1"),
		Relationship: FieldReady(RelationshipState{Trust: 0.8}, "rel", "v1"),
		Beliefs:      FieldReady(BeliefSet{Beliefs: []ResolvedBelief{{Key: "k1", Value: "v1", Confidence: 0.9}}}, "bel", "v1"),
		Memories:     FieldReady(MemorySet{Count: 1}, "mem", "v1"),
		Life:         FieldUnavailable[LifeState]("mood"),
		Channel:      FieldReady(ChannelCapabilities{Channel: "web", SupportsText: true}, "ch", "v1"),
		AssembledAt:  now,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal err: %v", err)
	}

	var got ContextSnapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal err: %v", err)
	}

	if got.Version != orig.Version {
		t.Fatal("version")
	}
	if got.RuntimeProfile.Value.PersonalitySource != "p1" {
		t.Fatal("runtimeProfile")
	}
	if got.Conversation.Value.MessageCount != 5 {
		t.Fatal("conversation")
	}
	if got.Psyche.Value.Stress != 0.3 {
		t.Fatal("psyche stress")
	}
	if got.Relationship.Value.Trust != 0.8 {
		t.Fatal("rel trust")
	}
	if got.Beliefs.Value.Beliefs[0].Key != "k1" {
		t.Fatal("belief key")
	}
	if got.Memories.Value.Count != 1 {
		t.Fatal("mem count")
	}
	if got.Life.Status != LoadStatusUnavailable {
		t.Fatal("life unavailable")
	}
	if got.Channel.Value.Channel != "web" {
		t.Fatal("channel")
	}
	if !got.AssembledAt.Equal(now) {
		t.Fatal("time")
	}
}

func TestRelationshipStateAllFields(t *testing.T) {
	rs := RelationshipState{Trust: 0.9, Familiarity: 0.8, Security: 0.7, Tension: 0.2, RepairConfidence: 0.6, Boundary: 0.5}
	if rs.Trust != 0.9 {
		t.Fatal("trust")
	}
	if rs.Familiarity != 0.8 {
		t.Fatal("familiarity")
	}
	if rs.Security != 0.7 {
		t.Fatal("security")
	}
	if rs.Tension != 0.2 {
		t.Fatal("tension")
	}
	if rs.RepairConfidence != 0.6 {
		t.Fatal("repair")
	}
	if rs.Boundary != 0.5 {
		t.Fatal("boundary")
	}
}

func TestConversationStateExtendedFields(t *testing.T) {
	scope := &InteractionScope{UserID: "u1", CharacterID: "c1", ConversationID: "conv1", Channel: "web"}
	cs := ConversationState{
		ConversationID:         "conv1",
		MessageCount:           12,
		CurrentTopic:           "scheduling",
		ActiveThreads:          []string{"reminder", "weather"},
		LastInteractionSummary: "用户讨论了明天的安排",
		AttentionState: &AttentionState{
			FocusTarget: "c1",
			FocusType:   "user_message",
			Intensity:   0.9,
		},
		StateVersion: "5",
		Scope:        scope,
		EmotionSnapshot: &EmotionSnapshot{
			Primary:   "joy",
			Secondary: "anticipation",
			Intensity: 0.7,
			Values:    map[string]float64{"joy": 0.7, "anticipation": 0.5},
		},
		RelationshipSnapshot: &RelationshipSnapshot{
			TargetID:    "u1",
			Trust:       0.8,
			Familiarity: 0.6,
			Values:      map[string]float64{"affection": 0.5},
		},
	}
	if cs.CurrentTopic != "scheduling" {
		t.Fatal("currentTopic")
	}
	if len(cs.ActiveThreads) != 2 {
		t.Fatal("activeThreads")
	}
	if cs.LastInteractionSummary != "用户讨论了明天的安排" {
		t.Fatal("lastInteractionSummary")
	}
	if cs.AttentionState == nil || cs.AttentionState.Intensity != 0.9 {
		t.Fatal("attentionState")
	}
	if cs.StateVersion != "5" {
		t.Fatal("stateVersion")
	}
	if cs.Scope == nil || cs.Scope.UserID != "u1" {
		t.Fatal("scope")
	}
	if cs.EmotionSnapshot == nil || cs.EmotionSnapshot.Primary != "joy" {
		t.Fatal("emotionSnapshot")
	}
	if cs.RelationshipSnapshot == nil || cs.RelationshipSnapshot.Trust != 0.8 {
		t.Fatal("relationshipSnapshot")
	}
}

func TestConversationStateMarshalExtended(t *testing.T) {
	scope := &InteractionScope{UserID: "u1", ConversationID: "c1", Channel: "web"}
	cs := ConversationState{
		ConversationID: "c1",
		MessageCount:   3,
		CurrentTopic:   "greeting",
		StateVersion:   "1",
		Scope:          scope,
		EmotionSnapshot: &EmotionSnapshot{
			Primary:   "neutral",
			Intensity: 0.5,
		},
	}
	data, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("marshal err: %v", err)
	}
	var got ConversationState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal err: %v", err)
	}
	if got.CurrentTopic != "greeting" {
		t.Fatal("topic roundtrip")
	}
	if got.Scope == nil || got.Scope.UserID != "u1" {
		t.Fatal("scope roundtrip")
	}
	if got.EmotionSnapshot == nil || got.EmotionSnapshot.Primary != "neutral" {
		t.Fatal("emotion roundtrip")
	}
}

func TestAttentionStateDefaults(t *testing.T) {
	as := AttentionState{}
	if as.Intensity != 0 {
		t.Fatal("intensity default")
	}
}

func TestEmotionSnapshotValues(t *testing.T) {
	es := EmotionSnapshot{
		Primary:   "sadness",
		Intensity: 0.3,
		Values:    map[string]float64{"sadness": 0.3, "disappointment": 0.2},
	}
	if es.Values["sadness"] != 0.3 {
		t.Fatal("values sadness")
	}
	if es.Values["disappointment"] != 0.2 {
		t.Fatal("values disappointment")
	}
}

func TestRelationshipSnapshotDefaults(t *testing.T) {
	rs := RelationshipSnapshot{}
	if rs.Trust != 0 {
		t.Fatal("trust default")
	}
	if rs.Familiarity != 0 {
		t.Fatal("familiarity default")
	}
}
