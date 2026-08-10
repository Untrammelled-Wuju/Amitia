package execution

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/observability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

var (
	_testAuditStore  *observability.MemoryStore
	_testAuditWriter observability.RecordWriter
)

func newTestTool(id string) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           id,
		ModelName:    "test_tool",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Test Tool",
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Enabled:      true,
		Compatible:   true,
		Retryable:    false,
		TimeoutMS:    5000,
		ToolVersion:  capability.ToolVersion{SchemaVersion: 1, Revision: "1"},
		State: capability.ToolState{
			Installed:         true,
			ModuleEnabled:     true,
			CapabilityEnabled: true,
			ScopeAllowed:      true,
			PermissionGranted: true,
			RuntimeReady:      true,
			DependencyReady:   true,
			Health:            capability.HealthReady,
		},
		ModelExposure: capability.ModelExposureRule{
			ExposedByDefault: true,
			Priority:         10,
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:        30 * time.Second,
			MaxConcurrency: 10,
			RetryPolicy: capability.RetryPolicy{
				MaxRetries:  3,
				BackoffBase: 100 * time.Millisecond,
			},
			Idempotent:       false,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         10,
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeBuiltin,
			RuntimeID:   "test",
		},
	}
}

func newTestInvocation(userID string) capability.ToolInvocationContext {
	return capability.ToolInvocationContext{
		InvocationID:   "inv-001",
		RootID:         "inv-001",
		TraceID:        "trace-001",
		OperationID:    "op-001",
		UserID:         userID,
		CharacterID:    "char-001",
		ConversationID: "conv-001",
		Source:         capability.InvocationSourceModel,
		ApprovalMode:   capability.ApprovalModeAuto,
	}
}

type testAdapter struct {
	result capability.UnifiedToolResult
	mu     sync.Mutex
	calls  int
}

func (a *testAdapter) Supports(binding capability.RuntimeBinding) bool {
	return true
}

func (a *testAdapter) Execute(ctx context.Context, binding capability.RuntimeBinding, inv capability.ToolInvocationContext, input json.RawMessage) capability.UnifiedToolResult {
	a.mu.Lock()
	a.calls++
	a.mu.Unlock()
	return a.result
}

func (a *testAdapter) Health(ctx context.Context, binding capability.RuntimeBinding) capability.HealthStatus {
	return capability.HealthReady
}

func (a *testAdapter) CallCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

func buildPipeline(adapter capability.RuntimeAdapter) *ExecutionPipeline {
	adapterRegistry := capability.NewRuntimeAdapterRegistry()
	adapterRegistry.Register(capability.RuntimeTypeBuiltin, adapter)

	mockScopeMgr := &mockScopeManager{}
	mockBroker := &mockPermissionBroker{}
	scopeStore := &mockScopeStore{}
	permSnapStore := &mockPermissionSnapshotStore{}

	p := &ExecutionPipeline{
		InvocationValidator: NewInvocationValidator(),
		InputValidator:      NewInputValidator(),
		AvailabilityGate:    NewAvailabilityGate(nil),
		ScopeGate:           NewScopeGate(),
		PermissionGate:      NewPermissionGate(),
		ApprovalGate:        NewApprovalGate(),
		ConcurrencyCtrl:     NewConcurrencyController(),
		RateLimiter:         NewRateLimiter(),
		IdempotencyGuard:    NewIdempotencyGuard(),
		RetryCtrl:           NewRetryController(),
		TimeoutCtrl:         NewTimeoutController(5 * time.Second),
		CancellationCtrl:    NewCancellationController(),
		DepthGuard:          NewDepthGuard(),
		Dispatcher:          NewRuntimeDispatcher(adapterRegistry),
		ResultValidator:     NewResultValidator(),
		Sanitizer:           NewSanitizer(),
		SideEffectRec:       NewSideEffectRecorder(),
	AuditSink: func() observability.ExecutionRecorder {
		_testAuditStore = observability.NewMemoryStore()
		_testAuditWriter = observability.NewRecordWriter(_testAuditStore, observability.DefaultWriterConfig())
		return observability.NewExecutionHook(_testAuditWriter, nil)
	}(),
		MetricsRec: NewMetricsRecorder(),
		CircuitBreaker:      NewCircuitBreakerCoordinator(),
		ToolResolver: func(ctx context.Context, toolID string) (capability.ToolDefinition, error) {
			return newTestTool(toolID), nil
		},
		ScopeStore:             scopeStore,
		PermissionSnapshotStore: permSnapStore,
	}
	p.ScopeGate.ScopeManager = mockScopeMgr
	p.PermissionGate.Broker = mockBroker
	return p
}

