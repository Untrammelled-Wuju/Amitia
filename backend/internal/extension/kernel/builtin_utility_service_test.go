package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type capabilityOnlyBrowserProvider struct {
	browser.BrowserProvider
	capabilities browser.BrowserCapabilities
}

func TestBuiltinFileToolsUseExplicitPathsWithoutWorkspace(t *testing.T) {
	service := NewBuiltinUtilityService(BuiltinUtilityDeps{})
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	if err := os.WriteFile(source, []byte("first\nsecond\nthird\n"), 0644); err != nil {
		t.Fatal(err)
	}
	listInput, _ := json.Marshal(map[string]any{"uri": root})
	listed, err := service.listFiles(context.Background(), listInput, capability.ToolInvocationContext{})
	if err != nil || !strings.Contains(string(listed), "source.txt") {
		t.Fatalf("listed=%s err=%v", listed, err)
	}
	readInput, _ := json.Marshal(map[string]any{"uri": source, "line_start": 2, "line_end": 2})
	read, err := service.readFilePart(context.Background(), readInput, capability.ToolInvocationContext{})
	if err != nil || !strings.Contains(string(read), "second") {
		t.Fatalf("read=%s err=%v", read, err)
	}
	target := filepath.Join(root, "nested", "result.txt")
	writeInput, _ := json.Marshal(map[string]any{"uri": target, "operation": "write", "content": "workspace-free"})
	if _, err := service.applyFile(context.Background(), writeInput, capability.ToolInvocationContext{}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "workspace-free" {
		t.Fatalf("content=%q err=%v", content, err)
	}
	relativeInput, _ := json.Marshal(map[string]any{"path": "source.txt", "line_start": 1, "line_end": 1})
	relative, err := service.readFilePart(context.Background(), relativeInput, capability.ToolInvocationContext{ExecContext: &execution.ExecutionContext{Metadata: map[string]any{"workingDirectory": root}}})
	if err != nil || !strings.Contains(string(relative), "first") {
		t.Fatalf("relative=%s err=%v", relative, err)
	}
}

func (p capabilityOnlyBrowserProvider) BrowserCapabilities() browser.BrowserCapabilities {
	return p.capabilities
}

func TestBuiltinUtilityBrowserToolsFollowProviderCapabilities(t *testing.T) {
	disabled := NewBuiltinUtilityService(BuiltinUtilityDeps{Browser: browser.NewDisabledProvider()})
	if disabled.Supports("visit_web") || disabled.Supports("browser_close_all") || disabled.Supports("browser_fill_form") {
		t.Fatal("disabled browser provider must not expose browser tools")
	}

	enabledProvider := capabilityOnlyBrowserProvider{
		capabilities: browser.BrowserCapabilities{
			SupportsNavigation:  true,
			SupportsDOM:         true,
			SupportsInteraction: true,
		},
	}
	enabled := NewBuiltinUtilityService(BuiltinUtilityDeps{Browser: enabledProvider})
	if !enabled.Supports("visit_web") || !enabled.Supports("browser_close_all") || !enabled.Supports("browser_fill_form") {
		t.Fatal("capable browser provider must expose browser tools")
	}
}

func TestResolveWorkspaceURIUsesBoundConversationWorkspace(t *testing.T) {
	invocation := capability.ToolInvocationContext{
		ExecContext: &execution.ExecutionContext{
			WorkspaceID: "ws-bound",
			Metadata: map[string]any{
				"workspaceRootUri": "amitia://workspace/@ws-bound/project/",
			},
		},
	}
	uri, err := resolveWorkspaceURI(map[string]any{"filePath": "src/main.go"}, invocation)
	if err != nil {
		t.Fatal(err)
	}
	if uri != "amitia://workspace/@ws-bound/project/src/main.go" {
		t.Fatalf("unexpected bound workspace URI: %s", uri)
	}
	uri, err = resolveWorkspaceURI(map[string]any{"uri": "workspace://README.md"}, invocation)
	if err != nil {
		t.Fatal(err)
	}
	if uri != "amitia://workspace/@ws-bound/project/README.md" {
		t.Fatalf("unexpected workspace URI alias resolution: %s", uri)
	}
}
