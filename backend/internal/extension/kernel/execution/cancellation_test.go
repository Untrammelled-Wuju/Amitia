package execution

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func newB21TestInvocation(parentID string) capability.ToolInvocationContext {
	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Source:         capability.InvocationSourceModel,
	})
	if parentID != "" {
		inv.ParentID = parentID
		inv.RootID = "root-1"
	}
	return inv
}

func TestB21RegisterAndCancel(t *testing.T) {
	ctrl := NewCancellationController()
	inv := newB21TestInvocation("")

	ctx, cleanup, err := ctrl.Register(context.Background(), inv)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer cleanup()

	if ctx == nil {
		t.Fatal("context should not be nil")
	}

	reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	result := ctrl.CancelInvocation(context.Background(), inv.InvocationID, reason)
	if !result.Requested {
		t.Fatal("expected cancellation requested")
	}
	if len(result.CancelledInvocationIDs) != 1 {
		t.Fatalf("expected 1 cancelled ID, got %d", len(result.CancelledInvocationIDs))
	}
	if result.CancelledInvocationIDs[0] != inv.InvocationID {
		t.Fatalf("expected invocation ID %s, got %s", inv.InvocationID, result.CancelledInvocationIDs[0])
	}

	if ctx.Err() == nil {
		t.Fatal("context should be cancelled")
	}
}

func TestB21DuplicateRegisterRejected(t *testing.T) {
	ctrl := NewCancellationController()
	inv := newB21TestInvocation("")

	_, cleanup, err := ctrl.Register(context.Background(), inv)
	if err != nil {
		t.Fatalf("First Register failed: %v", err)
	}
	defer cleanup()

	_, _, err = ctrl.Register(context.Background(), inv)
	if err == nil {
		t.Fatal("expected error for duplicate invocation ID")
	}
}

func TestB21ParentChildPropagation(t *testing.T) {
	ctrl := NewCancellationController()

	parent := newB21TestInvocation("")
	parent.RootID = "root-1"
	_, parentCleanup, err := ctrl.Register(context.Background(), parent)
	if err != nil {
		t.Fatalf("Register parent failed: %v", err)
	}
	defer parentCleanup()

	child1 := newB21TestInvocation(parent.InvocationID)
	child1.RootID = "root-1"
	_, child1Cleanup, err := ctrl.Register(context.Background(), child1)
	if err != nil {
		t.Fatalf("Register child1 failed: %v", err)
	}
	defer child1Cleanup()

	child2 := newB21TestInvocation(parent.InvocationID)
	child2.RootID = "root-1"
	_, child2Cleanup, err := ctrl.Register(context.Background(), child2)
	if err != nil {
		t.Fatalf("Register child2 failed: %v", err)
	}
	defer child2Cleanup()

	grandchild := newB21TestInvocation(child1.InvocationID)
	grandchild.RootID = "root-1"
	_, grandchildCleanup, err := ctrl.Register(context.Background(), grandchild)
	if err != nil {
		t.Fatalf("Register grandchild failed: %v", err)
	}
	defer grandchildCleanup()

	reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	result := ctrl.CancelInvocation(context.Background(), parent.InvocationID, reason)
	if !result.Requested {
		t.Fatal("expected cancellation requested")
	}
	if len(result.CancelledInvocationIDs) != 4 {
		t.Fatalf("expected 4 cancelled IDs (parent + 2 children + grandchild), got %d: %v", len(result.CancelledInvocationIDs), result.CancelledInvocationIDs)
	}

	if _, found := ctrl.CancelReason(grandchild.InvocationID); !found {
		t.Fatal("expected grandchild to have cancellation reason")
	}
}

func TestB21ChildIsolation(t *testing.T) {
	ctrl := NewCancellationController()

	parent := newB21TestInvocation("")
	parent.RootID = "root-1"
	_, parentCleanup, err := ctrl.Register(context.Background(), parent)
	if err != nil {
		t.Fatalf("Register parent failed: %v", err)
	}
	defer parentCleanup()

	child := newB21TestInvocation(parent.InvocationID)
	child.RootID = "root-1"
	childCtx, childCleanup, err := ctrl.Register(context.Background(), child)
	if err != nil {
		t.Fatalf("Register child failed: %v", err)
	}
	defer childCleanup()

	reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	result := ctrl.CancelInvocation(context.Background(), child.InvocationID, reason)
	if !result.Requested {
		t.Fatal("expected cancellation requested")
	}
	if len(result.CancelledInvocationIDs) != 1 {
		t.Fatalf("expected 1 cancelled ID, got %d", len(result.CancelledInvocationIDs))
	}

	if parentCancel, _ := ctrl.CancelReason(parent.InvocationID); parentCancel.Code != "" {
		t.Fatal("parent should not be cancelled")
	}
	if childCtx.Err() == nil {
		t.Fatal("child context should be cancelled")
	}
}

