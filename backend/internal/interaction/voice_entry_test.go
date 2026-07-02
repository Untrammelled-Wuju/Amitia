package interaction

import (
	"context"
	"testing"
	"time"
)

func TestVoiceEntryFinalTurnUsesUnifiedEntryAndPreservesVoiceEnvelope(t *testing.T) {
	processor := &captureRequestProcessor{}
	tracker := NewInMemoryTracker()
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, NewInMemoryOutboxStore())
	orch.SetReady(true)
	entry := NewVoiceEntryWithUnifiedEntry(orch, NewUnifiedEntry(orch, NewScopeResolver(fakeScopeBindingLookup{bindings: []ScopeBinding{
		{
			ID:             "bind-voice",
			UserID:         "user-bound",
			CharacterID:    "char-bound",
			ConversationID: "conv-bound",
			Channel:        "voice",
			PeerID:         "peer-voice",
			Source:         "voice-binding",
			State:          ScopeBindingStateActive,
		},
	}})))
	plan := &ExpressionPlan{
		Version:   ExpressionPlanVersionV1,
		ID:        "expr-voice",
		CreatedAt: time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC),
		Tones:     []ExpressionTone{ExpressionToneWarm},
	}

	result, err := entry.HandleTurn(context.Background(), &VoiceTurnRequest{
		SessionID:      "session-voice",
		TurnID:         "turn-voice",
		Text:           "hello by voice",
		IsFinal:        true,
		Channel:        "voice",
		PeerID:         "peer-voice",
		AudioUrl:       "https://example.test/audio.wav",
		AudioDuration:  3.5,
		VoiceMessage:   true,
		ExpressionPlan: plan,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("unexpected result: %#v", result)
	}
	if processor.req == nil {
		t.Fatal("processor was not called")
	}
	if processor.req.CharacterID != "char-bound" || processor.req.ConversationID != "conv-bound" {
		t.Fatalf("voice turn did not use unified resolved scope: %#v", processor.req)
	}
	if processor.req.Channel != "voice" || processor.req.PeerID != "peer-voice" || processor.req.Source != "voice-binding" {
		t.Fatalf("voice turn did not preserve unified channel source: %#v", processor.req)
	}
	if processor.req.RequestID != "turn-voice" || processor.req.SessionID != "session-voice" {
		t.Fatalf("voice envelope was not preserved: %#v", processor.req)
	}
	if !processor.req.VoiceMessage || processor.req.AudioUrl != "https://example.test/audio.wav" || processor.req.AudioDuration != 3.5 {
		t.Fatalf("voice media fields were not preserved: %#v", processor.req)
	}
	if processor.req.ExpressionPlan == nil || processor.req.ExpressionPlan.ID != "expr-voice" {
		t.Fatalf("expression plan was not preserved: %#v", processor.req.ExpressionPlan)
	}
	record, ok, err := tracker.Get(context.Background(), result.InteractionID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("interaction record was not persisted")
	}
	if record.Scope.CharacterID != "char-bound" || record.Scope.ConversationID != "conv-bound" || record.Scope.Source != "voice-binding" {
		t.Fatalf("persisted scope did not come from unified entry: %#v", record.Scope)
	}
}

func TestVoiceEntryWithNilUnifiedEntryFallsBackToDefault(t *testing.T) {
	processor := &captureRequestProcessor{}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, NewInMemoryTracker(), NewInMemoryOutboxStore())
	orch.SetReady(true)
	entry := NewVoiceEntryWithUnifiedEntry(orch, nil)

	result, err := entry.HandleTurn(context.Background(), &VoiceTurnRequest{
		SessionID:      "session-fallback",
		TurnID:         "turn-fallback",
		Text:           "fallback voice",
		IsFinal:        true,
		Channel:        "voice",
		CharacterID:    "char-fallback",
		ConversationID: "conv-fallback",
		VoiceMessage:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("unexpected result: %#v", result)
	}
	if processor.req == nil {
		t.Fatal("processor was not called")
	}
	if processor.req.Source != "voice" || !processor.req.VoiceMessage {
		t.Fatalf("fallback unified entry did not preserve voice request: %#v", processor.req)
	}
}