type mockScopeManager struct {
	denyAll bool
}

func (m *mockScopeManager) Bind(_ context.Context, _ scope.ScopeBindRequest) (scope.ScopeBinding, error) {
	return scope.ScopeBinding{}, nil
}
func (m *mockScopeManager) Unbind(_ context.Context, _ string) error { return nil }
func (m *mockScopeManager) Evaluate(_ context.Context, _ scope.ScopeEvaluationRequest) scope.ScopeDecision {
	if m.denyAll {
		return scope.ScopeDecision{Allowed: false, Reasons: []scope.ScopeReason{{Code: "test_deny"}}}
	}
	return scope.ScopeDecision{Allowed: true}
}
func (m *mockScopeManager) Snapshot(_ context.Context, req scope.ScopeResolveRequest) (scope.ScopeSnapshot, error) {
	return scope.CreateSnapshot(req.InvocationID, []scope.ScopeRef{
		scope.NewGlobalScope(),
	}, req.CharacterID, req.ConversationID, req.ExtensionID, req.ModuleID, req.Generation), nil
}
func (m *mockScopeManager) Invalidate(_ context.Context, _ scope.ScopeInvalidationFilter) error {
	return nil
}
func (m *mockScopeManager) ListBindings(_ context.Context, _ scope.ScopeBindingFilter) ([]scope.ScopeBinding, error) {
	return nil, nil
}

type mockPermissionBroker struct {
	evalCalls int
}

func (m *mockPermissionBroker) Evaluate(_ context.Context, req permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
	m.evalCalls++
	if len(req.Requirements) == 0 {
		return permission.PermissionEvaluationResult{Decision: permission.DecisionAllow}
	}
	return permission.PermissionEvaluationResult{Decision: permission.DecisionAllow, MatchedGrants: []permission.PermissionGrant{}}
}
func (m *mockPermissionBroker) Grant(_ context.Context, _ permission.PermissionGrantRequest) (permission.PermissionGrant, error) {
	return permission.PermissionGrant{}, nil
}
func (m *mockPermissionBroker) Revoke(_ context.Context, _ string) error { return nil }
func (m *mockPermissionBroker) RevokeBySubject(_ context.Context, _ permission.PermissionSubject) (int, error) {
	return 0, nil
}
func (m *mockPermissionBroker) RevokeByExtension(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (m *mockPermissionBroker) ListGrants(_ context.Context, _ permission.PermissionGrantFilter) ([]permission.PermissionGrant, error) {
	return nil, nil
}
func (m *mockPermissionBroker) Explain(_ context.Context, req permission.PermissionEvaluationRequest) permission.PermissionExplanation {
	return permission.PermissionExplanation{Decision: permission.DecisionAllow}
}
func (m *mockPermissionBroker) DetectUpgrade(_ context.Context, _, _ []permission.PermissionRequirement) []permission.PermissionUpgrade {
	return nil
}
func (m *mockPermissionBroker) RecordApproval(_ context.Context, _ permission.PermissionApprovalRecordRequest) (permission.PermissionApprovalRecord, error) {
	return permission.PermissionApprovalRecord{}, nil
}
func (m *mockPermissionBroker) ValidateSnapshot(_ context.Context, _ string, _ permission.PermissionEvaluationRequest) error {
	return nil
}

type mockScopeStore struct{}

func (m *mockScopeStore) SaveBinding(_ context.Context, _ scope.ScopeBinding) error { return nil }
func (m *mockScopeStore) GetBinding(_ context.Context, _ string) (scope.ScopeBinding, error) {
	return scope.ScopeBinding{}, errors.New("not found")
}
func (m *mockScopeStore) DeleteBinding(_ context.Context, _ string) error { return nil }
func (m *mockScopeStore) ListBindings(_ context.Context, _ scope.ScopeBindingFilter) ([]scope.ScopeBinding, error) {
	return nil, nil
}
func (m *mockScopeStore) SaveSnapshot(_ context.Context, _ scope.ScopeSnapshot) error { return nil }
func (m *mockScopeStore) GetSnapshot(_ context.Context, _ string) (scope.ScopeSnapshot, error) {
	return scope.ScopeSnapshot{SnapshotID: "found"}, nil
}
func (m *mockScopeStore) DeleteSnapshot(_ context.Context, _ string) error { return nil }
func (m *mockScopeStore) DeleteSnapshotsBySession(_ context.Context, _ string) error { return nil }

type mockPermissionSnapshotStore struct{}

func (m *mockPermissionSnapshotStore) SaveSnapshot(_ context.Context, _ permission.PermissionSnapshot) error {
	return nil
}
func (m *mockPermissionSnapshotStore) GetSnapshot(_ context.Context, _ string) (permission.PermissionSnapshot, error) {
	return permission.PermissionSnapshot{SnapshotID: "found"}, nil
}
func (m *mockPermissionSnapshotStore) DeleteSnapshot(_ context.Context, _ string) error { return nil }
func (m *mockPermissionSnapshotStore) RevokeSnapshot(_ context.Context, _ string) error { return nil }
func (m *mockPermissionSnapshotStore) DeleteBySession(_ context.Context, _ string) error { return nil }

func TestPipelineSuccessPath(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "hello"},
			},
		},
	}

	p := buildPipeline(adapter)
	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{"key":"value"}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if adapter.CallCount() != 1 {
		t.Fatalf("expected 1 adapter call, got %d", adapter.CallCount())
	}
	if _testAuditStore == nil {
		t.Fatal("expected audit store to be initialized")
	}
	_ = _testAuditWriter.Flush(ctx)
	invList, _, _ := _testAuditStore.ListInvocations(ctx, observability.InvocationFilter{})
	if len(invList) == 0 {
		t.Fatal("expected at least 1 invocation audit entry")
	}
}

