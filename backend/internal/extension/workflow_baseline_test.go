package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestLegacy_Workflow_Compiler_OutputReferenceValidation(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)

	t.Run("missing_step_reference", func(t *testing.T) {
		wf := WorkflowDefinition{
			SchemaVersion: "1.0.0",
			Steps:         []WorkflowStep{transformStepForTest()},
			Output:        json.RawMessage(`{"result":{"$ref":"steps.missing"}}`),
			Limits:        DefaultWorkflowLimits(),
		}
		_, issues, err := compiler.Compile(context.Background(), wf)
		if err == nil {
			t.Fatal("expected compile error for missing output reference")
		}
		if !hasIssueWithCode(issues, ErrWorkflowReferenceInvalid) {
			t.Fatalf("expected %s, got %#v", ErrWorkflowReferenceInvalid, issues)
		}
	})

	t.Run("valid_step_reference", func(t *testing.T) {
		wf := WorkflowDefinition{
			SchemaVersion: "1.0.0",
			Steps:         []WorkflowStep{transformStepForTest()},
			Output:        json.RawMessage(`{"result":{"$ref":"steps.result"}}`),
			Limits:        DefaultWorkflowLimits(),
		}
		_, issues, err := compiler.Compile(context.Background(), wf)
		if err != nil {
			t.Fatalf("unexpected compile error: %v %#v", err, issues)
		}
	})

	t.Run("output_missing_field", func(t *testing.T) {
		wf := WorkflowDefinition{
			SchemaVersion: "1.0.0",
			Steps:         []WorkflowStep{transformStepForTest()},
			Limits:        DefaultWorkflowLimits(),
		}
		_, issues, err := compiler.Compile(context.Background(), wf)
		if err == nil {
			t.Fatal("expected compile error for missing output")
		}
		hasOutput := false
		for _, issue := range issues {
			if strings.Contains(issue.Code, "OUTPUT") {
				hasOutput = true
			}
		}
		if !hasOutput {
			t.Fatalf("expected output-related issue, got %#v", issues)
		}
	})
}

func TestLegacy_Workflow_Compiler_ChecksumStability(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)
	wf := WorkflowDefinition{
		SchemaVersion: "1.0.0",
		Steps: []WorkflowStep{
			{ID: "step_a", Type: "template", Input: json.RawMessage(`{"template":"hello"}`)},
			{ID: "step_b", Type: "transform", Input: json.RawMessage(`{"op":"pick","value":{"$ref":"steps.step_a.text"},"fields":["text"]}`)},
		},
		Output: json.RawMessage(`{"$ref":"steps.step_b"}`),
		Limits: DefaultWorkflowLimits(),
	}

	compiled1, issues1, err1 := compiler.Compile(context.Background(), wf)
	if err1 != nil {
		t.Fatalf("first compile failed: %v %#v", err1, issues1)
	}
	compiled2, issues2, err2 := compiler.Compile(context.Background(), wf)
	if err2 != nil {
		t.Fatalf("second compile failed: %v %#v", err2, issues2)
	}
	if compiled1.Checksum != compiled2.Checksum {
		t.Fatalf("checksum changed between identical compiles: %s vs %s", compiled1.Checksum, compiled2.Checksum)
	}
	if compiled1.Checksum == "" {
		t.Fatal("checksum is empty")
	}
}

func TestLegacy_Workflow_Compiler_StepCountLimit(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)
	steps := make([]WorkflowStep, 0, 100)
	for i := 0; i < 100; i++ {
		steps = append(steps, WorkflowStep{
			ID:    fmt.Sprintf("step_%d", i),
			Type:  "template",
			Input: json.RawMessage(`{"template":"x"}`),
		})
	}
	wf := WorkflowDefinition{
		SchemaVersion: "1.0.0",
		Steps:         steps,
		Output:        json.RawMessage(`{"$ref":"steps.step_0"}`),
		Limits:        DefaultWorkflowLimits(),
	}
	_, issues, err := compiler.Compile(context.Background(), wf)
	if err == nil {
		t.Fatal("expected compile error for excessive steps")
	}
	if !hasIssueWithCode(issues, ErrWorkshopSandboxLimit) {
		t.Fatalf("expected %s, got %#v", ErrWorkshopSandboxLimit, issues)
	}
}

