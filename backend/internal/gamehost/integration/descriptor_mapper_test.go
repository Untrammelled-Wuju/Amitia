package integration

import (
	"context"
	"testing"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	gamehostdomain "github.com/u-ai/backend/internal/gamehost/domain"
)

func mapperTestExtension(id kerneldomain.ExtensionID, name string, version kerneldomain.SemanticVersion) kerneldomain.ExtensionDefinition {
	return kerneldomain.ExtensionDefinition{
		ID:      id,
		Name:    kerneldomain.LocalizedText{Default: name},
		Version: version,
		Domain:  kerneldomain.ExtensionDomainGame,
		Modules: []kerneldomain.ModuleDefinition{{
			ID:          "runtime",
			ExtensionID: id,
			Type:        kerneldomain.ModuleTypeService,
			Runtime: &kerneldomain.RuntimeDefinition{
				Type:       kerneldomain.RuntimeTypeService,
				EntryPoint: "bin/runtime",
			},
		}},
	}
}

func TestMapGamePluginContribution(t *testing.T) {
	mapper := NewDefaultGamePluginContributionMapper()
	ctx := context.Background()

	ext := mapperTestExtension("com.example.test", "Test Extension", kerneldomain.SemanticVersion{Major: 1})

	contrib := kerneldomain.ContributionDefinition{
		ID:   "main",
		Kind: kerneldomain.ContributionKindGamePlugin,
		Name: kerneldomain.LocalizedText{Default: "Test Game Plugin"},
		Definition: map[string]any{
			"protocolVersion": "amitia-game-host/1",
			"runtimeModuleId": "runtime",
			"hostFeatures":    []interface{}{"realtime_control", "custom_rpc"},
			"network":         map[string]any{"mode": "none"},
		},
		Metadata: map[string]any{
			"author": "test",
		},
	}

	desc, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedID := gamehostdomain.PluginID("com.example.test/main")
	if desc.ID != expectedID {
		t.Errorf("expected ID %s, got %s", expectedID, desc.ID)
	}

	if desc.Name != "Test Game Plugin" {
		t.Errorf("expected name Test Game Plugin, got %s", desc.Name)
	}
}

func TestMapperPreservesExtensionID(t *testing.T) {
	mapper := NewDefaultGamePluginContributionMapper()
	ctx := context.Background()

	ext := mapperTestExtension("com.example.game", "Example Game", kerneldomain.SemanticVersion{Major: 1, Minor: 2})

	contrib := kerneldomain.ContributionDefinition{
		ID:         "core",
		Kind:       kerneldomain.ContributionKindGamePlugin,
		Name:       kerneldomain.LocalizedText{Default: "Example Game Core"},
		Definition: map[string]any{"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": map[string]any{"mode": "none"}},
	}

	desc, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if desc.ExtensionID != string(ext.ID) {
		t.Errorf("expected ExtensionID %s, got %s", ext.ID, desc.ExtensionID)
	}
}

func TestMapperGeneratesStablePluginID(t *testing.T) {
	mapper := NewDefaultGamePluginContributionMapper()
	ctx := context.Background()

	ext := mapperTestExtension("com.example.stable", "Stable", kerneldomain.SemanticVersion{Major: 1})

	contrib := kerneldomain.ContributionDefinition{
		ID:         "plugin",
		Kind:       kerneldomain.ContributionKindGamePlugin,
		Name:       kerneldomain.LocalizedText{Default: "Plugin"},
		Definition: map[string]any{"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": map[string]any{"mode": "none"}},
	}

	desc1, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err != nil {
		t.Fatalf("first conversion error: %v", err)
	}

	desc2, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err != nil {
		t.Fatalf("second conversion error: %v", err)
	}

	if desc1.ID != desc2.ID {
		t.Errorf("PluginID not stable: %s vs %s", desc1.ID, desc2.ID)
	}
}

func TestMapperPreservesProtocolVersion(t *testing.T) {
	mapper := NewDefaultGamePluginContributionMapper()
	ctx := context.Background()

	ext := mapperTestExtension("com.example.protocol", "Protocol", kerneldomain.SemanticVersion{Major: 1})

	contrib := kerneldomain.ContributionDefinition{
		ID:         "main",
		Kind:       kerneldomain.ContributionKindGamePlugin,
		Name:       kerneldomain.LocalizedText{Default: "Main"},
		Definition: map[string]any{"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": map[string]any{"mode": "none"}},
	}

	desc, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if desc.ProtocolVersion != "amitia-game-host/1" {
		t.Errorf("expected protocol version amitia-game-host/1, got %s", desc.ProtocolVersion)
	}
}

