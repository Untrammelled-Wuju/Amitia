package execution

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func newConcurrencyTool() capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:             "test/tool",
		ExtensionID:    "ext-a",
		HasSideEffects: false,
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeBuiltin,
			RuntimeID:   "test",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:        30 * time.Second,
			MaxConcurrency: 10,
		},
	}
}

func newConcurrencyInvocation(userID string) capability.ToolInvocationContext {
	return capability.ToolInvocationContext{
		InvocationID:   "inv-001",
		UserID:         userID,
		CharacterID:    "char-001",
		ConversationID: "conv-001",
	}
}

func TestConcurrencyEffectiveLimit_ToolIsStricter(t *testing.T) {
	policy := ConcurrencyPolicy{PerToolLimit: 10}
	resolver := NewConcurrencyLimitResolver(policy)
	tool := capability.ToolDefinition{ExecutionPolicy: capability.ToolExecutionPolicy{MaxConcurrency: 3}}
	limits, err := resolver.Resolve(tool, newConcurrencyInvocation("u1"))
	if err != nil {
		t.Fatal(err)
	}
	if limits.Tool != 3 {
		t.Fatalf("expected effective tool limit 3, got %d", limits.Tool)
	}
}

func TestConcurrencyEffectiveLimit_PolicyIsStricter(t *testing.T) {
	policy := ConcurrencyPolicy{PerToolLimit: 2}
	resolver := NewConcurrencyLimitResolver(policy)
	tool := capability.ToolDefinition{ExecutionPolicy: capability.ToolExecutionPolicy{MaxConcurrency: 10}}
	limits, err := resolver.Resolve(tool, newConcurrencyInvocation("u1"))
	if err != nil {
		t.Fatal(err)
	}
	if limits.Tool != 2 {
		t.Fatalf("expected effective tool limit 2, got %d", limits.Tool)
	}
}

func TestConcurrencyEffectiveLimit_NoLimit(t *testing.T) {
	policy := ConcurrencyPolicy{PerToolLimit: 0}
	resolver := NewConcurrencyLimitResolver(policy)
	tool := capability.ToolDefinition{ExecutionPolicy: capability.ToolExecutionPolicy{MaxConcurrency: 0}}
	limits, err := resolver.Resolve(tool, newConcurrencyInvocation("u1"))
	if err != nil {
		t.Fatal(err)
	}
	if limits.Tool != 0 {
		t.Errorf("expected 0 (unlimited), got %d", limits.Tool)
	}
}

func TestConcurrencyEffectiveLimit_NegativePolicy(t *testing.T) {
	policy := ConcurrencyPolicy{PerToolLimit: -1}
	resolver := NewConcurrencyLimitResolver(policy)
	if _, err := resolver.Resolve(newConcurrencyTool(), newConcurrencyInvocation("u1")); err == nil {
		t.Fatal("expected error for negative policy")
	}
}

func TestConcurrencyController_AcquireRelease(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{GlobalLimit: 2})
	if err != nil {
		t.Fatal(err)
	}
	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")

	l1, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	l2, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}

	snap := ctrl.Snapshot()
	if snap.GlobalInUse != 2 {
		t.Fatalf("expected 2 global in-use, got %d", snap.GlobalInUse)
	}

	l1.Release()
	l2.Release()

	snap = ctrl.Snapshot()
	if snap.GlobalInUse != 0 {
		t.Fatalf("expected 0 after release, got %d", snap.GlobalInUse)
	}
}

