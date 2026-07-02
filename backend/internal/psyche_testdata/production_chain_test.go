package psyche_testdata

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
)

type productionCaseProcessor struct {
	mu       sync.Mutex
	seen     map[string]*interaction.ProcessRequest
	runtimes map[string]*interaction.RuntimeAssembly
}

func (p *productionCaseProcessor) ProcessMessageCtx(ctx context.Context, req *interaction.ProcessRequest) (*interaction.ProcessResponse, error) {
	p.mu.Lock()
	if p.seen == nil {
		p.seen = map[string]*interaction.ProcessRequest{}
	}
	if p.runtimes == nil {
		p.runtimes = map[string]*interaction.RuntimeAssembly{}
	}
	p.seen[req.RequestID] = req
	p.runtimes[req.RequestID] = req.Runtime
	p.mu.Unlock()
	return &interaction.ProcessResponse{
		ConversationID: req.ConversationID,
		Reply:          "ok:" + req.RequestID,
		CharacterID:    req.CharacterID,
		RequestID:      req.RequestID,
		MessageIDs:     []string{"msg-" + req.RequestID},
	}, nil
}

func TestAllCasesEnterUnifiedProductionChain(t *testing.T) {
	cases, err := LoadCases(DefaultCasesPath())
	if err != nil {
		t.Fatal(err)
	}
	processor := &productionCaseProcessor{}
	tracker := interaction.NewInMemoryTracker()
	outbox := interaction.NewInMemoryOutboxStore()
	orch := interaction.NewOrchestratorWithStores(interaction.DefaultOrchestratorConfig(), processor, tracker, outbox)
	orch.SetReady(true)
	entry := interaction.NewUnifiedEntry(orch, interaction.NewScopeResolver(nil))

	for i, c := range cases {
		requestID := fmt.Sprintf("%04d-%s", i, c.ID)
		req := productionUnifiedEntryRequest(c, requestID)
		result, err := entry.Handle(context.Background(), req)
		if err != nil {
			t.Fatalf("%s failed unified entry: %v", c.ID, err)
		}
		if result.Outcome != interaction.OutcomeCompleted {
			t.Fatalf("%s outcome = %s", c.ID, result.Outcome)
		}
		if result.Response == nil || result.Response.RequestID != requestID {
			t.Fatalf("%s response did not preserve request id: %#v", c.ID, result.Response)
		}
		record, ok, err := tracker.Get(context.Background(), result.InteractionID)
		if err != nil {
			t.Fatalf("%s tracker get failed: %v", c.ID, err)
		}
		if !ok || record.Status != interaction.InteractionStatusCompleted {
			t.Fatalf("%s did not persist completed interaction: %#v", c.ID, record)
		}
		if len(result.Events) != 3 {
			t.Fatalf("%s expected three outbox events, got %d", c.ID, len(result.Events))
		}
	}

	if len(processor.seen) != len(cases) {
		t.Fatalf("processor handled %d cases, want %d", len(processor.seen), len(cases))
	}
	for i, c := range cases {
		requestID := fmt.Sprintf("%04d-%s", i, c.ID)
		req := processor.seen[requestID]
		if req == nil {
			t.Fatalf("%s did not reach processor", c.ID)
		}
		if req.Runtime == nil || processor.runtimes[requestID] == nil {
			t.Fatalf("%s did not receive runtime assembly", c.ID)
		}
		if req.Runtime.Delivery.Channel == "" || req.Runtime.Transaction.Name == "" {
			t.Fatalf("%s runtime missing delivery or transaction: %#v", c.ID, req.Runtime)
		}
	}
	pending, err := outbox.ListPending()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != len(cases)*3 {
		t.Fatalf("pending outbox events = %d, want %d", len(pending), len(cases)*3)
	}
}

func productionUnifiedEntryRequest(c Case, requestID string) *interaction.UnifiedEntryRequest {
	channel := stringFromCaseInput(c, "channel", "web")
	message := stringFromCaseInput(c, "content", c.ID)
	eventType := stringFromCaseInput(c, "type", "text")
	return &interaction.UnifiedEntryRequest{
		Channel:        channel,
		Source:         channel,
		UserID:         "user-" + c.ID,
		CharacterID:    "char-" + c.Category,
		ConversationID: "conv-" + c.ID,
		PeerID:         "peer-" + c.ID,
		SessionID:      "session-" + c.Category,
		RequestID:      requestID,
		Message:        message,
		VoiceMessage:   eventType == "voice" || eventType == "audio",
	}
}

func stringFromCaseInput(c Case, key, fallback string) string {
	if value, ok := c.InputEvent[key]; ok && value != nil {
		text := fmt.Sprint(value)
		if text != "" {
			return text
		}
	}
	return fallback
}
