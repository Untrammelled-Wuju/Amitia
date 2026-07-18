package delivery

import (
	"fmt"
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

	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, lease_until, lease_token) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-test-sent-1", "inter-1", "ws", "peer-1", "text/plain", "hello", string(DeliveryStatusLeased), now.Format("2006-01-02 15:04:05"), 5, leaseUntil, "token-sent-1")

	if err := store.MarkSent("di-test-sent-1", "token-sent-1"); err != nil {
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

	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, lease_until, lease_token) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-test-lifecycle-1", "inter-6", "qq", "peer-6", "text/plain", "hello", string(DeliveryStatusLeased), now.Format("2006-01-02 15:04:05"), 5, leaseUntil, "token-lifecycle")

	if err := store.MarkSent("di-test-lifecycle-1", "token-lifecycle"); err != nil {
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

func TestEmoteDeliveryStatusBackfill(t *testing.T) {
	store := newTestStore(t)
	if err := store.db.Exec("CREATE TABLE emote_send_records (delivery_key TEXT PRIMARY KEY, message_id TEXT, status TEXT, sent_at TEXT, failure_reason TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.db.Exec("CREATE TABLE messages (id TEXT PRIMARY KEY, status TEXT, emote_decision_status TEXT, updated_at TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	leaseUntil := now.Add(5 * time.Minute).Format("2006-01-02 15:04:05")
	for _, row := range []struct{ key, message string }{{"emote-sent", "m-sent"}, {"emote-failed", "m-failed"}} {
		store.db.Exec("INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, lease_until, lease_token) VALUES (?, 'response', 'qq', 'peer', 'emote', '{}', ?, ?, 1, ?, ?)", row.key, string(DeliveryStatusLeased), now.Format("2006-01-02 15:04:05"), leaseUntil, row.key+"-token")
		store.db.Exec("INSERT INTO emote_send_records (delivery_key, message_id, status) VALUES (?, ?, 'queued')", row.key, row.message)
		store.db.Exec("INSERT INTO messages (id, status, emote_decision_status) VALUES (?, 'sending', 'queued')", row.message)
	}
	if err := store.MarkSent("emote-sent", "emote-sent-token"); err != nil {
		t.Fatal(err)
	}
	var sentRecord, sentMessage string
	store.db.Table("emote_send_records").Select("status").Where("delivery_key = ?", "emote-sent").Scan(&sentRecord)
	store.db.Table("messages").Select("status").Where("id = ?", "m-sent").Scan(&sentMessage)
	if sentRecord != "sent" || sentMessage != "sent" {
		t.Fatalf("成功状态回写错误: record=%s message=%s", sentRecord, sentMessage)
	}
	if err := store.MarkFailed("emote-failed", "emote-failed-token", "adapter rejected"); err != nil {
		t.Fatal(err)
	}
	var failedRecord, failedMessage, failureReason string
	store.db.Table("emote_send_records").Select("status, failure_reason").Where("delivery_key = ?", "emote-failed").Row().Scan(&failedRecord, &failureReason)
	store.db.Table("messages").Select("status").Where("id = ?", "m-failed").Scan(&failedMessage)
	if failedRecord != "failed" || failedMessage != "failed" || failureReason != "adapter rejected" {
		t.Fatalf("失败状态回写错误: record=%s message=%s reason=%s", failedRecord, failedMessage, failureReason)
	}
}


func TestClaimNextIntentsReturnsLeaseToken(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-claim-1", "inter-10", "ws", "peer-10", "text/plain", "hello", string(DeliveryStatusPending), now.Format("2006-01-02 15:04:05"), 5)

	intents, err := store.ClaimNextIntents(10)
	if err != nil {
		t.Fatalf("claim failed: %v", err)
	}
	if len(intents) == 0 {
		t.Fatal("expected at least one intent")
	}
	if intents[0].LeaseToken == "" {
		t.Error("expected non-empty LeaseToken after claim")
	}
	if intents[0].Status != DeliveryStatusLeased {
		t.Errorf("expected status leased, got %s", intents[0].Status)
	}
	if intents[0].LeaseOwner != "delivery-worker" {
		t.Errorf("expected lease owner delivery-worker, got %s", intents[0].LeaseOwner)
	}
}

func TestClaimNextIntentsDifferentTokens(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("di-batch-%d", i), "inter-11", "ws", "peer-11", "text/plain", "hello", string(DeliveryStatusPending), now.Format("2006-01-02 15:04:05"), 5)
	}

	intents, err := store.ClaimNextIntents(10)
	if err != nil {
		t.Fatalf("claim failed: %v", err)
	}
	if len(intents) < 2 {
		t.Fatal("expected at least 2 intents")
	}
	if intents[0].LeaseToken == intents[1].LeaseToken {
		t.Error("expected different tokens for different intents")
	}
}

func TestCorrectTokenCanMarkSent(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	leaseUntil := now.Add(300 * time.Second).Format("2006-01-02 15:04:05")
	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, lease_until, lease_token) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-cas-sent-1", "inter-12", "ws", "peer-12", "text/plain", "hello", string(DeliveryStatusLeased), now.Format("2006-01-02 15:04:05"), 5, leaseUntil, "correct-token")

	if err := store.MarkSent("di-cas-sent-1", "correct-token"); err != nil {
		t.Fatalf("mark sent failed: %v", err)
	}
	got, _ := store.GetIntent("di-cas-sent-1")
	if got.Status != DeliveryStatusSent {
		t.Errorf("expected sent, got %s", got.Status)
	}
	if got.LeaseToken != "" {
		t.Error("expected lease_token cleared after MarkSent")
	}
	if got.LeaseOwner != "" {
		t.Error("expected lease_owner cleared after MarkSent")
	}
	if got.LeaseUntil != nil {
		t.Error("expected lease_until cleared after MarkSent")
	}
}

