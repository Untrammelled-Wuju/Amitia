package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestListContextProvidersFiltersAndOrdersByPriority(t *testing.T) {
	registry := capability.NewToolRegistry()
	contextSchema := json.RawMessage(`{"type":"object","additionalProperties":true}`)
	definitions := []capability.ToolDefinition{
		{
			ID:           "com.example/weather/context",
			ModelName:    "weather_context",
			ExtensionID:  "com.example/weather",
			Enabled:      true,
			InputSchema:  contextSchema,
			OutputSchema: contextSchema,
			Metadata: map[string]any{
				contextProviderSlotsMetadataKey:    []any{"chat.realtime.schedule"},
				contextProviderPriorityMetadataKey: float64(20),
				contextProviderKeyMetadataKey:      "weather",
			},
		},
		{
			ID:           "com.example/lifestyle/context",
			ModelName:    "lifestyle_context",
			ExtensionID:  "com.example/lifestyle",
			Enabled:      true,
			Internal:     true,
			InputSchema:  contextSchema,
			OutputSchema: contextSchema,
			Metadata: map[string]any{
				contextProviderSlotsMetadataKey:    []string{"chat.realtime.schedule"},
				contextProviderPriorityMetadataKey: float64(230),
				contextProviderKeyMetadataKey:      "lifestyle",
			},
		},
		{
			ID:           "com.example/unrelated/context",
			ModelName:    "unrelated_context",
			ExtensionID:  "com.example/unrelated",
			Enabled:      true,
			InputSchema:  contextSchema,
			OutputSchema: contextSchema,
			Metadata: map[string]any{
				contextProviderSlotsMetadataKey: []string{"other.context"},
			},
		},
	}
	if err := registry.BatchRegister(context.Background(), definitions); err != nil {
		t.Fatalf("register tools: %v", err)
	}

	facade := NewToolFacade(registry, nil)
	providers := facade.ListContextProviders(context.Background(), "chat.realtime.schedule")
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	if providers[0].ExtensionID != "com.example/lifestyle" {
		t.Fatalf("expected lifestyle provider first, got %s", providers[0].ExtensionID)
	}
	if ContextProviderSource(providers[0]) != "com.example/lifestyle:lifestyle" {
		t.Fatalf("unexpected provider source: %s", ContextProviderSource(providers[0]))
	}
}
