package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func testRepository(t *testing.T) (*gorm.DB, *Repository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionsMigration(), migration.PluginRuntimeMigration()}); err != nil {
		t.Fatal(err)
	}
	return db, NewRepository(db)
}

func testDefinition(t *testing.T, id string, inputSchema, outputSchema json.RawMessage, handler SkillHandler) (SkillDefinition, SkillHandler) {
	t.Helper()
	manifest := Manifest{
		Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill",
		Metadata: ManifestMetadata{ID: id, Name: id, Version: "1.0.0", Description: "test"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0", EngineMaxExclusive: "2.0.0"},
		Entry: SkillEntry{Kind: "builtin", Name: id}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual, TriggerLLM},
		Execution: ManifestExecution{TimeoutMS: 100, Idempotent: true}, InputSchema: inputSchema, OutputSchema: outputSchema,
		ConfigSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`), DefaultConfig: json.RawMessage(`{}`), Enabled: true, AllowLLM: true, AllowManual: true,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	definition := SkillDefinition{ID: id, ModelName: id, Name: id, Description: "test", Version: "1.0.0", Source: SkillSourceBuiltin, Entry: manifest.Entry, InputSchema: inputSchema, OutputSchema: outputSchema, ConfigSchema: manifest.ConfigSchema, DefaultConfig: manifest.DefaultConfig, Triggers: manifest.Triggers, Timeout: 100 * time.Millisecond, TimeoutMS: 100, Idempotent: true, Enabled: true, Compatible: true, Manifest: raw}
	return definition, handler
}

func syncDefinitionManifest(t *testing.T, definition *SkillDefinition) {
	t.Helper()
	var manifest Manifest
	if err := json.Unmarshal(definition.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Metadata.ID = definition.ID
	manifest.Metadata.Name = definition.Name
	manifest.Metadata.Version = definition.Version
	manifest.Metadata.Description = definition.Description
	manifest.Metadata.Author = definition.Author
	manifest.Metadata.License = definition.License
	manifest.Entry = definition.Entry
	manifest.Capabilities = append([]string{}, definition.Capabilities...)
	manifest.Triggers = definition.Triggers
	manifest.Execution = ManifestExecution{TimeoutMS: definition.TimeoutMS, HasSideEffects: definition.HasSideEffects, Retryable: definition.Retryable, Idempotent: definition.Idempotent}
	manifest.InputSchema = definition.InputSchema
	manifest.OutputSchema = definition.OutputSchema
	manifest.ConfigSchema = definition.ConfigSchema
	manifest.DefaultConfig = definition.DefaultConfig
	manifest.AllowLLM = hasTrigger(definition.Triggers, TriggerLLM)
	manifest.AllowManual = hasTrigger(definition.Triggers, TriggerManual)
	definition.Manifest, _ = json.Marshal(manifest)
}

func testRuntimeParts(t *testing.T, definition SkillDefinition, handler SkillHandler) (*Registry, *Executor, *Repository, *DefaultPermissionEvaluator) {
	t.Helper()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	mismatched := definition
	mismatched.ID = "dev.amitia.skill.manifest-mismatch"
	mismatched.Description = "changed without manifest"
	if err := NewRegistry("1.0.0", validator, nil).Register(context.Background(), mismatched, handler); err == nil || asExtensionError(err).Code != ErrSkillManifestInvalid {
		t.Fatalf("manifest mismatch accepted: %v", err)
	}
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	return registry, executor, repository, permissions
}

func TestManifestAndRegistryValidation(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"name":{"type":"string"}},"required":["name"]}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}},"required":["ok"]}`)
	handler := func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{"ok":true}`)}, nil
	}
	definition, _ := testDefinition(t, "dev.amitia.skill.valid", input, output, handler)
	if err := validator.ValidateManifest(definition.Manifest); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, nil)
	invalidID := definition
	invalidID.ID = "Invalid_ID"
	if err := registry.Register(context.Background(), invalidID, handler); err == nil {
		t.Fatal("invalid id accepted")
	}
	invalidVersion := definition
	invalidVersion.ID = "dev.amitia.skill.bad-version"
	invalidVersion.Version = "1.0"
	if err := registry.Register(context.Background(), invalidVersion, handler); err == nil {
		t.Fatal("invalid semver accepted")
	}
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(context.Background(), definition, handler); err == nil || asExtensionError(err).Code != ErrSkillDuplicateID {
		t.Fatalf("expected duplicate id, got %v", err)
	}
}

func TestIncompatibleSkillCannotEnable(t *testing.T) {
	input := json.RawMessage(`{"type":"object"}`)
	output := json.RawMessage(`{"type":"object"}`)
	handler := func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Output: json.RawMessage(`{}`)}, nil
	}
	definition, _ := testDefinition(t, "dev.amitia.skill.future", input, output, handler)
	var manifest Manifest
	json.Unmarshal(definition.Manifest, &manifest)
	manifest.Compatibility.EngineMin = "2.0.0"
	definition.Manifest, _ = json.Marshal(manifest)
	validator, _ := NewSchemaValidator()
	registry := NewRegistry("1.0.0", validator, nil)
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetEnabled(context.Background(), definition.ID, true); err == nil || asExtensionError(err).Code != ErrSkillIncompatible {
		t.Fatalf("expected incompatible error, got %v", err)
	}
}

func TestExecutorSchemaPermissionDisableAndTrigger(t *testing.T) {
	input := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"name":{"type":"string"}},"required":["name"]}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}},"required":["ok"]}`)
	handler := func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{"ok":true}`)}, nil
	}
	definition, _ := testDefinition(t, "dev.amitia.skill.schema", input, output, handler)
	definition.Capabilities = []string{"runtime.time.read"}
	var manifest Manifest
	json.Unmarshal(definition.Manifest, &manifest)
	manifest.Capabilities = definition.Capabilities
	definition.Manifest, _ = json.Marshal(manifest)
	registry, executor, _, permissions := testRuntimeParts(t, definition, handler)
	scope := ExecutionScope{CharacterID: "char-1", ConversationID: "conv-1", Trigger: TriggerManual}
	_, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{"unknown":true}`), Scope: scope})
	if err == nil || asExtensionError(err).Code != ErrSkillInputInvalid {
		t.Fatalf("expected input invalid, got %v", err)
	}
	_, err = executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{"name":"a"}`), Scope: scope})
	if err == nil || asExtensionError(err).Code != ErrSkillPermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
	permissions.GrantSystemPolicy(definition.ID, "runtime.time.read", DecisionAllowAlways)
	if _, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{"name":"a"}`), Scope: scope}); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetEnabled(context.Background(), definition.ID, false); err != nil {
		t.Fatal(err)
	}
	if available, _ := registry.Available(context.Background(), ExecutionScope{Trigger: TriggerLLM}); len(available) != 0 {
		t.Fatal("disabled skill exposed to llm")
	}
	_, err = executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{"name":"a"}`), Scope: scope})
	if err == nil || asExtensionError(err).Code != ErrSkillDisabled {
		t.Fatalf("expected disabled, got %v", err)
	}
	if err := registry.SetEnabled(context.Background(), definition.ID, true); err != nil {
		t.Fatal(err)
	}
	item, _ := registry.Get(context.Background(), definition.ID)
	item.Definition.Triggers = []SkillTrigger{TriggerManual}
	registry.mu.Lock()
	registry.items[definition.ID] = item
	registry.mu.Unlock()
	if available, _ := registry.Available(context.Background(), ExecutionScope{Trigger: TriggerLLM}); len(available) != 0 {
		t.Fatal("manual-only skill exposed to llm")
	}
}

func TestOutputValidationTimeoutCancelPanicAndIdempotency(t *testing.T) {
	input := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}},"required":["ok"]}`)
	t.Run("output", func(t *testing.T) {
		definition, handler := testDefinition(t, "dev.amitia.skill.output", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
			return SkillResult{Output: json.RawMessage(`{"bad":true}`)}, nil
		})
		_, executor, _, _ := testRuntimeParts(t, definition, handler)
		_, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: ExecutionScope{CharacterID: "c", ConversationID: "x", Trigger: TriggerManual}})
		if err == nil || asExtensionError(err).Code != ErrSkillOutputInvalid {
			t.Fatalf("expected output invalid, got %v", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		definition, handler := testDefinition(t, "dev.amitia.skill.timeout", input, output, func(ctx context.Context, _ ExecuteSkillRequest) (SkillResult, error) {
			<-ctx.Done()
			return SkillResult{}, ctx.Err()
		})
		definition.Timeout = 10 * time.Millisecond
		_, executor, _, _ := testRuntimeParts(t, definition, handler)
		result, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: ExecutionScope{CharacterID: "c", ConversationID: "x", Trigger: TriggerManual}})
		if err == nil || result.Status != RunTimedOut {
			t.Fatalf("expected timeout, got %s %v", result.Status, err)
		}
	})
	t.Run("noncooperative timeout", func(t *testing.T) {
		release := make(chan struct{})
		definition, handler := testDefinition(t, "dev.amitia.skill.noncooperative-timeout", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
			<-release
			return SkillResult{Output: json.RawMessage(`{"ok":true}`)}, nil
		})
		definition.Timeout = 10 * time.Millisecond
		_, executor, _, _ := testRuntimeParts(t, definition, handler)
		started := time.Now()
		result, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: ExecutionScope{CharacterID: "c", ConversationID: "x", Trigger: TriggerManual}})
		close(release)
		if err == nil || result.Status != RunTimedOut || time.Since(started) > 250*time.Millisecond {
			t.Fatalf("noncooperative handler blocked timeout: %s %v %s", result.Status, err, time.Since(started))
		}
	})
	t.Run("cancel", func(t *testing.T) {
		definition, handler := testDefinition(t, "dev.amitia.skill.cancel", input, output, func(ctx context.Context, _ ExecuteSkillRequest) (SkillResult, error) {
			<-ctx.Done()
			return SkillResult{}, ctx.Err()
		})
		_, executor, _, _ := testRuntimeParts(t, definition, handler)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result, err := executor.Execute(ctx, ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: ExecutionScope{CharacterID: "c", ConversationID: "x", Trigger: TriggerManual}})
		if err == nil || result.Status != RunCancelled {
			t.Fatalf("expected cancelled, got %s %v", result.Status, err)
		}
	})
	t.Run("panic", func(t *testing.T) {
		definition, handler := testDefinition(t, "dev.amitia.skill.panic", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) { panic("boom") })
		_, executor, _, _ := testRuntimeParts(t, definition, handler)
		result, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: ExecutionScope{CharacterID: "c", ConversationID: "x", Trigger: TriggerManual}})
		if err == nil || result.Status != RunFailed || strings.Contains(err.Error(), "stack") {
			t.Fatalf("panic not normalized: %s %v", result.Status, err)
		}
	})
	t.Run("idempotency", func(t *testing.T) {
		var count atomic.Int32
		definition, handler := testDefinition(t, "dev.amitia.skill.idempotent", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
			count.Add(1)
			return SkillResult{Output: json.RawMessage(`{"ok":true}`)}, nil
		})
		_, executor, _, _ := testRuntimeParts(t, definition, handler)
		scope := ExecutionScope{CharacterID: "char-1", ConversationID: "conv", Trigger: TriggerManual}
		first, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: scope, IdempotencyKey: "same"})
		if err != nil {
			t.Fatal(err)
		}
		second, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: scope, IdempotencyKey: "same"})
		if err != nil || first.RunID != second.RunID || count.Load() != 1 {
			t.Fatalf("idempotency failed: %s %s %d %v", first.RunID, second.RunID, count.Load(), err)
		}
		scope.CharacterID = "char-2"
		if _, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: scope, IdempotencyKey: "same"}); err != nil {
			t.Fatal(err)
		}
		if count.Load() != 2 {
			t.Fatal("idempotency crossed character scope")
		}
	})
}

