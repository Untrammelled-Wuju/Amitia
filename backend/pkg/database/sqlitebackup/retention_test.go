package sqlitebackup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneBackupsKeepsRecentAndDailySnapshots(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	var current string
	for index := 0; index < 10; index++ {
		base := filepath.Join(dir, fmt.Sprintf("backup-%02d.db", index))
		if err := os.WriteFile(base, []byte("db"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("backup-%02d.json", index)), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(base+"-wal", []byte("wal"), 0o600); err != nil {
			t.Fatal(err)
		}
		modTime := now.Add(-time.Duration(index) * 24 * time.Hour)
		if err := os.Chtimes(base, modTime, modTime); err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			current = base
		}
	}
	deleted, err := PruneBackups(dir, current, RetentionPolicy{KeepRecent: 2, KeepDailyDays: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 7 {
		t.Fatalf("deleted %d backups, want 7", len(deleted))
	}
	for index := 0; index < 3; index++ {
		base := filepath.Join(dir, fmt.Sprintf("backup-%02d.db", index))
		if _, err := os.Stat(base); err != nil {
			t.Fatalf("retained backup %d missing: %v", index, err)
		}
	}
	for index := 3; index < 10; index++ {
		base := filepath.Join(dir, fmt.Sprintf("backup-%02d.db", index))
		if _, err := os.Stat(base); !os.IsNotExist(err) {
			t.Fatalf("expired backup %d still exists: %v", index, err)
		}
		if _, err := os.Stat(base + "-wal"); !os.IsNotExist(err) {
			t.Fatalf("expired backup %d wal still exists: %v", index, err)
		}
	}
}
