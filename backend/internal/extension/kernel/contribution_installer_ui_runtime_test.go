package kernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/ui_provider"
)

func TestBuildUIContributionOpAllowsWebSandboxRuntimeBinding(t *testing.T) {
	definition := ui_contribution.UIContributionDefinition{
		ContributionID:  "minecraft-dashboard",
		ExtensionID:     "com.amitiax/minecraft",
		ModuleID:        "minecraft-runtime",
		Kind:            ui_contribution.UIContributionWebPage,
		ContractVersion: 1,
		Entry: ui_contribution.UIEntryDefinition{
			Type:        ui_contribution.SandboxClientModule,
			Path:        "assets/ui/minecraft-dashboard.html",
			RuntimeID:   "minecraft-runtime",
			ContentHash: "sha256:test",
		},
		Sandbox: ui_contribution.UISandboxPolicy{Type: ui_contribution.SandboxClientModule},
		Integrity: ui_contribution.ContributionIntegrity{
			DefinitionHash: "sha256:test",
		},
	}
	defData, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	installer := &TypedContributionInstaller{container: &Container{UIHost: ui_contribution.NewUIHost()}}
	op, err := installer.buildUIContributionOp(context.Background(), domain.ContributionDefinition{
		ID:          "minecraft-dashboard",
		ExtensionID: "com.amitiax/minecraft",
		ModuleID:    "minecraft-runtime",
		Kind:        domain.ContributionKindUIPage,
	}, defData, 1)
	if err != nil {
		t.Fatal(err)
	}
	if op.kind != domain.ContributionKindUIPage || op.doInstall == nil {
		t.Fatalf("unexpected install op: %+v", op)
	}
}

func TestBuildUIContributionOpKeepsHostRuntimeGate(t *testing.T) {
	definition := ui_contribution.UIContributionDefinition{
		ContributionID:  "host-runtime-page",
		ExtensionID:     "com.example/host-runtime",
		ModuleID:        "runtime",
		Kind:            ui_contribution.UIContributionSchemaPage,
		ContractVersion: 1,
		Entry: ui_contribution.UIEntryDefinition{
			Type:        ui_contribution.SandboxHostNative,
			Path:        "builtin://host-runtime",
			RuntimeID:   "minecraft-runtime",
			ContentHash: "builtin",
		},
		Sandbox: ui_contribution.UISandboxPolicy{Type: ui_contribution.SandboxHostNative},
		Integrity: ui_contribution.ContributionIntegrity{
			DefinitionHash: "builtin",
		},
	}
	defData, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	installer := &TypedContributionInstaller{container: &Container{UIHost: ui_contribution.NewUIHost()}}
	_, err = installer.buildUIContributionOp(context.Background(), domain.ContributionDefinition{
		ID:          "host-runtime-page",
		ExtensionID: "com.example/host-runtime",
		ModuleID:    "runtime",
		Kind:        domain.ContributionKindUIPage,
	}, defData, 1)
	if err == nil || !strings.Contains(err.Error(), "is not registered") {
		t.Fatalf("host runtime allowlist must stay enforced, got %v", err)
	}
}

func TestActivateUIProviderReplacesExistingDefinition(t *testing.T) {
	registry := ui_provider.NewRegistry()
	providerID := "weekday-page-provider"
	extensionID := domain.ExtensionID("com.example/weekday")
	initial := ui_provider.ProviderDefinition{
		ProviderID:  providerID,
		ExtensionID: string(extensionID),
		ModuleID:    "weekday-ui",
		Capability:  ui_provider.CapabilityPageProvider,
		Mode:        ui_provider.ModeReplace,
		Placement:   ui_provider.PlacementDevice,
		TrustLevel:  "user_trusted",
		Enabled:     true,
		Entries: map[string]ui_provider.Entry{
			"electron_macos": {
				ContributionID: "weekday-page",
				Type:           ui_provider.EntryWebModule,
				Path:           "modules/weekday-ui/index.mjs",
				ContentHash:    "sha256:old",
			},
		},
	}
	if err := registry.Register(initial); err != nil {
		t.Fatal(err)
	}
	installer := &TypedContributionInstaller{container: &Container{UIProviderRegistry: registry}}
	err := installer.activateUIProvider(context.Background(), domain.ContributionDefinition{
		ID:          domain.ContributionID(providerID),
		ExtensionID: extensionID,
		ModuleID:    domain.ModuleID("weekday-ui"),
		Kind:        domain.ContributionKindUIProvider,
		Definition: map[string]any{
			"providerId":  providerID,
			"extensionId": string(extensionID),
			"moduleId":    "weekday-ui",
			"capability":  "page.provider",
			"mode":        "replace",
			"placement":   "device",
			"trustLevel":  "user_trusted",
			"entries": map[string]any{
				"electron_macos": map[string]any{
					"contributionId": "weekday-page",
					"type":           "web_module",
					"path":           "modules/weekday-ui/index.mjs",
					"contentHash":    "sha256:new",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, ok := registry.Get(providerID)
	if !ok {
		t.Fatal("provider was not registered")
	}
	if got := updated.Entries["electron_macos"].ContentHash; got != "sha256:new" {
		t.Fatalf("provider definition was not replaced, got %q", got)
	}
}
