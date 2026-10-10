package conversationstream

import (
	"context"
	"testing"
	"time"
)

func TestManagerLimitsConcurrentExecutions(t *testing.T) {
	manager := NewManager()
	manager.SetMaxConcurrentExecutions(1)

	_, cancelFirst, started := manager.BeginExecution("conv-a", "turn-a")
	if !started {
		t.Fatal("first execution did not start")
	}
	startedSecond := make(chan bool, 1)
	go func() {
		_, cancel, ok := manager.BeginExecution("conv-b", "turn-b")
		if ok {
			cancel()
		}
		startedSecond <- ok
	}()

	select {
	case <-startedSecond:
		t.Fatal("second execution started before the global slot was released")
	case <-time.After(50 * time.Millisecond):
	}

	manager.ClearExecution("conv-a", "turn-a")
	cancelFirst()
	select {
	case ok := <-startedSecond:
		if !ok {
			t.Fatal("second execution did not start after slot release")
		}
	case <-time.After(time.Second):
		t.Fatal("second execution remained blocked")
	}
}

func TestManagerRejectsConcurrentExecutionForSameConversation(t *testing.T) {
	manager := NewManager()
	_, cancelFirst, started := manager.BeginExecution("conv-a", "turn-a")
	if !started {
		t.Fatal("first execution did not start")
	}
	defer cancelFirst()

	if _, _, started = manager.BeginExecution("conv-a", "turn-b"); started {
		t.Fatal("same conversation started a second concurrent execution")
	}
}

func TestManagerRetainsReconciliationWaitingStateForSnapshotAndReplay(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()
	for _, event := range []AgentUIEvent{
		{ConversationID: "recovery-conv", TurnID: "turn-1", ExecutionID: "exec-1",
			Type: "turn.started", Status: "running"},
		{ConversationID: "recovery-conv", TurnID: "turn-1", ExecutionID: "exec-1",
			Type: "tool.running", BlockID: "call-1", BlockSequence: 1, Status: "running",
			Payload: map[string]any{"blockType": "tool_call"}},
		{ConversationID: "recovery-conv", TurnID: "turn-1", ExecutionID: "exec-1",
			Type: "turn.waiting", Status: "needs_reconciliation"},
	} {
		if _, err := manager.Publish(ctx, event, true); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := manager.RuntimeSnapshot("recovery-conv")
	if snapshot.ActiveTurn == nil || snapshot.ActiveTurn.TurnID != "turn-1" ||
		snapshot.ActiveTurn.Status != "needs_reconciliation" ||
		snapshot.ActiveTurn.ExecutionID != "exec-1" {
		t.Fatalf("snapshot lost waiting original execution: %+v", snapshot)
	}
	_, replay, _, unsubscribe := manager.Subscribe("recovery-conv", "replay-waiting", 1)
	defer unsubscribe()
	if len(replay) != 2 || replay[len(replay)-1].Type != "turn.waiting" ||
		replay[len(replay)-1].Status != "needs_reconciliation" {
		t.Fatalf("replay lost waiting decision: %+v", replay)
	}
}
