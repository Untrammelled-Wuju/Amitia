package hook

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestPlanCache_BuildAndGet(t *testing.T) {
	registry := NewHookPointRegistry()
	ctx := context.Background()
	if err := RegisterDefaultHookPoints(ctx, registry); err != nil {
		t.Fatalf("register points: %v", err)
	}

	store := NewMemoryContributionStore()
	bridge := NewDirectRuntimeBridge()
	pipeline := NewPipeline(registry, store, bridge)

	contrib := HookContributionDefinition{
		ContributionID:  "plan-test-1",
		ExtensionID:     "ext-1",
		HookPointID:     "message.before_send/1",
		ContractVersion: 1,
		Phase:           PhaseBefore,
		Priority:        100,
		Enabled:         true,
		RuntimeBinding:  RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-1", Entry: "handler"},
		DefinitionHash:  "hash-1",
	}
	if err := store.Register(ctx, contrib); err != nil {
		t.Fatalf("register: %v", err)
	}

	plan := pipeline.RebuildPlan(ctx, "message.before_send/1")
	if plan == nil {
		t.Fatal("expected plan, got nil")
	}
	if plan.HookPointID != "message.before_send/1" {
		t.Errorf("expected hook point ID message.before_send/1, got %s", plan.HookPointID)
	}
	if len(plan.Ordered) != 1 {
		t.Fatalf("expected 1 ordered contribution, got %d", len(plan.Ordered))
	}
	if plan.Ordered[0].ContributionID != "plan-test-1" {
		t.Errorf("expected contribution plan-test-1, got %s", plan.Ordered[0].ContributionID)
	}
	if plan.DefinitionHash == "" {
		t.Error("expected non-empty definition hash")
	}
	if plan.PlanGeneration != 1 {
		t.Errorf("expected generation 1, got %d", plan.PlanGeneration)
	}
}

func TestPlanCache_Invalidation(t *testing.T) {
	registry := NewHookPointRegistry()
	ctx := context.Background()
	if err := RegisterDefaultHookPoints(ctx, registry); err != nil {
		t.Fatalf("register points: %v", err)
	}

	store := NewMemoryContributionStore()
	bridge := NewDirectRuntimeBridge()
	pipeline := NewPipeline(registry, store, bridge)

	contrib := HookContributionDefinition{
		ContributionID:  "plan-inv-1",
		ExtensionID:     "ext-1",
		HookPointID:     "message.before_send/1",
		ContractVersion: 1,
		Phase:           PhaseBefore,
		Enabled:         true,
		RuntimeBinding:  RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-1", Entry: "handler"},
		DefinitionHash:  "hash-1",
	}
	if err := store.Register(ctx, contrib); err != nil {
		t.Fatalf("register: %v", err)
	}

	pipeline.RebuildPlan(ctx, "message.before_send/1")
	_, ok := pipeline.PlanCache.Get("message.before_send/1")
	if !ok {
		t.Fatal("expected plan in cache")
	}

	pipeline.InvalidatePlan("message.before_send/1")
	_, ok = pipeline.PlanCache.Get("message.before_send/1")
	if ok {
		t.Error("expected plan to be invalidated")
	}
}

func TestPlanCache_OrderingByPhase(t *testing.T) {
	registry := NewHookPointRegistry()
	ctx := context.Background()
	if err := RegisterDefaultHookPoints(ctx, registry); err != nil {
		t.Fatalf("register points: %v", err)
	}

	store := NewMemoryContributionStore()
	bridge := NewDirectRuntimeBridge()
	pipeline := NewPipeline(registry, store, bridge)

	contrib1 := HookContributionDefinition{
		ContributionID:  "order-1",
		ExtensionID:     "ext-1",
		HookPointID:     "message.before_send/1",
		ContractVersion: 1,
		Phase:           PhaseTransform,
		Priority:        100,
		Enabled:         true,
		RuntimeBinding:  RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-1", Entry: "handler"},
		DefinitionHash:  "h1",
	}
	contrib2 := HookContributionDefinition{
		ContributionID:  "order-2",
		ExtensionID:     "ext-1",
		HookPointID:     "message.before_send/1",
		ContractVersion: 1,
		Phase:           PhaseBefore,
		Priority:        50,
		Enabled:         true,
		RuntimeBinding:  RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-1", Entry: "handler"},
		DefinitionHash:  "h2",
	}
	if err := store.Register(ctx, contrib1); err != nil {
		t.Fatalf("register contrib1: %v", err)
	}
	if err := store.Register(ctx, contrib2); err != nil {
		t.Fatalf("register contrib2: %v", err)
	}

	plan := pipeline.RebuildPlan(ctx, "message.before_send/1")
	if len(plan.Ordered) != 2 {
		t.Fatalf("expected 2 ordered, got %d", len(plan.Ordered))
	}
	if plan.Ordered[0].Phase != PhaseBefore {
		t.Errorf("expected first phase before, got %s", plan.Ordered[0].Phase)
	}
	if plan.Ordered[1].Phase != PhaseTransform {
		t.Errorf("expected second phase transform, got %s", plan.Ordered[1].Phase)
	}
}