func TestPermissionGrantOnceAndCharacterIsolation(t *testing.T) {
	_, repository := testRepository(t)
	evaluator := NewPermissionEvaluator(repository)
	identity := ExtensionIdentity{SkillID: "dev.amitia.skill.permission"}
	if evaluator.EvaluateExecution(context.Background(), identity, "unknown", ExecutionScope{}) != DecisionDeny {
		t.Fatal("unknown capability allowed")
	}
	grants := []PermissionGrantInput{{Capability: "network.https", Decision: DecisionAllowOnce, ScopeType: ScopeCharacter, ScopeID: "char-1"}}
	if err := repository.ReplaceGrants(context.Background(), identity.SkillID, grants); err != nil {
		t.Fatal(err)
	}
	if evaluator.EvaluateExecution(context.Background(), identity, "network.https", ExecutionScope{CharacterID: "char-2"}) != DecisionDeny {
		t.Fatal("grant crossed character scope")
	}
	if evaluator.EvaluateExecution(context.Background(), identity, "network.https", ExecutionScope{CharacterID: "char-1"}) != DecisionAllowOnce {
		t.Fatal("allow_once was not granted")
	}
	if evaluator.EvaluateExecution(context.Background(), identity, "network.https", ExecutionScope{CharacterID: "char-1"}) != DecisionDeny {
		t.Fatal("allow_once was not consumed")
	}
}

