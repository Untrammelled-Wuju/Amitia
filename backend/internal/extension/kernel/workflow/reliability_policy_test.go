package workflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type blockingWorkflowHandler struct{}

func (blockingWorkflowHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestCompilerMaterializesNodeReliabilityPolicy(t *testing.T) {
	def := WorkflowDefinition{
		SchemaVersion: UserWorkflowSchemaVersion,
		ID:            "wf-reliability",
		Name:          "reliability",
		InputSchema:   json.RawMessage(`{"type":"object"}`),
		OutputSchema:  json.RawMessage(`{}`),
		Limits: WorkflowLimits{
			MaxStepDurationMS: 10_000,
		},
		Nodes: []WorkflowNode{{
			ID:        "node-1",
			Type:      "tool",
			TimeoutMS: 2_500,
			Retry: &WorkflowNodeRetryPolicy{
				MaxAttempts:      4,
				InitialBackoffMS: 150,
				MaxBackoffMS:     1_200,
				Multiplier:       2.5,
				Jitter:           0.1,
			},
			Step: WorkflowStepInput{Input: json.RawMessage(`{}`)},
		}},
	}

	dag, err := NewCompiler().Compile(def, DefaultCompileOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	node := dag.Nodes["node-1"]
	if node.Timeout == nil || *node.Timeout != 2500*time.Millisecond {
		t.Fatalf("timeout = %v, want 2500ms", node.Timeout)
	}
	if node.Retry == nil {
		t.Fatal("retry policy missing")
	}
	if node.Retry.MaxAttempts != 4 || node.Retry.InitialBackoff != 150*time.Millisecond || node.Retry.MaxBackoff != 1200*time.Millisecond {
		t.Fatalf("unexpected retry policy: %+v", node.Retry)
	}
	if node.Retry.Multiplier != 2.5 || node.Retry.Jitter != 0.1 {
		t.Fatalf("unexpected retry multiplier/jitter: %+v", node.Retry)
	}
}

func TestCompilerRejectsInvalidNodeReliabilityPolicy(t *testing.T) {
	base := WorkflowDefinition{
		SchemaVersion: UserWorkflowSchemaVersion,
		ID:            "wf-invalid-reliability",
		Name:          "invalid",
		InputSchema:   json.RawMessage(`{"type":"object"}`),
		OutputSchema:  json.RawMessage(`{}`),
		Limits:        WorkflowLimits{MaxStepDurationMS: 5_000},
		Nodes: []WorkflowNode{{
			ID:   "node-1",
			Type: "tool",
			Step: WorkflowStepInput{Input: json.RawMessage(`{}`)},
		}},
	}

	tooLong := base
	tooLong.Nodes = append([]WorkflowNode(nil), base.Nodes...)
	tooLong.Nodes[0].TimeoutMS = 6_000
	if _, err := NewCompiler().Compile(tooLong, DefaultCompileOptions()); err == nil {
		t.Fatal("expected node timeout validation error")
	}

	badRetry := base
	badRetry.Nodes = append([]WorkflowNode(nil), base.Nodes...)
	badRetry.Nodes[0].Retry = &WorkflowNodeRetryPolicy{MaxAttempts: 11}
	if _, err := NewCompiler().Compile(badRetry, DefaultCompileOptions()); err == nil {
		t.Fatal("expected retry validation error")
	}
}

func TestCompiledExecutorUsesNodeTimeout(t *testing.T) {
	executor := NewWorkflowExecutor(nil)
	policy := DefaultRetryPolicy()
	policy.MaxAttempts = 1
	timeout := 25 * time.Millisecond
	start := time.Now()
	result := executor.executeStepCompiled(
		context.Background(),
		blockingWorkflowHandler{},
		WorkflowNode{ID: "node-timeout", Type: "tool"},
		json.RawMessage(`{}`),
		WorkflowLimits{MaxStepDurationMS: 5_000},
		&timeout,
		policy,
		nil,
		"",
	)
	if result.Status != "failed" || result.Error != ErrStepTimeout.Error() {
		t.Fatalf("result = %+v, want step timeout failure", result)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("node timeout was not applied promptly: %v", time.Since(start))
	}
}

type ignoresDeadlineWorkflowHandler struct{}

func (ignoresDeadlineWorkflowHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	time.Sleep(40 * time.Millisecond)
	return json.RawMessage(`{"late":true}`), nil
}

func TestCompiledExecutorTreatsLateSuccessAsTimeout(t *testing.T) {
	executor := NewWorkflowExecutor(nil)
	policy := DefaultRetryPolicy()
	policy.MaxAttempts = 1
	timeout := 10 * time.Millisecond
	result := executor.executeStepCompiled(
		context.Background(),
		ignoresDeadlineWorkflowHandler{},
		WorkflowNode{ID: "late-success", Type: "tool"},
		json.RawMessage(`{}`),
		WorkflowLimits{MaxStepDurationMS: 5_000},
		&timeout,
		policy,
		nil,
		"",
	)
	if result.Status != "failed" || result.Error != ErrStepTimeout.Error() {
		t.Fatalf("result = %+v, want deadline timeout even when handler returns nil error", result)
	}
}

type staticWorkflowHandler struct {
	output json.RawMessage
}

func (h staticWorkflowHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	return h.output, nil
}

type retryCaptureHandler struct {
	calls int
	input json.RawMessage
}

func (h *retryCaptureHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	h.calls++
	h.input = append(h.input[:0], input...)
	if h.calls < 3 {
		return nil, ErrHandlerNotFound
	}
	return json.RawMessage(`{"ok":true}`), nil
}

func TestExecuteV2AppliesInputMappingAndRetryPolicy(t *testing.T) {
	def, err := NormalizeDefinition(WorkflowDefinition{
		SchemaVersion: UserWorkflowSchemaVersion,
		ID:            "wf-v2-runtime",
		Name:          "v2 runtime",
		Enabled:       true,
		InputSchema:   json.RawMessage(`{"type":"object"}`),
		OutputSchema:  json.RawMessage(`{}`),
		Limits: WorkflowLimits{
			MaxStepDurationMS:      5_000,
			MaxExecutionDurationMS: 10_000,
			MaxConcurrency:         2,
		},
		Nodes: []WorkflowNode{
			{ID: "source", Type: "source", Step: WorkflowStepInput{Input: json.RawMessage(`{}`)}},
			{
				ID:   "target",
				Type: "target",
				Step: WorkflowStepInput{Input: json.RawMessage(`{"answer":"steps.source.value","constant":"keep"}`), OnError: WorkflowOnError{Mode: "fail"}},
				Retry: &WorkflowNodeRetryPolicy{
					MaxAttempts:      3,
					InitialBackoffMS: 1,
					MaxBackoffMS:     2,
					Multiplier:       2,
					Jitter:           0,
				},
			},
		},
		Edges: []WorkflowEdge{{ID: "source-target", Source: "source", Target: "target"}},
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	registry := NewWorkflowRegistry()
	if err := registry.Register(def); err != nil {
		t.Fatalf("register: %v", err)
	}
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("source", staticWorkflowHandler{output: json.RawMessage(`{"value":42}`)})
	target := &retryCaptureHandler{}
	executor.RegisterHandler("target", target)

	result, err := executor.Execute(context.Background(), ExecuteRequest{WorkflowID: def.ID, Input: json.RawMessage(`{"enabled":true}`)})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.Success {
		t.Fatalf("execution failed: %+v", result)
	}
	if target.calls != 3 {
		t.Fatalf("target calls = %d, want 3", target.calls)
	}
	var captured map[string]any
	if err := json.Unmarshal(target.input, &captured); err != nil {
		t.Fatalf("decode captured input: %v", err)
	}
	if captured["answer"] != float64(42) || captured["constant"] != "keep" {
		t.Fatalf("unexpected mapped input: %#v", captured)
	}
	var targetStep *StepResult
	for i := range result.Steps {
		if result.Steps[i].NodeID == "target" {
			targetStep = &result.Steps[i]
		}
	}
	if targetStep == nil || targetStep.Attempt != 3 {
		t.Fatalf("target step = %+v, want attempt 3", targetStep)
	}
}

func TestExecuteV2EvaluatesEdgeConditions(t *testing.T) {
	condition := json.RawMessage(`{"op":"eq","left":{"ref":{"source":"input","path":["enabled"]}},"right":{"value":true}}`)
	def, err := NormalizeDefinition(WorkflowDefinition{
		SchemaVersion: UserWorkflowSchemaVersion,
		ID:            "wf-v2-edge-condition",
		Name:          "v2 condition",
		Enabled:       true,
		InputSchema:   json.RawMessage(`{"type":"object"}`),
		OutputSchema:  json.RawMessage(`{}`),
		Limits: WorkflowLimits{
			MaxStepDurationMS:      5_000,
			MaxExecutionDurationMS: 10_000,
			MaxConcurrency:         2,
		},
		Nodes: []WorkflowNode{
			{ID: "source", Type: "source", Step: WorkflowStepInput{Input: json.RawMessage(`{}`)}},
			{ID: "target", Type: "target", Step: WorkflowStepInput{Input: json.RawMessage(`{}`), OnError: WorkflowOnError{Mode: "fail"}}},
		},
		Edges: []WorkflowEdge{{ID: "source-target", Source: "source", Target: "target", Condition: condition}},
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	registry := NewWorkflowRegistry()
	if err := registry.Register(def); err != nil {
		t.Fatalf("register: %v", err)
	}
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("source", staticWorkflowHandler{output: json.RawMessage(`{"value":1}`)})
	target := &countingHandler{}
	executor.RegisterHandler("target", target)

	result, err := executor.Execute(context.Background(), ExecuteRequest{WorkflowID: def.ID, Input: json.RawMessage(`{"enabled":false}`)})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.Success {
		t.Fatalf("execution failed: %+v", result)
	}
	if target.calls != 0 {
		t.Fatalf("conditional target calls = %d, want 0", target.calls)
	}
	foundSkipped := false
	for _, step := range result.Steps {
		if step.NodeID == "target" && step.Status == "skipped" {
			foundSkipped = true
		}
	}
	if !foundSkipped {
		t.Fatalf("expected target to be skipped: %+v", result.Steps)
	}
}

func TestExecuteV2EvaluatesRuntimeCondition(t *testing.T) {
	when := json.RawMessage(`{"op":"eq","left":{"ref":{"source":"runtime","path":["userId"]}},"right":{"value":"user-allowed"}}`)
	def, err := NormalizeDefinition(WorkflowDefinition{
		SchemaVersion: UserWorkflowSchemaVersion,
		ID:            "wf-v2-runtime-condition",
		Name:          "runtime condition",
		Enabled:       true,
		InputSchema:   json.RawMessage(`{"type":"object"}`),
		OutputSchema:  json.RawMessage(`{}`),
		Limits: WorkflowLimits{
			MaxStepDurationMS:      5_000,
			MaxExecutionDurationMS: 10_000,
		},
		Nodes: []WorkflowNode{{
			ID:   "guarded",
			Type: "target",
			Step: WorkflowStepInput{Input: json.RawMessage(`{}`), When: &when, OnError: WorkflowOnError{Mode: "fail"}},
		}},
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	registry := NewWorkflowRegistry()
	if err := registry.Register(def); err != nil {
		t.Fatalf("register: %v", err)
	}
	executor := NewWorkflowExecutor(registry)
	target := &countingHandler{}
	executor.RegisterHandler("target", target)

	result, err := executor.Execute(context.Background(), ExecuteRequest{
		WorkflowID: def.ID,
		Input:      json.RawMessage(`{}`),
		Context:    ExecutionContext{UserID: "user-denied"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.Success || target.calls != 0 {
		t.Fatalf("runtime condition should skip target: result=%+v calls=%d", result, target.calls)
	}
}
