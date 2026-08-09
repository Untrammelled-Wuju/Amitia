package integration

import (
	"context"
	"testing"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	gamehostdomain "github.com/u-ai/backend/internal/gamehost/domain"
)

func TestMapGamePluginContribution(t *testing.T) {
	mapper := NewDefaultGamePluginContributionMapper()
	ctx := context.Background()

	ext := kerneldomain.ExtensionDefinition{
		ID:   "com.example.test",
		Name: kerneldomain.LocalizedText{Default: "Test Extension"},
		Version: kerneldomain.SemanticVersion{
			Major: 1,
			Minor: 0,
			Patch: 0,
		},
		Domain: kerneldomain.ExtensionDomainGame,
	}

	contrib := kerneldomain.ContributionDefinition{
		ID:   "main",
		Kind: kerneldomain.ContributionKindGamePlugin,
		Name: kerneldomain.LocalizedText{Default: "Test Game Plugin"},
		Definition: map[string]any{
			"capabilities": []interface{}{"realtime_control", "custom_rpc"},
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

	ext := kerneldomain.ExtensionDefinition{
		ID:   "com.example.minecraft",
		Name: kerneldomain.LocalizedText{Default: "Minecraft"},
		Version: kerneldomain.SemanticVersion{Major: 1, Minor: 2, Patch: 0},
		Domain: kerneldomain.ExtensionDomainGame,
	}

	contrib := kerneldomain.ContributionDefinition{
		ID:   "core",
		Kind: kerneldomain.ContributionKindGamePlugin,
		Name: kerneldomain.LocalizedText{Default: "Minecraft Core"},
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

	ext := kerneldomain.ExtensionDefinition{
		ID:   "com.example.stable",
		Name: kerneldomain.LocalizedText{Default: "Stable"},
		Version: kerneldomain.SemanticVersion{Major: 1},
		Domain: kerneldomain.ExtensionDomainGame,
	}

	contrib := kerneldomain.ContributionDefinition{
		ID:   "plugin",
		Kind: kerneldomain.ContributionKindGamePlugin,
		Name: kerneldomain.LocalizedText{Default: "Plugin"},
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

	ext := kerneldomain.ExtensionDefinition{
		ID:   "com.example.protocol",
		Name: kerneldomain.LocalizedText{Default: "Protocol"},
		Version: kerneldomain.SemanticVersion{Major: 1},
		Domain: kerneldomain.ExtensionDomainGame,
	}

	contrib := kerneldomain.ContributionDefinition{
		ID:   "main",
		Kind: kerneldomain.ContributionKindGamePlugin,
		Name: kerneldomain.LocalizedText{Default: "Main"},
	}

	desc, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if desc.ProtocolVersion != "amitia-game-host/1" {
		t.Errorf("expected protocol version amitia-game-host/1, got %s", desc.ProtocolVersion)
	}
}

func TestMapperPreservesCapabilities(t *testing.T) {
	mapper := NewDefaultGamePluginContributionMapper()
	ctx := context.Background()

	ext := kerneldomain.ExtensionDefinition{
		ID:   "com.example.cap",
		Name: kerneldomain.LocalizedText{Default: "Capabilities"},
		Version: kerneldomain.SemanticVersion{Major: 1},
		Domain: kerneldomain.ExtensionDomainGame,
	}

	contrib := kerneldomain.ContributionDefinition{
		ID:   "main",
		Kind: kerneldomain.ContributionKindGamePlugin,
		Name: kerneldomain.LocalizedText{Default: "Main"},
		Definition: map[string]any{
			"capabilities": []interface{}{"realtime_control", "custom_rpc", "minecraft.pathfinding"},
		},
	}

	desc, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCaps := map[gamehostdomain.Capability]struct{}{
		"realtime_control":     {},
		"custom_rpc":           {},
		"minecraft.pathfinding": {},
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

	ext := kerneldomain.ExtensionDefinition{
		ID:   "", // 非法空ID
		Name: kerneldomain.LocalizedText{Default: "Invalid"},
		Version: kerneldomain.SemanticVersion{Major: 1},
		Domain: kerneldomain.ExtensionDomainGame,
	}

	contrib := kerneldomain.ContributionDefinition{
		ID:   "main",
		Kind: kerneldomain.ContributionKindGamePlugin,
		Name: kerneldomain.LocalizedText{Default: "Main"},
	}

	_, err := mapper.ToDescriptor(ctx, ext, contrib)
	if err == nil {
		t.Error("expected error for invalid extension, got nil")
	}
}