func TestModelToolDiscoveryDoesNotConsumeAllowOnce(t *testing.T) {
	input := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	definition, handler := testDefinition(t, "dev.amitia.skill.once-preview", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Output: json.RawMessage(`{}`)}, nil
	})
	definition.Capabilities = []string{"network.https"}
	syncDefinitionManifest(t, &definition)
	registry, executor, repository, permissions := testRuntimeParts(t, definition, handler)
	if err := repository.ReplaceGrants(context.Background(), definition.ID, []PermissionGrantInput{{Capability: "network.https", Decision: DecisionAllowOnce, ScopeType: ScopeCharacter, ScopeID: "char-1"}}); err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{Registry: registry, Executor: executor, Permissions: permissions}
	scope := ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Trigger: TriggerLLM}
	definitions, err := runtime.ModelTools(context.Background(), scope)
	if err != nil || len(definitions) != 1 {
		t.Fatalf("allow_once skill was not discoverable: %d %v", len(definitions), err)
	}
	if _, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: scope}); err != nil {
		t.Fatalf("allow_once was consumed by discovery: %v", err)
	}
	if _, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: scope}); asExtensionError(err).Code != ErrSkillPermissionDenied {
		t.Fatalf("allow_once remained after execution: %v", err)
	}
}

func TestExplicitDenyOverridesSystemPolicy(t *testing.T) {
	_, repository := testRepository(t)
	evaluator := NewPermissionEvaluator(repository)
	identity := ExtensionIdentity{SkillID: "dev.amitia.skill.policy"}
	evaluator.GrantSystemPolicy(identity.SkillID, "runtime.time.read", DecisionAllowAlways)
	if evaluator.EvaluateExecution(context.Background(), identity, "runtime.time.read", ExecutionScope{}) != DecisionAllowAlways {
		t.Fatal("system policy was not applied")
	}
	if err := repository.ReplaceGrants(context.Background(), identity.SkillID, []PermissionGrantInput{{Capability: "runtime.time.read", Decision: DecisionDeny, ScopeType: ScopeGlobal}}); err != nil {
		t.Fatal(err)
	}
	if evaluator.EvaluateExecution(context.Background(), identity, "runtime.time.read", ExecutionScope{}) != DecisionDeny {
		t.Fatal("explicit deny did not override system policy")
	}
}

