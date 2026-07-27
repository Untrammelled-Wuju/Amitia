package hook

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestPipeline_NoContributions(t *testing.T) {
	registry := NewHookPointRegistry()
	ctx := context.Background()
	if err := RegisterDefaultHookPoints(ctx, registry); err != nil {
		t.Fatalf("register points: %v", err)
	}

	store := NewMemoryContributionStore()
	bridge := NewDirectRuntimeBridge()
	pipeline := NewPipeline(registry, store, bridge)

	payload := json.RawMessage(`{"message":"hello"}`)
	hookCtx := HookContextSnapshot{
		InvocationID: "inv-1",
		OperationID:  "op-1",
		Timestamp:    time.Now().UTC(),
	}

	result := pipeline.Invoke(ctx, InvokeRequest{
		HookPointID: "message.before_send/1",
		Payload:     payload,
		Context:     hookCtx,
	})

	if result.Aborted {
		t.Errorf("expected not aborted, got abort reason: %s", result.AbortReason)
	}
	if result.Decision != DecisionContinue {
		t.Errorf("expected continue, got %s", result.Decision)
	}
	if len(result.Executions) != 0 {
		t.Errorf("expected 0 executions, got %d", len(result.Executions))
	}
}

func TestPipeline_HookPointNotFound(t *testing.T) {
	registry := NewHookPointRegistry()
	store := NewMemoryContributionStore()
	bridge := NewDirectRuntimeBridge()
	pipeline := NewPipeline(registry, store, bridge)

	result := pipeline.Invoke(context.Background(), InvokeRequest{
		HookPointID: "nonexistent.point/1",
		Payload:     json.RawMessage(`{}`),
		Context:     HookContextSnapshot{},
	})

	if !result.Aborted {
		t.Error("expected aborted for unknown hook point")
	}
}

func TestCircuitBreaker_OpenAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker()
	contribID := "test-contrib-1"
	threshold := 5

	for i := 0; i < threshold; i++ {
		cb.RecordFailure(contribID, ErrCodeHookRuntimeError)
	}

	if !cb.IsOpen(contribID) {
		t.Error("expected circuit to be open after threshold failures")
	}

	stats := cb.GetStats(contribID)
	if stats.State != CircuitOpen {
		t.Errorf("expected CircuitOpen, got %s", stats.State)
	}
	if stats.ConsecutiveFails != threshold {
		t.Errorf("expected %d consecutive fails, got %d", threshold, stats.ConsecutiveFails)
	}
}

func TestCircuitBreaker_ResetAfterSuccess(t *testing.T) {
	cb := NewCircuitBreaker()
	contribID := "test-contrib-2"

	cb.RecordFailure(contribID, ErrCodeHookRuntimeError)
	cb.RecordFailure(contribID, ErrCodeHookRuntimeError)
	cb.RecordSuccess(contribID)

	stats := cb.GetStats(contribID)
	if stats.State != CircuitClosed {
		t.Errorf("expected CircuitClosed after success, got %s", stats.State)
	}
	if stats.ConsecutiveFails != 0 {
		t.Errorf("expected 0 consecutive fails after success, got %d", stats.ConsecutiveFails)
	}
	if stats.TotalSuccess != 1 {
		t.Errorf("expected 1 total success, got %d", stats.TotalSuccess)
	}
}

func TestCircuitBreaker_ManualReset(t *testing.T) {
	cb := NewCircuitBreaker()
	contribID := "test-contrib-3"
	threshold := 5

	for i := 0; i < threshold; i++ {
		cb.RecordFailure(contribID, ErrCodeHookRuntimeError)
	}

	cb.Reset(contribID)

	if cb.IsOpen(contribID) {
		t.Error("expected circuit to be closed after manual reset")
	}

	stats := cb.GetStats(contribID)
	if stats.State != CircuitClosed {
		t.Errorf("expected CircuitClosed, got %s", stats.State)
	}
}

func TestDepthGuard_RecursionDetection(t *testing.T) {
	guard := NewDepthGuard(5)
	invocationID := "inv-rec-1"
	contribID := "contrib-rec-1"
	pointID := "message.before_send/1"

	_, err := guard.CheckAndEnter(invocationID, contribID, pointID, 0)
	if err != nil {
		t.Fatalf("first enter: %v", err)
	}

	_, err = guard.CheckAndEnter(invocationID, contribID, pointID, 1)
	if !errors.Is(err, ErrRecursion) {
		t.Errorf("expected ErrRecursion, got %v", err)
	}
}

