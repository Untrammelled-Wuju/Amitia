package interaction

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/temporal"
)

func TestVoiceEntryFinalTurnUsesUnifiedEntryAndPreservesVoiceEnvelope(t *testing.T) {
	tracker := NewInMemoryTracker()
	orch := NewRuntimeContextService(tracker, 16)
	orch.SetReady(true)
	unifiedEntry := NewUnifiedEntry(orch, NewScopeResolver(fakeScopeBindingLookup{bindings: []ScopeBinding{
		{
			ID:             "bind-voice",
			SpaceID:        "user-bound",
			CharacterID:    "char-bound",
			ConversationID: "conv-bound",
			Channel:        "voice",
			PeerID:         "peer-voice",
			Source:         "voice-binding",
			State:          ScopeBindingStateActive,
		},
	}}), temporal.SystemClock{})
	executor := &canonicalTestExecutor{entry: unifiedEntry}
	unifiedEntry.SetEntryExecutor(executor)
	entry := NewVoiceEntry(unifiedEntry)
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
	if executor.request == nil {
		t.Fatal("processor was not called")
	}
	if executor.request.CharacterID != "char-bound" || executor.request.ConversationID != "conv-bound" {
		t.Fatalf("voice turn did not use unified resolved scope: %#v", executor.request)
	}
	if executor.request.Channel != "voice" || executor.request.PeerID != "peer-voice" || executor.request.Source != "voice" {
		t.Fatalf("voice turn did not preserve unified channel source: %#v", executor.request)
	}
	if executor.request.RequestID != "turn-voice" || executor.request.SessionID != "session-voice" {
		t.Fatalf("voice envelope was not preserved: %#v", executor.request)
	}
	if !executor.request.VoiceMessage || executor.request.AudioUrl != "https://example.test/audio.wav" || executor.request.AudioDuration != 3.5 {
		t.Fatalf("voice media fields were not preserved: %#v", executor.request)
	}
	if executor.request.ExpressionPlan == nil || executor.request.ExpressionPlan.ID != "expr-voice" {
		t.Fatalf("expression plan was not preserved: %#v", executor.request.ExpressionPlan)
	}

}

func TestVoiceEntryRequiresCanonicalUnifiedEntry(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when unifiedEntry is nil, but NewVoiceEntry did not panic")
		}
	}()
	NewVoiceEntry(nil)
}
