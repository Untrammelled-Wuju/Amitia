package extension

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/migration"
)

func TestLegacy_Executor_NormalExecution(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	handler := func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		if request.SkillID != "dev.test.exec.normal" {
			t.Fatalf("unexpected skill ID: %s", request.SkillID)
		}
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{"ok":true}`)}, nil
	}
	definition, _ := testDefinition(t, "dev.test.exec.normal", json.RawMessage(`{}`), json.RawMessage(`{}`), handler)
	for _, cap := range definition.Capabilities {
		permissions.GrantSystemPolicy(definition.ID, cap, DecisionAllowAlways)
	}
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	result, err := executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.normal", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunSucceeded {
		t.Fatalf("expected succeeded, got %s", result.Status)
	}
	var output map[string]interface{}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatal(err)
	}
	if output["ok"] != true {
		t.Fatal("expected ok=true in output")
	}
}

func TestLegacy_Executor_DisabledSkill(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	definition, handler := testDefinition(t, "dev.test.exec.dis", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	definition.Enabled = false
	syncDefinitionManifest(t, &definition)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.dis", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, ErrSkillDisabled)
}

func TestLegacy_Executor_NotFound(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.nonexist", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, ErrSkillNotFound)
}

func TestLegacy_Executor_PermissionDenied(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	definition, handler := testDefinition(t, "dev.test.exec.perm", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	definition.Capabilities = []string{"network.https"}
	syncDefinitionManifest(t, &definition)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.perm", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, ErrSkillPermissionDenied)
}

func TestLegacy_Executor_InputSchemaValidation(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	inputSchema := json.RawMessage(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)
	definition, handler := testDefinition(t, "dev.test.exec.input", inputSchema, json.RawMessage(`{"type":"object"}`), noopHandler)
	for _, cap := range definition.Capabilities {
		permissions.GrantSystemPolicy(definition.ID, cap, DecisionAllowAlways)
	}
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.input", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, ErrSkillInputInvalid)
}

func TestLegacy_Executor_InstructionsNotExecutable(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	definition := SkillDefinition{
		ID: "dev.test.exec.ins", ModelName: "dev.test.exec.ins", Name: "ins", Description: "test", Version: "1.0.0",
		Source: SkillSourceInstructions, Entry: SkillEntry{Kind: "instructions", Path: "SKILL.md", ArtifactID: "artifact-ins"}, Triggers: []SkillTrigger{TriggerLLM},
		Timeout: 100 * time.Millisecond, TimeoutMS: 100, Enabled: true, Compatible: true,
	}
	definition.InputSchema = json.RawMessage(`{"type":"object"}`)
	definition.OutputSchema = json.RawMessage(`{"type":"object"}`)
	definition.ConfigSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)
	definition.DefaultConfig = json.RawMessage(`{}`)
	manifest := Manifest{
		Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill",
		Metadata:      ManifestMetadata{ID: "dev.test.exec.ins", Name: "ins", Version: "1.0.0", Description: "test"},
		Compatibility: ManifestCompatibility{EngineMin: "1.0.0", EngineMaxExclusive: "2.0.0"},
		Entry:         SkillEntry{Kind: "instructions", Path: "SKILL.md", ArtifactID: "artifact-ins"},
		Capabilities:  []string{}, Triggers: []SkillTrigger{TriggerLLM},
		Execution:   ManifestExecution{TimeoutMS: 100, Idempotent: false},
		InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`),
		ConfigSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`), DefaultConfig: json.RawMessage(`{}`),
		Enabled: true, AllowLLM: true, AllowManual: false,
	}
	raw, _ := json.Marshal(manifest)
	definition.Manifest = raw
	if err := registry.Register(ctx, definition, nil); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.ins", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerLLM}})
	assertExtensionErrorCode(t, err, ErrSkillNotExecutable)
}

func TestLegacy_Executor_IncompatibleSkill(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	definition, handler := testDefinition(t, "dev.test.exec.incompat", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	var manifest Manifest
	json.Unmarshal(definition.Manifest, &manifest)
	manifest.Compatibility.EngineMin = "2.0.0"
	definition.Manifest, _ = json.Marshal(manifest)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.incompat", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, ErrSkillIncompatible)
}

func TestLegacy_Executor_TriggerNotAllowed(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	definition, handler := testDefinition(t, "dev.test.exec.trig", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	definition.Triggers = []SkillTrigger{TriggerManual}
	syncDefinitionManifest(t, &definition)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.trig", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerLLM}})
	assertExtensionErrorCode(t, err, ErrSkillTriggerNotAllowed)
}

func TestLegacy_Executor_Timeout(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	handler := func(ctx context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		select {
		case <-ctx.Done():
			return SkillResult{}, ctx.Err()
		case <-time.After(5 * time.Second):
		}
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{}`)}, nil
	}
	definition, _ := testDefinition(t, "dev.test.exec.timeout", json.RawMessage(`{}`), json.RawMessage(`{}`), handler)
	definition.Timeout = 50 * time.Millisecond
	definition.TimeoutMS = 50
	syncDefinitionManifest(t, &definition)
	for _, cap := range definition.Capabilities {
		permissions.GrantSystemPolicy(definition.ID, cap, DecisionAllowAlways)
	}
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.timeout", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestLegacy_Executor_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	definition, handler := testDefinition(t, "dev.test.exec.cancel", json.RawMessage(`{}`), json.RawMessage(`{}`), noopHandler)
	for _, cap := range definition.Capabilities {
		permissions.GrantSystemPolicy(definition.ID, cap, DecisionAllowAlways)
	}
	if err := registry.Register(context.Background(), definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.cancel", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, ErrSkillCancelled)
}