func TestPipelineInputValidationShortCircuit(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "ok"},
			},
		},
	}

	p := buildPipeline(adapter)
	p.InputValidator.MaxInputBytes = 5

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`"too large input"`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != capability.ErrorCodeInvalidInput {
		t.Fatalf("expected invalid_input error, got %+v", result.Error)
	}
	if adapter.CallCount() != 0 {
		t.Fatalf("expected 0 adapter calls (short-circuit), got %d", adapter.CallCount())
	}
}

func TestPipelineMissingInvocationID(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusSuccess,
		},
	}

	p := buildPipeline(adapter)
	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID: "test/tool",
		Input:  json.RawMessage(`{}`),
		Invocation: capability.ToolInvocationContext{
			InvocationID: "",
			UserID:       "user-001",
			Source:       capability.InvocationSourceModel,
		},
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status for missing invocation_id, got %s", result.Status)
	}
}

func TestPipelinePermissionDenied(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusSuccess,
		},
	}

	p := buildPipeline(adapter)
	p.PermissionGate.OnEvaluate = func(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) PermissionDecision {
		return PermissionDeny
	}

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != capability.ErrorCodePermissionDenied {
		t.Fatalf("expected permission_denied error, got %+v", result.Error)
	}
	if adapter.CallCount() != 0 {
		t.Fatalf("expected 0 adapter calls (permission denied), got %d", adapter.CallCount())
	}
}

func TestPipelineScopeDenied(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusSuccess,
		},
	}

	p := buildPipeline(adapter)
	if sm, ok := p.ScopeGate.ScopeManager.(*mockScopeManager); ok {
		sm.denyAll = true
	}

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != capability.ErrorCodeScopeDenied {
		t.Fatalf("expected scope_denied error, got %+v", result.Error)
	}
}

func TestPipelineIdempotencyGuard(t *testing.T) {
	successResult := capability.UnifiedToolResult{
		InvocationID: "inv-001",
		Status:       capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{Type: capability.ToolContentText, Text: "first call"},
		},
	}

	adapter := &testAdapter{result: successResult}
	p := buildPipeline(adapter)

	ctx := context.Background()
	inv := newTestInvocation("user-001")
	inv.IdempotencyKey = "idem-key-001"

	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: inv,
	}

	result1 := p.Execute(ctx, req)
	result2 := p.Execute(ctx, req)

	if adapter.CallCount() != 1 {
		t.Fatalf("expected 1 adapter call for idempotent request, got %d", adapter.CallCount())
	}
	if result1.Content[0].Text != result2.Content[0].Text {
		t.Fatalf("expected identical results for idempotent requests")
	}
}

