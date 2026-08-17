package migration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	if err := db.AutoMigrate(&migrationLockRecord{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func closeTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Logf("warning: could not get sql.DB: %v", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		t.Logf("warning: could not close sql.DB: %v", err)
	}
}

func TestPersistentLock_CrashRecovery(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	lockDir := t.TempDir()
	ctx := context.Background()
	ttl := 2 * time.Second

	inst1 := NewPersistentLock(db, lockDir)
	inst2 := NewPersistentLock(db, lockDir)

	if err := inst1.Acquire(ctx, "test-lock", ttl); err != nil {
		t.Fatalf("inst1 acquire: %v", err)
	}

	lockFile := filepath.Join(lockDir, "test-lock.lock")
	if _, err := os.Stat(lockFile); err != nil {
		t.Fatalf("lock file should exist: %v", err)
	}

	time.Sleep(ttl + 500*time.Millisecond)

	if _, err := os.Stat(lockFile); err != nil {
		t.Fatalf("stale lock file should still exist: %v", err)
	}

	if err := inst2.Acquire(ctx, "test-lock", ttl); err != nil {
		t.Fatalf("inst2 should take over after expiry: %v", err)
	}

	data, err := os.ReadFile(lockFile)
	if err != nil {
		t.Fatalf("lock file should exist: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("lock file should not be empty")
	}

	if err := inst2.Release("test-lock"); err != nil {
		t.Fatalf("inst2 release: %v", err)
	}
}

func TestPersistentLock_FileLockFailureReleasesDBLease(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	lockDir := t.TempDir()
	ctx := context.Background()
	ttl := 30 * time.Second

	inst1 := NewPersistentLock(db, lockDir)
	if err := inst1.Acquire(ctx, "test-lock", ttl); err != nil {
		t.Fatalf("inst1 acquire: %v", err)
	}
	inst1.Release("test-lock")

	lockFile := filepath.Join(lockDir, "test-lock.lock")
	if err := os.WriteFile(lockFile, []byte("corrupted"), 0o600); err != nil {
		t.Fatalf("write corrupted lock: %v", err)
	}

	inst2 := NewPersistentLock(db, lockDir)
	err := inst2.Acquire(ctx, "test-lock", ttl)
	if err == nil {
		inst2.Release("test-lock")
		t.Fatal("expected acquire to fail with stale lock file (not after takeover)")
	}

	var count int64
	db.Model(&migrationLockRecord{}).Where("lock_name = ?", "test-lock").Count(&count)
	if count != 0 {
		t.Fatalf("DB lease should be released after file lock failure, got %d records", count)
	}
}

func TestPersistentLock_ConcurrentAcquire(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	lockDir := t.TempDir()
	ctx := context.Background()
	ttl := 30 * time.Second

	inst1 := NewPersistentLock(db, lockDir)
	inst2 := NewPersistentLock(db, lockDir)

	if err := inst1.Acquire(ctx, "test-lock", ttl); err != nil {
		t.Fatalf("inst1 acquire: %v", err)
	}

	if err := inst2.Acquire(ctx, "test-lock", ttl); err == nil {
		inst2.Release("test-lock")
		t.Fatal("inst2 should not acquire while inst1 holds the lock")
	}

	if err := inst1.Release("test-lock"); err != nil {
		t.Fatalf("inst1 release: %v", err)
	}

	if err := inst2.Acquire(ctx, "test-lock", ttl); err != nil {
		t.Fatalf("inst2 should acquire after inst1 releases: %v", err)
	}
	inst2.Release("test-lock")
}
