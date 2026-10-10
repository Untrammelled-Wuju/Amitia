package interaction

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type recoveringParentProcessor struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (p *recoveringParentProcessor) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	p.calls.Add(1)
	if p.started != nil {
		select {
		case p.started <- struct{}{}:
		default:
		}
	}
	if p.release != nil {
		select {
		case <-p.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &ProcessResponse{ConversationID: req.ConversationID, CharacterID: req.CharacterID, RequestID: req.RequestID, Reply: "verified restored parent"}, nil
}

func setupRecoverableParent(t *testing.T, proc *recoveringParentProcessor) (*Orchestrator, *InMemoryTracker, *InteractionRecord) {
	t.Helper()
	tracker := NewInMemoryTracker()
	record := NewInteractionRecord(InteractionScope{
		SpaceID: "space", CharacterID: "character", ConversationID: "conversation",
		Channel: "web", Source: "web", RequestID: "original-request",
	})
	record.Status = InteractionStatusContextReady
	record.StatusVersion = 3
	if err := tracker.Create(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), proc, tracker, nil)
	orch.SetReady(true)
	return orch, tracker, record
}

func recoverParentInput(rec *InteractionRecord) *ProcessRequest {
	return &ProcessRequest{
		SpaceID: rec.Scope.SpaceID, CharacterID: rec.Scope.CharacterID,
		ConversationID: rec.Scope.ConversationID, Channel: rec.Scope.Channel,
		Source: rec.Scope.Source, RequestID: rec.Scope.RequestID,
		Message: "original persisted request", TurnID: "original-turn",
		ReservedInteractionID: rec.ID, RecoverExistingTurn: true,
	}
}

func TestOrchestratorRestoresExistingParentInteractionWithoutDuplicateRecord(t *testing.T) {
	proc := &recoveringParentProcessor{}
	orch, tracker, original := setupRecoverableParent(t, proc)
	result, err := orch.Process(context.Background(), recoverParentInput(original))
	if err != nil {
		t.Fatal(err)
	}
	if result.InteractionID != original.ID || result.Outcome != OutcomeCompleted || proc.calls.Load() != 1 {
		t.Fatalf("restored parent changed interaction or lost process: %+v calls=%d", result, proc.calls.Load())
	}
	count := 0
	if err := tracker.Range(context.Background(), func(*InteractionRecord) bool { count++; return true }); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("restored parent created %d interactions", count)
	}
	again, err := orch.Process(context.Background(), recoverParentInput(original))
	if err != nil || again.InteractionID != original.ID || proc.calls.Load() != 1 {
		t.Fatalf("completed resumed parent executed again: %+v err=%v", again, err)
	}
}

func TestOrchestratorRejectsDuplicateParentResumeDuringExecution(t *testing.T) {
	proc := &recoveringParentProcessor{started: make(chan struct{}, 1), release: make(chan struct{})}
	orch, _, original := setupRecoverableParent(t, proc)
	resultCh := make(chan error, 1)
	go func() { _, err := orch.Process(context.Background(), recoverParentInput(original)); resultCh <- err }()
	select {
	case <-proc.started:
	case <-time.After(2 * time.Second):
		t.Fatal("restoration never entered original processor")
	}
	_, err := orch.Process(context.Background(), recoverParentInput(original))
	if !errors.Is(err, ErrOrchestratorProcessing) {
		t.Fatalf("simultaneous duplicate was not stopped: %v", err)
	}
	close(proc.release)
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("original recovery did not finish")
	}
	if proc.calls.Load() != 1 {
		t.Fatalf("duplicate restoration ran %d processors", proc.calls.Load())
	}
}

func TestOrchestratorParentRecoveryRequiresOriginalIdentity(t *testing.T) {
	proc := &recoveringParentProcessor{}
	orch, _, original := setupRecoverableParent(t, proc)
	for _, mutate := range []func(*ProcessRequest){
		func(r *ProcessRequest) { r.ReservedInteractionID = "another-interaction" },
		func(r *ProcessRequest) { r.TurnID = "" },
		func(r *ProcessRequest) { r.ConversationID = "another-conversation" },
		func(r *ProcessRequest) { r.RequestID = "another-request" },
	} {
		input := recoverParentInput(original)
		mutate(input)
		if _, err := orch.Process(context.Background(), input); err == nil {
			t.Fatalf("unverified parent identity unexpectedly resumed: %+v", input)
		}
	}
	if proc.calls.Load() != 0 {
		t.Fatal("rejected recovery unexpectedly executed processor")
	}
}

type reconciliationHoldingProcessor struct{}

func (reconciliationHoldingProcessor) ProcessMessageCtx(context.Context, *ProcessRequest) (*ProcessResponse, error) {
	return nil, ErrToolReconciliationRequired
}

func TestOrchestratorKeepsParentCheckpointAfterUnknownToolSideEffect(t *testing.T) {
	proc := &recoveringParentProcessor{}
	orchestrator, tracker, record := setupRecoverableParent(t, proc)
	orchestrator.processor = reconciliationHoldingProcessor{}
	result, err := orchestrator.Process(context.Background(), recoverParentInput(record))
	if !errors.Is(err, ErrToolReconciliationRequired) || result == nil || result.Outcome != OutcomeDeliveryUnknown {
		t.Fatalf("unknown tool side effect was not represented as unknown: result=%+v err=%v", result, err)
	}
	stored, ok, err := tracker.Get(context.Background(), record.ID)
	if err != nil || !ok || stored.Status != InteractionStatusContextReady ||
		stored.CancelReason != "" || stored.CommitID != "" {
		t.Fatalf("recovery interaction was incorrectly finalized: stored=%+v err=%v", stored, err)
	}
}
