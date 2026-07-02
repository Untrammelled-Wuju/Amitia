package pipelinecheckpoint

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openManagerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "checkpoint.db")), &gorm.Config{})
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
	stmts := []string{
		`CREATE TABLE messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, sequence INTEGER NOT NULL DEFAULT 0, role TEXT NOT NULL, content TEXT NOT NULL, created_at TEXT DEFAULT '')`,
		`CREATE TABLE pipeline_checkpoints (conversation_id TEXT NOT NULL, pipeline_type TEXT NOT NULL, last_message_sequence INTEGER NOT NULL DEFAULT 0, checkpoint_version INTEGER NOT NULL DEFAULT 1, idempotency_key TEXT DEFAULT '', created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '', PRIMARY KEY (conversation_id, pipeline_type))`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestPendingRangeReturnsContextAndNewMessages(t *testing.T) {
	db := openManagerTestDB(t)
	rows := []struct {
		id       string
		sequence int
		role     string
		content  string
	}{
		{"m1", 1, "user", "一"},
		{"m2", 2, "assistant", "二"},
		{"m3", 3, "user", "三"},
		{"m4", 4, "assistant", "四"},
	}
	for _, row := range rows {
		if err := db.Exec("INSERT INTO messages (id, conversation_id, sequence, role, content) VALUES (?, 'conv-1', ?, ?, ?)", row.id, row.sequence, row.role, row.content).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("INSERT INTO pipeline_checkpoints (conversation_id, pipeline_type, last_message_sequence) VALUES ('conv-1', 'memory', 2)").Error; err != nil {
		t.Fatal(err)
	}
	manager := New(db)
	messages, maxSequence, err := manager.PendingRange("conv-1", "memory", 1)
	if err != nil {
		t.Fatal(err)
	}
	if maxSequence != 4 {
		t.Fatalf("maxSequence = %d, want 4", maxSequence)
	}
	if len(messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(messages))
	}
	if messages[0]["content"] != "二" || messages[2]["content"] != "四" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
}

func TestAdvanceKeepsMonotonicSequence(t *testing.T) {
	db := openManagerTestDB(t)
	manager := New(db)
	if err := manager.Advance("conv-1", "memory", 5, "k1"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Advance("conv-1", "memory", 3, "k2"); err != nil {
		t.Fatal(err)
	}
	record, err := manager.Load("conv-1", "memory")
	if err != nil {
		t.Fatal(err)
	}
	if record.LastMessageSequence != 5 {
		t.Fatalf("last_message_sequence = %d, want 5", record.LastMessageSequence)
	}
	if record.IdempotencyKey != "k2" {
		t.Fatalf("idempotency_key = %q, want k2", record.IdempotencyKey)
	}
}

func TestAcquirePendingRangeBlocksDuplicateWhileLeaseActive(t *testing.T) {
	db := openManagerTestDB(t)
	for _, row := range []struct {
		id       string
		sequence int
		content  string
	}{
		{"m1", 1, "一"},
		{"m2", 2, "二"},
	} {
		if err := db.Exec("INSERT INTO messages (id, conversation_id, sequence, role, content) VALUES (?, 'conv-1', ?, 'user', ?)", row.id, row.sequence, row.content).Error; err != nil {
			t.Fatal(err)
		}
	}

	manager := New(db)
	messages, maxSequence, acquired, err := manager.AcquirePendingRange("conv-1", "episodic", 1, "worker-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired || maxSequence != 2 || len(messages) != 2 {
		t.Fatalf("unexpected first acquire: acquired=%v max=%d messages=%d", acquired, maxSequence, len(messages))
	}

	messages, _, acquired, err = manager.AcquirePendingRange("conv-1", "episodic", 1, "worker-2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if acquired || len(messages) != 0 {
		t.Fatalf("expected active lease to block duplicate acquire, got acquired=%v messages=%d", acquired, len(messages))
	}

	record, err := manager.Load("conv-1", "episodic")
	if err != nil {
		t.Fatal(err)
	}
	if record.LastMessageSequence != 0 || record.ProcessingEndSeq != 2 || record.LeaseOwner != "worker-1" {
		t.Fatalf("unexpected lease record: %#v", record)
	}
}

func TestAcquirePendingRangeRecoversExpiredLeaseAndAdvanceRequiresOwner(t *testing.T) {
	db := openManagerTestDB(t)
	for _, row := range []struct {
		id       string
		sequence int
		content  string
	}{
		{"m1", 1, "一"},
		{"m2", 2, "二"},
		{"m3", 3, "三"},
	} {
		if err := db.Exec("INSERT INTO messages (id, conversation_id, sequence, role, content) VALUES (?, 'conv-1', ?, 'user', ?)", row.id, row.sequence, row.content).Error; err != nil {
			t.Fatal(err)
		}
	}

	manager := New(db)
	_, maxSequence, acquired, err := manager.AcquirePendingRange("conv-1", "episodic", 1, "worker-1", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired || maxSequence != 3 {
		t.Fatalf("unexpected first acquire: acquired=%v max=%d", acquired, maxSequence)
	}
	time.Sleep(20 * time.Millisecond)

	messages, maxSequence, acquired, err := manager.AcquirePendingRange("conv-1", "episodic", 1, "worker-2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired || maxSequence != 3 || len(messages) != 3 {
		t.Fatalf("expected expired lease recovery, got acquired=%v max=%d messages=%d", acquired, maxSequence, len(messages))
	}

	if err := manager.AdvanceLeased("conv-1", "episodic", 3, "old", "worker-1"); err == nil {
		t.Fatal("expected stale worker advance to fail")
	}
	if err := manager.AdvanceLeased("conv-1", "episodic", 3, "new", "worker-2"); err != nil {
		t.Fatal(err)
	}
	record, err := manager.Load("conv-1", "episodic")
	if err != nil {
		t.Fatal(err)
	}
	if record.LastMessageSequence != 3 || record.LeaseOwner != "" || record.ProcessingEndSeq != 0 {
		t.Fatalf("expected completed checkpoint lease to clear, got %#v", record)
	}
}