func TestLegacyAdapterAndRuntimeRegistration(t *testing.T) {
	adapter := NewLegacyToolAdapter()
	var current tool.Tool
	for _, item := range tool.GetAll() {
		if item.Function.Name == "get_current_time" {
			current = item
			break
		}
	}
	definition, handler, err := adapter.Adapt(current, false)
	if err != nil {
		t.Fatal(err)
	}
	if definition.ID != "dev.amitia.skill.get-current-time" || definition.ModelName != "get_current_time" || !strings.Contains(string(definition.InputSchema), `"additionalProperties":false`) {
		t.Fatalf("legacy mapping invalid: %+v %s", definition, definition.InputSchema)
	}
	result, err := handler(context.Background(), ExecuteSkillRequest{Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	if err != nil || result.Status != RunSucceeded || !strings.Contains(result.VisibleText, "系统参考时间") {
		t.Fatalf("legacy result invalid: %+v %v", result, err)
	}
	missingDefinition, missingHandler, err := adapter.Adapt(tool.Tool{Type: "function", Function: tool.Function{Name: "missing_legacy", Description: "missing", Parameters: tool.Parameters{Type: "object", Properties: map[string]tool.Property{}, Required: []string{}}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = missingHandler(context.Background(), ExecuteSkillRequest{SkillID: missingDefinition.ID, Input: json.RawMessage(`{}`)})
	if err == nil || !errors.Is(err, err) || asExtensionError(err).Code != ErrSkillExecutionFailed {
		t.Fatalf("legacy error not mapped: %v", err)
	}
	db, _ := testRepository(t)
	runtime, err := NewRuntime(context.Background(), db, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	items, _ := runtime.Registry.List(context.Background(), SkillFilter{})
	expected := len(tool.GetAll()) + len(tool.GetMemoryTools()) + 1
	if len(items) != expected {
		t.Fatalf("expected %d legacy skills, got %d", expected, len(items))
	}
}

func TestRunPersistenceRedactionAndScope(t *testing.T) {
	input := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"apiKey":{"type":"string"}},"required":["apiKey"]}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"ok":{"type":"boolean"}},"required":["ok"]}`)
	definition, handler := testDefinition(t, "dev.amitia.skill.audit", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Output: json.RawMessage(`{"ok":true}`), SideEffects: []SideEffectRecord{{Type: "test.write", TargetID: "target-1", Confirmed: true}}}, nil
	})
	_, executor, repository, _ := testRuntimeParts(t, definition, handler)
	_, err := executor.Execute(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{"apiKey":"secret-value"}`), Scope: ExecutionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Trigger: TriggerManual, TraceID: "trace-1"}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := repository.ListRuns(context.Background(), ExecutionScope{UserID: "user-1", CharacterID: "char-1"}, RunFilter{Page: 1, PageSize: 20})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("run not persisted: %+v %v", page, err)
	}
	if strings.Contains(page.Items[0].InputSummary, "secret-value") || !strings.Contains(page.Items[0].InputSummary, "REDACTED") {
		t.Fatal("secret leaked in run record")
	}
	if len(page.Items[0].SideEffects) != 1 || page.Items[0].SideEffects[0].TargetID != "target-1" {
		t.Fatal("side effects were not persisted")
	}
	other, err := repository.ListRuns(context.Background(), ExecutionScope{UserID: "user-2", CharacterID: "char-1"}, RunFilter{Page: 1, PageSize: 20})
	if err != nil || len(other.Items) != 0 {
		t.Fatal("run crossed user scope")
	}
}

