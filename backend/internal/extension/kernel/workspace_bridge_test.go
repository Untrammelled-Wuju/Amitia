package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/workspace"
	workspacegit "github.com/u-ai/backend/internal/workspace/git"
)

func workspaceBridgeFixture(t *testing.T) (*WorkspaceBridge, workspace.WorkspaceMount, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello workspace"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry := workspace.NewRegistry()
	if err := registry.RegisterBackend(workspace.WorkspaceKindLocal, workspace.NewLocalBackend(t.TempDir())); err != nil {
		t.Fatal(err)
	}
	service := workspace.NewService(registry, nil)
	mount, err := service.RegisterLocalMount(context.Background(), "fixture", root, false)
	if err != nil {
		t.Fatal(err)
	}
	engine := workspacegit.NewFakeGitEngine(t.TempDir())
	if err := engine.Init(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	controller := workspacegit.NewGitController(
		service,
		registry,
		engine,
		workspacegit.DefaultGitPolicy,
		workspacegit.NewAmitiaDataRootResolver(t.TempDir()),
	)
	bridge := NewWorkspaceBridge(service)
	bridge.SetGitHandler(workspacegit.NewGitHandler(controller))
	return bridge, mount, root
}

func TestWorkspaceBridgeReadsBrowsesAndWrites(t *testing.T) {
	bridge, mount, root := workspaceBridgeFixture(t)
	ctx := context.Background()

	mounts, err := bridge.Read(ctx, json.RawMessage(`{"operation":"list_mounts"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := mounts.([]workspace.WorkspaceMount); len(got) != 1 || got[0].ID != mount.ID {
		t.Fatalf("unexpected mounts: %+v", mounts)
	}

	browsed, err := bridge.Read(ctx, json.RawMessage(`{"operation":"browse","payload":{"uri":"`+mount.RootURI+`hello.txt"}}`))
	if err != nil {
		t.Fatal(err)
	}
	page := browsed.(map[string]any)
	if page["kind"] != workspace.WorkspaceEntryTypeFile || page["content"] != "hello workspace" {
		t.Fatalf("unexpected browse result: %+v", page)
	}

	if _, err := bridge.Write(ctx, json.RawMessage(`{"operation":"write_file","payload":{"uri":"`+mount.RootURI+`created.txt","content":"created","overwrite":true,"atomic":true}}`)); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "created.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "created" {
		t.Fatalf("unexpected file content %q", content)
	}
}

func TestWorkspaceBridgeRejectsGitMutationOnReadOnlyMount(t *testing.T) {
	root := t.TempDir()
	registry := workspace.NewRegistry()
	if err := registry.RegisterBackend(workspace.WorkspaceKindLocal, workspace.NewLocalBackend(t.TempDir())); err != nil {
		t.Fatal(err)
	}
	service := workspace.NewService(registry, nil)
	mount, err := service.RegisterLocalMount(context.Background(), "readonly", root, true)
	if err != nil {
		t.Fatal(err)
	}
	engine := workspacegit.NewFakeGitEngine(t.TempDir())
	if err := engine.Init(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	controller := workspacegit.NewGitController(
		service,
		registry,
		engine,
		workspacegit.DefaultGitPolicy,
		workspacegit.NewAmitiaDataRootResolver(t.TempDir()),
	)
	bridge := NewWorkspaceBridge(service)
	bridge.SetGitHandler(workspacegit.NewGitHandler(controller))
	_, err = bridge.Write(context.Background(), json.RawMessage(`{"operation":"git_add","payload":{"workspaceUri":"`+mount.RootURI+`","paths":["hello.txt"]}}`))
	if !errors.Is(err, workspace.ErrReadOnly) {
		t.Fatalf("git mutation error=%v, want read-only", err)
	}
}

func TestHostWorkspaceRoutesUseBridge(t *testing.T) {
	bridge, mount, _ := workspaceBridgeFixture(t)
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{Workspace: bridge}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID: "workspace-read-1",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID:  "runtime-1",
			ExtensionID: domain.ExtensionID("com.amitia/workspace"),
			ModuleID:    "workspace-runtime",
		},
		Method:  host_api.MethodWorkspaceRead,
		Version: 1,
		Input:   json.RawMessage(`{"operation":"list_mounts"}`),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("workspace route failed: %s %+v", result.Status, result.Error)
	}
	var mounts []workspace.WorkspaceMount
	if err := json.Unmarshal(result.Output, &mounts); err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 1 || mounts[0].ID != mount.ID {
		t.Fatalf("unexpected workspace route output: %+v", mounts)
	}
}
