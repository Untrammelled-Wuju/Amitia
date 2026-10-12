package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/workspace"
)

func TestContainerBuilderRegistersExecutableWorkspaceProvider(t *testing.T) {
	root := t.TempDir()
	registry := workspace.NewRegistry()
	if err := registry.RegisterBackend(workspace.WorkspaceKindLocal, workspace.NewLocalBackend(root)); err != nil {
		t.Fatal(err)
	}
	mount, err := registry.RegisterLocalMount(context.Background(), "test-root", root, false)
	if err != nil {
		t.Fatal(err)
	}
	container, err := NewContainerBuilder().
		WithDBPath(filepath.Join(root, "kernel.db")).
		WithExtensionRoot(filepath.Join(root, "extensions")).
		WithWorkspaceService(workspace.NewService(registry, nil)).
		WithRuntimeIdentity(runtimeidentity.Identity{SpaceID: "realm", DeviceID: "local-device", RuntimeID: "local-runtime"}).
		Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })

	instances := container.CapabilityProviders.ListInstancesByCapability("workspace.write")
	if len(instances) == 0 {
		t.Fatal("workspace.write provider instance was not registered")
	}
	executable := 0
	for _, instance := range instances {
		if instance.IsExecutable() {
			executable++
		}
	}
	if executable == 0 {
		t.Fatalf("workspace.write has no executable provider instance: %+v", instances)
	}

	result, err := container.CapabilityService.Resolve(capability.CapabilityResolutionRequest{
		CapabilityID:       "workspace.write",
		SpaceID:            "realm",
		RequiredPlacement:  capability.ProviderPlacementCore,
		PreferredPlacement: capability.ProviderPlacementCore,
		AllowCore:          true,
		AllowDevice:        false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasResult() || result.ExecutionTarget.Placement != "core" {
		t.Fatalf("workspace.write did not resolve to an executable core provider: %+v", result)
	}
	if err := container.ContributionInstaller.ActivateContributions(context.Background(), "com.amitia.builtin.workspace"); err != nil {
		t.Fatal(err)
	}
	if _, err := container.Store.DB().ExecContext(context.Background(), `INSERT INTO kernel_device_owned_resources(owner_id, kind, resource_id, role_id, source_id, revision, deleted, body, updated_at) VALUES(?, 'conversation', ?, ?, '', 1, 0, '{}', ?)`, "space", "conversation", "character", "2026-10-10T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	execCtx := execution.NewExecutionContext("root", "realm")
	execCtx.ConversationID = "conversation"
	execCtx.PrimaryWorkspaceID = string(mount.ID)
	execCtx.WorkspaceID = string(mount.ID)
	execCtx.WorkspaceBindings = []execution.WorkspaceBinding{{
		ID:       string(mount.ID),
		DeviceID: "local-device",
		Name:     mount.Name,
		Kind:     string(mount.Kind),
		RootURI:  mount.RootURI,
	}}
	execCtx.RuntimeTarget = &execution.RuntimeTarget{Placement: "device", DeviceID: "local-device"}
	execCtx.Metadata = map[string]any{"ownedDeviceTarget": true}
	dispatch, found := container.ToolFacade.ExecuteModelTool(context.Background(), "workspace.write", json.RawMessage(`{"uri":"e2e-proof.txt","text":"AMITIA_E2E_OK","overwrite":true}`), InvocationScope{
		SpaceID:        "realm",
		DeviceID:       "local-device",
		ConversationID: "conversation",
		PermissionMode: "full_access",
		ExecContext:    &execCtx,
	}, "")
	if !found {
		t.Fatal("workspace.write tool not found")
	}
	if dispatch.Status != "success" || dispatch.Error != nil {
		t.Fatalf("workspace.write failed: status=%s error=%+v output=%s", dispatch.Status, dispatch.Error, string(dispatch.Output))
	}
	content, err := os.ReadFile(filepath.Join(root, "e2e-proof.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "AMITIA_E2E_OK" {
		t.Fatalf("unexpected file content: %q", string(content))
	}
}
