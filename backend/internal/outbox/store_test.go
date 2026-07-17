package outbox

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newOutboxTestDB(t *testing.T) *SQLiteOutboxStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&OutboxRecordModel{}, &DeadLetterRecordModel{}); err != nil {
		t.Fatalf("migrate outbox: %v", err)
	}
	cfg := OutboxStoreConfig{
		LeaseTTL:     5 * time.Second,
		RenewWindow:  1 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 100 * time.Millisecond,
	}
	return NewSQLiteOutboxStore(db, cfg)
}

func seedRecord(t *testing.T, store *SQLiteOutboxStore, id, eventType, payload, status string) {
	t.Helper()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	rec := OutboxRecordModel{
		ID:             id,
		AggregateID:    "agg-1",
		EventType:      eventType,
		Payload:        payload,
		PayloadVersion: "v1",
		Status:         status,
		AvailableAt:    now,
		MaxRetries:     3,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := store.db.Create(&rec).Error; err != nil {
		t.Fatalf("seed record: %v", err)
	}
}

func TestClaimNextSingle(t *testing.T) {
	store := newOutboxTestDB(t)
	seedRecord(t, store, "evt-1", "test.event", `{"v":1}`, string(OutboxStatusPending))

	records, err := store.ClaimNext(10, "worker-1")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 claimed, got %d", len(records))
	}
	r := records[0]
	if r.ID != "evt-1" {
		t.Errorf("expected evt-1, got %s", r.ID)
	}
	if r.Status != OutboxStatusLeased {
		t.Errorf("expected leased, got %s", r.Status)
	}
	if r.LeaseOwner != "worker-1" {
		t.Errorf("expected worker-1, got %s", r.LeaseOwner)
	}
	if r.LeaseToken == "" {
		t.Error("expected non-empty lease token")
	}
}

func TestClaimNextDoubleWorkerConflict(t *testing.T) {
	store := newOutboxTestDB(t)
	seedRecord(t, store, "evt-1", "test.event", `{"v":1}`, string(OutboxStatusPending))

	r1, _ := store.ClaimNext(10, "worker-1")
	if len(r1) != 1 {
		t.Fatal("worker-1 should claim evt-1")
	}

	r2, err := store.ClaimNext(10, "worker-2")
	if err != nil {
		t.Fatalf("worker-2 ClaimNext: %v", err)
	}
	if len(r2) != 0 {
		t.Errorf("worker-2 should claim 0, got %d", len(r2))
	}
}

func TestMarkPublishedWithValidToken(t *testing.T) {
	store := newOutboxTestDB(t)
	seedRecord(t, store, "evt-1", "test.event", `{"v":1}`, string(OutboxStatusPending))

	records, _ := store.ClaimNext(10, "worker-1")
	r := records[0]

	err := store.MarkPublished(r.ID, r.LeaseToken)
	if err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}

	var m OutboxRecordModel
	store.db.Where("id = ?", r.ID).Take(&m)
	if string(OutboxStatusPublished) != m.Status {
		t.Errorf("expected published, got %s", m.Status)
	}
}

func TestMarkPublishedWithWrongToken(t *testing.T) {
	store := newOutboxTestDB(t)
	seedRecord(t, store, "evt-1", "test.event", `{"v":1}`, string(OutboxStatusPending))

	records, _ := store.ClaimNext(10, "worker-1")
	r := records[0]

	err := store.MarkPublished(r.ID, "wrong-token")
	if err != ErrLeaseConflict {
		t.Errorf("expected ErrLeaseConflict, got %v", err)
	}
}

func TestMarkFailedThenRetry(t *testing.T) {
	store := newOutboxTestDB(t)
	seedRecord(t, store, "evt-1", "test.event", `{"v":1}`, string(OutboxStatusPending))

	records, _ := store.ClaimNext(10, "worker-1")
	r := records[0]

	err := store.MarkFailed(r.ID, r.LeaseToken, "timeout")
	if err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	var m OutboxRecordModel
	store.db.Where("id = ?", r.ID).Take(&m)
	if m.Status != string(OutboxStatusRetry) {
		t.Errorf("expected retry, got %s", m.Status)
	}
	if m.RetryCount != 1 {
		t.Errorf("expected retryCount 1, got %d", m.RetryCount)
	}
	if m.LastError != "timeout" {
		t.Errorf("expected timeout, got %s", m.LastError)
	}
}

