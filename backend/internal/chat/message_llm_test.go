package chat

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	applog "github.com/u-ai/backend/log"
)

func TestInvokeLLMWithToolsTracksReasoningDuration(t *testing.T) {
	svc := &service{}
	svc.llmWithTools = func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		time.Sleep(30 * time.Millisecond)
		return "回复", "思考", nil, 12, nil
	}

	reply, reasoning, forceVoice, tokens, durationMS, err := svc.invokeLLMWithTools(
		context.Background(),
		&ModelConfig{},
		nil,
		applog.TraceFields{},
		nil,
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		nil,
		nil,
		nil,
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "回复" || reasoning != "思考" {
		t.Fatalf("unexpected reply or reasoning: %q %q", reply, reasoning)
	}
	if forceVoice {
		t.Fatal("expected forceVoice to be false")
	}
	if tokens != 12 {
		t.Fatalf("expected 12 tokens, got %d", tokens)
	}
	if durationMS <= 0 {
		t.Fatalf("expected positive reasoning duration, got %d", durationMS)
	}
}
