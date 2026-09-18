package ui_handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type fixedResourceLinkResolver struct {
	fullPath string
	err      error
}

func (r fixedResourceLinkResolver) ResolveResourceLink(string) (string, string, string, error) {
	return "com.example/media", "data", r.fullPath, r.err
}

func TestResolveExtensionBasePathUsesCurrentGeneration(t *testing.T) {
	root := t.TempDir()
	extensionID := "com.example/webui"
	generationID := "generation-current"
	generationPath := filepath.Join(root, "installations", "com.example__webui", "generations", generationID)
	if err := os.MkdirAll(generationPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(generationPath, "manifest.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join(root, "installations", "com.example__webui", "current.json")
	if err := os.WriteFile(currentPath, []byte(`{"generationID":"generation-current"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	handler := &HTTPHandler{extRoot: root}
	if got := handler.resolveExtensionBasePath(extensionID); got != generationPath {
		t.Fatalf("resolveExtensionBasePath() = %q, want %q", got, generationPath)
	}
}

func TestResolveExtensionBasePathFallsBackToLegacyInstall(t *testing.T) {
	root := t.TempDir()
	extensionID := "com.example/legacy"
	legacyPath := filepath.Join(root, "installed", "com.example__legacy", "1.0.0", "bundle")
	if err := os.MkdirAll(legacyPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyPath, "manifest.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	handler := &HTTPHandler{extRoot: root}
	if got := handler.resolveExtensionBasePath(extensionID); got != legacyPath {
		t.Fatalf("resolveExtensionBasePath() = %q, want %q", got, legacyPath)
	}
}

func TestResourceLinkHandlerServesSignedResource(t *testing.T) {
	resourcePath := filepath.Join(t.TempDir(), "sample.png")
	if err := os.WriteFile(resourcePath, []byte{137, 80, 78, 71}, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := &HTTPHandler{resourceLinks: fixedResourceLinkResolver{fullPath: resourcePath}}
	request := httptest.NewRequest(http.MethodGet, "/api/extension/resources/token", nil)
	recorder := httptest.NewRecorder()
	handler.handleResourceLink(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("Content-Type") == "" {
		t.Fatal("expected content type")
	}
	if recorder.Body.Len() != 4 {
		t.Fatalf("expected resource body, got %d bytes", recorder.Body.Len())
	}
}

func TestResourceLinkHandlerRejectsInvalidToken(t *testing.T) {
	handler := &HTTPHandler{resourceLinks: fixedResourceLinkResolver{err: errors.New("invalid token")}}
	request := httptest.NewRequest(http.MethodGet, "/api/extension/resources/token", nil)
	recorder := httptest.NewRecorder()
	handler.handleResourceLink(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}