func TestPipelineRetryOnFailure(t *testing.T) {
	failResult := capability.UnifiedToolResult{
		InvocationID: "inv-001",
		Status:       capability.ToolResultStatusFailed,
		Error: &capability.ToolError{
			Code:      capability.ErrorCodeConnectionLost,
			Message:   "temporary error",
			Retryable: true,
		},
	}

	adapter := &testAdapter{}
	p := buildPipeline(adapter)

	p.ToolResolver = func(ctx context.Context, toolID string) (capability.ToolDefinition, error) {
		tool := newTestTool(toolID)
		tool.Retryable = true
		tool.ExecutionPolicy.RetryPolicy.MaxRetries = 2
		return tool, nil
	}

	// DefaultRetryController: canonical policy; tool.Retryable no longer bypasses error classification.
	// MaxRetries=2 with connection_lost (canonical allowlist) yields exactly 3 total attempts.

	adapter.result = failResult

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status after retries, got %s", result.Status)
	}
	if adapter.CallCount() != 3 {
		t.Fatalf("expected 3 adapter calls (1 initial + 2 retries), got %d", adapter.CallCount())
	}
}

func TestPipelineDepthGuard(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "ok"},
			},
		},
	}

	p := buildPipeline(adapter)
	p.DepthGuard.MaxDepth = 2

	ctx := context.Background()
	inv := newTestInvocation("user-001")
	inv.ParentID = "parent-inv"
	inv.RootID = "root-inv"

	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: inv,
	}

	p.ToolResolver = func(ctx context.Context, toolID string) (capability.ToolDefinition, error) {
		tool := newTestTool(toolID)
		return tool, nil
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status for depth exceeded, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != "max_depth_exceeded" {
		t.Fatalf("expected max_depth_exceeded error, got %+v", result.Error)
	}
}

func TestPipelineSanitizer(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "result"},
			},
			Structured: json.RawMessage(`{"api_key":"secret123","safe":"public"}`),
		},
	}

	p := buildPipeline(adapter)
	p.Sanitizer.RegisterSensitiveField("api_key")
	p.Sanitizer.RegisterSensitiveField("token")

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	var obj map[string]any
	if err := json.Unmarshal(result.Structured, &obj); err != nil {
		t.Fatalf("failed to unmarshal sanitized result: %v", err)
	}
	if obj["api_key"] != "[redacted]" {
		t.Fatalf("expected api_key to be redacted, got %v", obj["api_key"])
	}
	if obj["safe"] != "public" {
		t.Fatalf("expected safe field to be preserved, got %v", obj["safe"])
	}
}

func TestPipelineCancellation(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "ok"},
			},
		},
	}

	p := buildPipeline(adapter)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	p.ConcurrencyCtrl.Policy.GlobalLimit = 1

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusCancelled {
		t.Fatalf("expected cancelled status for cancelled context, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != capability.ErrorCodeCancelled {
		t.Fatalf("expected cancelled error code, got: %v", result.Error)
	}
}

func TestPipelineTimeout(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "ok"},
			},
		},
	}

	p := buildPipeline(adapter)

	p.ToolResolver = func(ctx context.Context, toolID string) (capability.ToolDefinition, error) {
		tool := newTestTool(toolID)
		tool.TimeoutMS = 1
		return tool, nil
	}

	p.Dispatcher = NewRuntimeDispatcher(capability.NewRuntimeAdapterRegistry())

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed && result.Status != capability.ToolResultStatusTimedOut {
		t.Fatalf("expected failed or timed_out status for timeout, got %s", result.Status)
	}
}

func TestPipelineSideEffectRecording(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "done"},
			},
			SideEffects: []capability.RecordedSideEffect{
				{Type: "write", Target: "file-1", Description: "created file", Reversible: false},
				{Type: "send", Target: "msg-1", Description: "sent message", Reversible: false},
			},
		},
	}

	p := buildPipeline(adapter)
	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if p.SideEffectRec.Count("inv-001") != 2 {
		t.Fatalf("expected 2 recorded side effects, got %d", p.SideEffectRec.Count("inv-001"))
	}
}

