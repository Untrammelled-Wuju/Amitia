package sqlitebackup

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotFileURIWindowsDrivePath(t *testing.T) {
	uri := snapshotFileURIFromPath("D:/Program Files/Amitia Data/备份 #1%.db", "mode=ro&immutable=1")
	const prefix = "file:///D:/Program%20Files/Amitia%20Data/%E5%A4%87%E4%BB%BD%20%231%25.db?"
	if !strings.HasPrefix(uri, prefix) {
		t.Fatalf("snapshot URI = %q, want prefix %q", uri, prefix)
	}
	if !strings.HasSuffix(uri, "mode=ro&immutable=1") {
		t.Fatalf("snapshot URI = %q, missing query", uri)
	}
	if strings.HasPrefix(uri, "file://D:") {
		t.Fatalf("snapshot URI = %q, drive must not be a URI authority", uri)
	}
}

func TestVerifySnapshotWithAbsolutePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE snapshot_data (id INTEGER PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO snapshot_data(value) VALUES (?)", "ok"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := VerifySnapshot(context.Background(), path); err != nil {
		t.Fatal(err)
	}
}
