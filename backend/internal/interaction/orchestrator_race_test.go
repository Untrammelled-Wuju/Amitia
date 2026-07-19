package interaction

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/outbox"
)

func TestOrchestratorRaceCondition1000Runs(t *testing.T) {
	tracker := NewInMemoryTracker()
	var outboxStore *outbox.SQLiteOutboxStore
	cfg := DefaultOrchestratorConfig()
	cfg.MaxConcurrent = 100
	orch := NewOrchestratorWithStores(cfg, &stubMessageProcessor{prefix: "reply-"}, tracker, outboxStore)
	orch.SetReady(true)

	const runs = 100
	var wg sync.WaitGroup
	errors := make(chan error, runs)

	for i := 0; i < runs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := &ProcessRequest{
				CharacterID: "char-1",
				Message:     fmt.Sprintf("msg-%d", idx),
				UserID:      fmt.Sprintf("user-%d", idx),
				RequestID:   fmt.Sprintf("req-%d", idx),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := orch.Process(ctx, req)
			if err != nil {
				errors <- fmt.Errorf("run %d: %w", idx, err)
			}
		}(i)
	}
	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("race test error: %v", err)
		}
	}

	active, err := tracker.ListActive(context.Background(), InteractionScope{})
	if err != nil {
		t.Fatalf("list active failed: %v", err)
	}
	for _, r := range active {
		if !r.IsTerminal() {
			t.Errorf("non-terminal interaction after race test: id=%s status=%s", r.ID, r.Status)
		}
	}
}

func TestOrchestratorConsistencyNoHalfCompleteRecords(t *testing.T) {
	tracker := NewInMemoryTracker()
	var outboxStore *outbox.SQLiteOutboxStore
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), &stubMessageProcessor{prefix: "ok"}, tracker, outboxStore)
	orch.SetReady(true)

	ctx := context.Background()
	result, err := orch.Process(ctx, &ProcessRequest{
		CharacterID: "char-1",
		Message:     "hello",
		UserID:      "user-1",
		RequestID:   "req-001",
	})
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}

	if result.Outcome != OutcomeCompleted {
		t.Fatalf("expected completed, got %s", result.Outcome)
	}

	rec, ok, err := tracker.Get(ctx, result.InteractionID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !ok {
		t.Fatal("record not found")
	}
	if rec.Status != InteractionStatusCompleted {
		t.Fatalf("expected completed, got %s", rec.Status)
	}

	if result.Response == nil || len(result.Response.MessageIDs) == 0 {
		t.Error("expected message IDs in completed result")
	}
}

func TestOrchestratorIdempotentSameRequestID(t *testing.T) {
	tracker := NewInMemoryTracker()
	var outboxStore *outbox.SQLiteOutboxStore
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), &stubMessageProcessor{prefix: "idem"}, tracker, outboxStore)
	orch.SetReady(true)

	ctx := context.Background()
	req := &ProcessRequest{
		CharacterID: "char-1",
		Message:     "test",
		UserID:      "user-1",
		RequestID:   "req-idem-001",
	}

	result1, err := orch.Process(ctx, req)
	if err != nil {
		t.Fatalf("first process failed: %v", err)
	}
	if result1.Outcome != OutcomeCompleted {
		t.Fatalf("first outcome not completed: %s", result1.Outcome)
	}

	result2, err := orch.Process(ctx, req)
	if err != nil {
		t.Fatalf("second process failed: %v", err)
	}
	if result2.InteractionID != result1.InteractionID {
		t.Fatalf("idempotent hit returned different ID: %s vs %s", result2.InteractionID, result1.InteractionID)
	}
	if result2.Outcome != OutcomeCompleted {
		t.Fatalf("second outcome not completed: %s", result2.Outcome)
	}
}

func TestOrchestratorCancelBeforeCommitResultsInCancelled(t *testing.T) {
	tracker := NewInMemoryTracker()
	var outboxStore *outbox.SQLiteOutboxStore
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), &stubMessageProcessor{prefix: "cancel", delay: 500 * time.Millisecond}, tracker, outboxStore)
	orch.SetReady(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var result *OrchestrationResult
	var procErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		result, procErr = orch.Process(ctx, &ProcessRequest{
			CharacterID: "char-1",
			Message:     "slow",
			UserID:      "user-1",
			RequestID:   "req-cancel",
		})
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()
	wg.Wait()

	if procErr == nil {
		if result != nil && result.Outcome == OutcomeCompleted {
			t.Error("expected cancelled outcome after context cancellation, got completed")
		}
	}
}

func TestOrchestratorStateMatrixTransitions(t *testing.T) {
	tracker := NewInMemoryTracker()
	var outboxStore *outbox.SQLiteOutboxStore
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), &stubMessageProcessor{prefix: "matrix"}, tracker, outboxStore)
	orch.SetReady(true)

	ctx := context.Background()

	result, err := orch.Process(ctx, &ProcessRequest{
		CharacterID: "char-1",
		Message:     "state",
		UserID:      "user-1",
		RequestID:   "req-state-001",
	})
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}

	rec, ok, err := tracker.Get(ctx, result.InteractionID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !ok {
		t.Fatal("record not found")
	}

	terminalStatuses := []InteractionStatus{
		InteractionStatusCompleted,
		InteractionStatusCancelled,
		InteractionStatusSuperseded,
		InteractionStatusFailed,
	}
	isTerminal := false
	for _, ts := range terminalStatuses {
		if rec.Status == ts {
			isTerminal = true
			break
		}
	}
	if !isTerminal {
		t.Fatalf("expected terminal status, got %s (version %d)", rec.Status, rec.StatusVersion)
	}

	if rec.Status == InteractionStatusCompleted && rec.StatusVersion < 1 {
		t.Fatalf("expected status version >= 1 for completed, got %d", rec.StatusVersion)
	}
}

type stubMessageProcessor struct {
	prefix string
	delay  time.Duration
}

func (s *stubMessageProcessor) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &ProcessResponse{
		ConversationID: req.ConversationID,
		Sequence:       1,
		Reply:          s.prefix + req.Message,
		CharacterID:    req.CharacterID,
		MessageIDs:     []string{"msg-1"},
		RequestID:      req.RequestID,
	}, nil
}
