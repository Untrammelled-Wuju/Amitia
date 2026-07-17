package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func workflowForTest(steps ...WorkflowStep) WorkflowDefinition {
	return WorkflowDefinition{SchemaVersion: "1.0.0", Steps: steps, Output: json.RawMessage(`{"$ref":"steps.result"}`), Limits: DefaultWorkflowLimits()}
}

func transformStepForTest() WorkflowStep {
	return WorkflowStep{ID: "result", Type: "transform", Input: json.RawMessage(`{"op":"pick","value":{"ok":true},"fields":["ok"]}`), OnError: WorkflowErrorPolicy{Mode: "fail"}}
}

func TestWorkflowCompilerStaticPolicies(t *testing.T) {
	compiler := NewWorkflowCompiler(nil)
	tests := []struct {
		name     string
		workflow WorkflowDefinition
		wantCode string
	}{
		{"valid_transform", workflowForTest(transformStepForTest()), ""},
		{"empty_workflow", WorkflowDefinition{Output: json.RawMessage(`{}`)}, "WORKFLOW_EMPTY"},
		{"duplicate_step", workflowForTest(transformStepForTest(), transformStepForTest()), "WORKFLOW_DUPLICATE_STEP_ID"},
		{"reserved_id", workflowForTest(WorkflowStep{ID: "input", Type: "template", Input: json.RawMessage(`{"template":"ok"}`)}), ErrWorkflowStepInvalid},
		{"numeric_id", workflowForTest(WorkflowStep{ID: "1bad", Type: "template", Input: json.RawMessage(`{"template":"ok"}`)}), ErrWorkflowStepInvalid},
		{"unknown_step", workflowForTest(WorkflowStep{ID: "result", Type: "script", Input: json.RawMessage(`{}`)}), ErrWorkflowStepInvalid},
		{"future_reference", workflowForTest(WorkflowStep{ID: "first", Type: "template", Input: json.RawMessage(`{"template":"{{steps.later.text}}"}`)}, WorkflowStep{ID: "later", Type: "template", Input: json.RawMessage(`{"template":"ok"}`)}), ErrWorkflowReferenceInvalid},
		{"unknown_transform", workflowForTest(WorkflowStep{ID: "result", Type: "transform", Input: json.RawMessage(`{"op":"eval","value":{}}`)}), ErrWorkflowStepInvalid},
		{"schedule_without_idempotency", workflowForTest(WorkflowStep{ID: "result", Type: "schedule", Input: json.RawMessage(`{"timezone":"Asia/Shanghai","dueAt":"2026-08-01T10:00:00+08:00"}`)}), ErrWorkflowStepInvalid},
		{"context_budget", workflowForTest(WorkflowStep{ID: "result", Type: "context_contribution", Input: json.RawMessage(`{"content":"x","tokenLimit":1025}`)}), ErrWorkshopSandboxLimit},
		{"notification_cross_scope", workflowForTest(WorkflowStep{ID: "result", Type: "notification", Input: json.RawMessage(`{"content":"x","recipient":"other_user"}`)}), ErrWorkshopSessionForbidden},
		{"memory_without_source", workflowForTest(WorkflowStep{ID: "result", Type: "memory_candidate", Input: json.RawMessage(`{"key":"k","value":"v"}`)}), ErrWorkflowStepInvalid},
		{"delete_without_high_risk_flag", workflowForTest(WorkflowStep{ID: "result", Type: "http", Input: json.RawMessage(`{"url":"https://example.com/a","method":"DELETE"}`)}), ErrWorkshopPermissionRequired},
		{"secret_in_body", workflowForTest(WorkflowStep{ID: "result", Type: "http", Input: json.RawMessage(`{"url":"https://example.com/a","method":"POST","body":{"token":{"$secret":"api_token"}}}`)}), ErrWorkshopSecretDetected},
		{"plain_authorization", workflowForTest(WorkflowStep{ID: "result", Type: "http", Input: json.RawMessage(`{"url":"https://example.com/a","headers":{"Authorization":"Bearer abcdefghijklmnop"}}`)}), ErrWorkshopSecretDetected},
		{"invalid_expected_status", workflowForTest(WorkflowStep{ID: "result", Type: "http", Input: json.RawMessage(`{"url":"https://example.com/a","expectedStatus":[700]}`)}), ErrWorkflowStepInvalid},
		{"invalid_response_type", workflowForTest(WorkflowStep{ID: "result", Type: "http", Input: json.RawMessage(`{"url":"https://example.com/a","responseType":"binary"}`)}), ErrWorkflowStepInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, issues, err := compiler.Compile(context.Background(), test.workflow)
			if test.wantCode == "" {
				if err != nil {
					t.Fatalf("unexpected compile error: %v %#v", err, issues)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %s", test.wantCode)
			}
			for _, issue := range issues {
				if issue.Code == test.wantCode {
					return
				}
			}
			t.Fatalf("missing issue %s: %#v", test.wantCode, issues)
		})
	}
}

