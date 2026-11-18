package kernel

import (
	"context"
	"encoding/base64"
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

type fixtureConversationWorkspaceResolver struct {
	workspace ConversationWorkspace
	err       error
}

func (r fixtureConversationWorkspaceResolver) ResolveConversationWorkspace(context.Context, string) (ConversationWorkspace, error) {
	return r.workspace, r.err
}

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

func TestWorkspaceBridgeResolvesDSHSessionFiles(t *testing.T) {
	bridge, mount, root := workspaceBridgeFixture(t)
	content := "one\ntwo\nthree\nfour"
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	bridge.SetConversationWorkspaceResolver(fixtureConversationWorkspaceResolver{
		workspace: ConversationWorkspace{
			ProjectID:   "project-1",
			WorkspaceID: string(mount.ID),
			RootURI:     mount.RootURI,
		},
	})
	ctx := context.Background()
	stat, err := bridge.Read(ctx, json.RawMessage(`{"operation":"file_stat","payload":{"sessionId":"conversation-1","path":"notes.txt"}}`))
	if err != nil {
		t.Fatal(err)
	}
	statValue := stat.(map[string]any)
	expectedPath, err := filepath.EvalSymlinks(filepath.Join(root, "notes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if statValue["absolutePath"] != expectedPath || statValue["bytes"] != int64(len(content)) {
		t.Fatalf("unexpected stat: %+v", statValue)
	}
	page, err := bridge.Read(ctx, json.RawMessage(`{"operation":"file_read_text","payload":{"sessionId":"conversation-1","path":"notes.txt","offset":2,"limit":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	pageValue := page.(map[string]any)
	if pageValue["text"] != "two\nthree" || pageValue["lines"] != 2 || pageValue["eof"] != false {
		t.Fatalf("unexpected text page: %+v", pageValue)
	}
	binary, err := bridge.Read(ctx, json.RawMessage(`{"operation":"file_read_bytes","payload":{"sessionId":"conversation-1","path":"notes.txt","offset":4,"length":3}}`))
	if err != nil {
		t.Fatal(err)
	}
	binaryValue := binary.(map[string]any)
	decoded, err := base64.StdEncoding.DecodeString(binaryValue["dataBase64"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "two" || binaryValue["eof"] != false {
		t.Fatalf("unexpected byte page: %q %+v", decoded, binaryValue)
	}
	rootListing, err := bridge.Read(ctx, json.RawMessage(`{"operation":"file_list","payload":{"sessionId":"conversation-1","path":""}}`))
	if err != nil {
		t.Fatal(err)
	}
	rootValue := rootListing.(map[string]any)
	rootEntries := rootValue["entries"].([]map[string]any)
	if rootValue["path"] != "" || len(rootEntries) != 3 || rootEntries[0]["name"] != "folder" || rootEntries[1]["name"] != "hello.txt" || rootEntries[2]["name"] != "notes.txt" {
		t.Fatalf("unexpected root listing: %+v", rootValue)
	}
	folderListing, err := bridge.Read(ctx, json.RawMessage(`{"operation":"file_list","payload":{"sessionId":"conversation-1","path":"folder"}}`))
	if err != nil {
		t.Fatal(err)
	}
	folderValue := folderListing.(map[string]any)
	if folderValue["path"] != "folder" || len(folderValue["entries"].([]map[string]any)) != 0 {
		t.Fatalf("unexpected folder listing: %+v", folderValue)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "notes.md"), []byte("search target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden.txt"), []byte("hidden target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "ignored.js"), []byte("ignored target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	search, err := bridge.Read(ctx, json.RawMessage(`{"operation":"file_search","payload":{"sessionId":"conversation-1","query":"notes.md","excludedPaths":[],"includeHidden":false,"respectGitignore":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	searchValue := search.(map[string]any)
	searchMatches := searchValue["matches"].([]dshSearchMatch)
	if len(searchMatches) != 2 || searchMatches[0].Path != "src" || searchMatches[0].Type != "directory" || searchMatches[1].Path != "src/notes.md" || searchMatches[1].Type != "file" {
		t.Fatalf("unexpected workspace search: %+v", searchValue)
	}
	ignored, err := bridge.Read(ctx, json.RawMessage(`{"operation":"file_search","payload":{"sessionId":"conversation-1","query":"ignored","excludedPaths":[],"includeHidden":false,"respectGitignore":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := ignored.(map[string]any)["matches"].([]dshSearchMatch); len(got) != 0 {
		t.Fatalf("gitignored workspace file was returned: %+v", got)
	}
	hidden, err := bridge.Read(ctx, json.RawMessage(`{"operation":"file_search","payload":{"sessionId":"conversation-1","query":"hidden","excludedPaths":[],"includeHidden":true,"respectGitignore":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := hidden.(map[string]any)["matches"].([]dshSearchMatch); len(got) != 1 || got[0].Path != ".hidden.txt" {
		t.Fatalf("hidden workspace file search failed: %+v", got)
	}
	if _, err := bridge.Read(ctx, json.RawMessage(`{"operation":"file_stat","payload":{"sessionId":"conversation-1","path":"../outside.txt"}}`)); err == nil {
		t.Fatal("workspace traversal was accepted")
	}
}

func TestWorkspaceBridgePreservesConversationWorkspaceFailure(t *testing.T) {
	bridge, _, _ := workspaceBridgeFixture(t)
	bridge.SetConversationWorkspaceResolver(fixtureConversationWorkspaceResolver{err: errors.New("conversation workspace missing")})
	_, err := bridge.Read(context.Background(), json.RawMessage(`{"operation":"file_stat","payload":{"sessionId":"conversation-1","path":"hello.txt"}}`))
	if err == nil || err.Error() != "conversation workspace missing" {
		t.Fatalf("unexpected error: %v", err)
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