func TestDepthGuard_MaxDepthExceeded(t *testing.T) {
	guard := NewDepthGuard(3)
	invocationID := "inv-depth-1"

	for i := 0; i < 3; i++ {
		_, err := guard.CheckAndEnter(invocationID, "contrib-"+string(rune(i)), "point-1", i)
		if err != nil {
			t.Fatalf("enter at depth %d: %v", i, err)
		}
	}

	_, err := guard.CheckAndEnter(invocationID, "contrib-4", "point-1", 3)
	if err == nil {
		t.Error("expected depth exceeded error")
	}
}

func TestMemoryStore_RegisterAndGet(t *testing.T) {
	store := NewMemoryContributionStore()
	ctx := context.Background()

	contrib := HookContributionDefinition{
		ContributionID: "contrib-store-1",
		ExtensionID:    "ext-1",
		HookPointID:    "message.before_send/1",
		ContractVersion: 1,
		Phase:          PhaseBefore,
		Priority:       100,
		RuntimeBinding: RuntimeBinding{
			RuntimeType: "wasm",
			ModuleID:    "mod-1",
			Entry:       "hookHandler",
		},
	}

	if err := store.Register(ctx, contrib); err != nil {
		t.Fatalf("register: %v", err)
	}

	got, err := store.Get(ctx, "contrib-store-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.ContributionID != contrib.ContributionID {
		t.Errorf("expected contribution ID %s, got %s", contrib.ContributionID, got.ContributionID)
	}
	if got.HookPointID != contrib.HookPointID {
		t.Errorf("expected hook point ID %s, got %s", contrib.HookPointID, got.HookPointID)
	}
}

func TestMemoryStore_ListByHookPoint(t *testing.T) {
	store := NewMemoryContributionStore()
	ctx := context.Background()

	contribs := []HookContributionDefinition{
		{
			ContributionID: "contrib-1",
			ExtensionID:    "ext-1",
			HookPointID:    "message.before_send/1",
			ContractVersion: 1,
			Phase:          PhaseBefore,
			RuntimeBinding: RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-1", Entry: "handler"},
		},
		{
			ContributionID: "contrib-2",
			ExtensionID:    "ext-2",
			HookPointID:    "message.before_send/1",
			ContractVersion: 1,
			Phase:          PhaseAfter,
			RuntimeBinding: RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-2", Entry: "handler"},
		},
		{
			ContributionID: "contrib-3",
			ExtensionID:    "ext-3",
			HookPointID:    "model.before_request/1",
			ContractVersion: 1,
			Phase:          PhaseBefore,
			RuntimeBinding: RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-3", Entry: "handler"},
		},
	}

	for _, c := range contribs {
		if err := store.Register(ctx, c); err != nil {
			t.Fatalf("register %s: %v", c.ContributionID, err)
		}
	}

	list, err := store.ListByHookPoint(ctx, "message.before_send/1")
	if err != nil {
		t.Fatalf("list by hook point: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("expected 2 contributions, got %d", len(list))
	}
}

func TestMemoryStore_SetEnabled(t *testing.T) {
	store := NewMemoryContributionStore()
	ctx := context.Background()

	contrib := HookContributionDefinition{
		ContributionID: "contrib-enable-1",
		ExtensionID:    "ext-1",
		HookPointID:    "message.before_send/1",
		ContractVersion: 1,
		Phase:          PhaseBefore,
		RuntimeBinding: RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-1", Entry: "handler"},
	}

	if err := store.Register(ctx, contrib); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := store.SetEnabled(ctx, "contrib-enable-1", true); err != nil {
		t.Fatalf("set enabled: %v", err)
	}

	got, _ := store.Get(ctx, "contrib-enable-1")
	if !got.Enabled {
		t.Error("expected enabled to be true")
	}

	if err := store.SetEnabled(ctx, "contrib-enable-1", false); err != nil {
		t.Fatalf("set disabled: %v", err)
	}

	got, _ = store.Get(ctx, "contrib-enable-1")
	if got.Enabled {
		t.Error("expected enabled to be false")
	}
}

func TestPatchValidator_ValidPatch(t *testing.T) {
	validator := NewPatchValidator()
	point := HookPointDefinition{
		HookPointID:    "message.before_send/1",
		MaxPayloadBytes: 1024 * 1024,
		MaxResultBytes:  128 * 1024,
		AllowedMutations: []MutationRule{
			{Path: "/content", Operations: []string{MutationReplace, MutationAdd, MutationRemove}},
		},
	}
	contrib := HookContributionDefinition{
		ContributionID: "contrib-patch-1",
		Phase:          PhaseTransform,
		MutationClaims: []string{"/content"},
	}

	result := HookResult{
		Decision: DecisionReplace,
		Patch: []MutationOperation{
			{Operation: MutationReplace, Path: "/content", Value: json.RawMessage(`"modified content"`)},
		},
	}

	ctx := &ValidationContext{
		Point:        point,
		Contrib:      contrib,
		WrittenPaths: make(map[string]string),
	}

	errs := validator.Validate(result, ctx)
	if len(errs) != 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errs), errs[0])
	}
}