func TestConcurrencyController_GlobalLimitEnforced(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{GlobalLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")

	_, err = ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	blocked := make(chan struct{})
	go func() {
		ctrl.Acquire(ctx, tool, inv)
		close(blocked)
	}()

	select {
	case <-blocked:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected second acquire to block but it returned")
	}
}

func TestConcurrencyController_PerToolLimitAtomic(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{PerToolLimit: 1, GlobalLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")

	_, err = ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = ctrl.Acquire(ctx, tool, inv)
	if err == nil {
		t.Fatal("expected timeout, got success")
	}
}

func TestConcurrencyController_DoubleRelease(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{GlobalLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")

	lease, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
	lease.Release()

	snap := ctrl.Snapshot()
	if snap.GlobalInUse != 0 {
		t.Fatalf("expected 0 after double release, got %d", snap.GlobalInUse)
	}
}

func TestConcurrencyController_ConcurrentDoubleRelease(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{GlobalLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")

	lease, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease.Release()
		}()
	}
	wg.Wait()

	snap := ctrl.Snapshot()
	if snap.GlobalInUse != 0 {
		t.Fatalf("expected 0 after concurrent double release, got %d", snap.GlobalInUse)
	}
}

func TestConcurrencyController_ContextCancel(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{GlobalLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")

	_, err = ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = ctrl.Acquire(ctx, tool, inv)
	if err == nil {
		t.Fatal("expected error on canceled ctx")
	}
}

func TestConcurrencyController_WaitThenAcquire(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{GlobalLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")

	lease, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}

	got := make(chan error, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := ctrl.Acquire(ctx, tool, inv)
		got <- err
	}()

	lease.Release()

	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("expected successful acquire after release, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second acquire did not unblock after release")
	}
}

func TestConcurrencyController_PerExtension(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{PerExtensionLimit: 1, GlobalLimit: 100})
	if err != nil {
		t.Fatal(err)
	}

	tool1 := newConcurrencyTool()
	tool1.ExtensionID = "ext-a"
	tool1.ID = "ext-a/t1"
	tool2 := newConcurrencyTool()
	tool2.ExtensionID = "ext-a"
	tool2.ID = "ext-a/t2"

	inv := newConcurrencyInvocation("u1")

	_, err = ctrl.Acquire(context.Background(), tool1, inv)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = ctrl.Acquire(ctx, tool2, inv)
	if err == nil {
		t.Fatal("expected per-extension limit conflict")
	}

	tool3 := newConcurrencyTool()
	tool3.ExtensionID = "ext-b"
	tool3.ID = "ext-b/t1"

	_, err = ctrl.Acquire(context.Background(), tool3, inv)
	if err != nil {
		t.Fatalf("different extension should not conflict: %v", err)
	}
}

func TestConcurrencyController_PerCharacter(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{PerCharacterLimit: 1, GlobalLimit: 100})
	if err != nil {
		t.Fatal(err)
	}

	tool := newConcurrencyTool()
	inv1 := newConcurrencyInvocation("u1")
	inv1.CharacterID = "char1"
	inv2 := newConcurrencyInvocation("u1")
	inv2.CharacterID = "char1"

	_, err = ctrl.Acquire(context.Background(), tool, inv1)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = ctrl.Acquire(ctx, tool, inv2)
	if err == nil {
		t.Fatal("expected per-character conflict")
	}

	inv3 := newConcurrencyInvocation("u1")
	inv3.CharacterID = "char2"

	l3, err := ctrl.Acquire(context.Background(), tool, inv3)
	if err != nil {
		t.Fatalf("different character should not conflict: %v", err)
	}
	l3.Release()
}

func TestConcurrencyController_PerConversation(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{PerConversationLimit: 1, GlobalLimit: 100})
	if err != nil {
		t.Fatal(err)
	}

	tool := newConcurrencyTool()
	inv1 := newConcurrencyInvocation("u1")
	inv1.ConversationID = "conv1"
	inv2 := newConcurrencyInvocation("u1")
	inv2.ConversationID = "conv1"

	_, err = ctrl.Acquire(context.Background(), tool, inv1)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = ctrl.Acquire(ctx, tool, inv2)
	if err == nil {
		t.Fatal("expected per-conversation conflict")
	}

	inv3 := newConcurrencyInvocation("u1")
	inv3.ConversationID = "conv2"

	l3, err := ctrl.Acquire(context.Background(), tool, inv3)
	if err != nil {
		t.Fatalf("different conversation should not conflict: %v", err)
	}
	l3.Release()
}

func TestConcurrencyController_FiveDimensionsAtomic(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{
		GlobalLimit:          8,
		PerToolLimit:         4,
		PerExtensionLimit:    5,
		PerCharacterLimit:    3,
		PerConversationLimit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")

	for i := 0; i < 2; i++ {
		_, err := ctrl.Acquire(context.Background(), tool, inv)
		if err != nil {
			t.Fatalf("%d: expected acquire ok: %v", i, err)
		}
	}

	snap := ctrl.Snapshot()
	if snap.ActiveBuckets < 5 {
		t.Fatalf("expected 5 dimension buckets, got %d", snap.ActiveBuckets)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = ctrl.Acquire(ctx, tool, inv)
	if err == nil {
		t.Fatal("expected conversation limit to block 3rd acquire on same conversation")
	}

	inv2 := newConcurrencyInvocation("u1")
	inv2.ConversationID = "conv-x"
	l3, err := ctrl.Acquire(context.Background(), tool, inv2)
	if err != nil {
		t.Fatalf("expected different conversation to acquire: %v", err)
	}
	l3.Release()
}

func TestConcurrencyController_AllFiveDimensionBucketsDistinct(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{
		GlobalLimit:          100,
		PerToolLimit:         100,
		PerExtensionLimit:    100,
		PerCharacterLimit:    100,
		PerConversationLimit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")

	l, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()

	snap := ctrl.Snapshot()
	if snap.ActiveBuckets != 5 {
		t.Fatalf("expected 5 distinct dimension buckets, got %d", snap.ActiveBuckets)
	}
}

func TestConcurrencyController_SameUserCharacterDistinct(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{PerCharacterLimit: 1, GlobalLimit: 100})
	if err != nil {
		t.Fatal(err)
	}

	tool := newConcurrencyTool()
	inv1 := newConcurrencyInvocation("userA")
	inv1.CharacterID = "char-1"
	inv2 := newConcurrencyInvocation("userB")
	inv2.CharacterID = "char-1"

	l1, err := ctrl.Acquire(context.Background(), tool, inv1)
	if err != nil {
		t.Fatal(err)
	}
	l2, err := ctrl.Acquire(context.Background(), tool, inv2)
	if err != nil {
		t.Fatalf("cross-user same local character should be isolated: %v", err)
	}
	l1.Release()
	l2.Release()
}

func TestConcurrencyController_EmptyCharacterNoBucket(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{PerCharacterLimit: 1, GlobalLimit: 100})
	if err != nil {
		t.Fatal(err)
	}

	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")
	inv.CharacterID = ""

	l1, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	l2, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatalf("empty characterID should create no bucket poisoning: %v", err)
	}
	l1.Release()
	l2.Release()
}

