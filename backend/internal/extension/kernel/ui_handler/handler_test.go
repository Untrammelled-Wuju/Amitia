package ui_handler

import (
	"os"
	"path/filepath"
	"testing"
)

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