func TestPatchValidator_UnauthorizedPath(t *testing.T) {
	validator := NewPatchValidator()
	point := HookPointDefinition{
		HookPointID:    "message.before_send/1",
		MaxPayloadBytes: 1024 * 1024,
		MaxResultBytes:  128 * 1024,
		AllowedMutations: []MutationRule{
			{Path: "/content", Operations: []string{MutationReplace, MutationAdd, MutationRemove}},
		},
	}
	contrib := HookContributionDefinition{
		ContributionID: "contrib-patch-2",
		Phase:          PhaseTransform,
		MutationClaims: []string{"/content"},
	}

	result := HookResult{
		Decision: DecisionReplace,
		Patch: []MutationOperation{
			{Operation: MutationReplace, Path: "/unauthorized", Value: json.RawMessage(`"hacked"`)},
		},
	}

	ctx := &ValidationContext{
		Point:        point,
		Contrib:      contrib,
		WrittenPaths: make(map[string]string),
	}

	errs := validator.Validate(result, ctx)
	if len(errs) == 0 {
		t.Error("expected validation error for unauthorized path")
	}
}

func TestOrderHooks_ByPriority(t *testing.T) {
	contribs := []HookContributionDefinition{
		{ContributionID: "low", Priority: 100, Phase: PhaseBefore},
		{ContributionID: "high", Priority: 500, Phase: PhaseBefore},
		{ContributionID: "mid", Priority: 300, Phase: PhaseBefore},
	}

	result := OrderHooks(OrderingInput{Contributions: contribs})
	if len(result.Ordered) != 3 {
		t.Fatalf("expected 3 ordered, got %d", len(result.Ordered))
	}

	if result.Ordered[0].ContributionID != "high" {
		t.Errorf("expected 'high' first, got %s", result.Ordered[0].ContributionID)
	}
	if result.Ordered[1].ContributionID != "mid" {
		t.Errorf("expected 'mid' second, got %s", result.Ordered[1].ContributionID)
	}
	if result.Ordered[2].ContributionID != "low" {
		t.Errorf("expected 'low' third, got %s", result.Ordered[2].ContributionID)
	}
}

func TestHookError_Error(t *testing.T) {
	he := NewHookError(ErrCodeHookRuntimeError, "test error")
	expected := "hook_runtime_error: test error"
	if he.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, he.Error())
	}
	if he.Code != ErrCodeHookRuntimeError {
		t.Errorf("expected code %s, got %s", ErrCodeHookRuntimeError, he.Code)
	}
}

func TestShouldCountCircuitFailure(t *testing.T) {
	tests := []struct {
		code     HookErrorCode
		expected bool
	}{
		{ErrCodeHookRuntimeError, true},
		{ErrCodeHookTimeout, true},
		{ErrCodeHookResultInvalid, true},
		{ErrCodePermissionDenied, true},
		{ErrCodeScopeDenied, false},
		{ErrCodeHookCancelled, false},
		{ErrCodeCircuitOpen, true},
		{ErrCodeDependencyUnavailable, false},
	}

	for _, tt := range tests {
		got := ShouldCountCircuitFailure(tt.code)
		if got != tt.expected {
			t.Errorf("ShouldCountCircuitFailure(%s) = %v, want %v", tt.code, got, tt.expected)
		}
	}
}
