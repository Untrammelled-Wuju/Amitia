package kernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

func TestBuildUIContributionOpAllowsWebSandboxRuntimeBinding(t *testing.T) {
	definition := ui_contribution.UIContributionDefinition{
		ContributionID:  "minecraft-dashboard",
		ExtensionID:     "com.amitiax/minecraft",
		ModuleID:        "minecraft-runtime",
		Kind:            ui_contribution.UIContributionWebPage,
		ContractVersion: 1,
		Entry: ui_contribution.UIEntryDefinition{
			Type:        ui_contribution.SandboxWebRestricted,
			Path:        "assets/ui/minecraft-dashboard.html",
			RuntimeID:   "minecraft-runtime",
			ContentHash: "sha256:test",
		},
		Sandbox: ui_contribution.UISandboxPolicy{Type: ui_contribution.SandboxWebRestricted},
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