func TestPlanCache_CircuitOpenExcluded(t *testing.T) {
	registry := NewHookPointRegistry()
	ctx := context.Background()
	if err := RegisterDefaultHookPoints(ctx, registry); err != nil {
		t.Fatalf("register points: %v", err)
	}

	store := NewMemoryContributionStore()
	bridge := NewDirectRuntimeBridge()
	pipeline := NewPipeline(registry, store, bridge)

	contrib := HookContributionDefinition{
		ContributionID:  "circ-open-1",
		ExtensionID:     "ext-1",
		HookPointID:     "message.before_send/1",
		ContractVersion: 1,
		Phase:           PhaseBefore,
		Enabled:         true,
		RuntimeBinding:  RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-1", Entry: "handler"},
		DefinitionHash:  "h1",
	}
	if err := store.Register(ctx, contrib); err != nil {
		t.Fatalf("register: %v", err)
	}

	for i := 0; i < 5; i++ {
		pipeline.Circuit.RecordFailure("circ-open-1", ErrCodeHookRuntimeError)
	}

	plan := pipeline.RebuildPlan(ctx, "message.before_send/1")
	if len(plan.Ordered) != 0 {
		t.Errorf("expected 0 ordered (circuit open), got %d", len(plan.Ordered))
	}
}

func TestPlanCache_DisabledExcluded(t *testing.T) {
	registry := NewHookPointRegistry()
	ctx := context.Background()
	if err := RegisterDefaultHookPoints(ctx, registry); err != nil {
		t.Fatalf("register points: %v", err)
	}

	store := NewMemoryContributionStore()
	bridge := NewDirectRuntimeBridge()
	pipeline := NewPipeline(registry, store, bridge)

	contrib := HookContributionDefinition{
		ContributionID:  "dis-1",
		ExtensionID:     "ext-1",
		HookPointID:     "message.before_send/1",
		ContractVersion: 1,
		Phase:           PhaseBefore,
		Enabled:         false,
		RuntimeBinding:  RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-1", Entry: "handler"},
		DefinitionHash:  "h1",
	}
	if err := store.Register(ctx, contrib); err != nil {
		t.Fatalf("register: %v", err)
	}

	plan := pipeline.RebuildPlan(ctx, "message.before_send/1")
	if len(plan.Ordered) != 0 {
		t.Errorf("expected 0 ordered (disabled), got %d", len(plan.Ordered))
	}
}

func TestCompiledHookPlan_LookupByPhase(t *testing.T) {
	plan := &CompiledHookPlan{
		HookPointID: "test.point/1",
		Ordered: []CompiledHookContribution{
			{ContributionID: "a", Phase: PhaseBefore},
			{ContributionID: "b", Phase: PhaseTransform},
			{ContributionID: "c", Phase: PhaseTransform},
			{ContributionID: "d", Phase: PhaseAfter},
		},
	}

	before := plan.LookupByPhase(PhaseBefore)
	if len(before) != 1 || before[0].ContributionID != "a" {
		t.Errorf("expected 1 before, got %v", before)
	}

	transform := plan.LookupByPhase(PhaseTransform)
	if len(transform) != 2 {
		t.Errorf("expected 2 transform, got %d", len(transform))
	}

	observe := plan.LookupByPhase(PhaseObserve)
	if len(observe) != 0 {
		t.Errorf("expected 0 observe, got %d", len(observe))
	}
}

func TestCompiledHookPlan_FindContribution(t *testing.T) {
	plan := &CompiledHookPlan{
		HookPointID: "test.point/1",
		Ordered: []CompiledHookContribution{
			{ContributionID: "x", Phase: PhaseBefore},
			{ContributionID: "y", Phase: PhaseAfter},
		},
	}

	c, ok := plan.FindContribution("y")
	if !ok || c.ContributionID != "y" {
		t.Errorf("expected to find y, got %v, %v", c, ok)
	}

	_, ok = plan.FindContribution("z")
	if ok {
		t.Error("did not expect to find z")
	}
}

func TestCompiledHookPlan_IsStale(t *testing.T) {
	plan := &CompiledHookPlan{PlanGeneration: 5}
	if !plan.IsStale(6) {
		t.Error("expected stale when generations differ")
	}
	if plan.IsStale(5) {
		t.Error("expected not stale when generations match")
	}
}

func TestPlanCache_Generation(t *testing.T) {
	cache := NewPlanCache()
	if cache.Generation() != 0 {
		t.Errorf("expected initial generation 0, got %d", cache.Generation())
	}

	point := HookPointDefinition{
		HookPointID:     "gen.test/1",
		ContractVersion: 1,
		SupportedPhases: []HookPhase{PhaseBefore},
	}
	cache.BuildOrReplace(point, nil, nil)
	if cache.Generation() != 1 {
		t.Errorf("expected generation 1, got %d", cache.Generation())
	}

	cache.BuildOrReplace(point, nil, nil)
	if cache.Generation() != 2 {
		t.Errorf("expected generation 2, got %d", cache.Generation())
	}
}