func TestWorkflowCompilerRejectsTransitiveSkillCycle(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry("1.0.0", validator, nil)
	handler := func(context.Context, ExecuteSkillRequest) (SkillResult, error) { return SkillResult{}, nil }
	schema := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)
	dependencyC, dependencyCHandler := testDefinition(t, "dev.user.c", schema, schema, handler)
	dependencyC.Dependencies = []string{"dev.user.a"}
	if err := registry.Register(context.Background(), dependencyC, dependencyCHandler); err != nil {
		t.Fatal(err)
	}
	dependencyB, dependencyBHandler := testDefinition(t, "dev.user.b", schema, schema, handler)
	dependencyB.Dependencies = []string{"dev.user.c"}
	if err := registry.Register(context.Background(), dependencyB, dependencyBHandler); err != nil {
		t.Fatal(err)
	}
	compiler := NewWorkflowCompiler(registry)
	issues := compiler.AnalyzeDependencyCycles(context.Background(), "dev.user.a", []ResolvedSkillDependency{{SkillID: "dev.user.b"}})
	if len(issues) != 1 || issues[0].Code != ErrWorkshopDependencyCycle || !strings.Contains(issues[0].Message, "dev.user.a -> dev.user.b -> dev.user.c -> dev.user.a") {
		t.Fatalf("unexpected cycle issues: %#v", issues)
	}
}

func TestNetworkTargetPolicy(t *testing.T) {
	tests := []struct {
		name string
		url  string
		ok   bool
	}{
		{"public_https", "https://example.com/path", true},
		{"plain_http", "http://example.com", false},
		{"localhost", "https://localhost/a", false},
		{"loopback_v4", "https://127.0.0.1/a", false},
		{"private_v4", "https://10.0.0.1/a", false},
		{"link_local_v4", "https://169.254.169.254/latest/meta-data", false},
		{"loopback_v6", "https://[::1]/a", false},
		{"private_v6", "https://[fc00::1]/a", false},
		{"file_scheme", "file:///etc/passwd", false},
		{"dynamic_host", "https://{{input.host}}/a", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateNetworkTarget(test.url)
			if test.ok && err != nil {
				t.Fatal(err)
			}
			if !test.ok && err == nil {
				t.Fatal("unsafe target accepted")
			}
		})
	}
}

func TestForbiddenDraftContentMatrix(t *testing.T) {
	tests := []string{
		"package main\nfunc main() {}",
		"function run() { return 1 }",
		"#!/bin/bash\necho unsafe",
		"SELECT secret FROM users",
		"def run(value): return value",
		"<div>unsafe html</div>",
	}
	for _, value := range tests {
		t.Run(fmt.Sprintf("case_%d", len(value)), func(t *testing.T) {
			if issues := ScanWorkshopSecrets([]byte(value)); !hasIssueCode(issues, ErrWorkshopGenerationOutputInvalid) {
				t.Fatalf("forbidden content accepted: %s %#v", value, issues)
			}
		})
	}
}

func TestHTTPWorkflowAdapterMockContract(t *testing.T) {
	adapter := NewHTTPWorkflowAdapter()
	mock := HTTPMock{Method: "GET", URL: "https://example.com/a", Status: 404, ResponseBody: json.RawMessage(`{"error":"missing"}`)}
	request := WorkflowAdapterRequest{Input: json.RawMessage(`{"url":"https://example.com/a","method":"GET","expectedStatus":[200]}`), Mode: WorkflowMocked, HTTPMocks: []HTTPMock{mock}, Limits: DefaultWorkflowLimits()}
	if _, err := adapter.Execute(context.Background(), request); err == nil || !strings.Contains(err.Error(), "expectedStatus") {
		t.Fatalf("unexpected status accepted: %v", err)
	}
	request.Input = json.RawMessage(`{"url":"https://example.com/a","method":"GET","expectedStatus":[404]}`)
	result, err := adapter.Execute(context.Background(), request)
	if err != nil || !result.Mocked || !strings.Contains(string(result.Output), `"status":404`) {
		t.Fatalf("expected mocked response rejected: %s %v", result.Output, err)
	}
	strict := HTTPMock{Method: "POST", URL: "https://example.com/a", Body: json.RawMessage(`{"name":"expected"}`), Status: 200, ResponseBody: json.RawMessage(`{"ok":true}`)}
	request = WorkflowAdapterRequest{Input: json.RawMessage(`{"url":"https://example.com/a","method":"POST","body":{"name":"different"}}`), Mode: WorkflowMocked, HTTPMocks: []HTTPMock{strict}, Limits: DefaultWorkflowLimits()}
	if _, err := adapter.Execute(context.Background(), request); err == nil || !strings.Contains(err.Error(), "未命中") {
		t.Fatalf("HTTP mock accepted mismatched body: %v", err)
	}
}

