package sqlitebackup_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/database/mysql"
	"github.com/u-ai/backend/pkg/database/sqlitebackup"
)

func TestRealDatabaseSnapshotUnderConcurrentLoad(t *testing.T) {
	sourcePath := os.Getenv("AMITIA_REAL_DB_SNAPSHOT")
	if sourcePath == "" {
		t.Skip("AMITIA_REAL_DB_SNAPSHOT is not set")
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	targetPath := filepath.Join(dataDir, "app.db")
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		source.Close()
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		source.Close()
		target.Close()
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		target.Close()
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	db := mysql.NewSQLite(dataDir)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := sqlitebackup.VerifySnapshot(t.Context(), targetPath); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE scenario_events (id INTEGER PRIMARY KEY AUTOINCREMENT, payload TEXT NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var workers sync.WaitGroup
	errs := make(chan error, 12)
	for worker := 0; worker < 4; worker++ {
		worker := worker
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := 0; ; index++ {
				select {
				case <-stop:
					return
				default:
				}
				if err := db.Exec("INSERT INTO scenario_events(payload) VALUES (?)", fmt.Sprintf("w%d-%d", worker, index)).Error; err != nil {
					errs <- err
					return
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				var count int64
				if err := db.Table("scenario_events").Count(&count).Error; err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	backupDir := filepath.Join(dataDir, "scenario_backups")
	for index := 0; index < 5; index++ {
		path := filepath.Join(backupDir, fmt.Sprintf("snapshot-%d.db", index))
		if err := sqlitebackup.CreateConsistentSnapshot(context.Background(), db, path); err != nil {
			close(stop)
			workers.Wait()
			t.Fatal(err)
		}
	}
	close(stop)
	workers.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for index := 0; index < 5; index++ {
		path := filepath.Join(backupDir, fmt.Sprintf("snapshot-%d.db", index))
		if err := sqlitebackup.VerifySnapshot(t.Context(), path); err != nil {
			t.Fatal(err)
		}
	}
}
