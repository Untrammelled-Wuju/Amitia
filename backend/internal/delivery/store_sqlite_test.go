package delivery

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestStore(t *testing.T) *SQLiteDeliveryStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	store := NewSQLiteDeliveryStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return store
}

func TestMarkSentWritesSentAtOnly(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	leaseUntil := now.Add(300 * time.Second).Format("2006-01-02 15:04:05")

	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, lease_until) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-test-sent-1", "inter-1", "ws", "peer-1", "text/plain", "hello", string(DeliveryStatusLeased), now.Format("2006-01-02 15:04:05"), 5, leaseUntil)

	if err := store.MarkSent("di-test-sent-1"); err != nil {
		t.Fatalf("mark sent: %v", err)
	}

	got, err := store.GetIntent("di-test-sent-1")
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if got == nil {
		t.Fatal("intent not found")
	}

	if got.Status != DeliveryStatusSent {
		t.Errorf("expected status sent, got %s", got.Status)
	}
	if got.SentAt == nil {
		t.Error("expected SentAt to be set after MarkSent")
	}
	if got.DeliveredAt != nil {
		t.Error("expected DeliveredAt to be nil after MarkSent")
	}
}

func TestMarkDeliveredWritesDeliveredAtOnly(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	leaseUntil := now.Add(300 * time.Second).Format("2006-01-02 15:04:05")
	sentTime := now.Add(-10 * time.Second).Format("2006-01-02 15:04:05")

	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, sent_at, lease_until) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-test-delivered-1", "inter-2", "ws", "peer-2", "text/plain", "hello", string(DeliveryStatusSent), now.Format("2006-01-02 15:04:05"), 5, sentTime, leaseUntil)

	if err := store.MarkDelivered("di-test-delivered-1"); err != nil {
		t.Fatalf("mark delivered: %v", err)
	}

	got, err := store.GetIntent("di-test-delivered-1")
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if got == nil {
		t.Fatal("intent not found")
	}

	if got.Status != DeliveryStatusDelivered {
		t.Errorf("expected status delivered, got %s", got.Status)
	}
	if got.DeliveredAt == nil {
		t.Error("expected DeliveredAt to be set after MarkDelivered")
	}
}

func TestMarkSentDoesNotOverwriteDeliveredAt(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	sentTime := now.Add(-20 * time.Second).Format("2006-01-02 15:04:05")
	deliveredTime := now.Add(-10 * time.Second).Format("2006-01-02 15:04:05")

	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, sent_at, delivered_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-test-overwrite-1", "inter-3", "ws", "peer-3", "text/plain", "hello", string(DeliveryStatusDelivered), now.Format("2006-01-02 15:04:05"), 5, sentTime, deliveredTime)

	got, err := store.GetIntent("di-test-overwrite-1")
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if got == nil {
		t.Fatal("intent not found")
	}

	if got.SentAt == nil {
		t.Error("expected SentAt to be preserved")
	}
	if got.DeliveredAt == nil {
		t.Error("expected DeliveredAt to be preserved")
	}
	if got.SentAt != nil && got.DeliveredAt != nil && !got.SentAt.Before(*got.DeliveredAt) {
		t.Error("expected SentAt before DeliveredAt")
	}
}

func TestUpdateStatusSentSetsSentAt(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()

	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-test-update-1", "inter-4", "ws", "peer-4", "text/plain", "hello", string(DeliveryStatusPending), now.Format("2006-01-02 15:04:05"), 5)

	if err := store.UpdateStatus("di-test-update-1", DeliveryStatusSent, ""); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, err := store.GetIntent("di-test-update-1")
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if got == nil {
		t.Fatal("intent not found")
	}

	if got.SentAt == nil {
		t.Error("expected SentAt to be set via UpdateStatus sent")
	}
	if got.DeliveredAt != nil {
		t.Error("expected DeliveredAt to be nil after UpdateStatus sent")
	}
}

func TestUpdateStatusDeliveredSetsDeliveredAt(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	sentTime := now.Add(-10 * time.Second).Format("2006-01-02 15:04:05")

	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, sent_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-test-update-2", "inter-5", "ws", "peer-5", "text/plain", "hello", string(DeliveryStatusSent), now.Format("2006-01-02 15:04:05"), 5, sentTime)

	if err := store.UpdateStatus("di-test-update-2", DeliveryStatusDelivered, ""); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, err := store.GetIntent("di-test-update-2")
	if err != nil {
		t.Fatalf("get intent: %v", err)
	}
	if got == nil {
		t.Fatal("intent not found")
	}

	if got.DeliveredAt == nil {
		t.Error("expected DeliveredAt to be set via UpdateStatus delivered")
	}
}

func TestMarkSentToMarkDeliveredFullLifecycle(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	leaseUntil := now.Add(300 * time.Second).Format("2006-01-02 15:04:05")

	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, lease_until) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-test-lifecycle-1", "inter-6", "qq", "peer-6", "text/plain", "hello", string(DeliveryStatusLeased), now.Format("2006-01-02 15:04:05"), 5, leaseUntil)

	if err := store.MarkSent("di-test-lifecycle-1"); err != nil {
		t.Fatalf("mark sent: %v", err)
	}

	gotSent, err := store.GetIntent("di-test-lifecycle-1")
	if err != nil {
		t.Fatalf("get intent after sent: %v", err)
	}
	if gotSent == nil {
		t.Fatal("intent not found after sent")
	}
	if gotSent.Status != DeliveryStatusSent {
		t.Errorf("expected sent, got %s", gotSent.Status)
	}
	if gotSent.SentAt == nil {
		t.Error("expected SentAt after MarkSent")
	}
	if gotSent.DeliveredAt != nil {
		t.Error("expected DeliveredAt nil after MarkSent")
	}

	leaseUntil2 := now.Add(600 * time.Second).Format("2006-01-02 15:04:05")
	store.db.Exec("UPDATE delivery_intents SET lease_until = ? WHERE id = ?", leaseUntil2, "di-test-lifecycle-1")

	if err := store.MarkDelivered("di-test-lifecycle-1"); err != nil {
		t.Fatalf("mark delivered: %v", err)
	}

	gotDelivered, err := store.GetIntent("di-test-lifecycle-1")
	if err != nil {
		t.Fatalf("get intent after delivered: %v", err)
	}
	if gotDelivered == nil {
		t.Fatal("intent not found after delivered")
	}
	if gotDelivered.Status != DeliveryStatusDelivered {
		t.Errorf("expected delivered, got %s", gotDelivered.Status)
	}
	if gotDelivered.SentAt == nil {
		t.Error("expected SentAt preserved after MarkDelivered")
	}
	if gotDelivered.DeliveredAt == nil {
		t.Error("expected DeliveredAt after MarkDelivered")
	}
	if gotDelivered.SentAt != nil && gotDelivered.DeliveredAt != nil && !gotDelivered.SentAt.Before(*gotDelivered.DeliveredAt) {
		if gotDelivered.SentAt.After(*gotDelivered.DeliveredAt) {
			t.Error("expected SentAt not after DeliveredAt in lifecycle")
		}
	}
}
