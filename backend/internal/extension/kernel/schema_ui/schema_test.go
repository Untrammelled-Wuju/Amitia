package schema_ui

import (
	"encoding/json"
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

func TestSchemaDocumentCompatibilityAliasesAndNodeContracts(t *testing.T) {
	raw := []byte(`{
		"version":"schema-ui/1",
		"type":"page",
		"root":{
			"id":"root",
			"type":"page",
			"visibleWhen":[{"field":"host.ready","operator":"eq","value":true}],
			"disabledWhen":[{"field":"runtime.busy","operator":"eq","value":true}],
			"dataSource":{"source":"runtime","path":"status"},
			"children":[{"id":"body","type":"text","props":{"text":"ok"}}]
		},
		"performanceBudget":{"maxRenderTimeMs":50,"maxLayoutCount":10,"maxNodeCount":20,"maxDataFetchCount":2,"maxActionCount":4}
	}`)
	var doc SchemaUIDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(&doc)
	if err != nil {
		t.Fatalf("compat schema failed to compile: %v", err)
	}
	if compiled.Document.effectiveVersion() != SchemaUIVersion {
		t.Fatalf("unexpected effective version: %q", compiled.Document.effectiveVersion())
	}
	root := compiled.NodeIndex["root"]
	if root == nil {
		t.Fatal("root alias was not indexed")
	}
	if len(root.effectiveVisibility()) != 1 || len(root.DisabledWhen) != 1 || root.DataSource == nil {
		t.Fatalf("node contracts were not preserved: %#v", root)
	}
	if compiled.Document.PerformanceBudget == nil || compiled.Document.PerformanceBudget.MaxDataFetchCount != 2 {
		t.Fatalf("performance budget was not preserved: %#v", compiled.Document.PerformanceBudget)
	}
	encoded, err := json.Marshal(compiled.Document)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, expected := range []string{`"version":"schema-ui/1"`, `"root":`, `"visibleWhen":`, `"disabledWhen":`, `"dataSource":`, `"performanceBudget":`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("marshaled document lost %s: %s", expected, text)
		}
	}
}

func TestSchemaValidatorRejectsInvalidNodeDataSourceAndDisabledCondition(t *testing.T) {
	doc := validDocument("invalid contracts")
	doc.Children[0].DataSource = &SchemaUIBinding{Source: "unknown", Path: "bad path"}
	doc.Children[0].DisabledWhen = []UICondition{{Field: "bad field", Operator: "eq", Value: true}}
	result := NewValidator().Validate(doc)
	if result.Valid {
		t.Fatal("invalid node dataSource/disabledWhen were accepted")
	}
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, ErrInvalidBindingSource.Error()) || !strings.Contains(joined, ErrInvalidExpression.Error()) {
		t.Fatalf("expected binding/expression validation errors, got %s", joined)
	}
}
func TestSchemaFormStateBindingAlias(t *testing.T) {
	doc := validDocument("form-state alias")
	doc.Children[0].Bindings = []SchemaUIBinding{{Source: SourceFormState, Path: "profile.name"}}
	compiled, err := Compile(doc)
	if err != nil {
		t.Fatalf("form_state binding alias failed to compile: %v", err)
	}
	value := NewRenderer(compiled, ThemeTokens{}, "zh-CN").Render(map[string]any{
		"form_state": map[string]any{"profile": map[string]any{"name": "Amitia"}},
	})
	if len(value) != 1 || value[0].Props["profile.name"] != "Amitia" {
		t.Fatalf("form_state binding did not resolve: %#v", value)
	}
}
func TestSchemaDocumentDefaultsMatchClientContracts(t *testing.T) {
	raw := []byte(`{"children":[{"id":"root","type":"page"}]}`)
	var doc SchemaUIDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(&doc); err != nil {
		t.Fatalf("missing optional version/type should use client-compatible defaults: %v", err)
	}
	if doc.effectiveVersion() != SchemaUIVersion {
		t.Fatalf("unexpected default version: %q", doc.effectiveVersion())
	}
}
