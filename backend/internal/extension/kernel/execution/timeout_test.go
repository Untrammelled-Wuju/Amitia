package execution

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestResolveBudget_Default30s(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	inv := capability.ToolInvocationContext{InvocationID: "inv-1"}
	tool := capability.ToolDefinition{ID: "t1"}

	budget, err := ctrl.ResolveBudget(context.Background(), time.Now(), inv, tool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if budget.Source != TimeoutSourceKernelDefault {
		t.Errorf("expected source kernel_default, got %s", budget.Source)
	}
	if budget.ConfiguredTimeout != 30*time.Second {
		t.Errorf("expected 30s configured timeout, got %v", budget.ConfiguredTimeout)
	}
}

func TestResolveBudget_ToolTimeout(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	inv := capability.ToolInvocationContext{InvocationID: "inv-1"}
	tool := capability.ToolDefinition{ID: "t1", TimeoutMS: 90000}

	budget, err := ctrl.ResolveBudget(context.Background(), time.Now(), inv, tool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if budget.Source != TimeoutSourceToolPolicy {
		t.Errorf("expected source tool_policy, got %s", budget.Source)
	}
	if budget.ConfiguredTimeout != 90*time.Second {
		t.Errorf("expected 90s configured timeout, got %v", budget.ConfiguredTimeout)
	}
}

func TestResolveBudget_CallerEarlier(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	inv := capability.ToolInvocationContext{InvocationID: "inv-1"}
	tool := capability.ToolDefinition{ID: "t1", TimeoutMS: 90000}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	acceptedAt := time.Now()
	budget, err := ctrl.ResolveBudget(ctx, acceptedAt, inv, tool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if budget.Source != TimeoutSourceCaller {
		t.Errorf("expected source caller_deadline, got %s", budget.Source)
	}
	if budget.Remaining(acceptedAt) > 6*time.Second {
		t.Errorf("expected remaining <= 6s, got %v", budget.Remaining(acceptedAt))
	}
}

func TestResolveBudget_InvocationEarlier(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	acceptedAt := time.Now()
	inv := capability.ToolInvocationContext{
		InvocationID: "inv-1",
		ExpiresAt:    acceptedAt.Add(3 * time.Second),
	}
	tool := capability.ToolDefinition{ID: "t1", TimeoutMS: 90000}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	budget, err := ctrl.ResolveBudget(ctx, acceptedAt, inv, tool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if budget.Source != TimeoutSourceInvocation {
		t.Errorf("expected source invocation_expiry, got %s", budget.Source)
	}
	if budget.ConfiguredTimeout > 4*time.Second {
		t.Errorf("expected configured <= 4s, got %v", budget.ConfiguredTimeout)
	}
}

func TestResolveBudget_ExpiredInvocation(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	acceptedAt := time.Now()
	inv := capability.ToolInvocationContext{
		InvocationID: "inv-1",
		ExpiresAt:    acceptedAt.Add(-1 * time.Second),
	}
	tool := capability.ToolDefinition{ID: "t1"}

	budget, err := ctrl.ResolveBudget(context.Background(), acceptedAt, inv, tool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !budget.Expired(acceptedAt) {
		t.Errorf("expected budget to be expired immediately")
	}
}

func TestResolveBudget_NegativeSourceDeterminism(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	acceptedAt := time.Now()
	inv := capability.ToolInvocationContext{
		InvocationID: "inv-1",
		ExpiresAt:    acceptedAt.Add(5 * time.Second),
	}
	tool := capability.ToolDefinition{ID: "t1"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	budget, err := ctrl.ResolveBudget(ctx, acceptedAt, inv, tool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if budget.Source != TimeoutSourceCaller {
		t.Errorf("expected caller_deadline when equal, got %s", budget.Source)
	}
}

func TestBudget_RemainingAndExpired(t *testing.T) {
	ref := time.Now()
	budget := TimeoutBudget{
		AcceptedAt: ref,
		Deadline:   ref.Add(10 * time.Second),
	}
	if budget.Expired(ref) {
		t.Errorf("expected not expired at accepted time")
	}
	if !budget.Expired(ref.Add(11 * time.Second)) {
		t.Errorf("expected expired 1s after deadline")
	}
	remainingAtStart := budget.Remaining(ref)
	if remainingAtStart < 9*time.Second || remainingAtStart > 10*time.Second {
		t.Errorf("expected ~10s remaining at start, got %v", remainingAtStart)
	}
	remainingAt3s := budget.Remaining(ref.Add(3 * time.Second))
	if remainingAt3s < 6*time.Second || remainingAt3s > 7*time.Second {
		t.Errorf("expected ~7s remaining at 3s, got %v", remainingAt3s)
	}
}

func TestWrap_AndContextFires(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	acceptedAt := time.Now()
	budget := TimeoutBudget{
		AcceptedAt: acceptedAt,
		Deadline:   acceptedAt.Add(50 * time.Millisecond),
	}
	ctx := context.Background()
	wrapped, cancel, err := ctrl.Wrap(ctx, budget)
	if err != nil {
		t.Fatalf("wrap error: %v", err)
	}
	defer cancel()

	select {
	case <-wrapped.Done():
		if !errors.Is(wrapped.Err(), context.DeadlineExceeded) {
			t.Errorf("expected DeadlineExceeded, got %v", wrapped.Err())
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("expected context to be done after deadline")
	}
}

func TestWrap_ZeroDeadline(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	budget := TimeoutBudget{}
	_, _, err := ctrl.Wrap(context.Background(), budget)
	if err == nil {
		t.Errorf("expected error for zero deadline")
	}
}

func TestNewTimeoutController_Validation(t *testing.T) {
	ctrl := NewTimeoutController(0)
	inv := capability.ToolInvocationContext{InvocationID: "inv-1"}
	tool := capability.ToolDefinition{ID: "t1"}
	_, err := ctrl.ResolveBudget(context.Background(), time.Now(), inv, tool)
	if err == nil {
		t.Errorf("expected error for zero default timeout")
	}
}

func TestAttemptBudget(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	acceptedAt := time.Now()
	budget := TimeoutBudget{
		AcceptedAt: acceptedAt,
		Deadline:   acceptedAt.Add(10 * time.Second),
	}
	attempt := ctrl.NewAttemptBudget(budget, 2)
	if attempt.Attempt != 2 {
		t.Errorf("expected attempt 2, got %d", attempt.Attempt)
	}
	if attempt.TotalDeadline != budget.Deadline {
		t.Errorf("expected total deadline to match budget")
	}
}

func TestExpiredInvocation_ImmediateTimeout(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	inv := capability.ToolInvocationContext{
		InvocationID: "inv-expired",
		ExpiresAt:    time.Now().Add(-1 * time.Second),
	}
	tool := capability.ToolDefinition{ID: "t-expired"}

	pipeline := &ExecutionPipeline{
		TimeoutCtrl:      ctrl,
		CancellationCtrl: NewCancellationController(),
	}
	pipeline.ToolResolver = func(_ context.Context, _ string) (capability.ToolDefinition, error) {
		return tool, nil
	}

	req := ToolExecutionRequest{
		ToolID:     capability.CapabilityID("t-expired"),
		Invocation: inv,
	}

	result := pipeline.Execute(context.Background(), req)
	if result.Status != capability.ToolResultStatusTimedOut {
		t.Errorf("expected timed_out for expired invocation, got %s", result.Status)
	}
}

func TestRetryDoesNotRefreshDeadline(t *testing.T) {
	ctrl := NewTimeoutController(30 * time.Second)
	inv := capability.ToolInvocationContext{
		InvocationID: "inv-retry",
	}

	tool := capability.ToolDefinition{
		ID:        "t-retry",
		TimeoutMS: 200,
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeBuiltin,
			RuntimeID:   "test-retry",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			RetryPolicy: capability.RetryPolicy{
				MaxRetries:  5,
				BackoffBase: 10 * time.Millisecond,
			},
		},
	}

	adapter := &countingFailAdapter{
		failCount: 10,
	}

	adapterRegistry := capability.NewRuntimeAdapterRegistry()
	adapterRegistry.Register(capability.RuntimeTypeBuiltin, adapter)

	pipeline := &ExecutionPipeline{
		TimeoutCtrl:      ctrl,
		CancellationCtrl: NewCancellationController(),
		Dispatcher:       NewRuntimeDispatcher(adapterRegistry),
		RetryCtrl:        NewRetryController(),
	}
	pipeline.ToolResolver = func(_ context.Context, _ string) (capability.ToolDefinition, error) {
		return tool, nil
	}

	req := ToolExecutionRequest{
		ToolID:     capability.CapabilityID("t-retry"),
		Invocation: inv,
	}

	start := time.Now()
	_ = pipeline.Execute(context.Background(), req)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("retry loop exceeded total deadline, elapsed=%v", elapsed)
	}
}

type countingFailAdapter struct {
	failCount int
	mu        sync.Mutex
	calls     int
}

func (a *countingFailAdapter) Supports(capability.RuntimeBinding) bool { return true }

func (a *countingFailAdapter) Execute(_ context.Context, _ capability.RuntimeBinding, inv capability.ToolInvocationContext, _ json.RawMessage) capability.UnifiedToolResult {
	a.mu.Lock()
	a.calls++
	calls := a.calls
	a.mu.Unlock()

	if calls <= a.failCount {
		return capability.NewToolFailureResult(inv.InvocationID, "t-retry", &capability.ToolError{
			Code:      capability.ErrorCodeExecutionFailed,
			Message:   "simulated failure",
			Retryable: true,
		})
	}
	return capability.NewToolSuccessResult(inv.InvocationID, "t-retry")
}

func (a *countingFailAdapter) Health(_ context.Context, _ capability.RuntimeBinding) capability.HealthStatus {
	return capability.HealthReady
}