func TestConfigRedactionAndPlaceholderRestore(t *testing.T) {
	input := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	definition, handler := testDefinition(t, "dev.amitia.skill.config", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Output: json.RawMessage(`{}`)}, nil
	})
	definition.ConfigSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"apiKey":{"type":"string"},"region":{"type":"string"}},"required":["apiKey","region"]}`)
	definition.DefaultConfig = json.RawMessage(`{"apiKey":"","region":"cn"}`)
	syncDefinitionManifest(t, &definition)
	registry, executor, repository, _ := testRuntimeParts(t, definition, handler)
	validator, _ := NewSchemaValidator()
	service := NewService(registry, executor, repository, validator)
	if err := service.UpdateSkillConfig(context.Background(), ExecutionScope{}, definition.ID, json.RawMessage(`{"apiKey":"stored-secret","region":"cn"}`)); err != nil {
		t.Fatal(err)
	}
	visible, err := service.GetSkillConfig(context.Background(), ExecutionScope{}, definition.ID)
	if err != nil || strings.Contains(string(visible), "stored-secret") || !strings.Contains(string(visible), "REDACTED") {
		t.Fatalf("config secret leaked: %s %v", visible, err)
	}
	if err := service.UpdateSkillConfig(context.Background(), ExecutionScope{}, definition.ID, json.RawMessage(`{"apiKey":"[REDACTED]","region":"us"}`)); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.GetConfig(context.Background(), definition.ID, PermissionScope{Type: ScopeGlobal}, definition.DefaultConfig)
	if err != nil || !strings.Contains(string(stored), "stored-secret") || !strings.Contains(string(stored), `"region":"us"`) {
		t.Fatalf("redacted placeholder was persisted: %s %v", stored, err)
	}
	var persisted string
	if err := repository.db.Table("extension_configs").Select("config_json").Where("extension_id = ?", definition.ID).Scan(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(persisted, "stored-secret") || !strings.HasPrefix(persisted, encryptedConfigPrefix) {
		t.Fatalf("configuration was stored in plaintext: %s", persisted)
	}
}

func TestConfigEncryptionSurvivesRepositoryRestart(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "runtime.db")
	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply([]migration.Migration{migration.ExtensionsMigration()}); err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	scope := PermissionScope{Type: ScopeGlobal}
	if err := repository.UpdateConfig(context.Background(), "dev.amitia.skill.restart-config", scope, json.RawMessage(`{"apiKey":"restart-secret"}`)); err != nil {
		t.Fatal(err)
	}
	var persisted string
	if err := db.Table("extension_configs").Select("config_json").Where("extension_id = ?", "dev.amitia.skill.restart-config").Scan(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(persisted, "restart-secret") || !strings.HasPrefix(persisted, encryptedConfigPrefix) {
		t.Fatal("configuration was not encrypted")
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	reopenedSQL, err := reopened.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = reopenedSQL.Close()
	})
	restored, err := NewRepository(reopened).GetConfig(context.Background(), "dev.amitia.skill.restart-config", scope, nil)
	if err != nil || string(restored) != `{"apiKey":"restart-secret"}` {
		t.Fatalf("configuration was not restored after restart: %s %v", restored, err)
	}
}

func TestUpdatedConfigIsInjectedIntoExecution(t *testing.T) {
	input := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"region":{"type":"string"}},"required":["region"]}`)
	definition, handler := testDefinition(t, "dev.amitia.skill.live-config", input, output, func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		var config struct {
			Region string `json:"region"`
		}
		if err := json.Unmarshal(request.Config, &config); err != nil {
			return SkillResult{}, err
		}
		value, err := json.Marshal(map[string]string{"region": config.Region})
		if err != nil {
			return SkillResult{}, err
		}
		return SkillResult{Output: value}, nil
	})
	definition.ConfigSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"region":{"type":"string"}},"required":["region"]}`)
	definition.DefaultConfig = json.RawMessage(`{"region":"cn"}`)
	syncDefinitionManifest(t, &definition)
	registry, executor, repository, _ := testRuntimeParts(t, definition, handler)
	validator, _ := NewSchemaValidator()
	service := NewService(registry, executor, repository, validator)
	if err := service.UpdateSkillConfig(context.Background(), ExecutionScope{}, definition.ID, json.RawMessage(`{"region":"us"}`)); err != nil {
		t.Fatal(err)
	}
	result, err := service.ExecuteSkill(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: ExecutionScope{UserID: "user-1", CharacterID: "char-1", Trigger: TriggerManual}})
	if err != nil || string(result.Output) != `{"region":"us"}` {
		t.Fatalf("updated config was not used: %s %v", result.Output, err)
	}
}

func TestRunDetailCannotCrossCharacterScope(t *testing.T) {
	input := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	definition, handler := testDefinition(t, "dev.amitia.skill.run-scope", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Output: json.RawMessage(`{}`)}, nil
	})
	registry, executor, repository, _ := testRuntimeParts(t, definition, handler)
	validator, _ := NewSchemaValidator()
	service := NewService(registry, executor, repository, validator)
	result, err := service.ExecuteSkill(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: ExecutionScope{UserID: "user-1", CharacterID: "char-1", Trigger: TriggerManual}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetRun(context.Background(), ExecutionScope{UserID: "user-1", CharacterID: "char-2"}, result.RunID); asExtensionError(err).Code != ErrSkillNotFound {
		t.Fatalf("cross-character run detail was readable: %v", err)
	}
}

func TestManualExecutionCannotCrossConversationScope(t *testing.T) {
	input := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	output := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	var calls atomic.Int32
	definition, handler := testDefinition(t, "dev.amitia.skill.conversation-scope", input, output, func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		calls.Add(1)
		return SkillResult{Output: json.RawMessage(`{}`)}, nil
	})
	registry, executor, repository, _ := testRuntimeParts(t, definition, handler)
	if err := repository.db.Exec("CREATE TABLE conversations (id TEXT PRIMARY KEY, character_id TEXT NOT NULL, channel TEXT NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.db.Exec("INSERT INTO conversations (id, character_id, channel) VALUES (?, ?, ?)", "conv-1", "char-1", "web").Error; err != nil {
		t.Fatal(err)
	}
	validator, _ := NewSchemaValidator()
	service := NewService(registry, executor, repository, validator)
	_, err := service.ExecuteSkill(context.Background(), ExecuteSkillRequest{SkillID: definition.ID, Input: json.RawMessage(`{}`), Scope: ExecutionScope{UserID: "user-1", CharacterID: "char-2", ConversationID: "conv-1", Channel: "web", Trigger: TriggerManual}})
	if asExtensionError(err).Code != ErrSkillPermissionDenied || calls.Load() != 0 {
		t.Fatalf("cross-conversation execution reached handler: %v %d", err, calls.Load())
	}
}

func TestExecuteProblemDetailsAreDirectRFC9457(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := testRepository(t)
	runtime, err := NewRuntime(context.Background(), db, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/api/extensions/skills/:id/execute", func(c *gin.Context) {
		c.Set(authenticatedUserKey, 1)
		NewHandler(runtime.Service).Execute(c)
	})
	body := bytes.NewBufferString(`{"characterId":"char-1","channel":"web","input":{"category":"preference","attribute_name":"test","attribute_value":"test"}}`)
	request := httptest.NewRequest(http.MethodPost, "/api/extensions/skills/dev.amitia.skill.save-profile/execute", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || response.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("unexpected response: %d %s %s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
	var problem map[string]interface{}
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem["code"] != ErrSkillPermissionDenied || problem["problem"] != nil || problem["result"] == nil {
		t.Fatalf("not direct problem details: %v", problem)
	}
}

func TestModelToolPathUsesRuntimeAndHonorsDisable(t *testing.T) {
	db, _ := testRepository(t)
	runtime, err := NewRuntime(context.Background(), db, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	scope := ExecutionScope{UserID: "1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", SessionID: "session-1", Trigger: TriggerLLM, TraceID: "trace-1", RequestID: "request-1", ToolCallID: "call-1"}
	definitions, err := runtime.ModelTools(context.Background(), scope)
	if err != nil || len(definitions) != len(tool.GetAll()) {
		t.Fatalf("model tool definitions changed: %d %d %v", len(definitions), len(tool.GetAll()), err)
	}
	result, found := runtime.ExecuteModelTool(context.Background(), "get_current_time", json.RawMessage(`{}`), scope, "")
	if !found || result.Status != RunSucceeded || result.RunID == "" || result.VisibleText == "" {
		t.Fatalf("model tool did not use runtime: %+v %t", result, found)
	}
	page, err := runtime.Repository.ListRuns(context.Background(), scope, RunFilter{SkillID: "dev.amitia.skill.get-current-time", Page: 1, PageSize: 20})
	if err != nil || len(page.Items) != 1 || page.Items[0].TraceID != "trace-1" {
		t.Fatalf("model run audit missing: %+v %v", page, err)
	}
	if err := runtime.Registry.SetEnabled(context.Background(), "dev.amitia.skill.get-current-time", false); err != nil {
		t.Fatal(err)
	}
	definitions, err = runtime.ModelTools(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range definitions {
		if definition.Function.Name == "get_current_time" {
			t.Fatal("disabled skill remained visible to model")
		}
	}
}