func TestMapperPreservesHostFeatures(t *testing.T) {
	mapper := NewDefaultGamePluginContributionMapper()
	ctx := context.Background()

	ext := mapperTestExtension("com.example.cap", "Capabilities", kerneldomain.SemanticVersion{Major: 1})

	contrib := kerneldomain.ContributionDefinition{
		ID:   "main",
		Kind: kerneldomain.ContributionKindGamePlugin,
		Name: kerneldomain.LocalizedText{Default: "Main"},
		Definition: map[string]any{
			"protocolVersion": "amitia-game-host/1",
			"runtimeModuleId": "runtime",
			"hostFeatures":    []interface{}{"realtime_control", "custom_rpc"},
			"network":         map[string]any{"mode": "none"},
		},
	}

	desc, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCaps := map[gamehostdomain.Capability]struct{}{
		"realtime_control": {},
		"custom_rpc":       {},
	}

	if len(desc.Capabilities) != len(expectedCaps) {
		t.Fatalf("expected %d capabilities, got %d", len(expectedCaps), len(desc.Capabilities))
	}

	for _, cap := range desc.Capabilities {
		if _, ok := expectedCaps[cap]; !ok {
			t.Errorf("unexpected capability: %s", cap)
		}
	}
}

func TestMapperRejectsInvalidDescriptor(t *testing.T) {
	mapper := NewDefaultGamePluginContributionMapper()
	ctx := context.Background()

	ext := mapperTestExtension("", "Invalid", kerneldomain.SemanticVersion{Major: 1}) // 非法空ID

	contrib := kerneldomain.ContributionDefinition{
		ID:         "main",
		Kind:       kerneldomain.ContributionKindGamePlugin,
		Name:       kerneldomain.LocalizedText{Default: "Main"},
		Definition: map[string]any{"protocolVersion": "amitia-game-host/1", "runtimeModuleId": "runtime", "network": map[string]any{"mode": "none"}},
	}

	_, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err == nil {
		t.Error("expected error for invalid extension, got nil")
	}
}

func TestMapperSupportsServicesOnlyMultiServiceSpec(t *testing.T) {
	mapper := NewDefaultGamePluginContributionMapper()
	ctx := context.Background()
	ext := mapperTestExtension("com.example.multi-service", "Multi Service", kerneldomain.SemanticVersion{Major: 1})
	ext.Modules = append(ext.Modules, kerneldomain.ModuleDefinition{
		ID:          "events-runtime",
		ExtensionID: ext.ID,
		Type:        kerneldomain.ModuleTypeService,
		Runtime: &kerneldomain.RuntimeDefinition{
			Type:       kerneldomain.RuntimeTypeService,
			EntryPoint: "bin/events-runtime",
		},
	})
	contrib := kerneldomain.ContributionDefinition{
		ID:   "main",
		Kind: kerneldomain.ContributionKindGamePlugin,
		Name: kerneldomain.LocalizedText{Default: "Main"},
		Definition: map[string]any{
			"protocolVersion": "amitia-game-host/1",
			"hostFeatures":    []any{"multi_service"},
			"services": []any{
				map[string]any{"id": "control", "moduleId": "runtime", "required": true},
				map[string]any{"id": "events", "moduleId": "events-runtime", "dependsOn": []any{"control"}},
			},
			"network": map[string]any{"mode": "none"},
		},
	}

	desc, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err != nil {
		t.Fatalf("ToDescriptor: %v", err)
	}
	if len(desc.Services) != 2 {
		t.Fatalf("services=%d want 2", len(desc.Services))
	}
	modules := map[gamehostdomain.ServiceID]string{}
	for _, service := range desc.Services {
		modules[service.ID] = service.Metadata["moduleId"]
	}
	if modules["control"] != "runtime" || modules["events"] != "events-runtime" {
		t.Fatalf("service module bindings=%v", modules)
	}
	if got := desc.Metadata["runtimeModuleId"]; got != "" {
		t.Fatalf("services-only spec should not synthesize legacy runtimeModuleId, got %q", got)
	}
}
