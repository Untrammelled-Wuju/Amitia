package interaction

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
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

func newTestSQLiteOutboxStore(t *testing.T) *SQLiteOutboxStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "outbox.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	store := NewSQLiteOutboxStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatal(err)
	}
	return store
}