func TestSkillMockControlledLiveContract(t *testing.T) {
	adapter := SkillWorkflowAdapter{}
	mock := SkillMock{SkillID: "dev.user.skill", Input: json.RawMessage(`{"name":"A"}`), Output: json.RawMessage(`{"ok":true}`), Status: RunSucceeded}
	request := WorkflowAdapterRequest{Input: json.RawMessage(`{"skillId":"dev.user.skill","input":{"name":"B"}}`), Mode: WorkflowControlledLive, SkillMocks: []SkillMock{mock}, Limits: DefaultWorkflowLimits()}
	if _, err := adapter.Execute(context.Background(), request); err == nil || !strings.Contains(err.Error(), "未命中") {
		t.Fatalf("Skill mock accepted mismatched input: %v", err)
	}
	request.Input = json.RawMessage(`{"skillId":"dev.user.skill","input":{"name":"A"}}`)
	result, err := adapter.Execute(context.Background(), request)
	if err != nil || !result.Mocked || !strings.Contains(string(result.Output), `"ok":true`) {
		t.Fatalf("Skill controlled mock failed: %s %v", result.Output, err)
	}
}

func TestHTTPCompiledStepUsesRequestedTimeout(t *testing.T) {
	workflow := workflowForTest(WorkflowStep{ID: "result", Type: "http", Input: json.RawMessage(`{"url":"https://example.com/a","timeoutMs":1250}`), OnError: WorkflowErrorPolicy{Mode: "fail"}})
	compiled, issues, err := NewWorkflowCompiler(nil).Compile(context.Background(), workflow)
	if err != nil || len(issues) != 0 || len(compiled.Steps) != 1 || compiled.Steps[0].TimeoutMS != 1250 {
		t.Fatalf("HTTP timeout was not compiled: %#v %#v %v", compiled.Steps, issues, err)
	}
}

func TestRestrictedValuesAndSecrets(t *testing.T) {
	values := map[string]interface{}{"input": map[string]interface{}{"name": "A"}, "secrets": map[string]interface{}{"token": "top-secret-value"}, "steps": map[string]interface{}{}, "runtime": map[string]interface{}{"traceId": "trace"}}
	t.Run("secret_header_resolution", func(t *testing.T) {
		resolved, err := resolveJSON(json.RawMessage(`{"Authorization":{"$secret":"token"}}`), values, 256, true)
		if err != nil || !strings.Contains(string(resolved), "top-secret-value") {
			t.Fatalf("secret not resolved: %s %v", resolved, err)
		}
	})
	t.Run("secret_visible_output_denied", func(t *testing.T) {
		if _, err := resolveJSON(json.RawMessage(`{"value":{"$secret":"token"}}`), values, 256, false); err == nil {
			t.Fatal("secret output accepted")
		}
	})
	t.Run("prototype_path_denied", func(t *testing.T) {
		if _, err := resolveReference("input.__proto__.value", values); err == nil {
			t.Fatal("prototype path accepted")
		}
	})
	t.Run("template_function_denied", func(t *testing.T) {
		if _, err := renderTemplate("{{input.name()}}", values, 256, false); err == nil {
			t.Fatal("function-like template accepted")
		}
	})
	t.Run("condition_type_error", func(t *testing.T) {
		_, err := evalCondition(&ConditionExpression{Op: "gt", Left: "not-number", Right: 1}, values, 8)
		if err == nil {
			t.Fatal("type mismatch accepted")
		}
	})
}

