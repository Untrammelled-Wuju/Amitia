package interaction

import (
	"context"
	"sync"
	"testing"
	"time"
)

type captureRequestProcessor struct {
	mu  sync.Mutex
	req *ProcessRequest
}

func (p *captureRequestProcessor) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	p.mu.Lock()
	p.req = req
	p.mu.Unlock()
	return &ProcessResponse{
		ConversationID: req.ConversationID,
		Reply:          "ok",
		CharacterID:    req.CharacterID,
		RequestID:      req.RequestID,
	}, nil
}

func TestUnifiedEntryPreservesClientRequestIDAndEnvelope(t *testing.T) {
	processor := &captureRequestProcessor{}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, NewInMemoryTracker(), NewInMemoryOutboxStore())
	orch.SetReady(true)
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil))

	result, err := entry.Handle(context.Background(), &UnifiedEntryRequest{
		Channel:        "web",
		Source:         "web",
		PeerID:         "peer-1",
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		SessionID:      "session-1",
		RequestID:      "request-1",
		Message:        "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Response == nil || result.Response.RequestID != "request-1" {
		t.Fatalf("request id was not returned in response: %#v", result.Response)
	}
	if processor.req == nil {
		t.Fatal("processor was not called")
	}
	if processor.req.RequestID != "request-1" || processor.req.SessionID != "session-1" || processor.req.PeerID != "peer-1" || processor.req.UserID != "user-1" {
		t.Fatalf("entry envelope was not preserved: %#v", processor.req)
	}
	record, ok, err := orch.GetTracker().Get(context.Background(), result.InteractionID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("interaction record was not persisted")
	}
	if record.Scope.RequestID != "request-1" || record.Scope.SessionID != "session-1" || record.Scope.PeerID != "peer-1" || record.Scope.UserID != "user-1" {
		t.Fatalf("persisted scope was not preserved: %#v", record.Scope)
	}
}

func TestUnifiedEntryGeneratesRequestIDWhenMissing(t *testing.T) {
	processor := &captureRequestProcessor{}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, NewInMemoryTracker(), NewInMemoryOutboxStore())
	orch.SetReady(true)
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil))

	result, err := entry.Handle(context.Background(), &UnifiedEntryRequest{
		Channel:        "web",
		Source:         "web",
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Message:        "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if processor.req == nil || processor.req.RequestID == "" {
		t.Fatalf("entry did not generate request id: %#v", processor.req)
	}
	if result.Response == nil || result.Response.RequestID != processor.req.RequestID {
		t.Fatalf("generated request id was not returned: response=%#v req=%#v", result.Response, processor.req)
	}
}

func TestUnifiedEntryBackpressureConfigNormalizesExtremeValues(t *testing.T) {
	processor := &captureRequestProcessor{}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, NewInMemoryTracker(), NewInMemoryOutboxStore())
	orch.SetReady(true)
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil))
	entry.SetBackpressureConfig(BackpressureConfig{
		MaxQueueDepth: -1,
		WarningRatio:  2,
		SheddingRatio: -0.1,
		CooldownBase:  -time.Second,
		CooldownMax:   -time.Second,
	})

	result, err := entry.Handle(context.Background(), &UnifiedEntryRequest{
		Channel:        "web",
		Source:         "web",
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Message:        "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeCompleted {
		t.Fatalf("unexpected outcome: %s", result.Outcome)
	}
	if status := entry.GetBackpressureStatus(); status != BackpressureNormal {
		t.Fatalf("unexpected backpressure status: %s", status)
	}
}

func TestUnifiedEntryBackpressureConcurrentConfigAndHandle(t *testing.T) {
	processor := &captureRequestProcessor{}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, NewInMemoryTracker(), NewInMemoryOutboxStore())
	orch.SetReady(true)
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			entry.SetBackpressureConfig(BackpressureConfig{
				MaxQueueDepth: 1 + i%5,
				WarningRatio:  0.2,
				SheddingRatio: 0.8,
				CooldownBase:  time.Millisecond,
				CooldownMax:   10 * time.Millisecond,
			})
		}(i)
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = entry.Handle(context.Background(), &UnifiedEntryRequest{
				Channel:        "web",
				Source:         "web",
				UserID:         "user-1",
				CharacterID:    "char-1",
				ConversationID: "conv-concurrent",
				Message:        "hello",
				RequestID:      "req-concurrent",
			})
		}(i)
	}
	wg.Wait()
}

func TestOrchestratorNormalizesZeroValueConfig(t *testing.T) {
	processor := &captureRequestProcessor{}
	orch := NewOrchestratorWithStores(OrchestratorConfig{}, processor, NewInMemoryTracker(), NewInMemoryOutboxStore())
	orch.SetReady(true)

	result, err := orch.Process(context.Background(), &ProcessRequest{
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Channel:        "web",
		Source:         "web",
		Message:        "hello",
		RequestID:      "req-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeCompleted {
		t.Fatalf("unexpected outcome: %s", result.Outcome)
	}
}

func TestInteractionRecordActiveCoversAllNonTerminalStates(t *testing.T) {
	activeStatuses := []InteractionStatus{
		InteractionStatusReceived,
		InteractionStatusNormalized,
		InteractionStatusQueued,
		InteractionStatusProcessing,
		InteractionStatusContextReady,
		InteractionStatusDecided,
		InteractionStatusGenerated,
		InteractionStatusCommitted,
		InteractionStatusDeliveryPending,
		InteractionStatusDelivered,
	}
	for _, status := range activeStatuses {
		rec := NewInteractionRecord(InteractionScope{CharacterID: "char", ConversationID: "conv"})
		rec.Status = status
		if !rec.IsActive() {
			t.Fatalf("expected %s to be active", status)
		}
	}
	terminalStatuses := []InteractionStatus{
		InteractionStatusCompleted,
		InteractionStatusSuperseded,
		InteractionStatusCancelled,
		InteractionStatusFailed,
		InteractionStatusInterrupted,
		InteractionStatusArchived,
	}
	for _, status := range terminalStatuses {
		rec := NewInteractionRecord(InteractionScope{CharacterID: "char", ConversationID: "conv"})
		rec.Status = status
		if rec.IsActive() {
			t.Fatalf("expected %s to be terminal", status)
		}
	}
}
