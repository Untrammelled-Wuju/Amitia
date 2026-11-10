package coordinator

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

func TestInstallRequestHashDoesNotIncludeGeneratedInstallationID(t *testing.T) {
	left := &operation.InstallationOperation{
		OperationType:   operation.TypeInstall,
		SpaceID:         "space",
		DeviceID:        "device",
		InstallationID:  "generated-left",
		PetID:           "pet",
		TargetReleaseID: "release",
	}
	right := *left
	right.InstallationID = "generated-right"
	if operationRequestHash(left, nil) != operationRequestHash(&right, nil) {
		t.Fatal("generated installation id changed install request hash")
	}
}

func TestIdempotencyLockLifetime(t *testing.T) {
	coordinator := &Coordinator{idempotencyLocks: make(map[string]*idempotencyLock)}
	op := &operation.InstallationOperation{
		OperationType:  operation.TypeEnable,
		SpaceID:        "space",
		DeviceID:       "device",
		IdempotencyKey: "same-key",
	}

	unlock := coordinator.lockIdempotency(op)
	if got := coordinator.idempotencyLockCount(); got != 1 {
		t.Fatalf("lock count while held = %d, want 1", got)
	}
	unlock()
	if got := coordinator.idempotencyLockCount(); got != 0 {
		t.Fatalf("lock count after release = %d, want 0", got)
	}

	firstUnlock := coordinator.lockIdempotency(op)
	secondAcquired := make(chan struct{})
	go func() {
		secondUnlock := coordinator.lockIdempotency(op)
		close(secondAcquired)
		secondUnlock()
	}()
	select {
	case <-secondAcquired:
		t.Fatal("same idempotency key acquired concurrently")
	case <-time.After(20 * time.Millisecond):
	}
	firstUnlock()
	select {
	case <-secondAcquired:
	case <-time.After(time.Second):
		t.Fatal("same idempotency key remained locked after release")
	}
	if got := coordinator.idempotencyLockCount(); got != 0 {
		t.Fatalf("lock count after concurrent release = %d, want 0", got)
	}
}

func TestIdempotencyLockHighCardinalityDoesNotRetainKeys(t *testing.T) {
	coordinator := &Coordinator{idempotencyLocks: make(map[string]*idempotencyLock)}
	const workers = 32
	const keysPerWorker = 250
	var next atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < keysPerWorker; j++ {
				id := next.Add(1)
				op := &operation.InstallationOperation{
					OperationType:  operation.TypeUpgrade,
					SpaceID:        "space",
					DeviceID:       "device-" + strconv.FormatInt(id%8, 10),
					IdempotencyKey: strconv.FormatInt(id, 10),
				}
				unlock := coordinator.lockIdempotency(op)
				unlock()
			}
		}()
	}
	wg.Wait()
	if got := coordinator.idempotencyLockCount(); got != 0 {
		t.Fatalf("retained idempotency locks = %d, want 0", got)
	}
}

func (c *Coordinator) idempotencyLockCount() int {
	c.idempotencyMu.Lock()
	defer c.idempotencyMu.Unlock()
	return len(c.idempotencyLocks)
}