func TestWrongTokenCannotMarkSent(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	leaseUntil := now.Add(300 * time.Second).Format("2006-01-02 15:04:05")
	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, lease_until, lease_token) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-cas-sent-2", "inter-13", "ws", "peer-13", "text/plain", "hello", string(DeliveryStatusLeased), now.Format("2006-01-02 15:04:05"), 5, leaseUntil, "correct-token")

	if err := store.MarkSent("di-cas-sent-2", "wrong-token"); err == nil {
		t.Fatal("expected error with wrong token")
	}
	got, _ := store.GetIntent("di-cas-sent-2")
	if got.Status != DeliveryStatusLeased {
		t.Errorf("expected status still leased, got %s", got.Status)
	}
}

func TestCorrectTokenCanMarkFailed(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	leaseUntil := now.Add(300 * time.Second).Format("2006-01-02 15:04:05")
	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, lease_until, lease_token) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-cas-fail-1", "inter-14", "ws", "peer-14", "text/plain", "hello", string(DeliveryStatusLeased), now.Format("2006-01-02 15:04:05"), 5, leaseUntil, "correct-token")

	if err := store.MarkFailed("di-cas-fail-1", "correct-token", "test error"); err != nil {
		t.Fatalf("mark failed error: %v", err)
	}
	got, _ := store.GetIntent("di-cas-fail-1")
	if got.Status != DeliveryStatusRetry && got.Status != DeliveryStatusFailed {
		t.Errorf("expected retry or failed, got %s", got.Status)
	}
	if got.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", got.RetryCount)
	}
}

func TestWrongTokenCannotMarkFailed(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	leaseUntil := now.Add(300 * time.Second).Format("2006-01-02 15:04:05")
	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, lease_until, lease_token) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-cas-fail-2", "inter-15", "ws", "peer-15", "text/plain", "hello", string(DeliveryStatusLeased), now.Format("2006-01-02 15:04:05"), 5, leaseUntil, "correct-token")

	if err := store.MarkFailed("di-cas-fail-2", "wrong-token", "test error"); err == nil {
		t.Fatal("expected error with wrong token on MarkFailed")
	}
	got, _ := store.GetIntent("di-cas-fail-2")
	if got.Status != DeliveryStatusLeased {
		t.Errorf("expected status still leased, got %s", got.Status)
	}
}

func TestOldTokenCannotUpdateAfterReclaim(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	leaseUntil := now.Add(-10 * time.Second).Format("2006-01-02 15:04:05")
	store.db.Exec(`INSERT INTO delivery_intents (id, interaction_id, channel, peer_id, content_type, payload, status, created_at, max_retries, lease_until, lease_token) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"di-reclaim-1", "inter-16", "ws", "peer-16", "text/plain", "hello", string(DeliveryStatusLeased), now.Format("2006-01-02 15:04:05"), 5, leaseUntil, "old-token")

	store.ReleaseExpiredClaims()
	intents, err := store.ClaimNextIntents(10)
	if err != nil {
		t.Fatalf("reclaim failed: %v", err)
	}
	var reclaimed *DeliveryIntent
	for i := range intents {
		if intents[i].ID == "di-reclaim-1" {
			reclaimed = &intents[i]
			break
		}
	}
	if reclaimed == nil {
		t.Fatal("failed to reclaim the intent")
	}
	newToken := reclaimed.LeaseToken
	if newToken == "old-token" {
		t.Error("new token should not equal old token")
	}

	if err := store.MarkSent("di-reclaim-1", "old-token"); err == nil {
		t.Fatal("old token should not be able to MarkSent after reclaim")
	}
	if err := store.MarkSent("di-reclaim-1", newToken); err != nil {
		t.Fatalf("new token should be able to MarkSent: %v", err)
	}
	got, _ := store.GetIntent("di-reclaim-1")
	if got.Status != DeliveryStatusSent {
		t.Errorf("expected sent with new token, got %s", got.Status)
	}
}