func TestLegacy_Executor_HandlerPanicRecovery(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	handler := func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		panic("test panic in handler")
	}
	definition, _ := testDefinition(t, "dev.test.exec.panic", json.RawMessage(`{}`), json.RawMessage(`{}`), handler)
	for _, cap := range definition.Capabilities {
		permissions.GrantSystemPolicy(definition.ID, cap, DecisionAllowAlways)
	}
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.panic", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, ErrSkillExecutionFailed)
}

func TestLegacy_Executor_HandlerError(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	handler := func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{}, NewExtensionError("CUSTOM_ERROR", "custom handler error", "", false, nil)
	}
	definition, _ := testDefinition(t, "dev.test.exec.err", json.RawMessage(`{}`), json.RawMessage(`{}`), handler)
	for _, cap := range definition.Capabilities {
		permissions.GrantSystemPolicy(definition.ID, cap, DecisionAllowAlways)
	}
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.err", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	assertExtensionErrorCode(t, err, "CUSTOM_ERROR")
}

func TestLegacy_Executor_Idempotency(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	callCount := 0
	handler := func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		callCount++
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{}`)}, nil
	}
	definition, _ := testDefinition(t, "dev.test.exec.idem", json.RawMessage(`{}`), json.RawMessage(`{}`), handler)
	definition.Idempotent = true
	for _, cap := range definition.Capabilities {
		permissions.GrantSystemPolicy(definition.ID, cap, DecisionAllowAlways)
	}
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	scope := ExecutionScope{Trigger: TriggerManual}
	req := ExecuteSkillRequest{SkillID: "dev.test.exec.idem", Input: json.RawMessage(`{}`), Scope: scope, IdempotencyKey: "key-1"}
	result1, err := executor.Execute(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	result2, err := executor.Execute(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 1 {
		t.Fatalf("expected handler called once for idempotent skill, called %d times", callCount)
	}
	if result1.RunID != result2.RunID {
		t.Fatal("idempotent results should return same run ID")
	}
}

func TestLegacy_Executor_ResultNormalization(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	handler := func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{"data":"test"}`), VisibleText: "visible result"}, nil
	}
	definition, _ := testDefinition(t, "dev.test.exec.norm", json.RawMessage(`{}`), json.RawMessage(`{"type":"object"}`), handler)
	for _, cap := range definition.Capabilities {
		permissions.GrantSystemPolicy(definition.ID, cap, DecisionAllowAlways)
	}
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	result, err := executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.norm", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunSucceeded {
		t.Fatalf("expected succeeded, got %s", result.Status)
	}
	if result.DurationMS < 0 {
		t.Fatal("duration should be recorded")
	}
}

func TestLegacy_Executor_PermissionAllowOnce(t *testing.T) {
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
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	handler := func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{}`)}, nil
	}
	definition, _ := testDefinition(t, "dev.test.exec.once", json.RawMessage(`{}`), json.RawMessage(`{}`), handler)
	definition.Capabilities = []string{"network.https"}
	syncDefinitionManifest(t, &definition)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	permissions.GrantSystemPolicy(definition.ID, "network.https", DecisionAllowOnce)
	result, err := executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.once", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunSucceeded {
		t.Fatalf("first execution should succeed: %s", result.Status)
	}
	result2, err2 := executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.once", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	if err2 != nil {
		t.Fatal(err2)
	}
	if result2.Status != RunSucceeded {
		t.Fatalf("second execution should also succeed (KNOWN_LEGACY_BEHAVIOR: GrantSystemPolicy DecisionAllowOnce never consumed): %s", result2.Status)
	}
}

func TestLegacy_Executor_UnknownCapability(t *testing.T) {
	ctx := context.Background()
	_, repository := testRepository(t)
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, repository)
	permissions := NewPermissionEvaluator(repository)
	executor := NewExecutor(registry, validator, permissions, repository)
	handler := func(_ context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{}`)}, nil
	}
	definition, _ := testDefinition(t, "dev.test.exec.unkcap", json.RawMessage(`{}`), json.RawMessage(`{}`), handler)
	definition.Capabilities = []string{"network.https"}
	syncDefinitionManifest(t, &definition)
	if err := registry.Register(ctx, definition, handler); err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ctx, ExecuteSkillRequest{SkillID: "dev.test.exec.unkcap", Input: json.RawMessage(`{}`), Scope: ExecutionScope{Trigger: TriggerManual}})
	if err == nil {
		t.Fatal("expected permission denied for unknown capability")
	}
}
