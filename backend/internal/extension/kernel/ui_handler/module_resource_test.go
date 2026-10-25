package ui_handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/ui_provider"
)

func TestSanitizeUIModulePathWithinExtension(t *testing.T) {
	root := t.TempDir()
	modulePath := filepath.Join(root, "modules", "page", "index.mjs")
	if err := os.MkdirAll(filepath.Dir(modulePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modulePath, []byte("export default {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	clean, err := sanitizeUIModulePath(root, "/modules/page/index.mjs")
	if err != nil {
		t.Fatal(err)
	}
	if clean != "modules/page/index.mjs" {
		t.Fatalf("unexpected clean path %q", clean)
	}
	if mime := uiModuleMIME(clean); mime != "text/javascript" {
		t.Fatalf("unexpected mime %q", mime)
	}
}

func TestSanitizeUIModulePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := sanitizeUIModulePath(root, "/../secret.mjs"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestProviderModuleCapability(t *testing.T) {
	provider := &ui_provider.ProviderDefinition{
		ProviderID: "provider",
		Entries: map[string]ui_provider.Entry{
			"web": {Type: ui_provider.EntryWebModule, Path: "module.mjs"},
		},
	}
	signed := signProviderModuleCapabilities(provider)
	token := signed.Entries["web"].ModuleToken
	if token == "" || !validProviderModuleCapability("provider", token) {
		t.Fatal("signed provider module capability should validate")
	}
	if validProviderModuleCapability("other-provider", token) {
		t.Fatal("provider mismatch accepted")
	}
	if validProviderModuleCapability("provider", token+"x") {
		t.Fatal("tampered capability accepted")
	}
	if provider.Entries["web"].ModuleToken != "" {
		t.Fatal("registry entry was mutated")
	}
}

func TestProviderModuleResourceRequiresCapability(t *testing.T) {
	root := t.TempDir()
	extensionID := "com.example/module"
	generationID := "generation-test"
	generationRoot := filepath.Join(root, "installations", "com.example__module", "generations", generationID)
	if err := os.MkdirAll(filepath.Join(generationRoot, "modules", "page"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(generationRoot, "manifest.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(generationRoot, "modules", "page", "index.mjs"), []byte("export default {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	current := map[string]string{"generationID": generationID}
	currentData, _ := json.Marshal(current)
	if err := os.WriteFile(filepath.Join(root, "installations", "com.example__module", "current.json"), currentData, 0o644); err != nil {
		t.Fatal(err)
	}
	registry := ui_provider.NewRegistry()
	provider := ui_provider.ProviderDefinition{
		ProviderID:  "provider",
		ExtensionID: extensionID,
		Capability:  ui_provider.CapabilityPageProvider,
		Mode:        ui_provider.ModeReplace,
		TrustLevel:  "official",
		Enabled:     true,
		Entries: map[string]ui_provider.Entry{
			"web": {
				Type:        ui_provider.EntryWebModule,
				Path:        "modules/page/index.mjs",
				ContentHash: "sha256:9aeb8d59b4d483ca6298e9a450fbf37dfd2b4c63990135ab1d040d76314087ec",
			},
		},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatal(err)
	}
	handler := NewHTTPHandler(nil, nil, nil)
	handler.SetExtensionRoot(root)
	handler.SetProviderRegistry(registry)
	request := httptest.NewRequest(http.MethodGet, "/ui/provider-module/provider/modules/page/index.mjs", nil)
	recorder := httptest.NewRecorder()
	handler.handleProviderModuleResource(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected capability denial, got %d", recorder.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/ui/provider-module/provider/modules/page/index.mjs", nil)
	request.Header.Set("X-Amitia-Module-Access", providerModuleCapability("provider", time.Now().Add(time.Minute).Unix()))
	recorder = httptest.NewRecorder()
	handler.handleProviderModuleResource(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "export default {}" {
		t.Fatalf("unexpected provider module response: %d %q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("provider module response is not CORS-readable")
	}
}
