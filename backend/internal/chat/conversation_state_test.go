package chat

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

func TestNewConversationStateProvider(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)
	if p == nil {
		t.Fatal("provider is nil")
	}
	if p.Count() != 0 {
		t.Fatal("initial count not zero")
	}
}

func TestUpsertAndGetState(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	scope := interaction.InteractionScope{UserID: "u1", CharacterID: "c1", ConversationID: "conv1", Channel: "web"}
	cs := &interaction.ConversationState{
		ConversationID: "conv1",
		MessageCount:   10,
		CurrentTopic:   "planning",
		Scope:          &scope,
	}

	p.UpsertState("conv1", cs)

	got := p.GetState("conv1")
	if got == nil {
		t.Fatal("state not found")
	}
	if got.CurrentTopic != "planning" {
		t.Fatalf("expected planning, got %s", got.CurrentTopic)
	}
	if got.StateVersion != "1" {
		t.Fatalf("expected version 1, got %s", got.StateVersion)
	}
	if p.Count() != 1 {
		t.Fatalf("expected count 1, got %d", p.Count())
	}
}

func TestGetVersionedState(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	cs := &interaction.ConversationState{ConversationID: "v1", CurrentTopic: "topic-a"}
	p.UpsertState("v1", cs)

	state, version := p.GetVersionedState("v1")
	if state == nil {
		t.Fatal("state not found")
	}
	if version != 1 {
		t.Fatalf("expected version 1, got %d", version)
	}

	state2, version2 := p.GetVersionedState("nonexistent")
	if state2 != nil {
		t.Fatal("expected nil for nonexistent")
	}
	if version2 != 0 {
		t.Fatalf("expected version 0 for nonexistent, got %d", version2)
	}
}

func TestVersionBumpOnUpsert(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	cs1 := &interaction.ConversationState{ConversationID: "vb", CurrentTopic: "first"}
	p.UpsertState("vb", cs1)
	if p.GetState("vb").StateVersion != "1" {
		t.Fatal("first version not 1")
	}

	cs2 := &interaction.ConversationState{ConversationID: "vb", CurrentTopic: "second"}
	p.UpsertState("vb", cs2)
	if p.GetState("vb").StateVersion != "2" {
		t.Fatal("second version not 2")
	}
	if p.GetState("vb").CurrentTopic != "second" {
		t.Fatal("topic not updated")
	}
}

func TestBuildFromWorkingMemory(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	cache.UpdateSummary("wm1", "用户聊了天气和日程安排")

	p := NewConversationStateProvider(cache)
	scope := interaction.InteractionScope{UserID: "u1", ConversationID: "wm1", Channel: "web"}

	cs := p.BuildFromWorkingMemory("wm1", scope)
	if cs == nil {
		t.Fatal("state is nil")
	}
	if cs.ConversationID != "wm1" {
		t.Fatalf("bad convID: %s", cs.ConversationID)
	}
	if cs.LastInteractionSummary != "用户聊了天气和日程安排" {
		t.Fatalf("bad summary: %s", cs.LastInteractionSummary)
	}
	if cs.Scope == nil || cs.Scope.Channel != "web" {
		t.Fatal("scope not set")
	}
}

func TestBuildFromWorkingMemoryEmpty(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	scope := interaction.InteractionScope{UserID: "u1", ConversationID: "empty", Channel: "web"}
	cs := p.BuildFromWorkingMemory("empty", scope)

	if cs == nil {
		t.Fatal("state is nil")
	}
	if cs.LastInteractionSummary != "" {
		t.Fatalf("expected empty summary, got: %s", cs.LastInteractionSummary)
	}
	if len(cs.ActiveThreads) != 0 {
		t.Fatalf("expected empty threads, got: %v", cs.ActiveThreads)
	}
}

func TestSetAttention(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	attention := &interaction.AttentionState{
		FocusTarget: "user_message",
		FocusType:   "text",
		Intensity:   0.8,
	}
	p.SetAttention("att1", attention)

	state, version := p.GetVersionedState("att1")
	if state.AttentionState == nil {
		t.Fatal("attention state not set")
	}
	if state.AttentionState.Intensity != 0.8 {
		t.Fatalf("bad intensity: %f", state.AttentionState.Intensity)
	}
	if version != 1 {
		t.Fatalf("expected version 1, got %d", version)
	}
}

