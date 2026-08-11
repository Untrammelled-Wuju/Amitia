package interaction

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/outbox"
)

type fakeReflectionOutbox struct {
	mu      sync.Mutex
	records []outbox.OutboxRecord
}

func (f *fakeReflectionOutbox) Append(record outbox.OutboxRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, record)
	return nil
}

func (f *fakeReflectionOutbox) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records)
}

type fakeEvidenceReader struct {
	relations []mindruntime.VerifiedRelation
	memories  []mindruntime.VerifiedMemory
	err       error
}

func (r *fakeEvidenceReader) LoadVerifiedEvidence(
	ctx context.Context,
	scope ReflectionScopeKey,
	cutoff time.Time,
	limit int,
) (ReflectionExternalEvidence, error) {
	if r.err != nil {
		return ReflectionExternalEvidence{}, r.err
	}
	return ReflectionExternalEvidence{
		Relations:     r.relations,
		Memories:      r.memories,
		AnomalyScores: nil,
	}, nil
}

func newTestReflectionService(outbox ReflectionOutbox, reader ReflectionEvidenceReader) *ReflectionService {
	return NewReflectionService(
		WithReflectionTriggerConfig(mindruntime.ReflectionTriggerConfig{
			TimeThreshold:           24 * time.Hour,
			EventCountThreshold:     2,
			RelationChangeThreshold: 3,
			AnomalyScoreThreshold:   0.7,
		}),
		WithReflectionRunConfig(mindruntime.DefaultReflectionRunConfig()),
		WithReflectionApprovalConfig(mindruntime.ReflectionApprovalConfig{
			MinEvidenceForApproval:   1,
			MinConfidenceForApproval: 0.1,
			MaxBeliefAdjustPerCycle:  5,
			MaxAbstractionsPerCycle:  10,
			RequireManualReview:      false,
			AutoApproveThreshold:     0.8,
		}),
		WithReflectionSupervisorConfig(mindruntime.DefaultSupervisorConfig()),
		WithReflectionOutbox(outbox),
		WithReflectionEvidenceReader(reader),
		WithReflectionNowFunc(func() time.Time { return time.Now().UTC() }),
	)
}

