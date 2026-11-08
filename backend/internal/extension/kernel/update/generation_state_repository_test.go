package update

import (
	"bytes"
	"context"
	"database/sql"
	"log"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSetActiveGenerationDoesNotReportCommittedRollbackAsError(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "generation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE kernel_generation_states (
			generation_id TEXT PRIMARY KEY,
			extension_id TEXT NOT NULL,
			version TEXT NOT NULL DEFAULT '',
			generation_num INTEGER NOT NULL DEFAULT 0,
			state TEXT NOT NULL DEFAULT 'preparing',
			definition_hash TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			activated_at TEXT,
			stopped_at TEXT,
			invocations INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO kernel_generation_states (generation_id, extension_id, state, created_at)
		VALUES ('old', 'ext', 'active', '2026-01-01T00:00:00Z'),
		       ('next', 'ext', 'retired', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	if err := NewGenerationStateRepository(db).SetActiveGeneration(context.Background(), "ext", "next"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), sql.ErrTxDone.Error()) {
		t.Fatalf("expected committed transaction rollback to be ignored, logs=%q", logs.String())
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM kernel_generation_states WHERE generation_id = 'next'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "active" {
		t.Fatalf("state=%q, want active", state)
	}
}