func TestB21CancelRoot(t *testing.T) {
	ctrl := NewCancellationController()

	inv1 := newB21TestInvocation("")
	inv1.RootID = "root-A"
	_, cleanup1, err := ctrl.Register(context.Background(), inv1)
	if err != nil {
		t.Fatalf("Register inv1 failed: %v", err)
	}
	defer cleanup1()

	inv2 := newB21TestInvocation("")
	inv2.RootID = "root-A"
	_, cleanup2, err := ctrl.Register(context.Background(), inv2)
	if err != nil {
		t.Fatalf("Register inv2 failed: %v", err)
	}
	defer cleanup2()

	other := newB21TestInvocation("")
	other.RootID = "root-B"
	_, otherCleanup, err := ctrl.Register(context.Background(), other)
	if err != nil {
		t.Fatalf("Register other failed: %v", err)
	}
	defer otherCleanup()

	reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	result := ctrl.CancelRoot(context.Background(), "root-A", reason)
	if !result.Requested {
		t.Fatal("expected cancellation requested")
	}
	if len(result.CancelledInvocationIDs) != 2 {
		t.Fatalf("expected 2 cancelled IDs, got %d", len(result.CancelledInvocationIDs))
	}

	if !ctrl.IsActive(other.InvocationID) {
		t.Fatal("other invocation (different root) should still be active")
	}
}

func TestB21CancelUnknownInvocation(t *testing.T) {
	ctrl := NewCancellationController()

	reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	result := ctrl.CancelInvocation(context.Background(), "non-existent", reason)
	if result.Requested {
		t.Fatal("expected cancellation to not be requested for unknown invocation")
	}
}

func TestB21DoubleCancellation(t *testing.T) {
	ctrl := NewCancellationController()
	inv := newB21TestInvocation("")

	_, cleanup, err := ctrl.Register(context.Background(), inv)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer cleanup()

	reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	result1 := ctrl.CancelInvocation(context.Background(), inv.InvocationID, reason)
	if !result1.Requested {
		t.Fatal("first cancellation should be requested")
	}

	result2 := ctrl.CancelInvocation(context.Background(), inv.InvocationID, reason)
	if result2.Requested {
		t.Fatal("second cancellation should not be requested again")
	}
}

func TestB21FinalizeCancellationWins(t *testing.T) {
	ctrl := NewCancellationController()
	inv := newB21TestInvocation("")

	ctx, cleanup, err := ctrl.Register(context.Background(), inv)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer cleanup()

	reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	ctrl.CancelInvocation(context.Background(), inv.InvocationID, reason)

	successResult := capability.NewToolSuccessResult(inv.InvocationID, "tool-1")
	finalResult := ctrl.Finalize(ctx, inv, successResult)

	if finalResult.Status != capability.ToolResultStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", finalResult.Status)
	}
	if finalResult.Error == nil || finalResult.Error.Code != capability.ErrorCodeCancelled {
		t.Fatalf("expected cancelled error code, got %v", finalResult.Error)
	}
}

func TestB21FinalizeSuccessWinsWhenFirst(t *testing.T) {
	ctrl := NewCancellationController()
	inv := newB21TestInvocation("")

	ctx, cleanup, err := ctrl.Register(context.Background(), inv)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer cleanup()

	successResult := capability.NewToolSuccessResult(inv.InvocationID, "tool-1")
	finalResult := ctrl.Finalize(ctx, inv, successResult)

	if finalResult.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success status, got %s", finalResult.Status)
	}
}

func TestB21FinalizePreservesSideEffects(t *testing.T) {
	ctrl := NewCancellationController()
	inv := newB21TestInvocation("")

	ctx, cleanup, err := ctrl.Register(context.Background(), inv)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer cleanup()

	reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	ctrl.CancelInvocation(context.Background(), inv.InvocationID, reason)

	previousResult := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		Status:       capability.ToolResultStatusSuccess,
		SideEffects: []capability.RecordedSideEffect{
			{Type: "file_written", Target: "/tmp/test"},
		},
	}

	finalResult := ctrl.Finalize(ctx, inv, previousResult)

	if finalResult.Status != capability.ToolResultStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", finalResult.Status)
	}
	if len(finalResult.SideEffects) != 1 {
		t.Fatalf("expected side effects to be preserved, got %d", len(finalResult.SideEffects))
	}
}

