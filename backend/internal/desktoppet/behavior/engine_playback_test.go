package behavior

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type playbackDecisionRepo struct {
	BehaviorStateRepository
	decision *BehaviorDecisionAudit
	err      error
}

func (r *playbackDecisionRepo) FindDecisionByID(_ context.Context, _ string) (*BehaviorDecisionAudit, error) {
	return r.decision, r.err
}

func TestPreparePlaybackEventEnrichesStartedForegroundMetadata(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	repo := &playbackDecisionRepo{decision: &BehaviorDecisionAudit{
		BehaviorDecision: BehaviorDecision{
			DecisionID:       "decision-1",
			UserID:           "user-1",
			CharacterID:      "character-1",
			InstallationID:   "install-1",
			RuntimeCommandID: "command-1",
			Semantic:         "dialogue_speaking",
			InterruptPolicy:  "queue",
			MinimumPlayMS:    500,
			MaximumPlayMS:    2500,
		},
	}}
	engine := &BehaviorEngine{repo: repo}
	raw, _ := json.Marshal(map[string]interface{}{
		"decisionId": "decision-1",
		"commandId":  "command-1",
		"actionKey":  "speaking",
	})
	event := BehaviorEventEnvelope{
		EventType:      "runtime.playback.action_started",
		OccurredAt:     now,
		UserID:         "user-1",
		CharacterID:    "character-1",
		InstallationID: "install-1",
		Payload:        raw,
	}

	prepared, err := engine.preparePlaybackEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("prepare playback: %v", err)
	}
	payload := parsePayload(prepared.Payload)
	if got := getString(payload, "semantic"); got != "dialogue_speaking" {
		t.Fatalf("semantic not rehydrated: %q", got)
	}
	if !getBool(payload, "interruptible") {
		t.Fatal("queue action must be interruptible after start")
	}
	if got := getInt(payload, "minimumPlayMs"); got != 500 {
		t.Fatalf("minimumPlayMs = %d", got)
	}
	if got := getInt(payload, "maximumPlayMs"); got != 2500 {
		t.Fatalf("maximumPlayMs = %d", got)
	}
}

func TestPreparePlaybackEventRejectsDecisionIdentityMismatch(t *testing.T) {
	repo := &playbackDecisionRepo{decision: &BehaviorDecisionAudit{
		BehaviorDecision: BehaviorDecision{
			DecisionID:  "decision-1",
			UserID:      "other-user",
			CharacterID: "character-1",
		},
	}}
	engine := &BehaviorEngine{repo: repo}
	raw, _ := json.Marshal(map[string]interface{}{"decisionId": "decision-1"})
	_, err := engine.preparePlaybackEvent(context.Background(), BehaviorEventEnvelope{
		EventType:   "runtime.playback.action_completed",
		UserID:      "user-1",
		CharacterID: "character-1",
		Payload:     raw,
	})
	if err == nil {
		t.Fatal("mismatched playback decision ownership must be rejected")
	}
}