func TestConcurrencyController_EmptyConversationNoBucket(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{PerConversationLimit: 1, GlobalLimit: 100})
	if err != nil {
		t.Fatal(err)
	}

	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")
	inv.ConversationID = ""

	l1, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	l2, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatalf("empty conversationID should create no bucket poisoning: %v", err)
	}
	l1.Release()
	l2.Release()
}

func TestConcurrencyController_EmptyExtensionNoBucket(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{PerExtensionLimit: 1, GlobalLimit: 100})
	if err != nil {
		t.Fatal(err)
	}

	tool := newConcurrencyTool()
	tool.ExtensionID = ""
	inv := newConcurrencyInvocation("u1")

	l1, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	l2, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatalf("empty extensionID should create no bucket poisoning: %v", err)
	}
	l1.Release()
	l2.Release()
}

func TestConcurrencyController_BucketCleanup(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{PerConversationLimit: 1, GlobalLimit: 100})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 50; i++ {
		tool := newConcurrencyTool()
		inv := newConcurrencyInvocation("u1")
		inv.ConversationID = "conv-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		l, err := ctrl.Acquire(context.Background(), tool, inv)
		if err != nil {
			t.Fatal(err)
		}
		l.Release()
	}

	snap := ctrl.Snapshot()
	snap2 := ctrl.Snapshot()
	if snap.ActiveBuckets != snap2.ActiveBuckets {
		t.Fatalf("expected buckets cleaned up, got %d (different snapshots: %d)", snap.ActiveBuckets, snap2.ActiveBuckets)
	}
}

func TestConcurrencyController_1000Concurrent(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{GlobalLimit: 32})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var inflight, max int32

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tool := newConcurrencyTool()
			inv := newConcurrencyInvocation("u1")
			l, err := ctrl.Acquire(context.Background(), tool, inv)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			cur := atomic.AddInt32(&inflight, 1)
			for {
				m := atomic.LoadInt32(&max)
				if cur <= m || atomic.CompareAndSwapInt32(&max, m, cur) {
					break
				}
			}
			l.Release()
			atomic.AddInt32(&inflight, -1)
		}()
	}
	wg.Wait()
	if max > 32 {
		t.Fatalf("expected max in-flight <= 32, got %d", max)
	}
}

func TestConcurrencyController_IdempotentCacheNoSlot(t *testing.T) {
	ctrl, err := NewConcurrencyController(ConcurrencyPolicy{GlobalLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	tool := newConcurrencyTool()
	inv := newConcurrencyInvocation("u1")

	l, err := ctrl.Acquire(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}

	snap := ctrl.Snapshot()
	if snap.GlobalInUse != 1 {
		t.Fatalf("expected 1, got %d", snap.GlobalInUse)
	}
	l.Release()
}

func TestConcurrencyController_ResolveWithToolDefinition(t *testing.T) {
	policy := ConcurrencyPolicy{
		GlobalLimit:          100,
		PerToolLimit:         5,
		PerExtensionLimit:    8,
		PerCharacterLimit:    3,
		PerConversationLimit: 2,
	}
	resolver := NewConcurrencyLimitResolver(policy)
	tool := newConcurrencyTool()
	tool.ExecutionPolicy.MaxConcurrency = 3
	inv := newConcurrencyInvocation("u1")

	limits, err := resolver.Resolve(tool, inv)
	if err != nil {
		t.Fatal(err)
	}

	if limits.Global != 100 {
		t.Errorf("Global=%d expected 100", limits.Global)
	}
	if limits.Tool != 3 {
		t.Errorf("Tool=%d expected 3", limits.Tool)
	}
	if limits.Extension != 8 {
		t.Errorf("Extension=%d expected 8", limits.Extension)
	}
	if limits.Character != 3 {
		t.Errorf("Character=%d expected 3", limits.Character)
	}
	if limits.Conversation != 2 {
		t.Errorf("Conversation=%d expected 2", limits.Conversation)
	}
}

func TestConcurrencyPolicy_Immutable(t *testing.T) {
	policy := ConcurrencyPolicy{GlobalLimit: 16}
	ctrl, err := NewConcurrencyController(policy)
	if err != nil {
		t.Fatal(err)
	}
	got := ctrl.Policy()
	if got.GlobalLimit != 16 {
		t.Fatal("Policy() should return a copy equal to input")
	}
	got.GlobalLimit = 99
	again := ctrl.Policy()
	if again.GlobalLimit != 16 {
		t.Fatal("Policy() should return an immutable view")
	}
}

func TestConcurrencyConstructor_NegativePolicy(t *testing.T) {
	_, err := NewConcurrencyController(ConcurrencyPolicy{GlobalLimit: -1})
	if err == nil {
		t.Fatal("expected constructor to reject negative policy")
	}
}
