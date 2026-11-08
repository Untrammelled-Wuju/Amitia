package kernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
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
	definitionData, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	definitionMap := map[string]any{}
	if err := json.Unmarshal(definitionData, &definitionMap); err != nil {
		t.Fatal(err)
	}
	installer := &TypedContributionInstaller{container: &Container{UIHost: ui_contribution.NewUIHost()}}
	contribution := domain.ContributionDefinition{
		ID:          "minecraft-dashboard",
		ExtensionID: "com.amitiax/minecraft",
		ModuleID:    "minecraft-runtime",
		Kind:        domain.ContributionKind("com.example.market/page"),
		Adapter:     domain.ContributionAdapterUI,
		Definition:  definitionMap,
	}
	op, err := installer.buildInstallOp(context.Background(), contribution, 1)
	if err != nil {
		t.Fatal(err)
	}
	if op.kind != contribution.Kind || op.doInstall == nil {
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

func TestBuildUIContributionOpAllowsInlineSchema(t *testing.T) {
	definition := ui_contribution.UIContributionDefinition{
		ContributionID: "inline-page", ExtensionID: "com.example/inline", ModuleID: "inline-ui",
		Kind: ui_contribution.UIContributionSchemaPage, ContractVersion: 1,
		Entry: ui_contribution.UIEntryDefinition{
			Type: ui_contribution.SandboxSchemaRenderer, ContentHash: "inline-v1",
		},
		Schema:  json.RawMessage(`{"schemaVersion":"schema-ui/1","type":"document","children":[{"id":"page","type":"page","children":[{"id":"title","type":"text","props":{"text":"Inline"}}]}]}`),
		Sandbox: ui_contribution.UISandboxPolicy{Type: ui_contribution.SandboxSchemaRenderer},
		Integrity: ui_contribution.ContributionIntegrity{
			DefinitionHash: "inline-v1",
		},
	}
	data, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	installer := &TypedContributionInstaller{container: &Container{
		UIHost: ui_contribution.NewUIHost(), SchemaValidator: schema_ui.NewValidator(),
		SchemaRegistry: schema_ui.NewSchemaRegistry(schema_ui.NewValidator(), nil),
	}}
	op, err := installer.buildUIContributionOp(context.Background(), domain.ContributionDefinition{
		ID: "inline-page", ExtensionID: "com.example/inline", ModuleID: "inline-ui",
		Kind: domain.ContributionKind("com.example/inline-page"), Adapter: domain.ContributionAdapterUI,
	}, data, 1)
	if err != nil {
		t.Fatal(err)
	}
	if op.kind != domain.ContributionKind("com.example/inline-page") || op.doInstall == nil {
		t.Fatalf("unexpected inline schema operation: %+v", op)
	}
}

func TestBuildUIContributionOpIsIdempotentForIdenticalRegistration(t *testing.T) {
	definition := ui_contribution.UIContributionDefinition{
		ContributionID: "stable-panel", ExtensionID: "com.example/stable", ModuleID: "stable-ui",
		Kind: ui_contribution.UIContributionPanel, ContractVersion: 1,
		Slot: ui_contribution.UISlotReference{SlotID: "provider.test.panel", ContractVersion: 1},
		Entry: ui_contribution.UIEntryDefinition{Type: ui_contribution.SandboxSchemaRenderer, ContentHash: "inline-v1"},
		Schema: json.RawMessage(`{"schemaVersion":"schema-ui/1","type":"document","children":[{"id":"title","type":"text","props":{"text":"Stable"}}]}`),
		Sandbox: ui_contribution.UISandboxPolicy{Type: ui_contribution.SandboxSchemaRenderer},
		Integrity: ui_contribution.ContributionIntegrity{DefinitionHash: "inline-v1"},
	}
	data, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	host := ui_contribution.NewUIHost()
	if err := host.RegisterContribution(&definition); err != nil {
		t.Fatal(err)
	}
	installer := &TypedContributionInstaller{container: &Container{
		UIHost: host, SchemaValidator: schema_ui.NewValidator(),
		SchemaRegistry: schema_ui.NewSchemaRegistry(schema_ui.NewValidator(), nil),
	}}
	op, err := installer.buildUIContributionOp(context.Background(), domain.ContributionDefinition{
		ID: "stable-panel", ExtensionID: "com.example/stable", ModuleID: "stable-ui",
		Kind: domain.ContributionKindUIPage, Adapter: domain.ContributionAdapterUI,
	}, data, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := op.doInstall(context.Background()); err != nil {
		t.Fatalf("identical registration must be idempotent: %v", err)
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
