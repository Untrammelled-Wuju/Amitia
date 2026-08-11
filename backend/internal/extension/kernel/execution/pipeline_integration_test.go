package execution

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/observability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func integTool(id string) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:          id,
		ModelName:   "itool",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Integration Tool",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Enabled:     true,
		Compatible:  true,
		Retryable:   false,
		TimeoutMS:   5000,
		Permissions: []capability.PermissionRequirement{{Capability: "itool.execute"}},
		SideEffect:  capability.SideEffectWrite,
		ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: "1"},
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
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:        30 * time.Second,
			MaxConcurrency: 10,
			Idempotent:     false,
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeBuiltin,
			RuntimeID:   "integ",
		},
	}
}

func integInvocation(userID, idemKey string) capability.ToolInvocationContext {
	return capability.ToolInvocationContext{
		InvocationID:   "integ-" + userID,
		UserID:         userID,
		CharacterID:    "char-x",
		ConversationID: "conv-x",
		Source:         capability.InvocationSourceModel,
		IdempotencyKey: idemKey,
	}
}

type spyingAdapter struct {
	mu      sync.Mutex
	calls   int
	panicOn bool
	blockCh chan struct{}
}

func (s *spyingAdapter) Supports(b capability.RuntimeBinding) bool { return true }

func (s *spyingAdapter) Execute(ctx context.Context, b capability.RuntimeBinding, inv capability.ToolInvocationContext, input json.RawMessage) capability.UnifiedToolResult {
	s.mu.Lock()
	s.calls++
	doPanic := s.panicOn
	s.mu.Unlock()
	if doPanic {
		panic("boom")
	}
	if s.blockCh != nil {
		select {
		case <-s.blockCh:
		case <-ctx.Done():
			return capability.UnifiedToolResult{
				InvocationID: inv.InvocationID,
				Status:       capability.ToolResultStatusFailed,
				Error:        &capability.ToolError{Code: capability.ErrorCodeTimeout},
			}
		}
	}
	return capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		Status:       capability.ToolResultStatusSuccess,
		Content:      []capability.ToolContent{{Type: capability.ToolContentText, Text: "ok"}},
	}
}

func (s *spyingAdapter) Health(ctx context.Context, b capability.RuntimeBinding) capability.HealthStatus {
	return capability.HealthReady
}

func (s *spyingAdapter) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type denyAllBroker struct{}