func TestSetEmotionSnapshot(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	snapshot := &interaction.EmotionSnapshot{
		Primary:   "excited",
		Secondary: "curious",
		Intensity: 0.75,
		Values:    map[string]float64{"excited": 0.8, "curious": 0.5},
	}
	p.SetEmotionSnapshot("emo1", snapshot)

	state := p.GetState("emo1")
	if state.EmotionSnapshot == nil {
		t.Fatal("emotion snapshot not set")
	}
	if state.EmotionSnapshot.Primary != "excited" {
		t.Fatalf("bad primary: %s", state.EmotionSnapshot.Primary)
	}
	if state.EmotionSnapshot.Values["excited"] != 0.8 {
		t.Fatalf("bad values")
	}
}

func TestSetRelationshipSnapshot(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	snapshot := &interaction.RelationshipSnapshot{
		TargetID:    "u99",
		Trust:       0.9,
		Familiarity: 0.7,
		Values:      map[string]float64{"loyalty": 0.8},
	}
	p.SetRelationshipSnapshot("rel1", snapshot)

	state := p.GetState("rel1")
	if state.RelationshipSnapshot == nil {
		t.Fatal("relationship snapshot not set")
	}
	if state.RelationshipSnapshot.TargetID != "u99" {
		t.Fatalf("bad target: %s", state.RelationshipSnapshot.TargetID)
	}
	if state.RelationshipSnapshot.Familiarity != 0.7 {
		t.Fatalf("bad familiarity: %f", state.RelationshipSnapshot.Familiarity)
	}
}

func TestUpdateTopic(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	p.UpdateTopic("topic1", "weather")
	state := p.GetState("topic1")
	if state.CurrentTopic != "weather" {
		t.Fatalf("bad topic: %s", state.CurrentTopic)
	}

	p.UpdateTopic("topic1", "sports")
	state = p.GetState("topic1")
	if state.CurrentTopic != "sports" {
		t.Fatalf("topic not updated")
	}
	_, version := p.GetVersionedState("topic1")
	if version != 2 {
		t.Fatalf("expected version 2, got %d", version)
	}
}

func TestUpdateSummary(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	p.UpdateSummary("sum1", "用户询问了关于旅行的建议")
	state := p.GetState("sum1")
	if state.LastInteractionSummary != "用户询问了关于旅行的建议" {
		t.Fatalf("bad summary: %s", state.LastInteractionSummary)
	}
}

func TestRemoveState(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	p.UpsertState("rm1", &interaction.ConversationState{ConversationID: "rm1", CurrentTopic: "test"})
	if p.Count() != 1 {
		t.Fatal("count not 1 before remove")
	}

	p.RemoveState("rm1")
	if p.Count() != 0 {
		t.Fatal("count not 0 after remove")
	}
	if p.GetState("rm1") != nil {
		t.Fatal("state still accessible after remove")
	}
}

func TestGetStateNonexistent(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	p := NewConversationStateProvider(cache)

	if p.GetState("ghost") != nil {
		t.Fatal("expected nil for nonexistent")
	}
}

func TestFullWorkflow(t *testing.T) {
	cache := NewWorkingMemoryCache(5 * time.Minute)
	cache.UpdateSummary("full", "用户讨论了工作计划和截止日期")

	p := NewConversationStateProvider(cache)
	scope := interaction.InteractionScope{UserID: "u1", CharacterID: "c1", ConversationID: "full", Channel: "web"}

	cs := p.BuildFromWorkingMemory("full", scope)
	p.UpsertState("full", cs)

	p.SetAttention("full", &interaction.AttentionState{FocusTarget: "planning", FocusType: "agenda", Intensity: 0.85})
	p.SetEmotionSnapshot("full", &interaction.EmotionSnapshot{Primary: "focused", Intensity: 0.6})
	p.SetRelationshipSnapshot("full", &interaction.RelationshipSnapshot{TargetID: "u1", Trust: 0.9, Familiarity: 0.8})
	p.UpdateTopic("full", "work_planning")
	p.UpdateSummary("full", "更新后的摘要：工作计划进入第二阶段")

	final, version := p.GetVersionedState("full")
	if final == nil {
		t.Fatal("final state nil")
	}
	if final.CurrentTopic != "work_planning" {
		t.Fatal("topic in final")
	}
	if final.AttentionState == nil || final.AttentionState.Intensity != 0.85 {
		t.Fatal("attention in final")
	}
	if final.EmotionSnapshot == nil || final.EmotionSnapshot.Primary != "focused" {
		t.Fatal("emotion in final")
	}
	if final.RelationshipSnapshot == nil || final.RelationshipSnapshot.Trust != 0.9 {
		t.Fatal("relationship in final")
	}
	if version != 6 {
		t.Fatalf("expected version 6, got %d", version)
	}
	if final.StateVersion != "6" {
		t.Fatalf("expected stateVersion 6, got %s", final.StateVersion)
	}
}
