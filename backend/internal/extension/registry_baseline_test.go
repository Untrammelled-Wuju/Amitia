package extension

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/migration"
)

func noopHandler(_ context.Context, req ExecuteSkillRequest) (SkillResult, error) {
	return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{}`)}, nil
}

func TestLegacy_Registry_NormalRegistration(t *testing.T) {
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	definition, handler := testDefinition(t, "dev.test.reg.normal", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	registered, err := registry.Get(context.Background(), "dev.test.reg.normal")
	if err != nil {
		t.Fatal(err)
	}
	if registered.Definition.ID != "dev.test.reg.normal" {
		t.Fatalf("unexpected ID: %s", registered.Definition.ID)
	}
}

func TestLegacy_Registry_DuplicateID(t *testing.T) {
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	definition, handler := testDefinition(t, "dev.test.reg.dup", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	def2, hdl2 := testDefinition(t, "dev.test.reg.dup", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	err = registry.Register(context.Background(), def2, hdl2)
	assertExtensionErrorCode(t, err, ErrSkillDuplicateID)
}

func TestLegacy_Registry_RegisterThenUnregister(t *testing.T) {
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	definition, handler := testDefinition(t, "dev.test.reg.rm", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	if err := registry.Unregister(context.Background(), "dev.test.reg.rm"); err != nil {
		t.Fatal(err)
	}
	_, err = registry.Get(context.Background(), "dev.test.reg.rm")
	assertExtensionErrorCode(t, err, ErrSkillNotFound)
}

func TestLegacy_Registry_ReRegister(t *testing.T) {
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	definition, handler := testDefinition(t, "dev.test.reg.rereg", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	if err := registry.Unregister(context.Background(), "dev.test.reg.rereg"); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	registered, err := registry.Get(context.Background(), "dev.test.reg.rereg")
	if err != nil {
		t.Fatal(err)
	}
	if registered.Definition.ID != "dev.test.reg.rereg" {
		t.Fatalf("unexpected ID after re-register: %s", registered.Definition.ID)
	}
}

func TestLegacy_Registry_NotFound(t *testing.T) {
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	_, err = registry.Get(context.Background(), "dev.test.reg.nonexistent")
	assertExtensionErrorCode(t, err, ErrSkillNotFound)
}

func TestLegacy_Registry_GetByModelName(t *testing.T) {
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	definition, handler := testDefinition(t, "dev.test.reg.model", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	registered, err := registry.GetByModelName(context.Background(), "dev.test.reg.model")
	if err != nil {
		t.Fatal(err)
	}
	if registered.Definition.ModelName != "dev.test.reg.model" {
		t.Fatalf("unexpected model name: %s", registered.Definition.ModelName)
	}
}

func TestLegacy_Registry_InvalidManifestRejected(t *testing.T) {
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	definition, handler := testDefinition(t, "INVALID_ID", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	err = registry.Register(context.Background(), definition, handler)
	assertExtensionErrorCode(t, err, ErrSkillManifestInvalid)
}

func TestLegacy_Registry_DisabledSkill(t *testing.T) {
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	definition, handler := testDefinition(t, "dev.test.reg.disabled", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	definition.Enabled = false
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetEnabled(context.Background(), "dev.test.reg.disabled", false); err != nil {
		t.Fatal(err)
	}
	registered, err := registry.Get(context.Background(), "dev.test.reg.disabled")
	if err != nil {
		t.Fatal(err)
	}
	if registered.Definition.Enabled {
		t.Fatal("skill should remain disabled")
	}
}

func TestLegacy_Registry_RegisterWithNilHandlerInstructions(t *testing.T) {
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	definition := SkillDefinition{
		ID: "dev.test.reg.instr", ModelName: "dev.test.reg.instr", Name: "test-instr", Description: "test", Version: "1.0.0",
		Source: SkillSourceInstructions, Entry: SkillEntry{Kind: "instructions", Path: "SKILL.md", ArtifactID: "artifact-1"},
		Triggers: []SkillTrigger{TriggerLLM}, Timeout: 100 * time.Millisecond, TimeoutMS: 100, Enabled: true, Compatible: true,
	}
	definition.InputSchema = json.RawMessage(`{"type":"object"}`)
	definition.OutputSchema = json.RawMessage(`{"type":"object"}`)
	definition.ConfigSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)
	definition.DefaultConfig = json.RawMessage(`{}`)
	manifest := Manifest{
		Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill",
		Metadata:      ManifestMetadata{ID: "dev.test.reg.instr", Name: "test-instr", Version: "1.0.0", Description: "test"},
		Compatibility: ManifestCompatibility{EngineMin: "1.0.0", EngineMaxExclusive: "2.0.0"},
		Entry:         SkillEntry{Kind: "instructions", Path: "SKILL.md", ArtifactID: "artifact-1"},
		Capabilities:  []string{}, Triggers: []SkillTrigger{TriggerLLM},
		Execution:   ManifestExecution{TimeoutMS: 100, Idempotent: false},
		InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`),
		ConfigSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`), DefaultConfig: json.RawMessage(`{}`),
		Enabled: true, AllowLLM: true, AllowManual: false,
	}
	raw, _ := json.Marshal(manifest)
	definition.Manifest = raw
	if err := registry.Register(context.Background(), definition, nil); err != nil {
		t.Fatalf("instructions skill should register without handler: %v", err)
	}
	registered, err := registry.Get(context.Background(), "dev.test.reg.instr")
	if err != nil {
		t.Fatal(err)
	}
	if registered.Definition.Entry.Kind != "instructions" {
		t.Fatal("entry kind should be instructions")
	}
}

func TestLegacy_Registry_AvailableFilter(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	def1, hdl1 := testDefinition(t, "dev.test.avail.a", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	def2, hdl2 := testDefinition(t, "dev.test.avail.b", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	def1.Triggers = []SkillTrigger{TriggerLLM}
	def2.Triggers = []SkillTrigger{TriggerManual}
	syncDefinitionManifest(t, &def1)
	syncDefinitionManifest(t, &def2)
	if err := registry.Register(ctx, def1, hdl1); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(ctx, def2, hdl2); err != nil {
		t.Fatal(err)
	}
	available, err := registry.Available(ctx, ExecutionScope{Trigger: TriggerLLM})
	if err != nil {
		t.Fatal(err)
	}
	if len(available) != 1 {
		t.Fatalf("expected 1 available skill for LLM trigger, got %d", len(available))
	}
	if available[0].ID != "dev.test.avail.a" {
		t.Fatalf("expected dev.test.avail.a available, got %s", available[0].ID)
	}
}

func TestLegacy_Registry_ListWithFilter(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	def1, hdl1 := testDefinition(t, "dev.test.list.src1", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	def2, hdl2 := testDefinition(t, "dev.test.list.src2", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	def1.Source = SkillSourceBuiltin
	def2.Source = SkillSourceLegacy
	if err := registry.Register(ctx, def1, hdl1); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(ctx, def2, hdl2); err != nil {
		t.Fatal(err)
	}
	results, err := registry.List(ctx, SkillFilter{Source: SkillSourceBuiltin})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Definition.ID != "dev.test.list.src1" {
		t.Fatalf("expected only builtin skills, got %d", len(results))
	}
}

func TestLegacy_Registry_ScopeEnabledBehavior(t *testing.T) {
	ctx := context.Background()
	db, repository := testRepository(t)
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionScopeBindingsMigration()}); err != nil {
		t.Fatal(err)
	}
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	definition, handler := testDefinition(t, "dev.test.scope.en", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	definition.Triggers = []SkillTrigger{TriggerLLM}
	syncDefinitionManifest(t, &definition)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetScopeEnabled(ctx, "dev.test.scope.en", ExecutionScope{CharacterID: "char-a"}, true); err != nil {
		t.Fatal(err)
	}
	global, err := registry.GetScoped(ctx, "dev.test.scope.en", ExecutionScope{})
	if err != nil {
		t.Fatal(err)
	}
	if !global.Definition.Enabled {
		t.Fatal("skill should be globally enabled by default")
	}
	scoped, err := registry.GetScoped(ctx, "dev.test.scope.en", ExecutionScope{CharacterID: "char-a"})
	if err != nil {
		t.Fatal(err)
	}
	if !scoped.Definition.Enabled || scoped.Definition.EffectiveScopeType != ScopeCharacter {
		t.Fatalf("skill should be enabled for char-a: enabled=%v scope=%s", scoped.Definition.Enabled, scoped.Definition.EffectiveScopeType)
	}
}

func TestLegacy_Registry_GetScopedDisabledFallsBack(t *testing.T) {
	ctx := context.Background()
	db, repository := testRepository(t)
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionScopeBindingsMigration()}); err != nil {
		t.Fatal(err)
	}
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	definition, handler := testDefinition(t, "dev.test.scope.dis", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	syncDefinitionManifest(t, &definition)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetScopeEnabled(ctx, "dev.test.scope.dis", ExecutionScope{CharacterID: "char-dis"}, false); err != nil {
		t.Fatal(err)
	}
	scoped, err := registry.GetScoped(ctx, "dev.test.scope.dis", ExecutionScope{CharacterID: "char-dis"})
	if err != nil {
		t.Fatal(err)
	}
	if scoped.Definition.Enabled || scoped.Definition.EffectiveScopeType != ScopeCharacter {
		t.Fatalf("skill should be disabled for char-dis: enabled=%v scope=%s", scoped.Definition.Enabled, scoped.Definition.EffectiveScopeType)
	}
}

func TestLegacy_Registry_ModelNameCollision(t *testing.T) {
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	def1, hdl1 := testDefinition(t, "dev.test.coll.a", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	def2, hdl2 := testDefinition(t, "dev.test.coll.b", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	def1.ModelName = "same-name"
	def2.ModelName = "same-name"
	if err := registry.Register(context.Background(), def1, hdl1); err != nil {
		t.Fatal(err)
	}
	err = registry.Register(context.Background(), def2, hdl2)
	assertExtensionErrorCode(t, err, ErrSkillDuplicateID)
}