func TestConditionOperatorMatrix(t *testing.T) {
	truth := ConditionExpression{Op: "eq", Left: 1, Right: 1}
	falsehood := ConditionExpression{Op: "eq", Left: 1, Right: 2}
	tests := []struct {
		name       string
		expression ConditionExpression
	}{
		{"eq", truth},
		{"neq", ConditionExpression{Op: "neq", Left: 1, Right: 2}},
		{"gt", ConditionExpression{Op: "gt", Left: 2, Right: 1}},
		{"gte", ConditionExpression{Op: "gte", Left: 2, Right: 2}},
		{"lt", ConditionExpression{Op: "lt", Left: 1, Right: 2}},
		{"lte", ConditionExpression{Op: "lte", Left: 2, Right: 2}},
		{"and", ConditionExpression{Op: "and", Args: []ConditionExpression{truth, truth}}},
		{"or", ConditionExpression{Op: "or", Args: []ConditionExpression{falsehood, truth}}},
		{"not", ConditionExpression{Op: "not", Args: []ConditionExpression{falsehood}}},
		{"exists", ConditionExpression{Op: "exists", Value: map[string]interface{}{"ref": "input.name"}}},
		{"empty", ConditionExpression{Op: "empty", Value: ""}},
		{"contains", ConditionExpression{Op: "contains", Left: "amitia-workshop", Right: "workshop"}},
		{"starts_with", ConditionExpression{Op: "starts_with", Left: "amitia", Right: "ami"}},
		{"ends_with", ConditionExpression{Op: "ends_with", Left: "amitia", Right: "tia"}},
		{"in", ConditionExpression{Op: "in", Left: "b", Right: []interface{}{"a", "b"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matched, err := evalCondition(&test.expression, map[string]interface{}{"input": map[string]interface{}{"name": "A"}}, 16)
			if err != nil || !matched {
				t.Fatalf("operator failed: %v %v", matched, err)
			}
		})
	}
}

func TestTransformOperationMatrix(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
	}{
		{"pick", map[string]interface{}{"op": "pick", "value": map[string]interface{}{"a": 1, "b": 2}, "fields": []interface{}{"a"}}},
		{"omit", map[string]interface{}{"op": "omit", "value": map[string]interface{}{"a": 1, "b": 2}, "fields": []interface{}{"b"}}},
		{"rename", map[string]interface{}{"op": "rename", "value": map[string]interface{}{"a": 1}, "mapping": map[string]interface{}{"a": "b"}}},
		{"set", map[string]interface{}{"op": "set", "value": map[string]interface{}{"a": 1}, "values": map[string]interface{}{"b": 2}}},
		{"merge", map[string]interface{}{"op": "merge", "value": map[string]interface{}{"a": 1}, "with": map[string]interface{}{"b": 2}}},
		{"flatten", map[string]interface{}{"op": "flatten", "value": []interface{}{[]interface{}{1, 2}, 3}}},
		{"array_map", map[string]interface{}{"op": "array_map", "value": []interface{}{map[string]interface{}{"name": "a", "id": 1}}, "fields": []interface{}{"name"}}},
		{"array_filter", map[string]interface{}{"op": "array_filter", "value": []interface{}{map[string]interface{}{"score": 2}, map[string]interface{}{"score": 1}}, "field": "score", "operator": "gte", "expected": 2}},
		{"array_take", map[string]interface{}{"op": "array_take", "value": []interface{}{1, 2, 3}, "count": 2}},
		{"array_sort", map[string]interface{}{"op": "array_sort", "value": []interface{}{map[string]interface{}{"name": "b"}, map[string]interface{}{"name": "a"}}, "field": "name"}},
		{"to_string", map[string]interface{}{"op": "to_string", "value": 12}},
		{"to_number", map[string]interface{}{"op": "to_number", "value": "12.5"}},
		{"to_boolean", map[string]interface{}{"op": "to_boolean", "value": "true"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := transformJSON(test.input, map[string]interface{}{}, 100)
			if err != nil || result == nil {
				t.Fatalf("transform failed: %#v %v", result, err)
			}
		})
	}
}

func TestWorkflowHostLimitClamping(t *testing.T) {
	host := DefaultWorkflowLimits()
	tests := []struct {
		name   string
		limits WorkflowLimits
		check  func(WorkflowLimits) bool
	}{
		{"steps", WorkflowLimits{MaxSteps: host.MaxSteps + 100}, func(value WorkflowLimits) bool { return value.MaxSteps == host.MaxSteps }},
		{"duration", WorkflowLimits{MaxExecutionDurationMS: host.MaxExecutionDurationMS + 1000}, func(value WorkflowLimits) bool { return value.MaxExecutionDurationMS == host.MaxExecutionDurationMS }},
		{"response", WorkflowLimits{MaxHTTPResponseBytes: host.MaxHTTPResponseBytes + 1}, func(value WorkflowLimits) bool { return value.MaxHTTPResponseBytes == host.MaxHTTPResponseBytes }},
		{"depth", WorkflowLimits{MaxSkillCallDepth: host.MaxSkillCallDepth + 1}, func(value WorkflowLimits) bool { return value.MaxSkillCallDepth == host.MaxSkillCallDepth }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if value := effectiveWorkflowLimits(test.limits); !test.check(value) {
				t.Fatalf("host limit was expanded: %#v", value)
			}
		})
	}
}

