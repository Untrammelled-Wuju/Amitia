package migration

import "testing"

func TestBaselineAcceptsLegacyMessageFeedbackBeforeScopeMigration(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := db.Exec(`CREATE TABLE message_feedback (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT DEFAULT '',
		feedback_type TEXT NOT NULL DEFAULT '',
		reason TEXT DEFAULT '',
		rating INTEGER DEFAULT 0,
		comment TEXT DEFAULT '',
		created_at TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("baseline rejected legacy message_feedback table: %v", err)
	}
}
