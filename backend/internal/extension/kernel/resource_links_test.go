package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func TestResourceLinkManagerSignsAndResolvesExtensionData(t *testing.T) {
	root := t.TempDir()
	manager, err := NewResourceLinkManager(root)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "data", safeDirectoryName("com.example/media"))
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	resourcePath := filepath.Join(dataDir, "images", "sample.png")
	if err := os.MkdirAll(filepath.Dir(resourcePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resourcePath, []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	link, err := manager.Sign("com.example/media", ResourceLinkScopeData, "images/sample.png")
	if err != nil {
		t.Fatal(err)
	}
	token := filepath.Base(link)
	extensionID, scope, resolved, err := manager.ResolveResourceLink(token)
	if err != nil {
		t.Fatal(err)
	}
	if extensionID != "com.example/media" || scope != ResourceLinkScopeData || resolved != resourcePath {
		t.Fatalf("unexpected resolved link: %s %s %s", extensionID, scope, resolved)
	}
	if _, _, _, err := manager.ResolveResourceLink(token + "x"); err == nil {
		t.Fatal("tampered token must fail")
	}
	if _, err := manager.Sign("com.example/media", ResourceLinkScopeData, "../escape.png"); err == nil {
		t.Fatal("path traversal must fail")
	}
}

func TestResourceLinkHostAPIReturnsSignedURL(t *testing.T) {
	root := t.TempDir()
	manager, err := NewResourceLinkManager(root)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "data", safeDirectoryName("com.example/media"))
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "sample.png"), []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{ExtensionRoot: root, ResourceLinks: manager}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-resource-link",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.example/media"), ModuleID: "runtime"},
		Method:          host_api.MethodResourceLink,
		Version:         1,
		Input:           json.RawMessage(`{"scope":"data","path":"sample.png"}`),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	var output struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatal(err)
	}
	if output.URL == "" {
		t.Fatal("expected signed URL")
	}
	if _, _, _, err := manager.ResolveResourceLink(filepath.Base(output.URL)); err != nil {
		t.Fatalf("resolve returned link: %v", err)
	}
}