func TestLegacy_Workflow_Compiler_CallSkillRegistered(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, nil)
	handler := func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		return SkillResult{Status: RunSucceeded, Output: json.RawMessage(`{"ok":true}`)}, nil
	}
	schema := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)
	def, _ := testDefinition(t, "dev.user.helper", schema, schema, handler)
	if err := registry.Register(context.Background(), def, handler); err != nil {
		t.Fatal(err)
	}

	compiler := NewWorkflowCompiler(registry)
	wf := WorkflowDefinition{
		SchemaVersion: "1.0.0",
		Steps: []WorkflowStep{
			{ID: "first", Type: "call_skill", Input: json.RawMessage(`{"skillId":"dev.user.helper","input":{"name":"test"}}`)},
		},
		Output: json.RawMessage(`{"$ref":"steps.first"}`),
		Limits: DefaultWorkflowLimits(),
	}
	compiled, issues, err := compiler.Compile(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected compile error: %v %#v", err, issues)
	}
	if len(compiled.Dependencies) != 1 || compiled.Dependencies[0].SkillID != "dev.user.helper" {
		t.Fatalf("dependency not resolved: %#v", compiled.Dependencies)
	}
}

func TestLegacy_Workflow_Compiler_CallSkillOptional(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)
	wf := WorkflowDefinition{
		SchemaVersion: "1.0.0",
		Steps: []WorkflowStep{
			{ID: "first", Type: "call_skill", Input: json.RawMessage(`{"skillId":"dev.user.missing","optional":true}`)},
		},
		Output: json.RawMessage(`{"$ref":"steps.first"}`),
		Limits: DefaultWorkflowLimits(),
	}
	compiled, issues, err := compiler.Compile(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected compile error for optional dep: %v %#v", err, issues)
	}
	if !hasIssueWithCode(issues, ErrWorkshopDependencyNotFound) {
		t.Fatalf("expected warning for optional dependency, got %#v", issues)
	}
	if !compiled.Idempotent {
		t.Fatal("optional call_skill with nil registry no longer treated as idempotent")
	}
}

func TestLegacy_Workflow_Compiler_ScheduleWithCron(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)
	wf := WorkflowDefinition{
		SchemaVersion: "1.0.0",
		Steps: []WorkflowStep{
			{
				ID:    "cron_step",
				Type:  "schedule",
				Input: json.RawMessage(`{"idempotencyKey":"cron-key","timezone":"Asia/Shanghai","cron":"0 9 * * *"}`),
			},
		},
		Output: json.RawMessage(`{"$ref":"steps.cron_step"}`),
		Limits: DefaultWorkflowLimits(),
	}
	_, issues, err := compiler.Compile(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected compile error for cron schedule: %v %#v", err, issues)
	}
	if len(issues) > 0 {
		t.Fatalf("unexpected issues for cron schedule: %#v", issues)
	}
}

func TestLegacy_Workflow_Compiler_SideEffectWithContinue(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)
	wf := WorkflowDefinition{
		SchemaVersion: "1.0.0",
		Steps: []WorkflowStep{
			{
				ID:      "notify",
				Type:    "notification",
				Input:   json.RawMessage(`{"content":"hello"}`),
				OnError: WorkflowErrorPolicy{Mode: "continue"},
			},
		},
		Output: json.RawMessage(`{"$ref":"steps.notify"}`),
		Limits: DefaultWorkflowLimits(),
	}
	_, issues, err := compiler.Compile(context.Background(), wf)
	if err == nil {
		t.Fatal("expected compile error for side effect with continue")
	}
	if !hasIssueWithCode(issues, "WORKFLOW_SIDE_EFFECT_CONTINUE") {
		t.Fatalf("expected WORKFLOW_SIDE_EFFECT_CONTINUE, got %#v", issues)
	}
}