func TestB21ExternalCallScope(t *testing.T) {
	ctrl := NewCancellationController()

	scopeA := capability.CancellationExternalScope{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-A",
		SessionID:      "",
	}
	scopeB := scopeA
	scopeB.ConversationID = "conv-B"

	invA := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-A",
		ExternalCallID: "call-123",
		Source:         capability.InvocationSourceModel,
	})
	invB := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-B",
		ExternalCallID: "call-123",
		Source:         capability.InvocationSourceModel,
	})

	_, cleanupA, err := ctrl.Register(context.Background(), invA)
	if err != nil {
		t.Fatalf("Register invA failed: %v", err)
	}
	defer cleanupA()

	_, cleanupB, err := ctrl.Register(context.Background(), invB)
	if err != nil {
		t.Fatalf("Register invB failed: %v", err)
	}
	defer cleanupB()

	reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	result := ctrl.CancelExternalCall(context.Background(), scopeA, "call-123", reason)
	if !result.Requested {
		t.Fatal("expected cancellation requested")
	}
	if len(result.CancelledInvocationIDs) != 1 {
		t.Fatalf("expected 1 cancelled ID, got %d", len(result.CancelledInvocationIDs))
	}
	if result.CancelledInvocationIDs[0] != invA.InvocationID {
		t.Fatalf("expected to cancel invA, got %s", result.CancelledInvocationIDs[0])
	}

	if !ctrl.IsActive(invB.InvocationID) {
		t.Fatal("invB (different conversation) should still be active")
	}
}

func TestB21CallerContextCancel(t *testing.T) {
	ctrl := NewCancellationController()

	ctx, cancel := context.WithCancel(context.Background())
	inv := newB21TestInvocation("")

	runCtx, cleanup, err := ctrl.Register(ctx, inv)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer cleanup()

	cancel()

	time.Sleep(50 * time.Millisecond)

	successResult := capability.NewToolSuccessResult(inv.InvocationID, "tool-1")
	finalResult := ctrl.Finalize(runCtx, inv, successResult)

	if finalResult.Status != capability.ToolResultStatusCancelled {
		t.Fatalf("expected cancelled status after caller context cancel, got %s", finalResult.Status)
	}
}

func TestB21CancelReasonInvalidRejected(t *testing.T) {
	ctrl := NewCancellationController()
	inv := newB21TestInvocation("")

	_, cleanup, err := ctrl.Register(context.Background(), inv)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer cleanup()

	invalidReason := capability.ToolCancellationReason{Code: "invalid_code"}
	result := ctrl.CancelInvocation(context.Background(), inv.InvocationID, invalidReason)
	if !result.Requested {
		t.Fatal("expected cancellation requested with default reason")
	}
}

func TestB21ConcurrentRegisterCancel(t *testing.T) {
	ctrl := NewCancellationController()
	const numGoroutines = 50

	var wg sync.WaitGroup
	results := make([]CancellationResult, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			inv := newB21TestInvocation("")
			_, cleanup, err := ctrl.Register(context.Background(), inv)
			if err != nil {
				t.Errorf("Register %d failed: %v", idx, err)
				return
			}
			defer cleanup()

			time.Sleep(time.Duration(idx%5) * time.Millisecond)

			reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
			results[idx] = ctrl.CancelInvocation(context.Background(), inv.InvocationID, reason)
		}(i)
	}

	wg.Wait()

	requested := 0
	for _, r := range results {
		if r.Requested {
			requested++
		}
	}
	if requested != numGoroutines {
		t.Fatalf("expected %d requested cancellations, got %d", numGoroutines, requested)
	}
}

func TestB21AttachRuntimeCancellerInvoked(t *testing.T) {
	ctrl := NewCancellationController()
	inv := newB21TestInvocation("")

	_, cleanup, err := ctrl.Register(context.Background(), inv)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer cleanup()

	var cancelCalled bool
	err = ctrl.AttachRuntimeCanceller(inv.InvocationID, func() {
		cancelCalled = true
	})
	if err != nil {
		t.Fatalf("AttachRuntimeCanceller failed: %v", err)
	}

	reason := capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	ctrl.CancelInvocation(context.Background(), inv.InvocationID, reason)

	if !cancelCalled {
		t.Fatal("expected runtime cancel function to be invoked")
	}
}

func TestB21CleanupIdempotency(t *testing.T) {
	ctrl := NewCancellationController()
	inv := newB21TestInvocation("")

	_, cleanup, err := ctrl.Register(context.Background(), inv)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	cleanup()
	cleanup()
	cleanup()

	if ctrl.IsActive(inv.InvocationID) {
		t.Fatal("invocation should not be active after cleanup")
	}
}