func (d *denyAllBroker) Evaluate(_ context.Context, _ permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult {
	return permission.PermissionEvaluationResult{Decision: permission.DecisionDeny}
}
func (d *denyAllBroker) Grant(_ context.Context, _ permission.PermissionGrantRequest) (permission.PermissionGrant, error) {
	return permission.PermissionGrant{}, nil
}
func (d *denyAllBroker) Revoke(_ context.Context, _ string) error { return nil }
func (d *denyAllBroker) RevokeBySubject(_ context.Context, _ permission.PermissionSubject) (int, error) {
	return 0, nil
}
func (d *denyAllBroker) RevokeByExtension(_ context.Context, _ string) (int, error) { return 0, nil }
func (d *denyAllBroker) ListGrants(_ context.Context, _ permission.PermissionGrantFilter) ([]permission.PermissionGrant, error) {
	return nil, nil
}
func (d *denyAllBroker) Explain(_ context.Context, _ permission.PermissionEvaluationRequest) permission.PermissionExplanation {
	return permission.PermissionExplanation{Decision: permission.DecisionDeny}
}
func (d *denyAllBroker) DetectUpgrade(_ context.Context, _, _ []permission.PermissionRequirement) []permission.PermissionUpgrade {
	return nil
}
func (d *denyAllBroker) RecordApproval(_ context.Context, _ permission.PermissionApprovalRecordRequest) (permission.PermissionApprovalRecord, error) {
	return permission.PermissionApprovalRecord{}, nil
}
func (d *denyAllBroker) ValidateSnapshot(_ context.Context, _ string, _ permission.PermissionEvaluationRequest) error {
	return nil
}

func integCircuitOpen(t *testing.T, key string) *CircuitBreakerCoordinator {
	t.Helper()
	config := DefaultCircuitBreakerConfig()
	config.FailureThreshold = 1
	config.OpenTimeout = time.Hour
	coord := NewCircuitBreakerCoordinatorWithConfig(config)
	tool := integTool(key)
	permit := coord.Acquire(context.Background(), tool)
	coord.Complete(permit, CircuitOutcomeFailure)
	return coord
}

func integPipeline(t *testing.T, adapter *spyingAdapter, idem IdempotencyStorage, denyPermission bool, denyScope bool, circuit *CircuitBreakerCoordinator) *ExecutionPipeline {
	t.Helper()
	adapterRegistry := capability.NewRuntimeAdapterRegistry()
	adapterRegistry.Register(capability.RuntimeTypeBuiltin, adapter)

	var broker permission.PermissionBroker = &mockPermissionBroker{}
	if denyPermission {
		broker = &denyAllBroker{}
	}

	rateLimitPolicy := RateLimitPolicy{Enabled: false}
	rateLimiter, err := NewRateLimiter(rateLimitPolicy)
	if err != nil {
		t.Fatal(err)
	}

	iden := idem
	if iden == nil {
		iden = newNoopIdempotencyStorage()
	}

	if circuit == nil {
		circuit = NewCircuitBreakerCoordinator()
	}

	p := &ExecutionPipeline{
		InvocationValidator:     NewInvocationValidator(),
		InputValidator:          NewInputValidator(),
		AvailabilityGate:        NewAvailabilityGate(nil),
		ScopeGate:               NewScopeGate(),
		PermissionGate:          NewPermissionGate(),
		ApprovalGate:            NewApprovalGate(),
		ConcurrencyCtrl:         mustTestConcurrency(NewTestConcurrencyPolicy()),
		RateLimiter:             rateLimiter,
		IdempotencyGuard:        NewIdempotencyGuard(iden),
		RetryCtrl:               NewRetryController(),
		TimeoutCtrl:             NewTimeoutController(5 * time.Second),
		CancellationCtrl:        NewCancellationController(),
		DepthGuard:              NewDepthGuard(),
		Dispatcher:              NewRuntimeDispatcher(adapterRegistry),
		ResultValidator:         NewResultValidator(),
		Sanitizer:               NewSanitizer(),
		SideEffectRec:           NewSideEffectRecorder(),
		AuditSink:               newIntegAuditHook(t),
		MetricsRec:              NewMetricsRecorder(),
		CircuitBreaker:          circuit,
		ToolResolver:            func(ctx context.Context, tid string) (capability.ToolDefinition, error) { return integTool(tid), nil },
		ScopeStore:              &mockScopeStore{},
		PermissionSnapshotStore: &mockPermissionSnapshotStore{},
	}
	p.ScopeGate.ScopeManager = &mockScopeManager{denyAll: denyScope}
	p.PermissionGate.Broker = broker
	return p
}

func newIntegAuditHook(t *testing.T) observability.ExecutionRecorder {
	t.Helper()
	_testAuditStore = observability.NewMemoryStore()
	_testAuditWriter = observability.NewRecordWriter(_testAuditStore, observability.DefaultWriterConfig())
	return observability.NewExecutionHook(_testAuditWriter, nil)
}

func TestB8_PipelineSuccess(t *testing.T) {
	adapter := &spyingAdapter{}
	p := integPipeline(t, adapter, nil, false, false, nil)
	req := ToolExecutionRequest{
		ToolID:     "itool/test",
		Input:      json.RawMessage(`{}`),
		Invocation: integInvocation("u1", ""),
	}
	result := p.Execute(context.Background(), req)
	if result.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success, got %s (%+v)", result.Status, result.Error)
	}
	if adapter.CallCount() != 1 {
		t.Fatalf("expected 1 adapter call, got %d", adapter.CallCount())
	}
}

func TestB8_PermissionDenyNoDispatch(t *testing.T) {
	adapter := &spyingAdapter{}
	p := integPipeline(t, adapter, nil, true, false, nil)
	req := ToolExecutionRequest{
		ToolID:     "itool/test",
		Input:      json.RawMessage(`{}`),
		Invocation: integInvocation("u1", ""),
	}
	result := p.Execute(context.Background(), req)
	if result.Error == nil || result.Error.Code != capability.ErrorCodePermissionDenied {
		t.Fatalf("expected permission_denied, got %+v", result.Error)
	}
	if adapter.CallCount() != 0 {
		t.Fatalf("permission deny should not dispatch, got %d", adapter.CallCount())
	}
}

