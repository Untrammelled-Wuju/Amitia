package interaction

import (
	"context"
	"testing"
	"time"
)

func TestOrchestratorWithoutDeadlineAllowsLongProcessing(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	if cfg.DefaultTimeout != 0 {
		t.Fatalf("expected no default deadline, got %v", cfg.DefaultTimeout)
	}

	orch := NewOrchestrator(cfg, &stubMessageProcessor{prefix: "ok-", delay: 80 * time.Millisecond}, nil)
	orch.SetReady(true)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := orch.Process(ctx, &ProcessRequest{
		SpaceID:        "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		RequestID:      "request-long-running",
		Message:        "hello",
	})
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if result == nil || result.Outcome != OutcomeCompleted || result.Response == nil {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Response.Reply != "ok-hello" {
		t.Fatalf("unexpected reply: %q", result.Response.Reply)
	}
}
