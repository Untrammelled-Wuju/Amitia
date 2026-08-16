package migration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Locker interface {
	Acquire(ctx context.Context, lockName string, ttl time.Duration) error
	Release(lockName string) error
}

type heartbeatController struct {
	stop chan struct{}
	done chan struct{}
}

type LeaseLostHandler func(lockName string)

type PersistentLock struct {
	db               *gorm.DB
	lockDir          string
	instance         string
	mu               sync.Mutex
	heartbeats       map[string]*heartbeatController
	leaseLostHandler LeaseLostHandler
}

func NewPersistentLock(db *gorm.DB, lockDir string) *PersistentLock {
	return &PersistentLock{
		db:         db,
		lockDir:    lockDir,
		instance:   uuid.New().String(),
		heartbeats: make(map[string]*heartbeatController),
	}
}

func (l *PersistentLock) SetLeaseLostHandler(handler LeaseLostHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.leaseLostHandler = handler
}

func (l *PersistentLock) Acquire(ctx context.Context, lockName string, ttl time.Duration) error {
	lockFile := filepath.Join(l.lockDir, lockName+".lock")
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	expires := now.Add(ttl).Format(time.RFC3339Nano)

	_ = os.Remove(lockFile)
	fh, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("migration: file lock already exists: %s", lockName)
		}
		return fmt.Errorf("migration: file lock open: %w", err)
	}
	if _, err := fmt.Fprintf(fh, "instance=%s\ncreated=%s\n", l.instance, nowStr); err != nil {
		fh.Close()
		os.Remove(lockFile)
		return fmt.Errorf("migration: write lock file: %w", err)
	}
	fh.Close()

	lease := migrationLockRecord{
		LockName:        lockName,
		OwnerInstanceID: l.instance,
		LeaseExpiresAt:  expires,
		HeartbeatAt:     nowStr,
	}

	err = l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing migrationLockRecord
		if err := tx.Where("lock_name = ?", lockName).Take(&existing).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return tx.Create(&lease).Error
		}
		expiresAt, parseErr := time.Parse(time.RFC3339Nano, existing.LeaseExpiresAt)
		if parseErr != nil || !expiresAt.After(now) {
			return tx.Model(&migrationLockRecord{}).
				Where("lock_name = ? AND owner_instance_id = ? AND lease_expires_at = ?",
					lockName, existing.OwnerInstanceID, existing.LeaseExpiresAt).
				Updates(map[string]interface{}{
					"owner_instance_id": l.instance,
					"lease_expires_at":  expires,
					"heartbeat_at":      nowStr,
				}).Error
		}
		if existing.OwnerInstanceID == l.instance {
			return tx.Model(&migrationLockRecord{}).
				Where("lock_name = ? AND owner_instance_id = ?", lockName, l.instance).
				Updates(map[string]interface{}{
					"lease_expires_at": expires,
					"heartbeat_at":     nowStr,
				}).Error
		}
		return fmt.Errorf("migration: db lease held by %s until %s", existing.OwnerInstanceID, existing.LeaseExpiresAt)
	})

	if err != nil {
		os.Remove(lockFile)
		return err
	}
	return nil
}

func (l *PersistentLock) Release(lockName string) error {
	l.stopHeartbeat(lockName)
	l.db.Where("lock_name = ? AND owner_instance_id = ?", lockName, l.instance).Delete(&migrationLockRecord{})
	lockFile := filepath.Join(l.lockDir, lockName+".lock")
	os.Remove(lockFile)
	return nil
}

func (l *PersistentLock) StartHeartbeat(lockName string, interval time.Duration, ttl time.Duration) error {
	l.stopHeartbeat(lockName)
	ctl := &heartbeatController{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	l.mu.Lock()
	l.heartbeats[lockName] = ctl
	l.mu.Unlock()
	go func() {
		defer close(ctl.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctl.stop:
				return
			case <-ticker.C:
				if err := l.renewLease(lockName, ttl); err != nil {
					l.mu.Lock()
					handler := l.leaseLostHandler
					l.mu.Unlock()
					if handler != nil {
						handler(lockName)
					}
					return
				}
			}
		}
	}()
	return nil
}

func (l *PersistentLock) stopHeartbeat(lockName string) {
	l.mu.Lock()
	ctl, ok := l.heartbeats[lockName]
	if ok {
		delete(l.heartbeats, lockName)
	}
	l.mu.Unlock()
	if !ok {
		return
	}
	close(ctl.stop)
	<-ctl.done
}

func (l *PersistentLock) renewLease(lockName string, ttl time.Duration) error {
	now := time.Now().UTC()
	expires := now.Add(ttl).Format(time.RFC3339Nano)
	nowStr := now.Format(time.RFC3339Nano)
	result := l.db.Model(&migrationLockRecord{}).
		Where("lock_name = ? AND owner_instance_id = ?", lockName, l.instance).
		Updates(map[string]interface{}{
			"lease_expires_at": expires,
			"heartbeat_at":     nowStr,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("migration: lease ownership lost")
	}
	return nil
}

type migrationLockRecord struct {
	LockName        string `gorm:"column:lock_name;primaryKey"`
	OwnerInstanceID string `gorm:"column:owner_instance_id"`
	LeaseExpiresAt  string `gorm:"column:lease_expires_at"`
	HeartbeatAt     string `gorm:"column:heartbeat_at"`
}

func (migrationLockRecord) TableName() string { return "desktop_pet_migration_locks" }