func TestWorkshopVersionSuggestion(t *testing.T) {
	baseSchema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)
	current := SkillDefinition{Version: "1.2.3", InputSchema: baseSchema, OutputSchema: baseSchema, ConfigSchema: json.RawMessage(`{"type":"object","properties":{}}`), Capabilities: []string{}}
	t.Run("patch_for_behavior_only", func(t *testing.T) {
		draft := ExtensionDraft{InputSchema: baseSchema, OutputSchema: baseSchema, ConfigSchema: current.ConfigSchema, Capabilities: []string{}}
		version, breaking := suggestWorkshopVersion(current, draft)
		if version != "1.2.4" || len(breaking) != 0 {
			t.Fatalf("unexpected patch suggestion: %s %#v", version, breaking)
		}
	})
	t.Run("minor_for_additive_schema", func(t *testing.T) {
		draft := ExtensionDraft{InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"locale":{"type":"string"}},"required":["name"]}`), OutputSchema: baseSchema, ConfigSchema: current.ConfigSchema, Capabilities: []string{}}
		version, breaking := suggestWorkshopVersion(current, draft)
		if version != "1.3.0" || len(breaking) != 0 {
			t.Fatalf("unexpected minor suggestion: %s %#v", version, breaking)
		}
	})
	t.Run("major_for_new_required_input", func(t *testing.T) {
		draft := ExtensionDraft{InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"locale":{"type":"string"}},"required":["name","locale"]}`), OutputSchema: baseSchema, ConfigSchema: current.ConfigSchema, Capabilities: []string{}}
		version, breaking := suggestWorkshopVersion(current, draft)
		if version != "2.0.0" || len(breaking) == 0 {
			t.Fatalf("breaking input was not major: %s %#v", version, breaking)
		}
	})
	t.Run("major_for_output_type_change", func(t *testing.T) {
		draft := ExtensionDraft{InputSchema: baseSchema, OutputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"number"}},"required":["name"]}`), ConfigSchema: current.ConfigSchema, Capabilities: []string{}}
		version, breaking := suggestWorkshopVersion(current, draft)
		if version != "2.0.0" || len(breaking) == 0 {
			t.Fatalf("breaking output was not major: %s %#v", version, breaking)
		}
	})
}

func TestWorkshopAssertionSchemaValidation(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	result := WorkflowExecutionResult{Output: json.RawMessage(`{"ok":true}`)}
	valid := evaluateAssertions([]TestAssertion{{Type: "matches_schema", Expected: map[string]interface{}{"type": "object", "required": []interface{}{"ok"}, "properties": map[string]interface{}{"ok": map[string]interface{}{"type": "boolean"}}}}}, result, validator)
	invalid := evaluateAssertions([]TestAssertion{{Type: "matches_schema", Expected: map[string]interface{}{"type": "array"}}}, result, validator)
	if len(valid) != 1 || !valid[0].Passed || len(invalid) != 1 || invalid[0].Passed {
		t.Fatalf("schema assertion contract failed: %#v %#v", valid, invalid)
	}
}

func TestWorkshopTestReportRedaction(t *testing.T) {
	report := WorkshopTestReport{Output: json.RawMessage(`{"token":"top-secret-value","message":"safe"}`), Error: NewExtensionError(ErrWorkshopTestFailed, "failed", "authorization=Bearer-secret-value", false, nil), StepResults: []WorkflowStepResult{{Error: NewExtensionError(ErrWorkshopTestFailed, "failed", "password=secret-value", false, nil)}}}
	redacted := redactWorkshopTestReport(report)
	raw, _ := json.Marshal(redacted)
	if strings.Contains(string(raw), "top-secret-value") || strings.Contains(string(raw), "Bearer-secret-value") || strings.Contains(string(raw), "password=secret-value") || !strings.Contains(string(raw), "[REDACTED]") {
		t.Fatalf("test report leaked secret: %s", raw)
	}
}

type workshopTestAdapter struct {
	output json.RawMessage
	err    error
	delay  time.Duration
	panic  bool
}

type workshopModelSequence struct {
	outputs []string
	errors  []error
	calls   int
	prompts []string
}

func (m *workshopModelSequence) GenerateWorkshopJSON(_ context.Context, system, user string) (string, string, string, error) {
	index := m.calls
	m.calls++
	m.prompts = append(m.prompts, system+"\n"+user)
	if index < len(m.errors) && m.errors[index] != nil {
		return "", "test-provider", "test-model", m.errors[index]
	}
	if index >= len(m.outputs) {
		return "", "test-provider", "test-model", fmt.Errorf("no output")
	}
	return m.outputs[index], "test-provider", "test-model", nil
}

func validWorkshopPlanJSON() string {
	return `{"goal":"生成问候","inputs":[],"outputs":[],"configs":[],"steps":[{"id":"result","type":"transform","purpose":"生成结果"}],"dependencies":[],"capabilities":[],"sideEffects":[],"risks":[],"missingDetails":[],"assumptions":[]}`
}

func TestWorkshopPlannerAndGeneratorPipeline(t *testing.T) {
	draftRaw, _ := json.Marshal(integrationDraft())
	t.Run("plan_then_draft", func(t *testing.T) {
		model := &workshopModelSequence{outputs: []string{validWorkshopPlanJSON(), string(draftRaw)}}
		draft, plan, _, provider, modelName, err := NewWorkshopGenerator(model, nil).Generate(context.Background(), "生成问候")
		if err != nil || plan.Goal == "" || draft.Metadata.Name == "" || provider != "test-provider" || modelName != "test-model" || model.calls != 2 || !strings.Contains(model.prompts[1], `"plan"`) {
			t.Fatalf("pipeline failed: %#v %#v %s %s %d %v", plan, draft.Metadata, provider, modelName, model.calls, err)
		}
	})
	t.Run("bounded_retries_with_feedback", func(t *testing.T) {
		model := &workshopModelSequence{outputs: []string{`not-json`, validWorkshopPlanJSON(), `{"draftVersion":"1"}`, string(draftRaw)}}
		_, _, _, _, _, err := NewWorkshopGenerator(model, nil).Generate(context.Background(), "生成问候")
		if err != nil || model.calls != 4 || !strings.Contains(model.prompts[1], "上次规划输出无效") || !strings.Contains(model.prompts[3], "上次 Draft 输出无效") {
			t.Fatalf("retry contract failed: %d %v %#v", model.calls, err, model.prompts)
		}
	})
	t.Run("invalid_plan_exhausts_limit", func(t *testing.T) {
		model := &workshopModelSequence{outputs: []string{`{}`, `{}`, `{}`}}
		_, _, _, _, _, err := NewWorkshopGenerator(model, nil).Generate(context.Background(), "生成问候")
		if err == nil || model.calls != 3 {
			t.Fatalf("invalid plan retry limit failed: %d %v", model.calls, err)
		}
	})
	t.Run("forbidden_requirement_never_reaches_model", func(t *testing.T) {
		model := &workshopModelSequence{}
		_, _, _, _, _, err := NewWorkshopGenerator(model, nil).Generate(context.Background(), "请使用 bash 执行脚本")
		if err == nil || model.calls != 0 {
			t.Fatalf("forbidden requirement reached model: %d %v", model.calls, err)
		}
	})
}

func TestWorkshopPlanValidationMatrix(t *testing.T) {
	base := WorkshopPlan{Goal: "goal", Steps: []PlannedStep{{ID: "result", Type: "transform"}}}
	tests := []struct {
		name      string
		mutate    func(WorkshopPlan) WorkshopPlan
		available map[string]bool
	}{
		{"missing_goal", func(value WorkshopPlan) WorkshopPlan { value.Goal = ""; return value }, nil},
		{"unknown_step", func(value WorkshopPlan) WorkshopPlan { value.Steps[0].Type = "script"; return value }, nil},
		{"invalid_step_id", func(value WorkshopPlan) WorkshopPlan { value.Steps[0].ID = "1bad"; return value }, nil},
		{"unknown_dependency", func(value WorkshopPlan) WorkshopPlan { value.Dependencies = []string{"missing"}; return value }, map[string]bool{}},
		{"unknown_capability", func(value WorkshopPlan) WorkshopPlan { value.Capabilities = []string{"root.access"}; return value }, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateWorkshopPlan(test.mutate(base), test.available); err == nil {
				t.Fatal("invalid plan accepted")
			}
		})
	}
}

func TestPlannedStepAcceptsDescriptionAliasAndRejectsOtherFields(t *testing.T) {
	var step PlannedStep
	if err := json.Unmarshal([]byte(`{"id":"result","type":"transform","description":"生成结果"}`), &step); err != nil {
		t.Fatal(err)
	}
	if step.Purpose != "生成结果" {
		t.Fatalf("purpose = %q", step.Purpose)
	}
	if err := json.Unmarshal([]byte(`{"id":"result","type":"transform","unexpected":true}`), &step); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

func TestWorkshopPlanAcceptsFalseSideEffectsAndRejectsTrue(t *testing.T) {
	base := `{"goal":"goal","inputs":[],"outputs":[],"configs":[],"steps":[{"id":"result","type":"transform","purpose":"result"}],"dependencies":[],"capabilities":[],"sideEffects":%s,"risks":[],"missingDetails":[],"assumptions":[]}`
	var plan WorkshopPlan
	if err := json.Unmarshal([]byte(fmt.Sprintf(base, "false")), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.SideEffects == nil || len(plan.SideEffects) != 0 {
		t.Fatalf("sideEffects = %#v", plan.SideEffects)
	}
	if err := json.Unmarshal([]byte(fmt.Sprintf(base, "true")), &plan); err == nil {
		t.Fatal("expected true sideEffects rejection")
	}
}

func TestDraftMessagesAcceptStringAliasesAndRejectUnknownObjectFields(t *testing.T) {
	var assumption DraftAssumption
	if err := json.Unmarshal([]byte(`"assumed"`), &assumption); err != nil || assumption.Message != "assumed" {
		t.Fatalf("assumption = %#v, err = %v", assumption, err)
	}
	var warning DraftWarning
	if err := json.Unmarshal([]byte(`"warning"`), &warning); err != nil || warning.Code != "MODEL_WARNING" || warning.Message != "warning" {
		t.Fatalf("warning = %#v, err = %v", warning, err)
	}
	if err := json.Unmarshal([]byte(`{"message":"x","unknown":true}`), &assumption); err == nil {
		t.Fatal("expected assumption unknown field rejection")
	}
	if err := json.Unmarshal([]byte(`{"code":"X","message":"x","unknown":true}`), &warning); err == nil {
		t.Fatal("expected warning unknown field rejection")
	}
}

func TestWorkshopMetricsExposeAllRequiredCounters(t *testing.T) {
	resetWorkshopMetrics()
	defer resetWorkshopMetrics()
	incrementWorkshopMetric(WorkshopMetricSessionCreated)
	recordWorkshopErrorMetric(NewExtensionError(ErrWorkshopNetworkDenied, "denied", "", false, nil))
	snapshot := WorkshopMetricsSnapshot()
	if len(snapshot) != len(workshopMetricNames) {
		t.Fatalf("metric count = %d", len(snapshot))
	}
	if snapshot[WorkshopMetricSessionCreated] != 1 || snapshot[WorkshopMetricNetworkDenied] != 1 {
		t.Fatalf("unexpected metrics: %#v", snapshot)
	}
	for _, name := range workshopMetricNames {
		if _, ok := snapshot[name]; !ok {
			t.Fatalf("missing metric %s", name)
		}
	}
}

func (a workshopTestAdapter) Execute(ctx context.Context, _ WorkflowAdapterRequest) (WorkflowAdapterResult, error) {
	if a.panic {
		panic("adapter panic")
	}
	if a.delay > 0 {
		select {
		case <-ctx.Done():
			return WorkflowAdapterResult{}, ctx.Err()
		case <-time.After(a.delay):
		}
	}
	return WorkflowAdapterResult{Output: a.output}, a.err
}

func TestWorkflowExecutorIsolationAndLimits(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatal(err)
	}
	makeExecutor := func(adapter WorkflowStepAdapter) *WorkflowExecutor {
		registry := NewWorkflowAdapterRegistry()
		if err := registry.Register("template", adapter); err != nil {
			t.Fatal(err)
		}
		return NewWorkflowExecutor(registry, validator)
	}
	compiled := CompiledWorkflow{SchemaVersion: "1.0.0", Steps: []CompiledStep{{ID: "result", Type: "template", Input: json.RawMessage(`{"template":"ok"}`), OnError: WorkflowErrorPolicy{Mode: "fail"}, TimeoutMS: 20}}, Output: json.RawMessage(`{"$ref":"steps.result"}`), Limits: DefaultWorkflowLimits()}
	outputSchema := json.RawMessage(`{"type":"object"}`)
	t.Run("success", func(t *testing.T) {
		result, err := makeExecutor(workshopTestAdapter{output: json.RawMessage(`{"ok":true}`)}).Execute(context.Background(), WorkflowExecutionRequest{Workflow: compiled, Input: json.RawMessage(`{}`), Config: json.RawMessage(`{}`), Mode: WorkflowDryRun}, outputSchema)
		if err != nil || !json.Valid(result.Output) || len(result.Steps) != 1 {
			t.Fatalf("unexpected result: %#v %v", result, err)
		}
	})
	t.Run("step_timeout", func(t *testing.T) {
		_, err := makeExecutor(workshopTestAdapter{delay: 100 * time.Millisecond}).Execute(context.Background(), WorkflowExecutionRequest{Workflow: compiled, Input: json.RawMessage(`{}`), Config: json.RawMessage(`{}`)}, outputSchema)
		if err == nil {
			t.Fatal("timeout not enforced")
		}
	})
	t.Run("panic_recovered", func(t *testing.T) {
		_, err := makeExecutor(workshopTestAdapter{panic: true}).Execute(context.Background(), WorkflowExecutionRequest{Workflow: compiled, Input: json.RawMessage(`{}`), Config: json.RawMessage(`{}`)}, outputSchema)
		if err == nil || !strings.Contains(err.Error(), "异常") {
			t.Fatalf("panic not recovered: %v", err)
		}
	})
	t.Run("context_cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := makeExecutor(workshopTestAdapter{output: json.RawMessage(`{}`)}).Execute(ctx, WorkflowExecutionRequest{Workflow: compiled, Input: json.RawMessage(`{}`), Config: json.RawMessage(`{}`)}, outputSchema)
		if err == nil {
			t.Fatal("cancelled context accepted")
		}
	})
	t.Run("output_schema", func(t *testing.T) {
		_, err := makeExecutor(workshopTestAdapter{output: json.RawMessage(`{"ok":true}`)}).Execute(context.Background(), WorkflowExecutionRequest{Workflow: compiled, Input: json.RawMessage(`{}`), Config: json.RawMessage(`{}`)}, json.RawMessage(`{"type":"array"}`))
		if err == nil {
			t.Fatal("invalid output accepted")
		}
	})
}

func TestControlledLiveBlocksHostSideEffects(t *testing.T) {
	called := false
	host := &WorkflowHostAdapter{Notification: func(context.Context, json.RawMessage, ExecutionScope) (json.RawMessage, []SideEffectRecord, error) {
		called = true
		return json.RawMessage(`{"sent":true}`), nil, nil
	}}
	adapter := SideEffectWorkflowAdapter{kind: "notification", host: host}
	result, err := adapter.Execute(context.Background(), WorkflowAdapterRequest{Input: json.RawMessage(`{"content":"x"}`), Mode: WorkflowControlledLive})
	if err != nil || called || len(result.SideEffects) != 1 || result.SideEffects[0].Confirmed {
		t.Fatalf("controlled live side effect escaped: %#v %v", result, err)
	}
}

func TestWorkshopStateMachine(t *testing.T) {
	valid := [][2]WorkshopSessionStatus{{WorkshopDraft, WorkshopGenerating}, {WorkshopGenerating, WorkshopGenerated}, {WorkshopGenerated, WorkshopValidating}, {WorkshopValidating, WorkshopValidated}, {WorkshopValidated, WorkshopAwaitingPermissions}, {WorkshopAwaitingPermissions, WorkshopTesting}, {WorkshopTesting, WorkshopTestPassed}, {WorkshopTestPassed, WorkshopInstalling}, {WorkshopInstalling, WorkshopInstalled}, {WorkshopInstalled, WorkshopArchived}}
	for _, transition := range valid {
		if !validWorkshopTransition(transition[0], transition[1]) {
			t.Fatalf("valid transition rejected: %s -> %s", transition[0], transition[1])
		}
	}
	invalid := [][2]WorkshopSessionStatus{{WorkshopDraft, WorkshopInstalled}, {WorkshopArchived, WorkshopGenerating}, {WorkshopGenerating, WorkshopEnabled}, {WorkshopValidated, WorkshopInstalled}, {WorkshopTesting, WorkshopArchived}}
	for _, transition := range invalid {
		if validWorkshopTransition(transition[0], transition[1]) {
			t.Fatalf("invalid transition accepted: %s -> %s", transition[0], transition[1])
		}
	}
}

func TestWorkflowSecretConfigIsolation(t *testing.T) {
	schemas := `{"config":{"type":"object","properties":{"endpoint":{"type":"string"},"api_token":{"type":"string","writeOnly":true,"format":"password"}}}}`
	config, secrets := splitWorkflowConfig(json.RawMessage(`{"endpoint":"https://example.com","api_token":"sensitive-value"}`), schemas)
	if strings.Contains(string(config), "sensitive-value") || secrets["api_token"] != "sensitive-value" {
		t.Fatalf("secret was not isolated: %s %#v", config, secrets)
	}
	if !workshopSecretFields(json.RawMessage(`{"type":"object","properties":{"api_token":{"writeOnly":true}}}`))["api_token"] {
		t.Fatal("writeOnly secret declaration not recognized")
	}
}
