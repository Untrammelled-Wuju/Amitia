package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extensioncontext"
)

type fakeExtensionContextFacade struct {
	providers []capability.ToolDefinition
	outputs   map[string]kernel.ToolDispatchResult
}

func (f fakeExtensionContextFacade) ListContextProviders(context.Context, string) []capability.ToolDefinition {
	return f.providers
}

func (f fakeExtensionContextFacade) ExecuteTool(
	_ context.Context,
	toolID capability.CapabilityID,
	_ json.RawMessage,
	_ kernel.InvocationScope,
	_ string,
	_ string,
) (kernel.ToolDispatchResult, bool) {
	result, ok := f.outputs[string(toolID)]
	return result, ok
}

func TestKernelExtensionContextProviderAggregatesContributions(t *testing.T) {
	provider := newKernelExtensionContextProvider(fakeExtensionContextFacade{
		providers: []capability.ToolDefinition{
			{
				ID:          "ext/lifestyle/context",
				ExtensionID: "ext/lifestyle",
				ModuleID:    "runtime",
				Metadata: map[string]any{
					"amitia.context.priority": float64(50),
					"amitia.context.key":      "lifestyle",
				},
			},
			{
				ID:          "ext/emotion/context",
				ExtensionID: "ext/emotion",
				ModuleID:    "runtime",
				Metadata: map[string]any{
					"amitia.context.priority": float64(40),
					"amitia.context.key":      "emotion",
				},
			},
		},
		outputs: map[string]kernel.ToolDispatchResult{
			"ext/lifestyle/context": {
				Status: "SUCCESS",
				Output: json.RawMessage(`{"stateLife":{"currentState":"IDLE"}}`),
			},
			"ext/emotion/context": {
				Status: "FAILED",
				Error:  &kernel.ToolDispatchError{Code: "unavailable", Message: "temporarily unavailable"},
			},
		},
	})

	raw, err := provider.Resolve(context.Background(), "chat.realtime.schedule", extensioncontext.Request{
		CharacterID: "char-1",
		At:          time.Now(),
	})
	if err != nil {
		t.Fatalf("resolve context: %v", err)
	}
	snapshot, err := extensioncontext.Decode(raw)
	if err != nil {
		t.Fatalf("decode context: %v", err)
	}
	if len(snapshot.Contributions) != 2 {
		t.Fatalf("expected 2 contributions, got %d", len(snapshot.Contributions))
	}
	if snapshot.Contributions[0].Source != "ext/lifestyle:lifestyle" {
		t.Fatalf("unexpected first source: %s", snapshot.Contributions[0].Source)
	}
	if snapshot.Contributions[1].Error == "" {
		t.Fatal("expected isolated provider error")
	}
}