func TestCircuitBreakerOpen(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:      capability.ErrorCodeConnectionLost,
				Retryable: true,
			},
		},
	}

	p := buildPipeline(adapter)
	p.CircuitBreaker = NewCircuitBreakerCoordinatorWithConfig(CircuitBreakerConfig{
		FailureThreshold:        2,
		OpenTimeout:             30 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	})
	p.circuitClassifier = NewCircuitResultClassifier()

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	for i := 0; i < 2; i++ {
		p.Execute(ctx, req)
	}

	ck := CircuitKey{RuntimeType: capability.RuntimeTypeBuiltin, RuntimeID: "test", ToolID: "test/tool"}
	if p.CircuitBreaker.SnapshotByKey(ck).State != CircuitStateOpen {
		t.Fatalf("expected circuit open after failures, got %s", p.CircuitBreaker.SnapshotByKey(ck).State)
	}

	beforeCalls := adapter.CallCount()
	p.Execute(ctx, req)
	if adapter.CallCount() != beforeCalls {
		t.Fatalf("OPEN must block dispatch: calls went from %d to %d", beforeCalls, adapter.CallCount())
	}

	result := p.Execute(ctx, req)
	if result.Error == nil || result.Error.Code != capability.ErrorCodeCircuitOpen {
		t.Fatalf("expected circuit_open error, got %+v", result.Error)
	}
}

func TestPipelineResultValidationEmpty(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
		},
	}

	p := buildPipeline(adapter)
	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed for empty result, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != capability.ErrorCodeInvalidResult {
		t.Fatalf("expected invalid_result error, got %+v", result.Error)
	}
}

func TestPipelineMetricsRecording(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "ok"},
			},
		},
	}

	p := buildPipeline(adapter)
	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	p.Execute(ctx, req)

	counter := p.MetricsRec.GetCounter("test/tool")
	if counter == nil {
		t.Fatalf("expected metrics counter for test/tool")
	}
	if counter.TotalCalls != 1 {
		t.Fatalf("expected 1 total call, got %d", counter.TotalCalls)
	}
	if counter.SuccessCalls != 1 {
		t.Fatalf("expected 1 success call, got %d", counter.SuccessCalls)
	}
}

func TestPipelineConcurrencySlotRelease(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "ok"},
			},
		},
	}

	p := buildPipeline(adapter)

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	p.Execute(ctx, req)

	if len(p.ConcurrencyCtrl.globalSem) != 0 {
		t.Fatalf("expected global semaphore to be released (empty), got %d occupied", len(p.ConcurrencyCtrl.globalSem))
	}
}

func TestPipelineApprovalGateRequire(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "ok"},
			},
		},
	}

	p := buildPipeline(adapter)
	p.PermissionGate.OnEvaluate = func(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) PermissionDecision {
		return PermissionRequireApproval
	}

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed when approval denied, got %s", result.Status)
	}
}

func TestPipelineApprovalApproved(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "ok"},
			},
		},
	}

	p := buildPipeline(adapter)
	p.PermissionGate.OnEvaluate = func(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) PermissionDecision {
		return PermissionRequireApproval
	}
	p.ApprovalGate.OnEvaluate = func(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, decision PermissionDecision) (bool, error) {
		return true, nil
	}

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success when approval granted, got %s", result.Status)
	}
}

func TestPipelineRateLimiter(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			InvocationID: "inv-001",
			Status:       capability.ToolResultStatusSuccess,
			Content: []capability.ToolContent{
				{Type: capability.ToolContentText, Text: "ok"},
			},
		},
	}

	p := buildPipeline(adapter)
	p.RateLimiter.OnAllow = func(ctx context.Context, tool capability.ToolDefinition) error {
		return errors.New("rate limited")
	}

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/tool",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed for rate limit, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != capability.ErrorCodeRateLimited {
		t.Fatalf("expected rate_limited error, got %+v", result.Error)
	}
}

func TestToolNotFound(t *testing.T) {
	adapter := &testAdapter{
		result: capability.UnifiedToolResult{
			Status: capability.ToolResultStatusSuccess,
		},
	}

	p := buildPipeline(adapter)
	p.ToolResolver = func(ctx context.Context, toolID string) (capability.ToolDefinition, error) {
		return capability.ToolDefinition{}, errors.New("tool not found")
	}

	ctx := context.Background()
	req := ToolExecutionRequest{
		ToolID:     "test/unknown",
		Input:      json.RawMessage(`{}`),
		Invocation: newTestInvocation("user-001"),
	}

	result := p.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed for unknown tool, got %s", result.Status)
	}
}

