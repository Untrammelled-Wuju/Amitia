package mysql

import (
	"testing"
)

func TestNewSQLiteConfiguresConcurrentWritePragmas(t *testing.T) {
	db := NewSQLite(t.TempDir())
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	var journalMode string
	if err := sqlDB.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := sqlDB.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy timeout = %d, want 5000", busyTimeout)
	}
}
