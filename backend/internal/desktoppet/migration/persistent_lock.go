// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PersistentLock struct {
	db       *gorm.DB
	lockDir  string
	instance string
}

func NewPersistentLock(db *gorm.DB, lockDir string) *PersistentLock {
	return &PersistentLock{
		db:       db,
		lockDir:  lockDir,
		instance: uuid.New().String(),
	}
}

func (l *PersistentLock) Acquire(ctx context.Context, lockName string, ttl time.Duration) error {
	lockFile := filepath.Join(l.lockDir, lockName+".lock")
	fh, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("migration: file lock already exists: %s", lockName)
		}
		return fmt.Errorf("migration: file lock open: %w", err)
	}
	defer fh.Close()

	expires := time.Now().UTC().Add(ttl).Format(time.RFC3339Nano)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	lease := migrationLock{
		LockName:        lockName,
		OwnerInstanceID: l.instance,
		LeaseExpiresAt:  expires,
		HeartbeatAt:     now,
	}

	tx := l.db.Begin()
	var existing migrationLock
	if err := tx.Where("lock_name = ?", lockName).First(&existing).Error; err == nil {
		if existing.LeaseExpiresAt > now && existing.OwnerInstanceID != l.instance {
			tx.Rollback()
			os.Remove(lockFile)
			return fmt.Errorf("migration: db lease held by %s until %s", existing.OwnerInstanceID, existing.LeaseExpiresAt)
		}
	}
	if err := tx.Save(&lease).Error; err != nil {
		tx.Rollback()
		os.Remove(lockFile)
		return fmt.Errorf("migration: db lease save: %w", err)
	}
	if err := tx.Commit().Error; err != nil {
		os.Remove(lockFile)
		return fmt.Errorf("migration: db commit: %w", err)
	}
	return nil
}

func (l *PersistentLock) Release(lockName string) error {
	lockFile := filepath.Join(l.lockDir, lockName+".lock")
	os.Remove(lockFile)
	l.db.Where("lock_name = ? AND owner_instance_id = ?", lockName, l.instance).Delete(&migrationLock{})
	return nil
}

type migrationLock struct {
	LockName        string `gorm:"column:lock_name;primaryKey"`
	OwnerInstanceID string `gorm:"column:owner_instance_id"`
	LeaseExpiresAt  string `gorm:"column:lease_expires_at"`
	HeartbeatAt     string `gorm:"column:heartbeat_at"`
}

func (migrationLock) TableName() string { return "desktop_pet_migration_locks" }