func TestLegacy_Workflow_Compiler_InvalidStepJSON(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)
	wf := WorkflowDefinition{
		SchemaVersion: "1.0.0",
		Steps: []WorkflowStep{
			{ID: "bad", Type: "template", Input: json.RawMessage(`{invalid`)},
		},
		Output: json.RawMessage(`{}`),
		Limits: DefaultWorkflowLimits(),
	}
	_, issues, err := compiler.Compile(context.Background(), wf)
	if err == nil {
		t.Fatal("expected compile error for invalid JSON input")
	}
	if !hasIssueWithCode(issues, ErrWorkflowStepInvalid) {
		t.Fatalf("expected %s, got %#v", ErrWorkflowStepInvalid, issues)
	}
}

func TestLegacy_Workflow_Compiler_InvalidErrorMode(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)
	wf := WorkflowDefinition{
		SchemaVersion: "1.0.0",
		Steps: []WorkflowStep{
			{
				ID:      "step",
				Type:    "template",
				Input:   json.RawMessage(`{"template":"x"}`),
				OnError: WorkflowErrorPolicy{Mode: "retry"},
			},
		},
		Output: json.RawMessage(`{"$ref":"steps.step"}`),
		Limits: DefaultWorkflowLimits(),
	}
	_, issues, err := compiler.Compile(context.Background(), wf)
	if err == nil {
		t.Fatal("expected compile error for invalid error mode")
	}
	if !hasIssueWithCode(issues, ErrWorkflowStepInvalid) {
		t.Fatalf("expected %s, got %#v", ErrWorkflowStepInvalid, issues)
	}
}

func TestLegacy_Workflow_Compiler_UseDefaultWithoutDefault(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)
	wf := WorkflowDefinition{
		SchemaVersion: "1.0.0",
		Steps: []WorkflowStep{
			{
				ID:      "step",
				Type:    "template",
				Input:   json.RawMessage(`{"template":"x"}`),
				OnError: WorkflowErrorPolicy{Mode: "use_default"},
			},
		},
		Output: json.RawMessage(`{"$ref":"steps.step"}`),
		Limits: DefaultWorkflowLimits(),
	}
	_, issues, err := compiler.Compile(context.Background(), wf)
	if err == nil {
		t.Fatal("expected compile error for use_default without default")
	}
	if !hasIssueWithCode(issues, ErrWorkflowStepInvalid) {
		t.Fatalf("expected %s, got %#v", ErrWorkflowStepInvalid, issues)
	}
}

