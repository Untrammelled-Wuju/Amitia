package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type capabilityOnlyBrowserProvider struct {
	browser.BrowserProvider
	capabilities browser.BrowserCapabilities
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
