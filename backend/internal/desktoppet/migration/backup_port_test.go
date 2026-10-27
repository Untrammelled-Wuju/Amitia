package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/database/mysql"
	"github.com/u-ai/backend/pkg/database/sqlitebackup"
)

func TestDomainMigrationBackupUsesConsistentSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	db := mysql.NewSQLite(dataDir)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.Exec("CREATE TABLE live_rows (id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	writerErr := make(chan error, 1)
	go func() {
		for index := 0; ; index++ {
			select {
			case <-stop:
				writerErr <- nil
				return
			default:
			}
			if err := db.Exec("INSERT INTO live_rows(value) VALUES (?)", fmt.Sprintf("row-%d", index)).Error; err != nil {
				writerErr <- err
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	port := NewDomainMigrationBackupPort(db, filepath.Join(dataDir, "migration_backups"))
	backupID, err := port.CreateBackup(context.Background())
	close(stop)
	writeErr := <-writerErr
	if err != nil {
		t.Fatal(err)
	}
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	exists, err := port.BackupExists(context.Background(), backupID)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("completed backup was not verifiable")
	}
	snapshotPath := filepath.Join(dataDir, "migration_backups", backupID+".db")
	if err := sqlitebackup.VerifySnapshot(t.Context(), snapshotPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(snapshotPath + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("consistent snapshot must not carry a WAL sidecar: %v", err)
	}
	if _, err := os.Stat(snapshotPath + "-shm"); !os.IsNotExist(err) {
		t.Fatalf("consistent snapshot must not carry an SHM sidecar: %v", err)
	}
}
