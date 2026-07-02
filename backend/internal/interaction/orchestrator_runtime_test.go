package interaction

import (
	"context"
	"errors"
	"testing"
	"time"
)

type runtimeCaptureProcessor struct {
	got   *RuntimeAssembly
	calls int
}

func (p *runtimeCaptureProcessor) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	p.calls++
	p.got = req.Runtime
	return &ProcessResponse{
		ConversationID: req.ConversationID,
		Reply:          "ok",
		CharacterID:    req.CharacterID,
		CharacterName:  "角色",
		MessageIDs:     []string{"msg-1"},
		RequestID:      req.RequestID,
	}, nil
}

type runtimePsycheLoader struct{}

func (l runtimePsycheLoader) Name() string           { return "psyche" }
func (l runtimePsycheLoader) IsRequired() bool       { return false }
func (l runtimePsycheLoader) Timeout() time.Duration { return time.Second }
func (l runtimePsycheLoader) CacheKey(scope InteractionScope, version string) string {
	return version + scope.CharacterID
}
func (l runtimePsycheLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	return FieldReady[any](PsycheState{Stress: 0.9, Fatigue: 0.2, Arousal: 0.3}, "psyche", version), nil
}

type runtimeBlockedBeliefLoader struct{}

func (l runtimeBlockedBeliefLoader) Name() string           { return "beliefs" }
func (l runtimeBlockedBeliefLoader) IsRequired() bool       { return false }
func (l runtimeBlockedBeliefLoader) Timeout() time.Duration { return time.Second }
func (l runtimeBlockedBeliefLoader) CacheKey(scope InteractionScope, version string) string {
	return version + scope.CharacterID
}
func (l runtimeBlockedBeliefLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	return FieldReady[any](BeliefSet{Conflict: &BeliefConflict{KeyA: "a", ValueA: "1", KeyB: "b", ValueB: "2", RiskLevel: "blocked"}}, "beliefs", version), nil
}

func TestOrchestratorAssemblesRuntimeBeforeProcessor(t *testing.T) {
	processor := &runtimeCaptureProcessor{}
	outbox := NewInMemoryOutboxStore()
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, NewInMemoryTracker(), outbox)
	registry := NewContextLoaderRegistry()
	registry.Register(runtimePsycheLoader{})
	registry.Register(NewChannelContextLoader())
	orch.SetRuntimePipeline(NewRuntimePipeline(registry, NewPathClassifier(), NewTokenBudgetManager(1200)))
	orch.SetReady(true)

	result, err := orch.Process(context.Background(), &ProcessRequest{
		CharacterID:    "char-runtime",
		ConversationID: "conv-runtime",
		Channel:        "web",
		Source:         "web",
		Message:        "我今天很焦虑，需要你认真听我说",
		RequestID:      "req-runtime",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeCompleted {
		t.Fatalf("unexpected outcome: %s", result.Outcome)
	}
	if processor.got == nil {
		t.Fatal("runtime assembly was not passed to processor")
	}
	if processor.got.Path != PathTypeStandard {
		t.Fatalf("expected standard path from emotional high-stress context, got %s", processor.got.Path)
	}
	if processor.got.Safety.Level != "conservative" {
		t.Fatalf("expected conservative safety, got %#v", processor.got.Safety)
	}
	if processor.got.Context.Psyche.Status != LoadStatusReady {
		t.Fatalf("psyche context not ready: %#v", processor.got.Context.Psyche)
	}
	if processor.got.Transaction.Name != TransactionBoundaryAll {
		t.Fatalf("expected all transaction boundary, got %s", processor.got.Transaction.Name)
	}
	records, err := outbox.ListPending()
	if err != nil {
		t.Fatal(err)
	}
	foundRuntimeEvent := false
	for _, record := range records {
		if record.EventType == "interaction.runtime_assembled" {
			foundRuntimeEvent = true
			break
		}
	}
	if !foundRuntimeEvent {
		t.Fatalf("runtime outbox event missing: %#v", records)
	}
}

func TestOrchestratorSafetyBlockedSkipsProcessor(t *testing.T) {
	processor := &runtimeCaptureProcessor{}
	tracker := NewInMemoryTracker()
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, NewInMemoryOutboxStore())
	registry := NewContextLoaderRegistry()
	registry.Register(runtimeBlockedBeliefLoader{})
	orch.SetRuntimePipeline(NewRuntimePipeline(registry, NewPathClassifier(), NewTokenBudgetManager(1200)))
	orch.SetReady(true)

	result, err := orch.Process(context.Background(), &ProcessRequest{
		CharacterID:    "char-safety",
		ConversationID: "conv-safety",
		Channel:        "web",
		Source:         "web",
		Message:        "test",
		RequestID:      "req-safety",
	})
	if err == nil {
		t.Fatal("expected safety blocked error")
	}
	if !errors.Is(err, ErrOrchestratorSafetyBlocked) {
		t.Fatalf("expected ErrOrchestratorSafetyBlocked, got %v", err)
	}
	if processor.calls != 0 {
		t.Fatalf("processor should not be called when safety blocks, got %d", processor.calls)
	}
	if result == nil || result.Outcome != OutcomeFailed {
		t.Fatalf("expected failed result, got %#v", result)
	}
	record, ok, err := tracker.Get(context.Background(), result.InteractionID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("interaction record missing")
	}
	if record.Status != InteractionStatusFailed || record.ErrorCode != "safety_blocked" {
		t.Fatalf("expected failed safety record, got %#v", record)
	}
}
