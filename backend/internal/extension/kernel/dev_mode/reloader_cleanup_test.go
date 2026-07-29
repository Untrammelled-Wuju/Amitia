package dev_mode

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memCleanupFailureStore struct {
	mu      sync.Mutex
	records map[string]*CleanupFailureRecord
	saveErr error
	listErr error
	delErr  error
	updErr  error
	cntErr  error
}

func newMemCleanupFailureStore() *memCleanupFailureStore {
	return &memCleanupFailureStore{records: make(map[string]*CleanupFailureRecord)}
}

func (s *memCleanupFailureStore) Save(_ context.Context, record *CleanupFailureRecord) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if record.FailureID == "" {
		record.FailureID = "fail-test-" + time.Now().Format("150405.000000")
	}
	cp := *record
	s.records[record.FailureID] = &cp
	return nil
}

func (s *memCleanupFailureStore) ListPending(_ context.Context) ([]*CleanupFailureRecord, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*CleanupFailureRecord
	for _, r := range s.records {
		if r.Status == CleanupFailurePending || r.Status == CleanupFailureRetrying {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *memCleanupFailureStore) ListAll(_ context.Context) ([]*CleanupFailureRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*CleanupFailureRecord
	for _, r := range s.records {
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}

func (s *memCleanupFailureStore) Delete(_ context.Context, failureID string) error {
	if s.delErr != nil {
		return s.delErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, failureID)
	return nil
}

func (s *memCleanupFailureStore) UpdateRetry(_ context.Context, failureID string, retryCount int, nextRetryAt time.Time, status CleanupFailureStatus) error {
	if s.updErr != nil {
		return s.updErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.records[failureID]; ok {
		r.RetryCount = retryCount
		r.NextRetryAt = nextRetryAt
		r.Status = status
		r.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (s *memCleanupFailureStore) Count(_ context.Context) (int64, error) {
	if s.cntErr != nil {
		return 0, s.cntErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for _, r := range s.records {
		if r.Status == CleanupFailurePending || r.Status == CleanupFailureRetrying {
			count++
		}
	}
	return count, nil
}

func TestReloadStopFailurePersistsCleanupRecord(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	store := newMemCleanupFailureStore()
	reloader.SetCleanupFailureStore(store)

	runner1 := &fakeCandidateRunner{instanceID: "inst-candidate-1"}
	reloader.SetCandidateRunner(runner1)
	_, _ = reloader.Reload(ctx, wsID, "test", nil)

	runner2 := &fakeCandidateRunner{
		instanceID: "inst-candidate-2",
		stopErr:    errors.New("process refused to terminate"),
	}
	reloader.SetCandidateRunner(runner2)

	ev2, err := reloader.Reload(ctx, wsID, "test2", nil)
	if err != nil {
		t.Fatalf("Reload should not return error: %v", err)
	}
	if !ev2.Success {
		t.Fatalf("Reload should succeed (new version promoted): %s", ev2.Error)
	}
	if !ev2.CleanupFailed {
		t.Fatal("expected CleanupFailed=true")
	}
	if ev2.Status != ReloadSucceededWithCleanupFailure {
		t.Fatalf("expected status reload_succeeded_with_cleanup_failure, got %s", ev2.Status)
	}

	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("store count error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 persisted cleanup failure, got %d", count)
	}

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("list pending error: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending record, got %d", len(pending))
	}
	if pending[0].OldInstanceID != "inst-candidate-1" {
		t.Fatalf("expected old_instance_id=inst-candidate-1, got %s", pending[0].OldInstanceID)
	}
	if pending[0].ErrorCode != "RUNTIME_STOP_FAILED" {
		t.Fatalf("expected error_code=RUNTIME_STOP_FAILED, got %s", pending[0].ErrorCode)
	}
}

func TestReloadStopFailureDoesNotRollbackNewVersion(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	store := newMemCleanupFailureStore()
	reloader.SetCleanupFailureStore(store)

	runner1 := &fakeCandidateRunner{instanceID: "inst-old"}
	reloader.SetCandidateRunner(runner1)
	_, _ = reloader.Reload(ctx, wsID, "test", nil)

	runner2 := &fakeCandidateRunner{
		instanceID: "inst-new",
		stopErr:    errors.New("cannot kill process"),
	}
	reloader.SetCandidateRunner(runner2)

	ev2, err := reloader.Reload(ctx, wsID, "test2", nil)
	if err != nil {
		t.Fatalf("Reload should not return error: %v", err)
	}
	if !ev2.Success {
		t.Fatalf("new version should remain promoted: %s", ev2.Error)
	}

	reloader.instanceMu.Lock()
	current := reloader.currentInstance[wsID]
	reloader.instanceMu.Unlock()
	if current != "inst-new" {
		t.Fatalf("expected current instance to be inst-new (not rolled back), got %s", current)
	}
}

func TestRecoverStaleInstancesFromPersistentStore(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	store := newMemCleanupFailureStore()
	reloader.SetCleanupFailureStore(store)

	failureRecord := &CleanupFailureRecord{
		WorkspaceID:   wsID,
		ExtensionID:   "com.test/ext",
		OldInstanceID: "inst-orphan-1",
		ErrorCode:     "RUNTIME_STOP_FAILED",
		ErrorMessage:  "timeout",
		MaxRetries:    5,
		NextRetryAt:   time.Now().UTC().Add(-1 * time.Minute),
		Status:        CleanupFailurePending,
	}
	if err := store.Save(ctx, failureRecord); err != nil {
		t.Fatal(err)
	}

	runner := &fakeCandidateRunner{instanceID: "inst-fresh"}
	reloader.SetCandidateRunner(runner)

	cleaned := reloader.RecoverStaleInstances(ctx)
	if cleaned != 1 {
		t.Fatalf("expected 1 cleaned from persistent store, got %d", cleaned)
	}

	count, _ := store.Count(ctx)
	if count != 0 {
		t.Fatalf("expected 0 pending after recovery, got %d", count)
	}
}

func TestRecoverStaleInstancesRetryBackoff(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	store := newMemCleanupFailureStore()
	reloader.SetCleanupFailureStore(store)

	failureRecord := &CleanupFailureRecord{
		WorkspaceID:   wsID,
		ExtensionID:   "com.test/ext",
		OldInstanceID: "inst-stubborn",
		ErrorCode:     "RUNTIME_STOP_FAILED",
		ErrorMessage:  "process stuck",
		RetryCount:    0,
		MaxRetries:    3,
		NextRetryAt:   time.Now().UTC().Add(-1 * time.Minute),
		Status:        CleanupFailurePending,
	}
	if err := store.Save(ctx, failureRecord); err != nil {
		t.Fatal(err)
	}

	runner := &fakeCandidateRunner{
		instanceID: "inst-fresh",
		stopErr:    errors.New("still stuck"),
	}
	reloader.SetCandidateRunner(runner)

	cleaned := reloader.RecoverStaleInstances(ctx)
	if cleaned != 0 {
		t.Fatalf("expected 0 cleaned (stop still fails), got %d", cleaned)
	}

	pending, _ := store.ListPending(ctx)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending record, got %d", len(pending))
	}
	if pending[0].RetryCount != 1 {
		t.Fatalf("expected retry_count=1 after first retry, got %d", pending[0].RetryCount)
	}
	if pending[0].Status != CleanupFailureRetrying {
		t.Fatalf("expected status=retrying, got %s", pending[0].Status)
	}
}

func TestRecoverStaleInstancesMaxRetriesExhausted(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	store := newMemCleanupFailureStore()
	reloader.SetCleanupFailureStore(store)

	failureRecord := &CleanupFailureRecord{
		WorkspaceID:   wsID,
		ExtensionID:   "com.test/ext",
		OldInstanceID: "inst-permanent-stuck",
		ErrorCode:     "RUNTIME_STOP_FAILED",
		ErrorMessage:  "process permanently stuck",
		RetryCount:    3,
		MaxRetries:    3,
		NextRetryAt:   time.Now().UTC().Add(-1 * time.Minute),
		Status:        CleanupFailureRetrying,
	}
	if err := store.Save(ctx, failureRecord); err != nil {
		t.Fatal(err)
	}

	runner := &fakeCandidateRunner{
		instanceID: "inst-fresh",
		stopErr:    errors.New("still stuck"),
	}
	reloader.SetCandidateRunner(runner)

	cleaned := reloader.RecoverStaleInstances(ctx)
	if cleaned != 0 {
		t.Fatalf("expected 0 cleaned (retries exhausted), got %d", cleaned)
	}

	all, _ := store.ListAll(ctx)
	if len(all) != 1 {
		t.Fatalf("expected 1 record total, got %d", len(all))
	}
	if all[0].Status != CleanupFailureExhausted {
		t.Fatalf("expected status=exhausted, got %s", all[0].Status)
	}
}

func TestReloadStopTimeoutRespected(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	store := newMemCleanupFailureStore()
	reloader.SetCleanupFailureStore(store)
	reloader.SetStopTimeout(100 * time.Millisecond)

	runner1 := &fakeCandidateRunner{instanceID: "inst-old"}
	reloader.SetCandidateRunner(runner1)
	_, _ = reloader.Reload(ctx, wsID, "test", nil)

	stopCalled := make(chan struct{})
	runner2 := &fakeCandidateRunner{
		instanceID: "inst-new",
		stopErr:    errors.New("timeout"),
	}
	reloader.SetCandidateRunner(runner2)

	ev2, _ := reloader.Reload(ctx, wsID, "test2", nil)
	if !ev2.CleanupFailed {
		t.Fatal("expected cleanup failure due to stop timeout")
	}
	close(stopCalled)

	count, _ := store.Count(ctx)
	if count != 1 {
		t.Fatalf("expected 1 persisted failure, got %d", count)
	}
}

func TestPendingCleanupFailuresCount(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	store := newMemCleanupFailureStore()
	reloader.SetCleanupFailureStore(store)

	count, err := reloader.PendingCleanupFailures(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 pending, got %d", count)
	}

	failureRecord := &CleanupFailureRecord{
		WorkspaceID:   wsID,
		OldInstanceID: "inst-orphan",
		NextRetryAt:   time.Now().UTC(),
		Status:        CleanupFailurePending,
	}
	_ = store.Save(ctx, failureRecord)

	count, err = reloader.PendingCleanupFailures(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 pending, got %d", count)
	}
}
