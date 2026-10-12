package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func TestPackageSwapRestartsRuntimeAndKeepsExtensionEnabled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/swap-runtime"
	fakeRT := runtime_supervisor.NewFakeRuntime()
	rt, container, _ := setupEnableTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	if err := rt.Enable(ctx, extID); err != nil {
		t.Fatalf("Enable must succeed: %v", err)
	}
	if !fakeRT.IsStarted() {
		t.Fatal("runtime must run after enable")
	}

	if err := rt.quiesceExtensionForPackageSwap(ctx, domain.ExtensionID(extID), runtime_supervisor.StopReasonUpdate); err != nil {
		t.Fatalf("quiesce must succeed: %v", err)
	}
	if fakeRT.IsStarted() {
		t.Fatal("package swap must stop the previous runtime instance")
	}
	if fakeRT.StopCount() != 1 {
		t.Fatalf("expected one runtime stop, got %d", fakeRT.StopCount())
	}
	definitionID := runtime_supervisor.BuildRuntimeDefinitionID(extID, "main", domain.RuntimeTypeGo)
	for _, instance := range container.RuntimeSupervisor.Snapshot(ctx, definitionID).Instances {
		if instance.Actual == runtime_supervisor.ActualReady {
			t.Fatalf("runtime instance %s must not stay ready across the package swap", instance.InstanceID)
		}
	}

	if err := rt.restoreExtensionAfterPackageSwap(ctx, domain.ExtensionID(extID), true); err != nil {
		t.Fatalf("restore must succeed: %v", err)
	}
	if !fakeRT.IsStarted() {
		t.Fatal("package swap must start the new runtime instance")
	}
	if fakeRT.StartCount() < 2 {
		t.Fatalf("expected a fresh runtime start, got %d", fakeRT.StartCount())
	}
	inst, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if err != nil {
		t.Fatalf("GetInstallation must succeed: %v", err)
	}
	if inst.EnablementState != domain.EnablementEnabled {
		t.Fatalf("update must preserve the enabled state, got %s", inst.EnablementState)
	}

	if err := rt.quiesceExtensionForPackageSwap(ctx, domain.ExtensionID(extID), runtime_supervisor.StopReasonRollback); err != nil {
		t.Fatalf("quiesce must succeed: %v", err)
	}
	if err := rt.restoreExtensionAfterPackageSwap(ctx, domain.ExtensionID(extID), false); err != nil {
		t.Fatalf("restore for a disabled extension must succeed: %v", err)
	}
	if fakeRT.IsStarted() {
		t.Fatal("a disabled extension must not start a runtime after the swap")
	}
}

func TestActivationReloadsSchemaWhenContentHashChanges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	extRoot := filepath.Join(tempDir, "extensions")
	extID := "com.amitia.test/uipage"
	installRoot := filepath.Join(extRoot, "installations", "com.amitia.test__uipage")
	schemaPath := "modules/uipage-ui/desktop/schema.json"
	writeSchemaGeneration(t, installRoot, "gen-1", schemaPath, "页面 v1")

	container, err := NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	contrib := schemaPageContribution(extID, "uipage-ui", "uipage-page", schemaPath, "sha256-old")
	if err := container.ContributionRepository.PutContribution(ctx, contrib); err != nil {
		t.Fatalf("PutContribution must succeed: %v", err)
	}
	if err := container.ContributionInstaller.InstallContributions(ctx, []domain.ContributionDefinition{contrib}, 1); err != nil {
		t.Fatalf("InstallContributions must succeed: %v", err)
	}
	assertRegisteredSchemaTitle(t, container, extID, "uipage-page", "页面 v1")

	writeSchemaGeneration(t, installRoot, "gen-2", schemaPath, "页面 v2")
	contrib = schemaPageContribution(extID, "uipage-ui", "uipage-page", schemaPath, "sha256-new")
	if err := container.ContributionRepository.PutContribution(ctx, contrib); err != nil {
		t.Fatalf("PutContribution must succeed: %v", err)
	}
	if err := container.ContributionInstaller.ActivateContributions(ctx, domain.ExtensionID(extID)); err != nil {
		t.Fatalf("ActivateContributions must succeed: %v", err)
	}
	assertRegisteredSchemaTitle(t, container, extID, "uipage-page", "页面 v2")
}

func schemaPageContribution(extensionID, moduleID, contributionID, schemaPath, contentHash string) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID:          domain.ContributionID(contributionID),
		ExtensionID: domain.ExtensionID(extensionID),
		ModuleID:    domain.ModuleID(moduleID),
		Kind:        domain.ContributionKindUIPage,
		Definition: map[string]any{
			"contract_version": 1,
			"contribution_id":  contributionID,
			"extension_id":     extensionID,
			"module_id":        moduleID,
			"kind":             "schema_page",
			"slot":             map[string]any{"slot_id": "provider.page.provider", "contract_version": 1},
			"display":          map[string]any{"title": map[string]any{"default": "UI Page"}},
			"entry":            map[string]any{"type": "schema_renderer", "schema_path": schemaPath, "content_hash": contentHash},
			"sandbox":          map[string]any{"type": "schema_renderer"},
			"visibility":       map[string]any{"platforms": []any{"windows"}},
			"actions":          []any{map[string]any{"action_id": "status", "title": map[string]any{"default": "Status"}, "risk_level": "low", "target": map[string]any{"type": "host_command", "command": "channel.status"}}},
			"integrity":        map[string]any{"definition_hash": contributionID + "-v1"},
		},
	}
}

func writeSchemaGeneration(t *testing.T, installRoot, generationID, schemaPath, title string) {
	t.Helper()
	generationRoot := filepath.Join(installRoot, "generations", generationID)
	schemaFile := filepath.Join(generationRoot, filepath.FromSlash(schemaPath))
	if err := os.MkdirAll(filepath.Dir(schemaFile), 0o755); err != nil {
		t.Fatalf("mkdir schema dir: %v", err)
	}
	document := map[string]any{
		"schemaVersion": "schema-ui/1",
		"type":          "page",
		"title":         title,
		"children":      []any{map[string]any{"id": "root", "type": "page"}},
		"actions":       []any{map[string]any{"actionId": "status", "target": "host_command"}},
	}
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	if err := os.WriteFile(schemaFile, payload, 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(generationRoot, "manifest.json"), []byte(`{"extension":{"id":"com.amitia.test/uipage"}}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	current := map[string]any{"generationID": generationID, "treeHash": "sha256-" + generationID}
	currentPayload, err := json.Marshal(current)
	if err != nil {
		t.Fatalf("marshal current pointer: %v", err)
	}
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatalf("mkdir install root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installRoot, "current.json"), currentPayload, 0o644); err != nil {
		t.Fatalf("write current pointer: %v", err)
	}
}

func assertRegisteredSchemaTitle(t *testing.T, container *Container, extensionID, contributionID, want string) {
	t.Helper()
	compiled, ok := container.SchemaRegistry.Get(extensionID, contributionID)
	if !ok || compiled == nil || compiled.Document == nil {
		t.Fatalf("schema %s/%s is not registered", extensionID, contributionID)
	}
	if !strings.Contains(compiled.Document.Title, want) {
		t.Fatalf("expected schema title %q, got %q", want, compiled.Document.Title)
	}
}
