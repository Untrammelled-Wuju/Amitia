package behavior

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type reconnectSyncRepo struct {
	BehaviorStateRepository
	current   BehaviorContextSnapshot
	committed *BehaviorContextSnapshot
	decision  *BehaviorDecisionAudit
}

func (r *reconnectSyncRepo) LoadContext(_ context.Context, _, _ string) (*BehaviorContextSnapshot, error) {
	copy := r.current.Copy()
	return &copy, nil
}

func (r *reconnectSyncRepo) CommitContextAndDecisionCAS(_ context.Context, currentRevision int64, next BehaviorContextSnapshot, decision BehaviorDecisionAudit) (bool, error) {
	if r.current.Revision != currentRevision {
		return false, nil
	}
	copy := next.Copy()
	r.current = copy
	r.committed = &copy
	decisionCopy := decision
	r.decision = &decisionCopy
	return true, nil
}

type reconnectActivePetPort struct {
	pet *ActivePetSnapshot
}

func (p reconnectActivePetPort) ResolveActivePet(_ context.Context, _, _ string) (*ActivePetSnapshot, error) {
	return p.pet, nil
}

func TestRuntimeConnectedConsumesSnapshotSyncBeforeDecision(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(now)
	base := NewDefaultContext("user-1", "character-1")
	repo := &reconnectSyncRepo{current: base}
	source := reconcileInteractionStateSource{interactions: []InteractionSnapshot{{
		InteractionID: "interaction-recovered",
		Phase:         "response_ready",
		StatusVersion: 3,
		CreatedAt:     now.Add(-time.Second),
		UpdatedAt:     now,
	}}}
	pet := reconcileTestActivePet()
	runtimePort := &reconcileRuntimePort{playback: &PlaybackSnapshot{
		RuntimeOnline: true,
		IsPlaying:     false,
	}}
	reconciler := NewReconciler(
		clock,
		nil,
		nil,
		nil,
		source,
		nil,
		nil,
		reconnectActivePetPort{pet: pet},
		runtimePort,
		repo,
	)
	config := DefaultEngineConfig()
	config.ShadowMode = true
	config.RuntimeCommandEnabled = false
	engine := NewBehaviorEngine(config, clock, repo, reconnectActivePetPort{pet: pet}, runtimePort, reconciler)

	status, err := engine.processEventOnce(context.Background(), BehaviorEventEnvelope{
		EventID:        "runtime-connect-1",
		EventType:      "runtime.connected",
		SchemaVersion:  1,
		OccurredAt:     now,
		ReceivedAt:     now,
		UserID:         "user-1",
		CharacterID:    "character-1",
		InstallationID: "install-1",
		PetInstanceID:  "runtime-1",
		Origin:         OriginRuntime,
		DedupKey:       "runtime-connect-1",
	}, "")
	if err != nil {
		t.Fatalf("process runtime.connected: %v", err)
	}
	if status != InboxProcessed {
		t.Fatalf("status = %q, want processed", status)
	}
	if repo.committed == nil {
		t.Fatal("runtime.connected did not commit reconciled context")
	}
	if repo.committed.Transient.InteractionID != "interaction-recovered" || repo.committed.Transient.InteractionPhase != "response_ready" {
		t.Fatalf("snapshot sync was not consumed before commit: %+v", repo.committed.Transient)
	}
	if repo.decision == nil {
		t.Fatal("runtime.connected did not produce a recovery decision")
	}
	if repo.decision.Semantic != "dialogue_speaking" || repo.decision.ActionKey != "speaking" {
		t.Fatalf("reconnect decision = %s/%s, want dialogue_speaking/speaking", repo.decision.Semantic, repo.decision.ActionKey)
	}
}

func TestForegroundTerminalPromotesStillActiveConcurrentInteraction(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(now)
	base := NewDefaultContext("user-1", "character-1")
	base.Transient = TransientBehaviorState{
		InteractionID:        "interaction-b",
		InteractionPhase:     "response_ready",
		InteractionStartedAt: now,
		StatusVersion:        4,
	}
	repo := &reconnectSyncRepo{current: base}
	// interaction-a started earlier and was hidden when interaction-b became the
	// visual foreground. Once B terminates, authoritative state must promote A.
	source := reconcileInteractionStateSource{interactions: []InteractionSnapshot{{
		InteractionID: "interaction-a",
		Phase:         "response_started",
		StatusVersion: 7,
		CreatedAt:     now.Add(-time.Minute),
		UpdatedAt:     now.Add(time.Second),
	}}}
	pet := reconcileTestActivePet()
	runtimePort := &reconcileRuntimePort{playback: &PlaybackSnapshot{RuntimeOnline: true}}
	reconciler := NewReconciler(
		clock,
		nil,
		nil,
		nil,
		source,
		nil,
		nil,
		reconnectActivePetPort{pet: pet},
		runtimePort,
		repo,
	)
	config := DefaultEngineConfig()
	config.ShadowMode = true
	config.RuntimeCommandEnabled = false
	engine := NewBehaviorEngine(config, clock, repo, reconnectActivePetPort{pet: pet}, runtimePort, reconciler)
	payload, err := json.Marshal(map[string]interface{}{
		"interactionId": "interaction-b",
		"statusVersion": 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	status, err := engine.processEventOnce(context.Background(), BehaviorEventEnvelope{
		EventID:       "complete-b",
		EventType:     "chat.response.completed",
		SchemaVersion: 1,
		OccurredAt:    now.Add(2 * time.Second),
		ReceivedAt:    now.Add(2 * time.Second),
		UserID:        "user-1",
		CharacterID:   "character-1",
		InteractionID: "interaction-b",
		Origin:        OriginInteraction,
		DedupKey:      "complete-b",
		Payload:       payload,
	}, "")
	if err != nil {
		t.Fatalf("process foreground terminal: %v", err)
	}
	if status != InboxProcessed {
		t.Fatalf("status = %q, want processed", status)
	}
	if repo.committed == nil || repo.committed.Transient.InteractionID != "interaction-a" || repo.committed.Transient.InteractionPhase != "response_started" {
		t.Fatalf("older still-active interaction was not promoted: %+v", repo.committed)
	}
	if repo.decision == nil || repo.decision.Semantic != "dialogue_thinking" || repo.decision.ActionKey != "thinking" {
		t.Fatalf("promoted interaction decision = %+v, want dialogue_thinking/thinking", repo.decision)
	}
}