func TestProcessReflectionNotTriggeredWhenBelowThreshold(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	svc := newTestReflectionService(ob, nil)

	now := time.Now().UTC()
	input := ReflectionProcessInput{
		Scope:        InteractionScope{CharacterID: "char1", ConversationID: "conv1"},
		Observation:  &decision.Observation{ID: "obs-1", Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now},
		GoalProgress: decision.GoalProgressBatchResult{},
		Continuation: decision.ContinuationDecision{},
		Now:          now,
	}

	result, err := svc.ProcessReflection(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Triggered {
		t.Fatal("expected not triggered with 1 event and threshold 2")
	}
	if ob.Count() != 0 {
		t.Errorf("expected 0 outbox events, got %d", ob.Count())
	}
}

func TestProcessReflectionTriggeredAfterTwoEvents(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	reader := &fakeEvidenceReader{
		memories: []mindruntime.VerifiedMemory{
			{ID: "mem-1", Topic: "test_topic", Content: "test content", CreatedAt: time.Now()},
			{ID: "mem-2", Topic: "test_topic", Content: "test content 2", CreatedAt: time.Now()},
		},
	}
	svc := newTestReflectionService(ob, reader)

	now := time.Now().UTC()
	scope := InteractionScope{CharacterID: "char1", ConversationID: "conv1"}

	input1 := ReflectionProcessInput{
		Scope:        scope,
		Observation:  &decision.Observation{ID: "obs-1", Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now},
		GoalProgress: decision.GoalProgressBatchResult{},
		Continuation: decision.ContinuationDecision{},
		Now:          now,
	}
	_, _ = svc.ProcessReflection(context.Background(), input1)

	input2 := ReflectionProcessInput{
		Scope:        scope,
		Observation:  &decision.Observation{ID: "obs-2", Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeFailed, ObservedAt: now.Add(time.Second)},
		GoalProgress: decision.GoalProgressBatchResult{},
		Continuation: decision.ContinuationDecision{},
		Now:          now.Add(time.Second),
	}

	result, err := svc.ProcessReflection(context.Background(), input2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Triggered {
		t.Fatal("expected triggered after two events")
	}
	if result.CandidateID == "" {
		t.Fatal("expected candidate ID")
	}
	if !result.Significant {
		t.Fatal("expected significant candidate with external memory evidence")
	}
}

func TestReflectionDuplicateObservationDedup(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	svc := newTestReflectionService(ob, nil)

	now := time.Now().UTC()
	scope := InteractionScope{CharacterID: "char1", ConversationID: "conv1"}
	obs := &decision.Observation{ID: "obs-dup", Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now}

	input := ReflectionProcessInput{
		Scope:        scope,
		Observation:  obs,
		GoalProgress: decision.GoalProgressBatchResult{},
		Continuation: decision.ContinuationDecision{},
		Now:          now,
	}

	r1, _ := svc.ProcessReflection(context.Background(), input)
	r2, _ := svc.ProcessReflection(context.Background(), input)
	if r1.Triggered && r2.Triggered {
		t.Fatal("duplicate observation should not increase event count twice")
	}
}

func TestReflectionContextCancelled(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	svc := newTestReflectionService(ob, nil)

	now := time.Now().UTC()
	input := ReflectionProcessInput{
		Scope:        InteractionScope{CharacterID: "char1"},
		Observation:  &decision.Observation{ID: "obs-ctx", Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now},
		GoalProgress: decision.GoalProgressBatchResult{},
		Continuation: decision.ContinuationDecision{},
		Now:          now,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := svc.ProcessReflection(ctx, input)
	if err != nil {
		t.Fatalf("expected nil error for cancelled context, got %v", err)
	}
	if result.Triggered {
		t.Fatal("expected not triggered when context cancelled")
	}
}

func TestReflectionCandidateIDDeterministic(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	reader := &fakeEvidenceReader{
		memories: []mindruntime.VerifiedMemory{
			{ID: "mem-1", Topic: "topic_a", Content: "c1", CreatedAt: time.Now()},
			{ID: "mem-2", Topic: "topic_a", Content: "c2", CreatedAt: time.Now()},
		},
	}
	svc1 := newTestReflectionService(ob, reader)
	svc2 := newTestReflectionService(ob, reader)

	now := time.Now().UTC()
	scope := InteractionScope{CharacterID: "char1", ConversationID: "conv1"}

	makeInput := func(obsID string, ts time.Time) ReflectionProcessInput {
		return ReflectionProcessInput{
			Scope:        scope,
			Observation:  &decision.Observation{ID: obsID, Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: ts},
			GoalProgress: decision.GoalProgressBatchResult{},
			Continuation: decision.ContinuationDecision{},
			Now:          ts,
		}
	}

	_, _ = svc1.ProcessReflection(context.Background(), makeInput("obs-d1", now))
	r1, _ := svc1.ProcessReflection(context.Background(), makeInput("obs-d2", now.Add(time.Second)))

	_, _ = svc2.ProcessReflection(context.Background(), makeInput("obs-d1", now.Add(2*time.Second)))
	r2, _ := svc2.ProcessReflection(context.Background(), makeInput("obs-d2", now.Add(3*time.Second)))

	if r1.CandidateID != r2.CandidateID {
		t.Errorf("expected deterministic candidate ID, got %s vs %s", r1.CandidateID, r2.CandidateID)
	}
}

func TestReflectionCrossCharacterIsolation(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	svc := newTestReflectionService(ob, nil)

	now := time.Now().UTC()
	scopeA := InteractionScope{CharacterID: "charA", ConversationID: "convA"}
	scopeB := InteractionScope{CharacterID: "charB", ConversationID: "convB"}

	makeInput := func(scope InteractionScope, id string) ReflectionProcessInput {
		return ReflectionProcessInput{
			Scope:        scope,
			Observation:  &decision.Observation{ID: id, Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now},
			GoalProgress: decision.GoalProgressBatchResult{},
			Continuation: decision.ContinuationDecision{},
			Now:          now,
		}
	}

	rA, _ := svc.ProcessReflection(context.Background(), makeInput(scopeA, "obs-a1"))
	rB, _ := svc.ProcessReflection(context.Background(), makeInput(scopeB, "obs-b1"))

	if rA.Triggered || rB.Triggered {
		t.Fatal("expected neither triggered with different chars and 1 event each")
	}
}

func TestReflectionCrossConversationIsolation(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	svc := newTestReflectionService(ob, nil)

	now := time.Now().UTC()
	scope1 := InteractionScope{CharacterID: "char1", ConversationID: "conv1"}
	scope2 := InteractionScope{CharacterID: "char1", ConversationID: "conv2"}

	makeInput := func(scope InteractionScope, id string) ReflectionProcessInput {
		return ReflectionProcessInput{
			Scope:        scope,
			Observation:  &decision.Observation{ID: id, Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now},
			GoalProgress: decision.GoalProgressBatchResult{},
			Continuation: decision.ContinuationDecision{},
			Now:          now,
		}
	}

	r1, _ := svc.ProcessReflection(context.Background(), makeInput(scope1, "obs-1"))
	r2, _ := svc.ProcessReflection(context.Background(), makeInput(scope2, "obs-2"))

	if r1.Triggered || r2.Triggered {
		t.Fatal("expected neither triggered with different conversations and 1 event each")
	}
}

func TestReflectionDoesNotModifyGoal(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	svc := newTestReflectionService(ob, nil)

	now := time.Now().UTC()
	input := ReflectionProcessInput{
		Scope:        InteractionScope{CharacterID: "char1", ConversationID: "conv1"},
		Observation:  &decision.Observation{ID: "obs-nogoal", Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now},
		GoalProgress: decision.GoalProgressBatchResult{},
		Continuation: decision.ContinuationDecision{},
		Now:          now,
	}

	_, err := svc.ProcessReflection(context.Background(), input)
	if err != nil {
		t.Fatalf("should not error on goal-free observation: %v", err)
	}
}

func TestReflectionInsignificantResetsTrigger(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	svc := NewReflectionService(
		WithReflectionTriggerConfig(mindruntime.ReflectionTriggerConfig{
			EventCountThreshold:     1,
			RelationChangeThreshold: 0,
			AnomalyScoreThreshold:   0,
		}),
		WithReflectionRunConfig(mindruntime.ReflectionRunConfig{
			MinEvidenceForAdjustment: 100,
			MinConfidenceForAdopt:    0.99,
			MaxAbstractionsPerRun:    100,
		}),
		WithReflectionApprovalConfig(mindruntime.DefaultReflectionApprovalConfig()),
		WithReflectionSupervisorConfig(mindruntime.DefaultSupervisorConfig()),
		WithReflectionOutbox(ob),
		WithReflectionNowFunc(func() time.Time { return time.Now().UTC() }),
	)

	now := time.Now().UTC()
	input := ReflectionProcessInput{
		Scope:        InteractionScope{CharacterID: "char1"},
		Observation:  &decision.Observation{ID: "obs-insig", Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now},
		GoalProgress: decision.GoalProgressBatchResult{},
		Continuation: decision.ContinuationDecision{},
		Now:          now,
	}

	result, err := svc.ProcessReflection(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Triggered {
		t.Fatal("expected triggered")
	}
	if result.Significant {
		t.Fatal("expected insignificant due to high min evidence threshold")
	}
	if ob.Count() != 0 {
		t.Errorf("expected 0 outbox events for insignificant candidate, got %d", ob.Count())
	}
}

func TestReflectionNoActionNotCounted(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	svc := newTestReflectionService(ob, nil)

	now := time.Now().UTC()
	input := ReflectionProcessInput{
		Scope:        InteractionScope{CharacterID: "char1"},
		Observation:  &decision.Observation{ID: "obs-noaction", Kind: decision.ObservationKindNoAction, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now},
		GoalProgress: decision.GoalProgressBatchResult{},
		Continuation: decision.ContinuationDecision{},
		Now:          now,
	}

	result, err := svc.ProcessReflection(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Triggered {
		t.Fatal("no_action observation should not trigger reflection")
	}
}

func TestReflectionEmptyScopeSkipped(t *testing.T) {
	ob := &fakeReflectionOutbox{}
	svc := newTestReflectionService(ob, nil)

	now := time.Now().UTC()
	input := ReflectionProcessInput{
		Scope:        InteractionScope{},
		Observation:  &decision.Observation{ID: "obs-empty", Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now},
		GoalProgress: decision.GoalProgressBatchResult{},
		Continuation: decision.ContinuationDecision{},
		Now:          now,
	}

	result, err := svc.ProcessReflection(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Triggered {
		t.Fatal("expected not triggered for empty scope")
	}
}