func TestInvocationValidatorAllFields(t *testing.T) {
	v := NewInvocationValidator()
	ctx := context.Background()

	tests := []struct {
		name    string
		request ToolExecutionRequest
		wantErr bool
	}{
		{
			name: "valid",
			request: ToolExecutionRequest{
				ToolID:     "test/tool",
				Invocation: newTestInvocation("user-001"),
			},
			wantErr: false,
		},
		{
			name: "missing invocation ID",
			request: ToolExecutionRequest{
				ToolID: "test/tool",
				Invocation: capability.ToolInvocationContext{
					UserID: "user-001",
					Source: capability.InvocationSourceModel,
				},
			},
			wantErr: true,
		},
		{
			name: "missing tool ID",
			request: ToolExecutionRequest{
				Invocation: newTestInvocation("user-001"),
			},
			wantErr: true,
		},
		{
			name: "missing user ID",
			request: ToolExecutionRequest{
				ToolID: "test/tool",
				Invocation: capability.ToolInvocationContext{
					InvocationID: "inv-001",
					Source:       capability.InvocationSourceModel,
				},
			},
			wantErr: true,
		},
		{
			name: "missing source",
			request: ToolExecutionRequest{
				ToolID: "test/tool",
				Invocation: capability.ToolInvocationContext{
					InvocationID: "inv-001",
					UserID:       "user-001",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(ctx, tt.request)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResultValidatorOutputSize(t *testing.T) {
	v := NewResultValidator()
	v.MaxOutputBytes = 10
	ctx := context.Background()
	tool := newTestTool("test/tool")

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/tool",
		Status:       capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{Type: capability.ToolContentText, Text: "this is way too long to fit"},
		},
	}

	validated := v.Validate(ctx, tool, inv, result)

	if validated.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed for oversized result, got %s", validated.Status)
	}
}

func TestSanitizerNested(t *testing.T) {
	s := NewSanitizer()
	s.RegisterSensitiveField("secret")
	s.RegisterSensitiveField("password")
	ctx := context.Background()

	result := capability.UnifiedToolResult{
		InvocationID: "inv-001",
		Status:       capability.ToolResultStatusSuccess,
		Structured:   json.RawMessage(`{"nested":{"secret":"hidden","safe":"visible"},"password":"abc"}`),
	}

	sanitized := s.Sanitize(ctx, result)

	var obj map[string]any
	if err := json.Unmarshal(sanitized.Structured, &obj); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	nested := obj["nested"].(map[string]any)
	if nested["secret"] != "[redacted]" {
		t.Fatalf("expected nested secret to be redacted")
	}
	if nested["safe"] != "visible" {
		t.Fatalf("expected safe field preserved")
	}
	if obj["password"] != "[redacted]" {
		t.Fatalf("expected password to be redacted")
	}
}

func TestAuditRecorderFull(t *testing.T) {
	a := NewAuditRecorder()
	ctx := context.Background()

	a.RecordStart(ctx, "inv-001", "tool-1")
	time.Sleep(10 * time.Millisecond)
	a.RecordFinish(ctx, "inv-001", "tool-1", "success", 10*time.Millisecond)

	entry := a.GetEntry("inv-001")
	if entry == nil {
		t.Fatalf("expected audit entry")
	}
	if entry.Status != "success" {
		t.Fatalf("expected success status, got %s", entry.Status)
	}
	if entry.ToolID != "tool-1" {
		t.Fatalf("expected tool-1, got %s", entry.ToolID)
	}
}

func TestSideEffectRecorderConcurrent(t *testing.T) {
	r := NewSideEffectRecorder()
	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Record(ctx, "inv-"+string(rune('0'+n%10)), "tool", []capability.RecordedSideEffect{
				{Type: "write", Target: "file"},
			})
		}(i)
	}

	wg.Wait()
}

func TestIdempotencyGuardConcurrentSameKey(t *testing.T) {
	g := NewIdempotencyGuard()
	ctx := context.Background()

	expected := capability.UnifiedToolResult{
		InvocationID: "inv-001",
		Status:       capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{Type: capability.ToolContentText, Text: "ok"},
		},
	}

	g.Record(ctx, "key-1", "tool-1", &expected)

	var wg sync.WaitGroup
	results := make([]capability.UnifiedToolResult, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r, _ := g.Check(ctx, "key-1", "tool-1")
			results[n] = r
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if r.Status != capability.ToolResultStatusSuccess {
			t.Fatalf("concurrent read %d failed: %s", i, r.Status)
		}
	}
}
