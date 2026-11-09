package ui_handler

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
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

func TestPageModuleResourceRequiresMatchingPageCapability(t *testing.T) {
	root := t.TempDir()
	extensionID := "com.example/page-module"
	pageID := "page"
	generationID := "generation-page"
	generationRoot := filepath.Join(root, "installations", "com.example__page-module", "generations", generationID)
	moduleRoot := filepath.Join(generationRoot, "modules", "page")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("export default { template: '<main>page</main>' }")
	modulePath := filepath.Join(moduleRoot, "index.mjs")
	if err := os.WriteFile(modulePath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(generationRoot, "manifest.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	currentData, _ := json.Marshal(map[string]string{"generationID": generationID})
	if err := os.WriteFile(filepath.Join(root, "installations", "com.example__page-module", "current.json"), currentData, 0o644); err != nil {
		t.Fatal(err)
	}
	pageHost := extension_page_host.NewPageHost(extension_page_host.NewPageRegistry(), extension_page_host.NewSessionManager())
	def := extension_page_host.NewExtensionPageDefinition(extension_page_host.PageRegistrationInput{
		PageID:          extension_page_host.PageID(pageID),
		ExtensionID:     extension_page_host.ExtensionID(extensionID),
		ModuleID:        "page-runtime",
		ContributionID:  extension_page_host.ContributionID(pageID),
		Generation:      1,
		ContractVersion: 1,
		EntryKind:       extension_page_host.PageKindWeb,
		EntryPath:       "modules/page/index.mjs",
		EntryHash:       "sha256-" + base64.StdEncoding.EncodeToString(sha256Sum(content)),
	})
	if err := pageHost.RegisterPage(t.Context(), def); err != nil {
		t.Fatal(err)
	}
	handler := NewHTTPHandler(nil, nil, pageHost)
	handler.SetExtensionRoot(root)
	token := pageModuleCapability(extensionID, pageID, time.Now().Add(time.Minute).Unix())
	request := httptest.NewRequest(http.MethodGet, "/ui/page-module/com.example%2Fpage-module/page/modules/page/index.mjs?accessToken="+token, nil)
	recorder := httptest.NewRecorder()
	handler.handlePageModuleResource(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != string(content) {
		t.Fatalf("unexpected page module response: %d %q", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/ui/page-module/com.example%2Fpage-module/other/modules/page/index.mjs?accessToken="+token, nil)
	recorder = httptest.NewRecorder()
	handler.handlePageModuleResource(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("page mismatch should be forbidden, got %d", recorder.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/ui/page-module/com.example%2Fpage-module/page/modules/other/index.mjs?accessToken="+token, nil)
	recorder = httptest.NewRecorder()
	handler.handlePageModuleResource(recorder, request)
	if recorder.Code != http.StatusForbidden && recorder.Code != http.StatusBadRequest {
		t.Fatalf("outside module directory should be rejected, got %d", recorder.Code)
	}
}

func TestPageOpenResponseAddsModuleCapability(t *testing.T) {
	result := pageOpenResultForResponse(&extension_page_host.OpenPageResult{
		Definition: &extension_page_host.ExtensionPageSpec{
			PageID:    "main",
			EntryKind: extension_page_host.PageKindWeb,
			EntryPath: "modules/page/index.mjs",
		},
	}, "com.example/page")
	if result.Definition.ModuleToken == "" || !validPageModuleCapability("com.example/page", "main", result.Definition.ModuleToken) {
		t.Fatal("page response did not receive a valid module capability")
	}
	schema := pageOpenResultForResponse(&extension_page_host.OpenPageResult{
		Definition: &extension_page_host.ExtensionPageSpec{
			PageID:    "schema",
			EntryKind: extension_page_host.PageKindSchema,
		},
	}, "com.example/page")
	if schema.Definition.ModuleToken != "" {
		t.Fatal("schema page should not receive a client module capability")
	}
}

func sha256Sum(value []byte) []byte {
	sum := sha256.Sum256(value)
	return sum[:]
}
