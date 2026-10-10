package execution

import (
	"sync"
	"testing"
)

func TestIdempotencyGuardCloseStopsBackgroundCleanup(t *testing.T) {
	guard := NewIdempotencyGuard(NewExecutionIdempotencyStorage(newTestIdempotencyDB(t)))
	var callers sync.WaitGroup
	for i := 0; i < 32; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			guard.Close()
		}()
	}
	callers.Wait()
	guard.Close()
	select {
	case <-guard.cleanupStop:
	default:
		t.Fatal("idempotency cleanup goroutine was not signaled to stop")
	}
}