func TestB8_SingleFlight_FollowerWaits(t *testing.T) {
	blockCh := make(chan struct{})
	adapter := &spyingAdapter{blockCh: blockCh}
	db := newTestIdempotencyDB(t)
	p := integPipeline(t, adapter, NewExecutionIdempotencyStorage(db), false, false, nil)

	req := ToolExecutionRequest{
		ToolID:     "itool/shared",
		Input:      json.RawMessage(`{}`),
		Invocation: integInvocation("u1", "shared-key"),
	}

	firstDone := make(chan struct{})
	var first capability.UnifiedToolResult
	go func() {
		defer close(firstDone)
		first = p.Execute(context.Background(), req)
	}()

	time.Sleep(200 * time.Millisecond)

	followerDone := make(chan struct{})
	var follower capability.UnifiedToolResult
	go func() {
		defer close(followerDone)
		follower = p.Execute(context.Background(), req)
	}()

	time.Sleep(100 * time.Millisecond)
	close(blockCh)

	<-firstDone
	<-followerDone

	if adapter.CallCount() != 1 {
		t.Fatalf("single-flight: expected 1 runtime dispatch, got %d", adapter.CallCount())
	}
	if first.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("first did not succeed: %+v", first.Error)
	}
	if follower.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("follower did not succeed: %+v", follower.Error)
	}
}

func TestB8_PanicSafety_RecoverAndFinalize(t *testing.T) {
	adapter := &spyingAdapter{panicOn: true}
	p := integPipeline(t, adapter, nil, false, false, nil)
	req := ToolExecutionRequest{
		ToolID:     "itool/test",
		Input:      json.RawMessage(`{}`),
		Invocation: integInvocation("u1", ""),
	}
	result := p.Execute(context.Background(), req)
	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status on panic, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != capability.ErrorCodeExecutionFailed {
		t.Fatalf("expected execution_failed, got %+v", result.Error)
	}
	if adapter.CallCount() != 1 {
		t.Fatalf("expected 1 call even on panic, got %d", adapter.CallCount())
	}
}

func TestB8_RateLimitRejectNoDispatch(t *testing.T) {
	adapter := &spyingAdapter{}
	p := integPipeline(t, adapter, nil, false, false, nil)
	rl, err := NewRateLimiter(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 1, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	})
	if err != nil {
		t.Fatal(err)
	}
	p.RateLimiter = rl
	first := p.Execute(context.Background(), ToolExecutionRequest{
		ToolID:     "itool/test",
		Input:      json.RawMessage(`{}`),
		Invocation: integInvocation("u1", "k1"),
	})
	if first.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("first call should succeed: %+v", first.Error)
	}
	second := p.Execute(context.Background(), ToolExecutionRequest{
		ToolID:     "itool/test",
		Input:      json.RawMessage(`{}`),
		Invocation: integInvocation("u2", "k2"),
	})
	if second.Error == nil || second.Error.Code != capability.ErrorCodeRateLimited {
		t.Fatalf("expected rate_limited, got %+v", second.Error)
	}
	if adapter.CallCount() != 1 {
		t.Fatalf("rate reject should not dispatch: got %d", adapter.CallCount())
	}
}

func TestB8_CircuitOpenNoDispatch(t *testing.T) {
	adapter := &spyingAdapter{}
	circuit := integCircuitOpen(t, "itool/test")
	p := integPipeline(t, adapter, nil, false, false, circuit)
	result := p.Execute(context.Background(), ToolExecutionRequest{
		ToolID:     "itool/test",
		Input:      json.RawMessage(`{}`),
		Invocation: integInvocation("u1", ""),
	})
	if result.Error == nil || result.Error.Code != capability.ErrorCodeCircuitOpen {
		t.Fatalf("expected circuit_open, got %+v", result.Error)
	}
	if adapter.CallCount() != 0 {
		t.Fatalf("circuit open should not dispatch: got %d", adapter.CallCount())
	}
}

func TestB8_IdempotencyReplayNoExtraDispatch(t *testing.T) {
	adapter := &spyingAdapter{}
	db := newTestIdempotencyDB(t)
	p := integPipeline(t, adapter, NewExecutionIdempotencyStorage(db), false, false, nil)
	req := ToolExecutionRequest{
		ToolID:     "itool/replay",
		Input:      json.RawMessage(`{}`),
		Invocation: integInvocation("u1", "replay-key"),
	}
	first := p.Execute(context.Background(), req)
	if first.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("first call should succeed: %+v", first.Error)
	}
	if adapter.CallCount() != 1 {
		t.Fatalf("expected 1 dispatch first call, got %d", adapter.CallCount())
	}
	adapter2 := &spyingAdapter{}
	p2 := integPipeline(t, adapter2, NewExecutionIdempotencyStorage(db), false, false, nil)
	second := p2.Execute(context.Background(), req)
	if second.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("replay should return success, got %+v", second.Error)
	}
}
