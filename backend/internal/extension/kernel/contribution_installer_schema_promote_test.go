package kernel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

func TestPromoteAdjacentSchema(t *testing.T) {
	root := t.TempDir()
	installationRoot := filepath.Join(root, "installations", "com.example__game")
	generationRoot := filepath.Join(installationRoot, "generations", "gen-1")
	base := filepath.Join(generationRoot, "assets", "ui")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installationRoot, "current.json"), []byte(`{"generationID":"gen-1"}`), 0o644); err != nil {
		t.Fatalf("write current: %v", err)
	}
	if err := os.WriteFile(filepath.Join(generationRoot, "manifest.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(base, "dashboard.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(base, "dashboard.schema.json"), []byte(`{"schemaVersion":"schema-ui/1","children":[]}`), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	if resolved := resolveExtensionBundlePath(root, "com.example/game"); resolved != generationRoot {
		t.Fatalf("unexpected bundle path %q want %q", resolved, generationRoot)
	}
	installer := NewTypedContributionInstaller(&Container{ExtRoot: root})
	definition := &ui_contribution.UIContributionDefinition{
		ExtensionID: "com.example/game",
		Kind:        ui_contribution.UIContributionWebPage,
		Entry: ui_contribution.UIEntryDefinition{
			Type: ui_contribution.SandboxWebRestricted,
			Path: "assets/ui/dashboard.html",
		},
		Sandbox: ui_contribution.UISandboxPolicy{Type: ui_contribution.SandboxWebRestricted, EnableScripts: true},
	}
	installer.promoteAdjacentSchema(definition)
	if definition.Kind != ui_contribution.UIContributionSchemaPage {
		t.Fatalf("expected schema page, got %s", definition.Kind)
	}
	if definition.Entry.SchemaPath != "assets/ui/dashboard.schema.json" {
		t.Fatalf("unexpected schema path %q", definition.Entry.SchemaPath)
	}
	if definition.Sandbox.Type != ui_contribution.SandboxSchemaRenderer || definition.Sandbox.EnableScripts {
		t.Fatalf("unexpected sandbox policy %#v", definition.Sandbox)
	}
	if len(definition.Visibility.Platforms) != 3 {
		t.Fatalf("expected legacy schema promotion to be desktop-only, got %#v", definition.Visibility.Platforms)
	}
}