func TestMarkFailedExhaustsRetriesToDead(t *testing.T) {
	store := newOutboxTestDB(t)
	cfg := OutboxStoreConfig{
		LeaseTTL:     5 * time.Second,
		RenewWindow:  1 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 100 * time.Millisecond,
	}
	store = NewSQLiteOutboxStore(store.db, cfg)
	seedRecord(t, store, "evt-dead", "test.event", `{"v":1}`, string(OutboxStatusPending))

	for i := 0; i < 3; i++ {
		records, err := store.ClaimNext(1, "worker-1")
		if err != nil {
			t.Fatalf("ClaimNext iter %d: %v", i, err)
		}
		if len(records) == 0 {
			var m OutboxRecordModel
			store.db.Where("id = ?", "evt-dead").Take(&m)
			t.Fatalf("ClaimNext iter %d: no records (status=%s retry=%d)", i, m.Status, m.RetryCount)
		}
		err = store.MarkFailed(records[0].ID, records[0].LeaseToken, "fail")
		if err != nil {
			t.Fatalf("MarkFailed iter %d: %v", i, err)
		}
		if i < 2 {
			store.db.Model(&OutboxRecordModel{}).Where("id = ?", records[0].ID).Updates(map[string]interface{}{
				"status":       string(OutboxStatusPending),
				"lease_owner":  "",
				"lease_token":  "",
				"leased_until": "",
			})
		}
	}

	var m OutboxRecordModel
	store.db.Where("id = ?", "evt-dead").Take(&m)
	if m.Status != string(OutboxStatusDead) {
		t.Errorf("expected dead, got %s", m.Status)
	}
}

func TestRenewLease(t *testing.T) {
	store := newOutboxTestDB(t)
	seedRecord(t, store, "evt-1", "test.event", `{"v":1}`, string(OutboxStatusPending))

	records, _ := store.ClaimNext(10, "worker-1")
	r := records[0]

	err := store.RenewLease(r.ID, r.LeaseToken)
	if err != nil {
		t.Fatalf("RenewLease: %v", err)
	}
}

func TestRenewLeaseWithExpiredLease(t *testing.T) {
	store := newOutboxTestDB(t)
	cfg := OutboxStoreConfig{
		LeaseTTL:     10 * time.Millisecond,
		RenewWindow:  5 * time.Millisecond,
		MaxRetries:   3,
		RetryBackoff: 100 * time.Millisecond,
	}
	db := store.db
	store = NewSQLiteOutboxStore(db, cfg)
	seedRecord(t, store, "evt-1", "test.event", `{"v":1}`, string(OutboxStatusPending))

	records, _ := store.ClaimNext(10, "worker-1")
	r := records[0]

	time.Sleep(15 * time.Millisecond)

	err := store.RenewLease(r.ID, r.LeaseToken)
	if err != ErrLeaseExpired {
		t.Errorf("expected ErrLeaseExpired, got %v", err)
	}
}

func TestReleaseExpiredLeases(t *testing.T) {
	store := newOutboxTestDB(t)
	cfg := OutboxStoreConfig{
		LeaseTTL:     10 * time.Millisecond,
		RenewWindow:  5 * time.Millisecond,
		MaxRetries:   3,
		RetryBackoff: 100 * time.Millisecond,
	}
	db := store.db
	store = NewSQLiteOutboxStore(db, cfg)
	seedRecord(t, store, "evt-1", "test.event", `{"v":1}`, string(OutboxStatusPending))

	records, _ := store.ClaimNext(10, "worker-1")
	_ = records[0]

	time.Sleep(15 * time.Millisecond)

	count, err := store.ReleaseExpiredLeases()
	if err != nil {
		t.Fatalf("ReleaseExpiredLeases: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 released, got %d", count)
	}

	var m OutboxRecordModel
	store.db.Where("id = ?", "evt-1").Take(&m)
	if m.Status != string(OutboxStatusPending) {
		t.Errorf("expected pending after release, got %s", m.Status)
	}
}

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		from, to OutboxStatus
		want     bool
	}{
		{OutboxStatusPending, OutboxStatusLeased, true},
		{OutboxStatusLeased, OutboxStatusPublished, true},
		{OutboxStatusLeased, OutboxStatusRetry, true},
		{OutboxStatusLeased, OutboxStatusFailed, true},
		{OutboxStatusRetry, OutboxStatusLeased, true},
		{OutboxStatusRetry, OutboxStatusDead, true},
		{OutboxStatusFailed, OutboxStatusLeased, true},
		{OutboxStatusFailed, OutboxStatusDead, true},
		{OutboxStatusPublished, OutboxStatusLeased, false},
		{OutboxStatusDead, OutboxStatusLeased, false},
		{OutboxStatusPending, OutboxStatusPublished, false},
		{OutboxStatusPending, OutboxStatusDead, false},
	}
	for _, tc := range tests {
		got := ValidateTransition(tc.from, tc.to)
		if got != tc.want {
			t.Errorf("%s->%s: got %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}