func TestLegacy_Workflow_Executor_MultiStepSequential(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewWorkflowAdapterRegistry()
	_ = registry.Register("template", ValueStepAdapter{kind: "template"})

	compiled := CompiledWorkflow{
		SchemaVersion: "1.0.0",
		Steps: []CompiledStep{
			{ID: "first", Type: "template", Input: json.RawMessage(`{"template":"hello"}`), TimeoutMS: 10000, OnError: WorkflowErrorPolicy{Mode: "fail"}},
			{ID: "second", Type: "template", Input: json.RawMessage(`{"template":"{{steps.first.text}} world"}`), TimeoutMS: 10000, OnError: WorkflowErrorPolicy{Mode: "fail"}},
		},
		Output: json.RawMessage(`{"$ref":"steps.second"}`),
		Limits: DefaultWorkflowLimits(),
	}
	executor := NewWorkflowExecutor(registry, validator)
	outputSchema := json.RawMessage(`{"type":"object"}`)
	result, err := executor.Execute(context.Background(), WorkflowExecutionRequest{
		Workflow: compiled,
		Input:    json.RawMessage(`{}`),
		Config:   json.RawMessage(`{}`),
		Mode:     WorkflowDryRun,
	}, outputSchema)
	if err != nil {
		t.Fatalf("multi-step execution failed: %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != "succeeded" || result.Steps[1].Status != "succeeded" {
		t.Fatalf("unexpected step statuses: %s %s", result.Steps[0].Status, result.Steps[1].Status)
	}
}

func TestLegacy_Workflow_Executor_ContinueOnError(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewWorkflowAdapterRegistry()
	_ = registry.Register("template", ValueStepAdapter{kind: "template"})

	compiled := CompiledWorkflow{
		SchemaVersion: "1.0.0",
		Steps: []CompiledStep{
			{ID: "bad", Type: "template", Input: json.RawMessage(`{"template":"{{invalid"}`), TimeoutMS: 10000, OnError: WorkflowErrorPolicy{Mode: "continue"}},
			{ID: "good", Type: "template", Input: json.RawMessage(`{"template":"recovered"}`), TimeoutMS: 10000, OnError: WorkflowErrorPolicy{Mode: "fail"}},
		},
		Output: json.RawMessage(`{"$ref":"steps.good"}`),
		Limits: DefaultWorkflowLimits(),
	}
	executor := NewWorkflowExecutor(registry, validator)
	outputSchema := json.RawMessage(`{"type":"object"}`)
	result, err := executor.Execute(context.Background(), WorkflowExecutionRequest{
		Workflow: compiled,
		Input:    json.RawMessage(`{}`),
		Config:   json.RawMessage(`{}`),
		Mode:     WorkflowDryRun,
	}, outputSchema)
	if err != nil {
		t.Fatalf("continue mode execution failed: %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != "failed" {
		t.Fatalf("first step should have failed, got %s", result.Steps[0].Status)
	}
	if result.Steps[1].Status != "succeeded" {
		t.Fatalf("second step should have succeeded, got %s", result.Steps[1].Status)
	}
}

func TestLegacy_Workflow_Executor_UseDefault(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewWorkflowAdapterRegistry()
	_ = registry.Register("template", ValueStepAdapter{kind: "template"})

	compiled := CompiledWorkflow{
		SchemaVersion: "1.0.0",
		Steps: []CompiledStep{
			{
				ID:        "maybe_fail",
				Type:      "template",
				Input:     json.RawMessage(`{"template":"{{invalid"}`),
				TimeoutMS: 10000,
				OnError:   WorkflowErrorPolicy{Mode: "use_default", Default: json.RawMessage(`{"text":"fallback"}`)},
			},
			{
				ID:        "use_result",
				Type:      "template",
				Input:     json.RawMessage(`{"template":"{{steps.maybe_fail.text}}"}`),
				TimeoutMS: 10000,
				OnError:   WorkflowErrorPolicy{Mode: "fail"},
			},
		},
		Output: json.RawMessage(`{"$ref":"steps.use_result"}`),
		Limits: DefaultWorkflowLimits(),
	}
	executor := NewWorkflowExecutor(registry, validator)
	outputSchema := json.RawMessage(`{"type":"object"}`)
	result, err := executor.Execute(context.Background(), WorkflowExecutionRequest{
		Workflow: compiled,
		Input:    json.RawMessage(`{}`),
		Config:   json.RawMessage(`{}`),
		Mode:     WorkflowDryRun,
	}, outputSchema)
	if err != nil {
		t.Fatalf("use_default mode execution failed: %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != "failed" {
		t.Fatalf("first step should be failed, got %s", result.Steps[0].Status)
	}
	if result.Steps[1].Status != "succeeded" {
		t.Fatalf("second step should have succeeded with fallback, got %s", result.Steps[1].Status)
	}
}

func TestLegacy_Workflow_Executor_DryRunProducesPlannedOutput(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewWorkflowAdapterRegistry()
	_ = registry.Register("http", NewHTTPWorkflowAdapter())

	compiled := CompiledWorkflow{
		SchemaVersion: "1.0.0",
		Steps: []CompiledStep{
			{ID: "call", Type: "http", Input: json.RawMessage(`{"url":"https://example.com/api","method":"GET"}`), TimeoutMS: 10000, OnError: WorkflowErrorPolicy{Mode: "fail"}},
		},
		Output: json.RawMessage(`{"$ref":"steps.call"}`),
		Limits: DefaultWorkflowLimits(),
	}
	executor := NewWorkflowExecutor(registry, validator)
	outputSchema := json.RawMessage(`{"type":"object"}`)
	result, err := executor.Execute(context.Background(), WorkflowExecutionRequest{
		Workflow: compiled,
		Input:    json.RawMessage(`{}`),
		Config:   json.RawMessage(`{}`),
		Mode:     WorkflowDryRun,
	}, outputSchema)
	if err != nil {
		t.Fatalf("dry run execution failed: %v", err)
	}
	if len(result.SideEffects) != 1 || result.SideEffects[0].Confirmed {
		t.Fatalf("dry run side effects should not be confirmed: %#v", result.SideEffects)
	}
	var output map[string]interface{}
	_ = json.Unmarshal(result.Output, &output)
	if planned, _ := output["planned"]; planned != true {
		t.Fatalf("dry run output should indicate planned=true: %s", result.Output)
	}
}

func TestLegacy_Workflow_Executor_InputSizeLimit(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewWorkflowAdapterRegistry()
	_ = registry.Register("template", ValueStepAdapter{kind: "template"})

	compiled := CompiledWorkflow{
		SchemaVersion: "1.0.0",
		Steps: []CompiledStep{
			{ID: "echo", Type: "template", Input: json.RawMessage(`{"template":"x"}`), TimeoutMS: 10000, OnError: WorkflowErrorPolicy{Mode: "fail"}},
		},
		Output: json.RawMessage(`{"$ref":"steps.echo"}`),
		Limits: WorkflowLimits{MaxSteps: 32, MaxExecutionDurationMS: 30000, MaxStepDurationMS: 10000, MaxInputBytes: 10, MaxOutputBytes: 262144, MaxIntermediateBytes: 524288},
	}
	executor := NewWorkflowExecutor(registry, validator)
	largeInput := make([]byte, 100)
	for i := range largeInput {
		largeInput[i] = 'x'
	}
	_, err = executor.Execute(context.Background(), WorkflowExecutionRequest{
		Workflow: compiled,
		Input:    json.RawMessage(fmt.Sprintf(`{"data":"%s"}`, string(largeInput))),
		Config:   json.RawMessage(`{}`),
	}, nil)
	if err == nil || asExtensionError(err).Code != ErrWorkshopSandboxLimit {
		t.Fatalf("expected sandbox limit for input size, got %v", err)
	}
}

func TestLegacy_Workflow_Executor_StepWhenCondition(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewWorkflowAdapterRegistry()
	_ = registry.Register("template", ValueStepAdapter{kind: "template"})

	compiled := CompiledWorkflow{
		SchemaVersion: "1.0.0",
		Steps: []CompiledStep{
			{ID: "run", Type: "template", Input: json.RawMessage(`{"template":"executed"}`), TimeoutMS: 10000, When: &ConditionExpression{Op: "eq", Left: 1, Right: 1}, OnError: WorkflowErrorPolicy{Mode: "fail"}},
			{ID: "skip", Type: "template", Input: json.RawMessage(`{"template":"skipped"}`), TimeoutMS: 10000, When: &ConditionExpression{Op: "eq", Left: 1, Right: 2}, OnError: WorkflowErrorPolicy{Mode: "fail"}},
		},
		Output: json.RawMessage(`{"$ref":"steps.run"}`),
		Limits: DefaultWorkflowLimits(),
	}
	executor := NewWorkflowExecutor(registry, validator)
	outputSchema := json.RawMessage(`{"type":"object"}`)
	result, err := executor.Execute(context.Background(), WorkflowExecutionRequest{
		Workflow: compiled,
		Input:    json.RawMessage(`{}`),
		Config:   json.RawMessage(`{}`),
	}, outputSchema)
	if err != nil {
		t.Fatalf("when condition execution failed: %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != "succeeded" {
		t.Fatalf("first step should have succeeded, got %s", result.Steps[0].Status)
	}
	if result.Steps[1].Status != "skipped" {
		t.Fatalf("second step should have been skipped, got %s", result.Steps[1].Status)
	}
}

func TestLegacy_Workflow_Executor_WhenConditionError(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewWorkflowAdapterRegistry()
	_ = registry.Register("template", ValueStepAdapter{kind: "template"})

	compiled := CompiledWorkflow{
		SchemaVersion: "1.0.0",
		Steps: []CompiledStep{
			{ID: "bad", Type: "template", Input: json.RawMessage(`{"template":"x"}`), TimeoutMS: 10000, When: &ConditionExpression{Op: "invalid_op", Left: 1, Right: 1}, OnError: WorkflowErrorPolicy{Mode: "fail"}},
		},
		Output: json.RawMessage(`{"$ref":"steps.bad"}`),
		Limits: DefaultWorkflowLimits(),
	}
	executor := NewWorkflowExecutor(registry, validator)
	_, err = executor.Execute(context.Background(), WorkflowExecutionRequest{
		Workflow: compiled,
		Input:    json.RawMessage(`{}`),
		Config:   json.RawMessage(`{}`),
	}, nil)
	if err == nil {
		t.Fatal("expected error for invalid when condition")
	}
	if asExtensionError(err).Code != ErrWorkflowStepInvalid {
		t.Fatalf("expected %s, got %v", ErrWorkflowStepInvalid, err)
	}
}

func TestLegacy_Workflow_Values_TemplateFormatting(t *testing.T) {
	context := map[string]interface{}{
		"input": map[string]interface{}{
			"name":  "amitia",
			"count": float64(3),
		},
		"steps":   map[string]interface{}{},
		"secrets": map[string]interface{}{},
		"runtime": map[string]interface{}{},
	}

	t.Run("json_formatter", func(t *testing.T) {
		result, err := renderTemplate(`{{input.name | json}}`, context, 256, true)
		if err != nil {
			t.Fatalf("json formatter failed: %v", err)
		}
		if result != `"amitia"` {
			t.Fatalf("unexpected json output: %s", result)
		}
	})

	t.Run("number_formatter", func(t *testing.T) {
		result, err := renderTemplate(`{{input.count | number}}`, context, 256, true)
		if err != nil {
			t.Fatalf("number formatter failed: %v", err)
		}
		if result != "3" {
			t.Fatalf("unexpected number output: %s", result)
		}
	})

	t.Run("default_formatter_filled", func(t *testing.T) {
		result, err := renderTemplate(`{{input.name | default:"fallback"}}`, context, 256, true)
		if err != nil {
			t.Fatalf("default formatter failed: %v", err)
		}
		if result != "amitia" {
			t.Fatalf("default should not replace filled value: %s", result)
		}
	})

	t.Run("default_formatter_empty", func(t *testing.T) {
		result, err := renderTemplate(`{{input.missing | default:"fallback"}}`, context, 256, true)
		if err == nil {
			t.Fatal("expected error for missing reference in default formatter")
		}
		if result != "" {
			t.Fatalf("unexpected result: %s", result)
		}
	})

	t.Run("truncate_formatter", func(t *testing.T) {
		result, err := renderTemplate(`{{input.name | truncate:3}}`, context, 256, true)
		if err != nil {
			t.Fatalf("truncate formatter failed: %v", err)
		}
		if result != "ami" {
			t.Fatalf("unexpected truncate output: %s", result)
		}
	})

	t.Run("string_formatter_default", func(t *testing.T) {
		result, err := renderTemplate(`{{input.name}}`, context, 256, true)
		if err != nil {
			t.Fatalf("string formatter failed: %v", err)
		}
		if result != "amitia" {
			t.Fatalf("unexpected string output: %s", result)
		}
	})

	t.Run("date_formatter", func(t *testing.T) {
		ctxWithDate := map[string]interface{}{
			"input": map[string]interface{}{
				"ts": "2026-01-15T10:30:00Z",
			},
			"steps":   map[string]interface{}{},
			"secrets": map[string]interface{}{},
			"runtime": map[string]interface{}{},
		}
		result, err := renderTemplate(`{{input.ts | date:2006-01-02}}`, ctxWithDate, 256, true)
		if err != nil {
			t.Fatalf("date formatter failed: %v", err)
		}
		if result != "2026-01-15" {
			t.Fatalf("unexpected date output: %s", result)
		}
	})

	t.Run("unknown_formatter", func(t *testing.T) {
		_, err := renderTemplate(`{{input.name | unknown}}`, context, 256, true)
		if err == nil {
			t.Fatal("expected error for unknown formatter")
		}
	})
}

func TestLegacy_Workflow_Values_ResolveJSONRef(t *testing.T) {
	context := map[string]interface{}{
		"input": map[string]interface{}{
			"deep": map[string]interface{}{
				"nested": map[string]interface{}{
					"value": "found",
				},
			},
		},
		"steps":   map[string]interface{}{},
		"secrets": map[string]interface{}{"token": "secret-value"},
		"runtime": map[string]interface{}{},
	}

	t.Run("ref_in_json_value", func(t *testing.T) {
		input := json.RawMessage(`{"result":{"$ref":"input.deep.nested.value"}}`)
		resolved, err := resolveJSON(input, context, 256, true)
		if err != nil {
			t.Fatalf("ref resolution failed: %v", err)
		}
		if !strings.Contains(string(resolved), "found") {
			t.Fatalf("ref not resolved: %s", resolved)
		}
	})

	t.Run("deeply_nested_resolution", func(t *testing.T) {
		input := json.RawMessage(`{"outer":{"inner":{"target":{"$ref":"input.deep.nested.value"}}}}`)
		resolved, err := resolveJSON(input, context, 256, true)
		if err != nil {
			t.Fatalf("deep resolution failed: %v", err)
		}
		if !strings.Contains(string(resolved), "found") {
			t.Fatalf("deep ref not resolved: %s", resolved)
		}
	})

	t.Run("string_template_with_ref", func(t *testing.T) {
		input := json.RawMessage(`"{{input.deep.nested.value}}"`)
		resolved, err := resolveJSON(input, context, 256, true)
		if err != nil {
			t.Fatalf("template ref resolution failed: %v", err)
		}
		if string(resolved) != `"found"` {
			t.Fatalf("template not resolved: %s", resolved)
		}
	})
}

func TestLegacy_Workflow_Values_SecretResolution(t *testing.T) {
	context := map[string]interface{}{
		"input":   map[string]interface{}{},
		"steps":   map[string]interface{}{},
		"secrets": map[string]interface{}{"api_key": "sk-123456"},
		"runtime": map[string]interface{}{},
	}

	t.Run("secret_in_header_allowed", func(t *testing.T) {
		input := json.RawMessage(`{"Authorization":{"$secret":"api_key"}}`)
		resolved, err := resolveJSON(input, context, 256, true)
		if err != nil {
			t.Fatalf("secret resolution failed: %v", err)
		}
		if !strings.Contains(string(resolved), "sk-123456") {
			t.Fatalf("secret not resolved in header: %s", resolved)
		}
	})

	t.Run("secret_in_body_denied", func(t *testing.T) {
		input := json.RawMessage(`{"body":{"token":{"$secret":"api_key"}}}`)
		_, err := resolveJSON(input, context, 256, false)
		if err == nil {
			t.Fatal("secret in body should be denied without allowSecrets")
		}
	})

	t.Run("secret_in_template_denied", func(t *testing.T) {
		_, err := renderTemplate("Secret is {{secrets.api_key}}", context, 256, false)
		if err == nil {
			t.Fatal("secret in visible template should be denied")
		}
	})

	t.Run("secret_in_template_allowed", func(t *testing.T) {
		result, err := renderTemplate("Bearer {{secrets.api_key}}", context, 256, true)
		if err != nil {
			t.Fatalf("secret in template failed: %v", err)
		}
		if !strings.Contains(result, "sk-123456") {
			t.Fatalf("secret not resolved in template: %s", result)
		}
	})

	t.Run("invalid_secret_name", func(t *testing.T) {
		input := json.RawMessage(`{"Authorization":{"$secret":"../token"}}`)
		_, err := resolveJSON(input, context, 256, true)
		if err == nil {
			t.Fatal("secret with invalid name should be denied")
		}
	})
}

func TestLegacy_Workflow_Compiler_HTTPMethodWhitelist(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			allowDelete := method == "DELETE"
			wf := WorkflowDefinition{
				SchemaVersion: "1.0.0",
				Steps: []WorkflowStep{
					{
						ID:    "http_step",
						Type:  "http",
						Input: json.RawMessage(fmt.Sprintf(`{"url":"https://example.com/api","method":"%s","allowDelete":%v}`, method, allowDelete)),
					},
				},
				Output: json.RawMessage(`{"$ref":"steps.http_step"}`),
				Limits: DefaultWorkflowLimits(),
			}
			_, issues, err := compiler.Compile(context.Background(), wf)
			if err != nil {
				t.Fatalf("method %s should compile: %v %#v", method, err, issues)
			}
			if len(issues) > 0 {
				t.Fatalf("unexpected issues for %s: %#v", method, issues)
			}
		})
	}

	t.Run("unsupported_method", func(t *testing.T) {
		wf := WorkflowDefinition{
			SchemaVersion: "1.0.0",
			Steps: []WorkflowStep{
				{ID: "http_step", Type: "http", Input: json.RawMessage(`{"url":"https://example.com/api","method":"OPTIONS"}`)},
			},
			Output: json.RawMessage(`{"$ref":"steps.http_step"}`),
			Limits: DefaultWorkflowLimits(),
		}
		_, issues, err := compiler.Compile(context.Background(), wf)
		if err == nil {
			t.Fatal("OPTIONS should be rejected")
		}
		if !hasIssueWithCode(issues, ErrWorkflowStepInvalid) {
			t.Fatalf("expected %s, got %#v", ErrWorkflowStepInvalid, issues)
		}
	})
}

func TestLegacy_Workflow_Executor_AdapterNotFound(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewWorkflowAdapterRegistry()

	compiled := CompiledWorkflow{
		SchemaVersion: "1.0.0",
		Steps: []CompiledStep{
			{ID: "missing", Type: "template", Input: json.RawMessage(`{"template":"x"}`), TimeoutMS: 10000, OnError: WorkflowErrorPolicy{Mode: "fail"}},
		},
		Output: json.RawMessage(`{"$ref":"steps.missing"}`),
		Limits: DefaultWorkflowLimits(),
	}
	executor := NewWorkflowExecutor(registry, validator)
	_, err = executor.Execute(context.Background(), WorkflowExecutionRequest{
		Workflow: compiled,
		Input:    json.RawMessage(`{}`),
		Config:   json.RawMessage(`{}`),
	}, nil)
	if err == nil {
		t.Fatal("expected error for missing adapter")
	}
	if asExtensionError(err).Code != ErrWorkflowStepInvalid {
		t.Fatalf("expected %s, got %v", ErrWorkflowStepInvalid, err)
	}
}

func hasIssueWithCode(issues []AnalysisIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
