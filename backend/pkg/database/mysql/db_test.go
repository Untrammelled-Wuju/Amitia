package mysql

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestNewSQLiteConfiguresConcurrentWritePragmas(t *testing.T) {
	db := NewSQLite(t.TempDir())
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer closeSQLiteTestDB(db, sqlDB)

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
	if busyTimeout != 10000 {
		t.Fatalf("busy timeout = %d, want 10000", busyTimeout)
	}
	if stats := sqlDB.Stats(); stats.MaxOpenConnections != 1 {
		t.Fatalf("max open connections = %d, want 1", stats.MaxOpenConnections)
	}
	pool, ok := db.ConnPool.(*sqliteRoutingPool)
	if !ok {
		t.Fatalf("connection pool type = %T, want *sqliteRoutingPool", db.ConnPool)
	}
	if stats := pool.reader.Stats(); stats.MaxOpenConnections != 10 {
		t.Fatalf("reader max open connections = %d, want 10", stats.MaxOpenConnections)
	}
}

func TestNewSQLiteSerializesConcurrentWrites(t *testing.T) {
	db := NewSQLite(t.TempDir())
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer closeSQLiteTestDB(db, sqlDB)

	if err := db.Exec("CREATE TABLE concurrent_writes (id INTEGER PRIMARY KEY, value TEXT NOT NULL)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	const workers = 24
	var wait sync.WaitGroup
	errs := make(chan error, workers)
	for index := 0; index < workers; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			errs <- db.Transaction(func(tx *gorm.DB) error {
				var count int64
				if err := tx.Model(&struct {
					ID    int    `gorm:"column:id;primaryKey"`
					Value string `gorm:"column:value"`
				}{}).Table("concurrent_writes").Count(&count).Error; err != nil {
					return err
				}
				return tx.Exec("INSERT INTO concurrent_writes(id, value) VALUES (?, ?)", index+1, fmt.Sprintf("worker-%d", index)).Error
			})
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent write failed: %v", err)
		}
	}

	var count int64
	if err := db.Table("concurrent_writes").Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != workers {
		t.Fatalf("row count = %d, want %d", count, workers)
	}
}

func TestNewSQLiteWriteTransactionsUseImmediateLock(t *testing.T) {
	dataDir := t.TempDir()
	db := NewSQLite(dataDir)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer closeSQLiteTestDB(db, sqlDB)

	if err := db.Exec("CREATE TABLE locked_rows (id INTEGER PRIMARY KEY, value INTEGER NOT NULL)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.Exec("INSERT INTO locked_rows(id, value) VALUES (1, 1)").Error; err != nil {
		t.Fatalf("insert row: %v", err)
	}

	dbPath := filepath.Join(dataDir, "app.db")
	external, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open external connection: %v", err)
	}
	defer external.Close()
	externalTx, err := external.Begin()
	if err != nil {
		t.Fatalf("begin external transaction: %v", err)
	}
	defer externalTx.Rollback()
	if _, err := externalTx.Exec("UPDATE locked_rows SET value = 2 WHERE id = 1"); err != nil {
		t.Fatalf("acquire external write lock: %v", err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	errs := make(chan error, 1)
	go func() {
		errs <- db.Transaction(func(tx *gorm.DB) error {
			close(started)
			var value int
			if err := tx.Table("locked_rows").Select("value").Where("id = 1").Scan(&value).Error; err != nil {
				return err
			}
			<-release
			return tx.Exec("UPDATE locked_rows SET value = value + 1 WHERE id = 1").Error
		})
	}()

	select {
	case <-started:
	case <-time.After(250 * time.Millisecond):
	}
	if err := externalTx.Commit(); err != nil {
		t.Fatalf("commit external transaction: %v", err)
	}
	close(release)
	if err := <-errs; err != nil {
		t.Fatalf("write transaction failed under snapshot contention: %v", err)
	}
}

func closeSQLiteTestDB(db *gorm.DB, sqlDB *sql.DB) {
	_ = sqlDB.Close()
	if pool, ok := db.ConnPool.(*sqliteRoutingPool); ok {
		_ = pool.reader.Close()
	}
}
