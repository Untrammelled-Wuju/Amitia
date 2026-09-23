package conversationstream

import (
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
