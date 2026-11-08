package capability

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestGameHostToolContributionRegistersCanonicalProvider(t *testing.T) {
	definition := domain.ExtensionDefinition{
		ID: "com.example/game",
		Modules: []domain.ModuleDefinition{{
			ID:          "game-runtime",
			ExtensionID: "com.example/game",
			Type:        domain.ModuleTypeDataOnly,
			Placement:   domain.ModulePlacementDevice,
			Contributions: []domain.ContributionDefinition{{
				ID:          "move-tool",
				ModuleID:    "game-runtime",
				ExtensionID: "com.example/game",
				Kind:        domain.ContributionKindTool,
				Definition: map[string]any{
					"toolId": "worldgame/move",
					"runtime": map[string]any{
						"runtimeType": "game_host",
						"handlerName": "worldgame.command",
						"pluginId":    "com.example/game/game-plugin",
						"serviceId":   "world-game-runtime",
					},
				},
			}},
		}},
	}

	definitions, err := ProviderDefinitionsFromExtension(definition)
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 1 {
		t.Fatalf("expected one synthesized provider, got %d", len(definitions))
	}
	provider := definitions[0]
	if provider.CapabilityID != CapabilityID("com.example/game/worldgame/move") {
		t.Fatalf("unexpected capability id: %s", provider.CapabilityID)
	}
	if provider.Runtime.RuntimeType != RuntimeTypeGameHost || provider.Runtime.HandlerName != "worldgame.command" || provider.Runtime.RuntimeID != "" {
		t.Fatalf("unexpected runtime binding: %+v", provider.Runtime)
	}
	if provider.Runtime.Metadata["pluginId"] != "com.example/game/game-plugin" || provider.Runtime.Metadata["serviceId"] != "world-game-runtime" {
		t.Fatalf("runtime selectors were not preserved: %+v", provider.Runtime.Metadata)
	}
}
