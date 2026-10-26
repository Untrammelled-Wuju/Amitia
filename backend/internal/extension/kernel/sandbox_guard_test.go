package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func sandboxExecContext(root string) *execution.ExecutionContext {
	return &execution.ExecutionContext{Sandbox: &execution.SandboxConstraint{Root: root, AllowNetwork: true}}
}

func sandboxInvocation(root string) capability.ToolInvocationContext {
	return capability.ToolInvocationContext{ExecContext: sandboxExecContext(root)}
}

func TestEnsureSandboxPathKeepsInsideRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "notes", "today.txt")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	execCtx := sandboxExecContext(root)
	if err := ensureSandboxPath(execCtx, inside); err != nil {
		t.Fatalf("path inside sandbox rejected: %v", err)
	}
	if err := ensureSandboxPath(execCtx, filepath.Join(root, "missing", "new.txt")); err != nil {
		t.Fatalf("new file inside sandbox rejected: %v", err)
	}
}

func TestEnsureSandboxPathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	execCtx := sandboxExecContext(root)
	if err := ensureSandboxPath(execCtx, filepath.Join(outside, "secret.txt")); !errors.Is(err, ErrSandboxEscape) {
		t.Fatalf("expected SANDBOX_ESCAPE for outside path, got %v", err)
	}
	if err := ensureSandboxPath(execCtx, filepath.Join(root, "..", "escape.txt")); !errors.Is(err, ErrSandboxEscape) {
		t.Fatalf("expected SANDBOX_ESCAPE for traversal path, got %v", err)
	}
}

func TestEnsureSandboxPathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	execCtx := sandboxExecContext(root)
	if err := ensureSandboxPath(execCtx, link); !errors.Is(err, ErrSandboxEscape) {
		t.Fatalf("expected SANDBOX_ESCAPE for symlinked path, got %v", err)
	}
}

func TestEnsureSandboxPathIgnoresUnsandboxedExecution(t *testing.T) {
	if err := ensureSandboxPath(nil, "/etc/hosts"); err != nil {
		t.Fatalf("nil execution context must not be restricted: %v", err)
	}
	if err := ensureSandboxPath(&execution.ExecutionContext{}, "/etc/hosts"); err != nil {
		t.Fatalf("execution without sandbox must not be restricted: %v", err)
	}
}

func TestDispatchRejectsEscapeToolsInsideSandbox(t *testing.T) {
	service := &BuiltinUtilityService{}
	invocation := sandboxInvocation(t.TempDir())
	for name := range sandboxDeniedTools {
		if _, err := service.Dispatch(context.Background(), name, json.RawMessage(`{}`), invocation); !errors.Is(err, ErrSandboxEscape) {
			t.Fatalf("tool %s must be rejected inside a sandbox, got %v", name, err)
		}
	}
}

func TestDirectLocalPathRejectsHomeExpansionInsideSandbox(t *testing.T) {
	root := t.TempDir()
	_, _, err := directLocalPath(map[string]any{"path": "~/secret.txt"}, sandboxInvocation(root))
	if !errors.Is(err, ErrSandboxEscape) {
		t.Fatalf("expected SANDBOX_ESCAPE for home expansion, got %v", err)
	}
}

func TestDirectLocalPathAllowsSandboxFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "note.txt")
	if err := os.WriteFile(target, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, direct, err := directLocalPath(map[string]any{"path": target}, sandboxInvocation(root))
	if err != nil {
		t.Fatalf("sandbox file rejected: %v", err)
	}
	if !direct || resolved != target {
		t.Fatalf("unexpected resolution direct=%v path=%q", direct, resolved)
	}
}

func TestHostCommandRunsInsideSandbox(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin seatbelt sandbox")
	}
	root := t.TempDir()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	service := &BuiltinUtilityService{}
	invocation := sandboxInvocation(root)

	inside, err := service.Dispatch(context.Background(), "execute_host_command", json.RawMessage(`{"command":"echo inside-ok > note.txt && cat note.txt"}`), invocation)
	if err != nil {
		t.Fatalf("sandboxed command failed: %v", err)
	}
	if !strings.Contains(string(inside), "inside-ok") {
		t.Fatalf("unexpected command output: %s", inside)
	}
	if _, err := os.Stat(filepath.Join(root, "note.txt")); err != nil {
		t.Fatalf("command did not write inside sandbox: %v", err)
	}

	outside, err := service.Dispatch(context.Background(), "execute_host_command", json.RawMessage(`{"command":"cat `+filepath.Join(home, "Library", "Preferences")+` 2>&1 | head -1"}`), invocation)
	if err != nil && !strings.Contains(err.Error(), "sandbox") {
		t.Logf("outside read returned error: %v", err)
	}
	if err == nil && !strings.Contains(string(outside), "Operation not permitted") && !strings.Contains(string(outside), "Permission denied") {
		t.Fatalf("host read inside sandbox was not blocked: %s", outside)
	}

	if _, escapeErr := service.Dispatch(context.Background(), "execute_host_command", json.RawMessage(`{"command":"echo leak > `+filepath.Join(home, "amitia-sandbox-escape.txt")+`"}`), invocation); escapeErr != nil {
		t.Logf("outside write errored as expected: %v", escapeErr)
	}
	if _, statErr := os.Stat(filepath.Join(home, "amitia-sandbox-escape.txt")); statErr == nil {
		os.Remove(filepath.Join(home, "amitia-sandbox-escape.txt"))
		t.Fatal("sandboxed command wrote outside the sandbox root")
	}
}