func TestPlanCache_InvalidateAll(t *testing.T) {
	cache := NewPlanCache()
	point := HookPointDefinition{
		HookPointID:     "inv.all/1",
		ContractVersion: 1,
		SupportedPhases: []HookPhase{PhaseBefore},
	}
	cache.BuildOrReplace(point, nil, nil)
	cache.InvalidateAll()
	_, ok := cache.Get("inv.all/1")
	if ok {
		t.Error("expected cache to be empty after InvalidateAll")
	}
}

func TestCompiledHash_Deterministic(t *testing.T) {
	ordered := []CompiledHookContribution{
		{ContributionID: "a", Phase: PhaseBefore},
		{ContributionID: "b", Phase: PhaseAfter},
	}
	hash1 := computePlanDefinitionHash("test/1", ordered)
	hash2 := computePlanDefinitionHash("test/1", ordered)
	if hash1 != hash2 {
		t.Errorf("expected same hash, got %s != %s", hash1, hash2)
	}

	different := []CompiledHookContribution{
		{ContributionID: "c", Phase: PhaseBefore},
	}
	hash3 := computePlanDefinitionHash("test/1", different)
	if hash1 == hash3 {
		t.Error("expected different hash for different contributions")
	}
}

func TestPipeline_UsesPlanCache(t *testing.T) {
	registry := NewHookPointRegistry()
	ctx := context.Background()
	if err := RegisterDefaultHookPoints(ctx, registry); err != nil {
		t.Fatalf("register points: %v", err)
	}

	store := NewMemoryContributionStore()
	bridge := NewDirectRuntimeBridge()
	pipeline := NewPipeline(registry, store, bridge)

	contrib := HookContributionDefinition{
		ContributionID:  "plan-cache-test-1",
		ExtensionID:     "ext-1",
		HookPointID:     "message.before_send/1",
		ContractVersion: 1,
		Phase:           PhaseBefore,
		Enabled:         true,
		RuntimeBinding:  RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-1", Entry: "handler"},
		DefinitionHash:  "h1",
	}
	if err := store.Register(ctx, contrib); err != nil {
		t.Fatalf("register: %v", err)
	}
	bridge.Bind("plan-cache-test-1", func(ctx context.Context, input HookInvocationInput) (HookResult, error) {
		return ContinueResult(), nil
	})

	pipeline.RebuildPlan(ctx, "message.before_send/1")

	req := InvokeRequest{
		HookPointID: "message.before_send/1",
		Payload:     json.RawMessage(`{"text":"hello"}`),
		Context:     HookContextSnapshot{InvocationID: "inv-1", Timestamp: time.Now().UTC()},
	}
	result := pipeline.Invoke(ctx, req)
	if result.Aborted {
		t.Errorf("expected not aborted, got: %s", result.AbortReason)
	}
	if len(result.Executions) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(result.Executions))
	}
	if result.Executions[0].Status != StatusSuccess {
		t.Errorf("expected success, got %s", result.Executions[0].Status)
	}
}

func TestPipeline_DepthPropagation(t *testing.T) {
	registry := NewHookPointRegistry()
	ctx := context.Background()
	if err := RegisterDefaultHookPoints(ctx, registry); err != nil {
		t.Fatalf("register points: %v", err)
	}

	store := NewMemoryContributionStore()
	bridge := NewDirectRuntimeBridge()
	pipeline := NewPipeline(registry, store, bridge)

	contrib := HookContributionDefinition{
		ContributionID:  "depth-test-1",
		ExtensionID:     "ext-1",
		HookPointID:     "message.before_send/1",
		ContractVersion: 1,
		Phase:           PhaseBefore,
		Enabled:         true,
		RuntimeBinding:  RuntimeBinding{RuntimeType: "wasm", ModuleID: "mod-1", Entry: "handler"},
		DefinitionHash:  "h1",
	}
	if err := store.Register(ctx, contrib); err != nil {
		t.Fatalf("register: %v", err)
	}
	bridge.Bind("depth-test-1", func(ctx context.Context, input HookInvocationInput) (HookResult, error) {
		depth := DepthFromContext(ctx)
		if depth != 1 {
			return HookResult{}, nil
		}
		return ContinueResult(), nil
	})

	nestedCtx := ContextWithDepth(context.Background(), 0)
	req := InvokeRequest{
		HookPointID: "message.before_send/1",
		Payload:     json.RawMessage(`{"text":"hello"}`),
		Context:     HookContextSnapshot{InvocationID: "depth-inv", Timestamp: time.Now().UTC()},
		Depth:       0,
	}
	result := pipeline.Invoke(nestedCtx, req)
	if result.Aborted {
		t.Errorf("expected not aborted, got: %s", result.AbortReason)
	}
}
