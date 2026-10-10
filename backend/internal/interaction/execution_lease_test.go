package interaction

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func testExecutionLeaseRecord(t *testing.T) (*SQLiteInteractionTracker, *InteractionRecord) {
	t.Helper()
	tracker := newTestSQLiteInteractionTracker(t)
	record := NewInteractionRecord(InteractionScope{
		SpaceID: "space", CharacterID: "character", ConversationID: "conversation",
		Channel: "web", RequestID: "lease-request",
	})
	if err := tracker.Create(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	updated, err := tracker.TransitionCAS(context.Background(), record.ID, record.StatusVersion, InteractionStatusProcessing)
	if err != nil {
		t.Fatal(err)
	}
	return tracker, updated
}

func TestExecutionLeasePreventsDuplicateOwners(t *testing.T) {
	tracker, record := testExecutionLeaseRecord(t)
	ctx := context.Background()
	staleBefore := time.Now().UTC().Add(-executionLeaseStaleAfter)
	owned, err := tracker.ClaimExecution(ctx, record.ID, record.StatusVersion, "first-owner", staleBefore)
	if err != nil || !owned {
		t.Fatalf("first claim failed: owned=%v err=%v", owned, err)
	}
	owned, err = tracker.ClaimExecution(ctx, record.ID, record.StatusVersion, "second-owner", staleBefore)
	if err != nil || owned {
		t.Fatalf("duplicate owner acquired live lease: owned=%v err=%v", owned, err)
	}
	if renewed, err := tracker.RenewExecution(ctx, record.ID, "second-owner", time.Now().UTC()); err != nil || renewed {
		t.Fatalf("non-owner renewed lease: renewed=%v err=%v", renewed, err)
	}
	if err := tracker.ReleaseExecution(ctx, record.ID, "second-owner"); err != nil {
		t.Fatal(err)
	}
	updated, found, err := tracker.Get(ctx, record.ID)
	if err != nil || !found || updated.OwnerInstanceID != "first-owner" {
		t.Fatalf("non-owner released active lease: %+v err=%v", updated, err)
	}
	if err := tracker.ReleaseExecution(ctx, record.ID, "first-owner"); err != nil {
		t.Fatal(err)
	}
	owned, err = tracker.ClaimExecution(ctx, record.ID, record.StatusVersion, "second-owner", staleBefore)
	if err != nil || !owned {
		t.Fatalf("released lease could not be reclaimed: owned=%v err=%v", owned, err)
	}
}

func TestExecutionLeaseStaleTakeoverFencesOriginalOwner(t *testing.T) {
	tracker, record := testExecutionLeaseRecord(t)
	ctx := context.Background()
	staleBefore := time.Now().UTC().Add(-executionLeaseStaleAfter)
	owned, err := tracker.ClaimExecution(ctx, record.ID, record.StatusVersion, "crashed-owner", staleBefore)
	if err != nil || !owned {
		t.Fatalf("first claim failed: %v %v", owned, err)
	}
	if err := tracker.db.Model(&InteractionRecordModel{}).Where("id = ?", record.ID).
		Update("heartbeat_at", time.Now().UTC().Add(-3*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	owned, err = tracker.ClaimExecution(ctx, record.ID, record.StatusVersion, "recovered-owner", staleBefore)
	if err != nil || !owned {
		t.Fatalf("stale lease could not be recovered: %v %v", owned, err)
	}
	renewed, err := tracker.RenewExecution(ctx, record.ID, "crashed-owner", time.Now().UTC())
	if err != nil || renewed {
		t.Fatalf("old owner was not fenced: %v %v", renewed, err)
	}
	if err := tracker.ReleaseExecution(ctx, record.ID, "crashed-owner"); err != nil {
		t.Fatal(err)
	}
	renewed, err = tracker.RenewExecution(ctx, record.ID, "recovered-owner", time.Now().UTC())
	if err != nil || !renewed {
		t.Fatalf("new owner lost lease: %v %v", renewed, err)
	}
}

func TestExecutionLeaseConcurrentClaims(t *testing.T) {
	tracker, record := testExecutionLeaseRecord(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	errs := []error{}
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			owner := UUID()
			ok, err := tracker.ClaimExecution(context.Background(), record.ID, record.StatusVersion,
				owner, time.Now().UTC().Add(-executionLeaseStaleAfter))
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
			}
			if ok {
				winners++
			}
		}()
	}
	wg.Wait()
	if winners != 1 {
		t.Fatalf("expected exactly one winner, got %d (errors=%v)", winners, errs)
	}
	if len(errs) > 0 {
		for _, err := range errs {
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Logf("contending attempt returned database error: %v", err)
			}
		}
	}
}

func TestSQLiteOrchestratorsFenceConcurrentParentRecovery(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	record := NewInteractionRecord(InteractionScope{
		SpaceID: "space", CharacterID: "character", ConversationID: "conversation",
		Channel: "web", Source: "web", RequestID: "original-request",
	})
	if err := tracker.Create(ctx, record); err != nil {
		t.Fatal(err)
	}
	processing, err := tracker.TransitionCAS(ctx, record.ID, record.StatusVersion, InteractionStatusProcessing)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := tracker.TransitionCAS(ctx, record.ID, processing.StatusVersion, InteractionStatusContextReady)
	if err != nil {
		t.Fatal(err)
	}
	proc := &recoveringParentProcessor{started: make(chan struct{}, 1), release: make(chan struct{})}
	first := NewOrchestratorWithStores(DefaultOrchestratorConfig(), proc, tracker, nil)
	second := NewOrchestratorWithStores(DefaultOrchestratorConfig(), proc, tracker, nil)
	first.SetReady(true)
	second.SetReady(true)
	complete := make(chan error, 1)
	go func() {
		_, err := first.Process(ctx, recoverParentInput(ready))
		complete <- err
	}()
	select {
	case <-proc.started:
	case <-time.After(3 * time.Second):
		t.Fatal("original parent was not resumed")
	}
	_, err = second.Process(ctx, recoverParentInput(ready))
	if !errors.Is(err, ErrOrchestratorProcessing) {
		t.Fatalf("independent orchestrator bypassed lease: %v", err)
	}
	close(proc.release)
	select {
	case err := <-complete:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("original parent did not complete")
	}
	if proc.calls.Load() != 1 {
		t.Fatalf("duplicate parent execution: %d", proc.calls.Load())
	}
}
