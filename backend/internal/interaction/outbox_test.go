package interaction

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type testOutboxPublisher struct {
	mu        sync.Mutex
	err       error
	published []string
}

func (p *testOutboxPublisher) Publish(record OutboxRecord) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.published = append(p.published, record.ID)
	return p.err
}

func (p *testOutboxPublisher) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.published)
}

func TestInMemoryOutboxLeasePendingAndReleaseExpiredLease(t *testing.T) {
	store := NewInMemoryOutboxStore()
	id, err := store.Append(&OutboxRecord{
		EventType:   "interaction.completed",
		AggregateID: "agg-1",
		Payload:     []byte(`{"ok":true}`),
		MaxRetries:  3,
	})
	if err != nil {
		t.Fatal(err)
	}

	leaseUntil := time.Now().Add(time.Minute)
	leased, err := store.LeasePending(1, leaseUntil)
	if err != nil {
		t.Fatal(err)
	}
	if len(leased) != 1 || leased[0].ID != id || leased[0].Status != OutboxStatusProcessing {
		t.Fatalf("unexpected leased records: %#v", leased)
	}
	if leased[0].LeasedUntil.IsZero() {
		t.Fatal("expected lease deadline")
	}

	leasedAgain, err := store.LeasePending(1, leaseUntil.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(leasedAgain) != 0 {
		t.Fatalf("leased processing record again: %#v", leasedAgain)
	}

	if err := store.ReleaseExpiredLeases(leaseUntil.Add(time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	rec, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != OutboxStatusPending || !rec.LeasedUntil.IsZero() {
		t.Fatalf("expired lease was not released: %#v", rec)
	}
}

func TestSQLiteOutboxLeasePendingIsExclusiveAndRecoverable(t *testing.T) {
	store := newTestSQLiteOutboxStore(t)
	id, err := store.Append(&OutboxRecord{
		EventType:   "interaction.completed",
		AggregateID: "agg-1",
		Payload:     []byte(`{"ok":true}`),
		MaxRetries:  3,
	})
	if err != nil {
		t.Fatal(err)
	}

	leaseUntil := time.Now().Add(time.Minute)
	leased, err := store.LeasePending(1, leaseUntil)
	if err != nil {
		t.Fatal(err)
	}
	if len(leased) != 1 || leased[0].ID != id || leased[0].Status != OutboxStatusProcessing {
		t.Fatalf("unexpected leased records: %#v", leased)
	}

	leasedAgain, err := store.LeasePending(1, leaseUntil.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(leasedAgain) != 0 {
		t.Fatalf("leased processing record again: %#v", leasedAgain)
	}

	if err := store.ReleaseExpiredLeases(leaseUntil.Add(time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	rec, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != OutboxStatusPending || !rec.LeasedUntil.IsZero() {
		t.Fatalf("expired lease was not released: %#v", rec)
	}
}

func TestSQLiteOutboxFailedRecordRespectsRetryBackoffBeforeLease(t *testing.T) {
	store := newTestSQLiteOutboxStore(t)
	id, err := store.Append(&OutboxRecord{
		EventType:   "interaction.completed",
		AggregateID: "agg-retry",
		Payload:     []byte(`{"retry":true}`),
		MaxRetries:  3,
	})
	if err != nil {
		t.Fatal(err)
	}
	leased, err := store.LeasePending(1, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(leased) != 1 {
		t.Fatalf("expected initial lease, got %#v", leased)
	}
	beforeFailure := time.Now()
	if err := store.MarkFailed(id, "temporary publish failure"); err != nil {
		t.Fatal(err)
	}
	rec, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != OutboxStatusFailed || rec.RetryCount != 1 {
		t.Fatalf("unexpected failed record: %#v", rec)
	}
	if rec.NextRetryAt.Before(beforeFailure.Add(DefaultRetryBackoff - 100*time.Millisecond)) {
		t.Fatalf("next retry was not backed off: %s", rec.NextRetryAt)
	}
	leasedAgain, err := store.LeasePending(1, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(leasedAgain) != 0 {
		t.Fatalf("failed record was leased before backoff: %#v", leasedAgain)
	}
	if err := store.db.Model(&OutboxRecordModel{}).Where("id = ?", id).Update("next_retry_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	retryLease, err := store.LeasePending(1, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(retryLease) != 1 || retryLease[0].ID != id || retryLease[0].RetryCount != 1 {
		t.Fatalf("failed record was not leased after backoff: %#v", retryLease)
	}
}

func TestSQLiteDeadLetterStorePersistsPendingRecords(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dead-letter.db")
	db := openTestSQLiteDB(t, dbPath)
	store := NewSQLiteDeadLetterStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatal(err)
	}
	id, err := store.Append(&DeadLetterRecord{
		OutboxID:    "outbox-1",
		EventType:   "interaction.completed",
		AggregateID: "agg-1",
		Payload:     []byte(`{"ok":true}`),
		LastError:   "publish failed",
		RetryCount:  3,
	})
	if err != nil {
		t.Fatal(err)
	}
	closeTestSQLiteDB(t, db)

	reopenedDB := openTestSQLiteDB(t, dbPath)
	defer closeTestSQLiteDB(t, reopenedDB)
	reopened := NewSQLiteDeadLetterStore(reopenedDB)
	pending, err := reopened.ListPending()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID != id || string(pending[0].Payload) != `{"ok":true}` {
		t.Fatalf("unexpected pending records: %#v", pending)
	}
	if err := reopened.MarkReplaying(id); err != nil {
		t.Fatal(err)
	}
	rec, err := reopened.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != DeadLetterStatusReplaying {
		t.Fatalf("record was not marked replaying: %#v", rec)
	}
	if err := reopened.MarkReplayed(id); err != nil {
		t.Fatal(err)
	}
	rec, err = reopened.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != DeadLetterStatusReplayed || rec.ReplayedAt.IsZero() {
		t.Fatalf("record was not marked replayed: %#v", rec)
	}
}

func TestOutboxDispatcherMovesExhaustedFailureToDeadLetter(t *testing.T) {
	store := NewInMemoryOutboxStore()
	deadStore := NewInMemoryDeadLetterStore()
	publisher := &testOutboxPublisher{err: errors.New("publish failed")}
	id, err := store.Append(&OutboxRecord{
		EventType:   "interaction.completed",
		AggregateID: "agg-1",
		Payload:     []byte(`{"ok":true}`),
		MaxRetries:  1,
	})
	if err != nil {
		t.Fatal(err)
	}

	dispatcher := NewOutboxDispatcher(store, deadStore, publisher)
	dispatcher.concurrency = 1
	dispatcher.flush(nil)

	rec, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != OutboxStatusDead || !rec.LeasedUntil.IsZero() {
		t.Fatalf("record was not marked dead: %#v", rec)
	}
	if publisher.Count() != 1 {
		t.Fatalf("expected one publish attempt, got %d", publisher.Count())
	}
	deadLetters, err := deadStore.ListPending()
	if err != nil {
		t.Fatal(err)
	}
	if len(deadLetters) != 1 || deadLetters[0].OutboxID != id || deadLetters[0].LastError != "publish failed" {
		t.Fatalf("unexpected dead letters: %#v", deadLetters)
	}
}

func TestOutboxDispatcherStartProcessesPendingAndStopsWithContext(t *testing.T) {
	store := NewInMemoryOutboxStore()
	deadStore := NewInMemoryDeadLetterStore()
	publisher := &testOutboxPublisher{}
	id, err := store.Append(&OutboxRecord{
		EventType:   "interaction.completed",
		AggregateID: "agg-1",
		Payload:     []byte(`{"ok":true}`),
		MaxRetries:  3,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	dispatcher := NewOutboxDispatcherWithConfig(store, deadStore, publisher, OutboxWorkerConfig{
		DispatcherInterval: time.Hour,
		Concurrency:        1,
	})
	dispatcher.Start(ctx)
	waitFor(t, time.Second, func() bool {
		return publisher.Count() == 1
	})
	cancel()
	dispatcher.Stop()

	rec, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != OutboxStatusPublished {
		t.Fatalf("record was not published: %#v", rec)
	}
}

func TestDeadLetterRecoveryWorkerStartRecoversPendingAndStops(t *testing.T) {
	deadStore := NewInMemoryDeadLetterStore()
	publisher := &testOutboxPublisher{}
	id, err := deadStore.Append(&DeadLetterRecord{
		OutboxID:    "outbox-1",
		EventType:   "interaction.completed",
		AggregateID: "agg-1",
		Payload:     []byte(`{"ok":true}`),
		Status:      DeadLetterStatusPending,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	worker := NewDeadLetterRecoveryWorkerWithConfig(deadStore, publisher, OutboxWorkerConfig{
		RecoveryInterval: time.Hour,
	})
	worker.Start(ctx)
	waitFor(t, time.Second, func() bool {
		return publisher.Count() == 1
	})
	cancel()
	worker.Stop()

	rec, err := deadStore.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != DeadLetterStatusReplayed {
		t.Fatalf("dead letter was not replayed: %#v", rec)
	}
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func newTestSQLiteOutboxStore(t *testing.T) *SQLiteOutboxStore {
	t.Helper()
	db := newTestSQLiteDB(t, "outbox.db")
	store := NewSQLiteOutboxStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatal(err)
	}
	return store
}

func newTestSQLiteDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := openTestSQLiteDB(t, filepath.Join(t.TempDir(), name))
	t.Cleanup(func() {
		closeTestSQLiteDB(t, db)
	})
	return db
}

func openTestSQLiteDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func closeTestSQLiteDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
}
