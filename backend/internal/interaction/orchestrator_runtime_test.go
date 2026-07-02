package interaction

import (
	"context"
	"testing"
	"time"
)

type runtimeCaptureProcessor struct {
	got *RuntimeAssembly
}

func (p *runtimeCaptureProcessor) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
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
