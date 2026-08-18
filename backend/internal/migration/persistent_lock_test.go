package migration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPersistentLockTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "lock.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&migrationLockRecord{}); err != nil {
		t.Fatalf("migrate lock table: %v", err)
	}
	return db
}

func TestPersistentLockOnlyOneOwner(t *testing.T) {
	db := newPersistentLockTestDB(t)
	lockDir := t.TempDir()
	first := NewPersistentLock(db, lockDir)
	second := NewPersistentLock(db, lockDir)

	if err := first.Acquire(context.Background(), "desktop-pet", time.Minute); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer func() { _ = first.Release("desktop-pet") }()
	if err := second.Acquire(context.Background(), "desktop-pet", time.Minute); err == nil {
		t.Fatal("second owner unexpectedly acquired active lease")
	}
}

func TestPersistentLockRecoversCrashAfterLeaseExpiry(t *testing.T) {
	db := newPersistentLockTestDB(t)
	lockDir := t.TempDir()
	crashed := NewPersistentLock(db, lockDir)
	if err := crashed.Acquire(context.Background(), "desktop-pet", time.Minute); err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	past := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	if err := db.Model(&migrationLockRecord{}).Where("lock_name = ?", "desktop-pet").Update("lease_expires_at", past).Error; err != nil {
		t.Fatalf("expire lease: %v", err)
	}

	replacement := NewPersistentLock(db, lockDir)
	if err := replacement.Acquire(context.Background(), "desktop-pet", time.Minute); err != nil {
		t.Fatalf("replacement acquire after stale lease/file: %v", err)
	}
	defer func() { _ = replacement.Release("desktop-pet") }()

	var row migrationLockRecord
	if err := db.Where("lock_name = ?", "desktop-pet").Take(&row).Error; err != nil {
		t.Fatalf("load replacement lease: %v", err)
	}
	if row.OwnerInstanceID != replacement.instance {
		t.Fatalf("owner=%s want=%s", row.OwnerInstanceID, replacement.instance)
	}
	data, err := os.ReadFile(filepath.Join(lockDir, "desktop-pet.lock"))
	if err != nil {
		t.Fatalf("read replacement lock file: %v", err)
	}
	if !strings.Contains(string(data), "instance="+replacement.instance) {
		t.Fatalf("replacement lock file has wrong owner: %s", data)
	}
}

func TestPersistentLockLateReleaseCannotDeleteNewOwnersFile(t *testing.T) {
	db := newPersistentLockTestDB(t)
	lockDir := t.TempDir()
	oldOwner := NewPersistentLock(db, lockDir)
	if err := oldOwner.Acquire(context.Background(), "desktop-pet", time.Minute); err != nil {
		t.Fatalf("old acquire: %v", err)
	}
	past := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	if err := db.Model(&migrationLockRecord{}).Where("lock_name = ?", "desktop-pet").Update("lease_expires_at", past).Error; err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	newOwner := NewPersistentLock(db, lockDir)
	if err := newOwner.Acquire(context.Background(), "desktop-pet", time.Minute); err != nil {
		t.Fatalf("new acquire: %v", err)
	}
	defer func() { _ = newOwner.Release("desktop-pet") }()

	if err := oldOwner.Release("desktop-pet"); err != nil {
		t.Fatalf("late old-owner release: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(lockDir, "desktop-pet.lock"))
	if err != nil {
		t.Fatalf("new owner lock file removed by old owner: %v", err)
	}
	if !strings.Contains(string(data), "instance="+newOwner.instance) {
		t.Fatalf("unexpected lock owner after late release: %s", data)
	}
}

func TestPersistentLockHeartbeatDetectsLostLease(t *testing.T) {
	db := newPersistentLockTestDB(t)
	lock := NewPersistentLock(db, t.TempDir())
	if err := lock.Acquire(context.Background(), "desktop-pet", time.Second); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer func() { _ = lock.Release("desktop-pet") }()

	lost := make(chan struct{}, 1)
	lock.SetLeaseLostHandler(func(string) { lost <- struct{}{} })
	if err := lock.StartHeartbeat("desktop-pet", 10*time.Millisecond, time.Second); err != nil {
		t.Fatalf("start heartbeat: %v", err)
	}
	if err := db.Where("lock_name = ?", "desktop-pet").Delete(&migrationLockRecord{}).Error; err != nil {
		t.Fatalf("delete lease: %v", err)
	}
	select {
	case <-lost:
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not report lost lease")
	}
}

func TestPersistentLockSameInstanceSecondAcquireDoesNotDropFirstLease(t *testing.T) {
	db := newPersistentLockTestDB(t)
	lock := NewPersistentLock(db, t.TempDir())
	if err := lock.Acquire(context.Background(), "desktop-pet", time.Minute); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer func() { _ = lock.Release("desktop-pet") }()
	if err := lock.Acquire(context.Background(), "desktop-pet", time.Minute); err == nil {
		t.Fatal("same instance second acquire should not claim a second file lock")
	}
	var count int64
	if err := db.Model(&migrationLockRecord{}).
		Where("lock_name = ? AND owner_instance_id = ?", "desktop-pet", lock.instance).
		Count(&count).Error; err != nil {
		t.Fatalf("count lease: %v", err)
	}
	if count != 1 {
		t.Fatalf("lease count=%d want=1", count)
	}
}

func TestPersistentLockHeartbeatRejectsInvalidTiming(t *testing.T) {
	db := newPersistentLockTestDB(t)
	lock := NewPersistentLock(db, t.TempDir())
	if err := lock.StartHeartbeat("", time.Second, time.Minute); err == nil {
		t.Fatal("empty heartbeat lock name must fail")
	}
	if err := lock.StartHeartbeat("desktop-pet", 0, time.Minute); err == nil {
		t.Fatal("zero heartbeat interval must fail")
	}
	if err := lock.StartHeartbeat("desktop-pet", time.Second, 0); err == nil {
		t.Fatal("zero heartbeat ttl must fail")
	}
	if err := lock.StartHeartbeat("desktop-pet", time.Minute, time.Minute); err == nil {
		t.Fatal("heartbeat interval >= ttl must fail")
	}
}
