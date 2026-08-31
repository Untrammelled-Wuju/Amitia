package behavior

import (
	"context"
	"testing"
	"time"
)

type reconcileInteractionStateSource struct {
	interactions []InteractionSnapshot
}

func (s reconcileInteractionStateSource) QueryActiveInteractions(_ context.Context, _, _ string) ([]InteractionSnapshot, error) {
	return append([]InteractionSnapshot(nil), s.interactions...), nil
}

func (s reconcileInteractionStateSource) QueryVoiceSession(_ context.Context, _, _ string) (*VoiceBehaviorState, error) {
	return nil, nil
}

func (s reconcileInteractionStateSource) QueryActiveTools(_ context.Context, _, _ string) (map[string]ToolOperationState, error) {
	return map[string]ToolOperationState{}, nil
}

func TestReconcilerSelectsLatestInteractionByCreationTimeNotCrossInteractionStatusVersion(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	source := reconcileInteractionStateSource{interactions: []InteractionSnapshot{
		{
			InteractionID: "older-high-version",
			Phase:         "response_ready",
			StatusVersion: 99,
			CreatedAt:     now.Add(-2 * time.Minute),
			UpdatedAt:     now,
		},
		{
			InteractionID: "newer-low-version",
			Phase:         "received",
			StatusVersion: 1,
			CreatedAt:     now.Add(-time.Minute),
			UpdatedAt:     now.Add(-30 * time.Second),
		},
	}}
	reconciler := &Reconciler{stateSource: source}
	base := NewDefaultContext("user-1", "character-1")

	next, err := reconciler.buildReconciledContext(context.Background(), "user-1", "character-1", &base, now)
	if err != nil {
		t.Fatal(err)
	}
	if next.Transient.InteractionID != "newer-low-version" || next.Transient.InteractionPhase != "received" {
		t.Fatalf("reconcile selected stale interaction by incomparable status version: %+v", next.Transient)
	}
	wantStartedAt := now.Add(-time.Minute)
	if !next.Transient.InteractionStartedAt.Equal(wantStartedAt) {
		t.Fatalf("reconcile foreground timestamp = %v, want %v", next.Transient.InteractionStartedAt, wantStartedAt)
	}
}

type reconcileActivePetPort struct {
	pet *ActivePetSnapshot
}

func (p reconcileActivePetPort) ResolveActivePet(_ context.Context, _, _ string) (*ActivePetSnapshot, error) {
	return p.pet, nil
}

type reconcileRuntimePort struct {
	playback  *PlaybackSnapshot
	queryErr  error
	submitted []BehaviorRuntimeCommand
}

func (p *reconcileRuntimePort) SubmitBehaviorCommand(_ context.Context, command BehaviorRuntimeCommand) (*CommandReceipt, error) {
	p.submitted = append(p.submitted, command)
	return &CommandReceipt{
		CommandID:  command.CommandID,
		Accepted:   true,
		Status:     CmdAccepted,
		ReceivedAt: time.Now(),
	}, nil
}

func (p *reconcileRuntimePort) QueryPlayback(_ context.Context, petInstanceID string) (*PlaybackSnapshot, error) {
	if p.queryErr != nil {
		return nil, p.queryErr
	}
	if p.playback == nil {
		return &PlaybackSnapshot{PetInstanceID: petInstanceID, RuntimeOnline: true}, nil
	}
	copy := *p.playback
	if copy.PetInstanceID == "" {
		copy.PetInstanceID = petInstanceID
	}
	return &copy, nil
}

func reconcileTestActivePet() *ActivePetSnapshot {
	return &ActivePetSnapshot{
		UserID:         "user-1",
		DeviceID:       "device-1",
		RuntimeID:      "runtime-1",
		InstallationID: "install-1",
		PetInstanceID:  "runtime-1",
		CharacterID:    "character-1",
		RuntimeOnline:  true,
		StateRevision:  7,
		DefaultAction:  "idle_normal",
		Actions: map[string]ActionCapability{
			"idle_normal": {Key: "idle_normal", Available: true},
			"listening":   {Key: "listening", Available: true},
			"thinking":    {Key: "thinking", Available: true},
			"speaking":    {Key: "speaking", Available: true},
		},
	}
}

func TestReconcilerRecoversActiveInteractionThroughNormalResolver(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(now)
	source := reconcileInteractionStateSource{interactions: []InteractionSnapshot{{
		InteractionID: "interaction-1",
		Phase:         "response_ready",
		StatusVersion: 4,
		CreatedAt:     now.Add(-time.Second),
		UpdatedAt:     now,
	}}}
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
		reconcileActivePetPort{pet: reconcileTestActivePet()},
		runtimePort,
		nil,
	)
	base := NewDefaultContext("user-1", "character-1")
	base.Foreground = ForegroundActionState{
		DecisionID:    "stale-decision",
		Semantic:      "fallback_idle",
		ActionKey:     "idle_normal",
		Interruptible: true,
	}

	decision, err := reconciler.ReconcileCharacter(context.Background(), "user-1", "character-1", &base)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if decision.Status != DecisionStatusCommandSubmitted {
		t.Fatalf("decision status = %q, want command_submitted", decision.Status)
	}
	if decision.Semantic != "dialogue_speaking" || decision.ActionKey != "speaking" {
		t.Fatalf("recovery decision = %s/%s, want dialogue_speaking/speaking", decision.Semantic, decision.ActionKey)
	}
	if len(runtimePort.submitted) != 1 || runtimePort.submitted[0].ActionKey != "speaking" {
		t.Fatalf("submitted commands = %+v", runtimePort.submitted)
	}
}

func TestReconcilerDoesNotRestartRuntimeConfirmedForeground(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(now)
	source := reconcileInteractionStateSource{interactions: []InteractionSnapshot{{
		InteractionID: "interaction-1",
		Phase:         "response_ready",
		StatusVersion: 4,
		CreatedAt:     now.Add(-time.Second),
		UpdatedAt:     now,
	}}}
	startedAt := now.Add(-500 * time.Millisecond)
	runtimePort := &reconcileRuntimePort{playback: &PlaybackSnapshot{
		CurrentActionKey:  "speaking",
		CurrentDecisionID: "decision-1",
		IsPlaying:         true,
		StartedAt:         &startedAt,
		RuntimeOnline:     true,
	}}
	reconciler := NewReconciler(
		clock,
		nil,
		nil,
		nil,
		source,
		nil,
		nil,
		reconcileActivePetPort{pet: reconcileTestActivePet()},
		runtimePort,
		nil,
	)
	base := NewDefaultContext("user-1", "character-1")
	base.Foreground = ForegroundActionState{
		DecisionID:    "decision-1",
		Semantic:      "dialogue_speaking",
		ActionKey:     "speaking",
		Interruptible: true,
	}

	decision, err := reconciler.ReconcileCharacter(context.Background(), "user-1", "character-1", &base)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if decision.Status != DecisionStatusNoAction {
		t.Fatalf("decision status = %q, want no_action_available for already-playing semantic", decision.Status)
	}
	if len(runtimePort.submitted) != 0 {
		t.Fatalf("reconcile restarted confirmed foreground: %+v", runtimePort.submitted)
	}
}
