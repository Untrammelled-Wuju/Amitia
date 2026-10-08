package migration

import "testing"

func TestBaselineDefersDuplicateTurnIndexToVersionedMigrationWithoutDeletingHistory(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DROP INDEX idx_assistant_turns_conv_request_unique").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO assistant_turns (id, conversation_id, request_id, created_at, sequence) VALUES ('first', 'chat', 'request', '2026-01-01', 1), ('second', 'chat', 'request', '2026-01-02', 2)").Error; err != nil {
		t.Fatal(err)
	}
	if err := ApplyBaseline(db); err != nil {
		t.Fatal("baseline prevented the registered duplicate repair", err)
	}
	var unchanged int64
	if err := db.Table("assistant_turns").Where("request_id = ?", "request").Count(&unchanged).Error; err != nil || unchanged != 2 {
		t.Fatal("baseline mutated historical rows", unchanged, err)
	}
	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]Migration{WebChatRequestIdempotencyMigration()}); err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		ID        string
		RequestID string
	}
	if err := db.Table("assistant_turns").Order("sequence").Find(&rows).Error; err != nil || len(rows) != 2 || rows[0].RequestID != "request" || rows[1].RequestID != "request:legacy:second" {
		t.Fatal("registered repair failed to preserve both turns", rows, err)
	}
	if err := db.Exec("INSERT INTO assistant_turns (id, conversation_id, request_id) VALUES ('repeated', 'chat', 'request')").Error; err == nil {
		t.Fatal("duplicate request accepted after versioned migration")
	}
	if err := ApplyBaseline(db); err != nil {
		t.Fatal("repeated baseline failed", err)
	}
}
