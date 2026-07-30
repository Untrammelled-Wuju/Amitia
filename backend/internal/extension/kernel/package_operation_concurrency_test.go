package kernel

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPackageOperationConcurrency100SameIdempotencyHasOneAuthority(t *testing.T) {
	repository := newOperationStateTestRepository(t)
	const workers = 100
	var wait sync.WaitGroup
	operationIDs := make(chan string, workers)
	errorsSeen := make(chan error, workers)
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			operation, _, err := repository.CreateOrGetOperation(context.Background(), operationFixture(fmt.Sprintf("acceptance-idempotency-%03d", index), "user-acceptance", "same-request", "sha256:same-request", "com.example/concurrent"))
			if err != nil {
				errorsSeen <- err
				return
			}
			operationIDs <- operation.OperationID
		}(index)
	}
	wait.Wait()
	close(operationIDs)
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatalf("concurrent idempotent create failed: %v", err)
	}
	unique := map[string]bool{}
	for operationID := range operationIDs {
		unique[operationID] = true
	}
	if len(unique) != 1 {
		t.Fatalf("expected one authoritative operation, got %v", unique)
	}
}

func TestPackageOperationConcurrencyCrossLifecycleUsesOneExtensionLease(t *testing.T) {
	repository := newOperationStateTestRepository(t)
	operations := []string{"install", "update", "rollback", "uninstall"}
	for _, operationType := range operations {
		operation := operationFixture("cross-"+operationType, "user-"+operationType, "key-"+operationType, "sha256:"+operationType, "com.example/cross")
		operation.OperationType = operationType
		if _, _, err := repository.CreateOrGetOperation(context.Background(), operation); err != nil {
			t.Fatal(err)
		}
	}
	var wait sync.WaitGroup
	var acquired atomic.Int32
	var conflicts atomic.Int32
	for _, operationType := range operations {
		operationType := operationType
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := repository.AcquireExtensionLease(context.Background(), "com.example/cross", "cross-"+operationType, "worker-"+operationType, time.Minute)
			if err == nil {
				acquired.Add(1)
				return
			}
			if IsPackageOperationError(err, OperationErrLeaseConflict) {
				conflicts.Add(1)
				return
			}
			t.Errorf("unexpected lease error: %v", err)
		}()
	}
	wait.Wait()
	if acquired.Load() != 1 || conflicts.Load() != 3 {
		t.Fatalf("same extension must have one lease: acquired=%d conflicts=%d", acquired.Load(), conflicts.Load())
	}
}

func TestPackageOperationConcurrencyDifferentExtensionsRunInParallel(t *testing.T) {
	repository := newOperationStateTestRepository(t)
	const workers = 24
	for index := 0; index < workers; index++ {
		extensionID := fmt.Sprintf("com.example/parallel-%02d", index)
		operation := operationFixture(fmt.Sprintf("parallel-%02d", index), "user-parallel", fmt.Sprintf("parallel-key-%02d", index), fmt.Sprintf("sha256:parallel-%02d", index), extensionID)
		if _, _, err := repository.CreateOrGetOperation(context.Background(), operation); err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	var wait sync.WaitGroup
	errorsSeen := make(chan error, workers)
	for index := 0; index < workers; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			extensionID := fmt.Sprintf("com.example/parallel-%02d", index)
			_, err := repository.AcquireExtensionLease(context.Background(), extensionID, fmt.Sprintf("parallel-%02d", index), fmt.Sprintf("worker-%02d", index), time.Minute)
			if err != nil {
				errorsSeen <- err
			}
		}()
	}
	close(start)
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatalf("different extension lease acquisition failed: %v", err)
	}
	var leaseCount int
	if err := repository.db.QueryRow(`SELECT COUNT(*) FROM extension_package_operation_leases`).Scan(&leaseCount); err != nil || leaseCount != workers {
		t.Fatalf("expected %d parallel leases, got %d err=%v", workers, leaseCount, err)
	}
}

func TestPackageOperationCrashResumeDoesNotRepeatSideEffect(t *testing.T) {
	repository := newOperationStateTestRepository(t)
	operation := operationFixture("crash-resume", "user-crash", "crash-key", "sha256:crash", "com.example/crash")
	if _, _, err := repository.CreateOrGetOperation(context.Background(), operation); err != nil {
		t.Fatal(err)
	}
	var sideEffects atomic.Int32
	execute := func() error {
		_, created, err := repository.BeginStep(context.Background(), operation.OperationID, "external-side-effect", 1, "sha256:stable-input", PackageWriteGuard{})
		if err != nil {
			return err
		}
		if created {
			sideEffects.Add(1)
		}
		_, err = repository.CompleteStep(context.Background(), operation.OperationID, "external-side-effect", "sha256:stable-input", `{"committed":true}`, "side-effect:stable-evidence", PackageWriteGuard{})
		return err
	}
	if err := execute(); err != nil {
		t.Fatal(err)
	}
	if err := execute(); err != nil {
		t.Fatal(err)
	}
	if sideEffects.Load() != 1 {
		t.Fatalf("crash resume repeated side effect %d times", sideEffects.Load())
	}
	var stepCount int
	var attemptCount int
	var evidence string
	if err := repository.db.QueryRow(`SELECT COUNT(*), MAX(attempt_count), MAX(side_effect_evidence) FROM extension_package_operation_steps WHERE operation_id=?`, operation.OperationID).Scan(&stepCount, &attemptCount, &evidence); err != nil || stepCount != 1 || attemptCount != 1 || evidence != "side-effect:stable-evidence" {
		t.Fatalf("unexpected replay journal: count=%d attempts=%d evidence=%s err=%v", stepCount, attemptCount, evidence, err)
	}
}
