package plugincontract

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllSourceManifestsUseCanonicalContract(t *testing.T) {
	workspaceRoot := filepath.Join("..", "..", "..", "..", "..")
	manifests := sourceManifestPaths(t, filepath.Join(workspaceRoot, "plugins-builtin"))
	manifests = append(manifests, sourceManifestPaths(t, filepath.Join(workspaceRoot, "plugins-code"))...)
	if len(manifests) == 0 {
		t.Fatal("no source manifests found")
	}
	for _, path := range manifests {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("%s: %v", path, readErr)
			continue
		}
		manifest, parseErr := Parse(data)
		if parseErr != nil {
			t.Errorf("%s: %v", path, parseErr)
			continue
		}
		for _, issue := range manifest.Validate().Issues {
			t.Errorf("%s: %s: %s", path, strings.TrimSpace(issue.Path), issue.Message)
		}
	}
}

func TestBuiltInAndOrdinaryPluginSourcesAreSeparated(t *testing.T) {
	workspaceRoot := filepath.Join("..", "..", "..", "..", "..")
	builtInRoot := filepath.Join(workspaceRoot, "plugins-builtin")
	ordinaryRoot := filepath.Join(workspaceRoot, "plugins-code")
	expectedBuiltIns := map[string]bool{
		"com.amitia/plugin-market":         true,
		"com.amitia/runtime-insights":      true,
		"com.amitia/workspace":             true,
		"com.amitia/chat-import":           true,
		"com.amitia/reminders":             true,
		"com.amitia/storage":               true,
		"com.amitia/chat-logs":             true,
		"com.amitia/memory-hub":            true,
		"com.amitia/character-hub":         true,
		"com.amitia/creative-workshop-hub": true,
		"com.amitia/game-center-hub":       true,
	}
	foundBuiltIns := make(map[string]bool, len(expectedBuiltIns))
	for _, path := range sourceManifestPaths(t, builtInRoot) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		manifest, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		foundBuiltIns[manifest.Plugin.ID] = true
		defaultInstall, _ := manifest.Plugin.Metadata["system.defaultInstall"].(bool)
		defaultEnabled, _ := manifest.Plugin.Metadata["system.defaultEnabled"].(bool)
		if !defaultInstall || !defaultEnabled {
			t.Errorf("%s must declare both built-in default metadata flags", path)
		}
	}
	for extensionID := range expectedBuiltIns {
		if !foundBuiltIns[extensionID] {
			t.Errorf("built-in plugin %s is missing from plugins-builtin", extensionID)
		}
	}
	for extID := range foundBuiltIns {
		if !expectedBuiltIns[extID] {
			t.Errorf("unexpected built-in plugin %s", extID)
		}
	}
	for _, path := range sourceManifestPaths(t, ordinaryRoot) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		manifest, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		defaultInstall, _ := manifest.Plugin.Metadata["system.defaultInstall"].(bool)
		defaultEnabled, _ := manifest.Plugin.Metadata["system.defaultEnabled"].(bool)
		if defaultInstall || defaultEnabled {
			t.Errorf("built-in plugin %s must not live under plugins-code", manifest.Plugin.ID)
		}
	}
}

func sourceManifestPaths(t *testing.T, root string) []string {
	t.Helper()
	var manifests []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case "node_modules", ".package-staging", "dist", "build":
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == "amitia-extension.json" {
			manifests = append(manifests, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan source manifests under %s: %v", root, err)
	}
	return manifests
}
