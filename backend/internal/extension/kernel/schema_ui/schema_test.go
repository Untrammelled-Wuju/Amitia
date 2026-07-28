package schema_ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validDocument(title string) *SchemaUIDocument {
	return &SchemaUIDocument{
		SchemaVersion: SchemaUIVersion,
		Type:          "page",
		Title:         title,
		Children: []SchemaUINode{
			{ID: "root", Type: NodePage},
		},
		Actions: []SchemaUIDeclaredAction{{ActionID: "save", Target: "runtime"}},
	}
}

func TestSchemaRegistrySeparatesContributionsAndInvalidatesGeneration(t *testing.T) {
	cache := NewCompilerCache()
	registry := NewSchemaRegistry(nil, cache)
	if err := registry.RegisterSchemaWithContext("ext-a", "page-a", 1, "zh-CN", "dark", validDocument("A")); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterSchemaWithContext("ext-a", "page-b", 1, "zh-CN", "dark", validDocument("B")); err != nil {
		t.Fatal(err)
	}
	if registry.Size() != 2 || cache.Size() != 2 {
		t.Fatalf("unexpected registry/cache size: %d/%d", registry.Size(), cache.Size())
	}
	if err := registry.RegisterSchemaWithContext("ext-a", "page-a", 2, "zh-CN", "dark", validDocument("A2")); err != nil {
		t.Fatal(err)
	}
	if registry.Size() != 2 || cache.Size() != 2 {
		t.Fatalf("update did not invalidate old cache: %d/%d", registry.Size(), cache.Size())
	}
	if doc, ok := registry.Get("ext-a", "page-a"); !ok || doc.Document.Title != "A2" {
		t.Fatal("updated schema unavailable")
	}
	if !registry.UnregisterSchema("ext-a", "page-a") {
		t.Fatal("schema was not unregistered")
	}
	if _, ok := registry.Get("ext-a", "page-a"); ok {
		t.Fatal("uninstalled schema remained accessible")
	}
}

func TestSchemaRegistryRejectsInvalidAndOversizedResources(t *testing.T) {
	registry := NewSchemaRegistry(nil, nil)
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "invalid.json"), []byte(`{"schemaVersion":"schema-ui/1","type":"page","children":[{"id":"x","type":"script"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := registry.LoadFromPath("ext", "invalid", base, "invalid.json"); err == nil {
		t.Fatal("invalid schema was accepted")
	}
	if err := registry.LoadFromBytes("ext", "large", []byte(strings.Repeat("x", int(DefaultLimits().MaxFileBytes)+1))); err == nil {
		t.Fatal("oversized schema was accepted")
	}
	if err := registry.LoadFromPath("ext", "escape", base, "../invalid.json"); err == nil {
		t.Fatal("path traversal was accepted")
	}
}

func TestActionDispatcherFailsClosedWithoutHandler(t *testing.T) {
	compiled, err := Compile(validDocument("actions"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewActionDispatcher(compiled, nil).Dispatch("save", nil)
	if err == nil || !strings.Contains(err.Error(), "dispatcher unavailable") {
		t.Fatalf("expected dispatcher unavailable, got %v", err)
	}
}
