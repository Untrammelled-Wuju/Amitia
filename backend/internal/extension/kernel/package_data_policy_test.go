package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestPackageUpdateBacksUpExtensionDataWhenDeclared(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	definition, err := container.DefinitionRepository.GetExtension(ctx, domain.ExtensionID(installed.ExtensionID), mustVersion(t, installed.Version))
	if err != nil {
		t.Fatal(err)
	}
	if definition.Metadata == nil {
		definition.Metadata = map[string]any{}
	}
	definition.Metadata["data.backup"] = true
	if err := container.DefinitionRepository.PutExtension(ctx, definition); err != nil {
		t.Fatal(err)
	}
	if _, err := container.ExtensionStateStore.Set(ctx, installed.ExtensionID, "main", "state", json.RawMessage(`{"value":1}`)); err != nil {
		t.Fatal(err)
	}
	installPackagePipelineVersion(t, runtime, "1.1.0")
	matches, err := filepath.Glob(filepath.Join(container.ExtRoot, "extension-data", "*", "backups", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one package data backup, got %v", matches)
	}
	payload, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	var exported struct {
		ExtensionID string `json:"extensionId"`
		Entries     []struct {
			ModuleID string          `json:"moduleId"`
			Key      string          `json:"key"`
			Value    json.RawMessage `json:"value"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(payload, &exported); err != nil {
		t.Fatal(err)
	}
	if exported.ExtensionID != installed.ExtensionID || len(exported.Entries) != 1 ||
		exported.Entries[0].ModuleID != "main" || exported.Entries[0].Key != "state" ||
		string(exported.Entries[0].Value) != `{"value":1}` {
		t.Fatalf("unexpected data backup: %+v", exported)
	}
}

func TestPackageUninstallAppliesExtensionDataPolicy(t *testing.T) {
	for _, policy := range []string{"retain", "delete", "export"} {
		t.Run(policy, func(t *testing.T) {
			ctx := context.Background()
			runtime, container := newPackagePipelineRuntime(t)
			installed := installPackagePipelineVersion(t, runtime, "1.0.0")
			definition, err := container.DefinitionRepository.GetExtension(ctx, domain.ExtensionID(installed.ExtensionID), mustVersion(t, installed.Version))
			if err != nil {
				t.Fatal(err)
			}
			if definition.Metadata == nil {
				definition.Metadata = map[string]any{}
			}
			definition.Metadata["data.uninstall"] = policy
			definition.Metadata["data.export"] = policy == "export"
			if err := container.DefinitionRepository.PutExtension(ctx, definition); err != nil {
				t.Fatal(err)
			}
			if _, err := container.ExtensionStateStore.Set(ctx, installed.ExtensionID, "main", "state", json.RawMessage(`{"value":1}`)); err != nil {
				t.Fatal(err)
			}
			if err := runtime.Disable(ctx, installed.ExtensionID); err != nil {
				t.Fatal(err)
			}
			preview, err := runtime.PreviewPackageUninstall(ctx, installed.ExtensionID, "user-1", "global", "")
			if err != nil || !preview.Installable {
				t.Fatalf("uninstall preview failed: %+v %v", preview, err)
			}
			confirmations := map[string]bool{}
			for _, item := range RequiredUninstallConfirmations(preview) {
				confirmations[item] = true
			}
			confirmation, err := runtime.ConfirmPackageUninstall(ctx, ConfirmPackageUninstallRequest{
				ExtensionID: installed.ExtensionID, SpaceID: "user-1", ScopeType: "global", Confirmations: confirmations,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runtime.ExecutePackageUninstall(ctx, ExecutePackageUninstallRequest{
				ExtensionID: installed.ExtensionID, SpaceID: "user-1", ScopeType: "global",
				ConfirmationToken: confirmation.Token, IdempotencyKey: "package-data-policy-" + policy,
			}); err != nil {
				t.Fatal(err)
			}
			_, _, found, err := container.ExtensionStateStore.Get(ctx, installed.ExtensionID, "main", "state")
			if err != nil {
				t.Fatal(err)
			}
			if policy == "retain" && !found {
				t.Fatal("retain policy removed package state")
			}
			if policy != "retain" && found {
				t.Fatal("delete or export policy retained package state")
			}
			if policy == "export" {
				matches, err := filepath.Glob(filepath.Join(container.ExtRoot, "extension-data", "*", "exports", "*.json"))
				if err != nil {
					t.Fatal(err)
				}
				if len(matches) != 1 {
					t.Fatalf("expected one package data export, got %v", matches)
				}
			}
		})
	}
}

func mustVersion(t *testing.T, value string) domain.SemanticVersion {
	t.Helper()
	version, err := domain.ParseVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
