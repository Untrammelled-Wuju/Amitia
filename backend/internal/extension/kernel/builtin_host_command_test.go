package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/workspace"
)

func TestHostCommandDirectoryResolution(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	canonicalSub, err := filepath.EvalSymlinks(sub)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "subdir", sub} {
		got, err := resolveHostCommandDirectory(root, value)
		if err != nil {
			t.Fatalf("resolve %q: %v", value, err)
		}
		if got != canonicalRoot && got != canonicalSub {
			t.Fatalf("unexpected working directory: %q", got)
		}
	}
	if got, err := resolveHostCommandDirectory(root, ".."); err != nil || got != filepath.Dir(canonicalRoot) {
		t.Fatalf("parent directory should resolve as an explicit working directory: %q %v", got, err)
	}
	if got, err := resolveHostCommandDirectory(root, filepath.Dir(root)); err != nil || got != filepath.Dir(canonicalRoot) {
		t.Fatalf("absolute directory should not be restricted by the project folder: %q %v", got, err)
	}
	if _, err := resolveHostCommandDirectory(root, "not-created"); err == nil {
		t.Fatal("nonexistent working directory must be rejected")
	}
}

func TestHostCommandOutputIsBounded(t *testing.T) {
	output := &hostCommandOutput{limit: 5}
	if n, err := output.Write([]byte("abcdefgh")); n != 8 || err != nil {
		t.Fatalf("write returned %d, %v", n, err)
	}
	content, truncated := output.Snapshot()
	if content != "abcde" || !truncated {
		t.Fatalf("output = %q, truncated = %v", content, truncated)
	}
}

func TestHostCommandExecutesInBoundWorkspace(t *testing.T) {
	registry := workspace.NewRegistry()
	mount, err := registry.RegisterLocalMount(context.Background(), "test", t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	service := NewBuiltinUtilityService(BuiltinUtilityDeps{Workspace: workspace.NewService(registry, nil)})
	if !service.Supports("execute_host_command") {
		t.Skip("host command execution is unavailable on this platform")
	}
	invocation := capability.ToolInvocationContext{
		ExecContext: &execution.ExecutionContext{WorkspaceID: string(mount.ID)},
	}
	result, err := service.executeHostCommand(context.Background(), json.RawMessage(`{"command":"echo amitia_host_command_ok","timeoutMs":30000}`), invocation)
	if err != nil {
		t.Fatalf("host command execution failed: %v", err)
	}
	if !strings.Contains(string(result), "amitia_host_command_ok") {
		t.Fatalf("unexpected command output: %s", result)
	}
	if _, err := service.executeHostCommand(context.Background(), json.RawMessage(`{"command":"echo explicit_parent","cwd":".."}`), invocation); err != nil {
		t.Fatalf("explicit parent working directory failed: %v", err)
	}
}

func TestHostCommandAcceptsExplicitCwdWithoutWorkspace(t *testing.T) {
	service := NewBuiltinUtilityService(BuiltinUtilityDeps{})
	if !service.Supports("execute_host_command") {
		t.Skip("host command execution is unavailable on this platform")
	}
	directory := t.TempDir()
	input, _ := json.Marshal(map[string]any{"command": "echo explicit_cwd_ok", "cwd": directory, "timeoutMs": 30000})
	result, err := service.executeHostCommand(context.Background(), input, capability.ToolInvocationContext{})
	if err != nil {
		t.Fatalf("explicit cwd failed: %v", err)
	}
	if !strings.Contains(string(result), "explicit_cwd_ok") {
		t.Fatalf("unexpected command output: %s", result)
	}
}
