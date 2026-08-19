// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

type AppSetting struct {
	ID        uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	Key       string         `gorm:"column:key;not null;uniqueIndex"`
	Value     string         `gorm:"column:value"`
	Revision  int64          `gorm:"column:revision;not null;default:1"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
	UpdatedAt *time.Time     `gorm:"column:updated_at"`
}

func (AppSetting) TableName() string {
	return "app_settings"
}

type SettingsStore struct {
	db *gorm.DB
}

func NewSettingsStore(db *gorm.DB) *SettingsStore {
	return &SettingsStore{db: db}
}

func (s *SettingsStore) Get(key string) (string, int64, error) {
	var row struct {
		Value    string
		Revision int64
	}
	err := s.db.Model(&AppSetting{}).Select("value, revision").Where("key = ? AND deleted_at IS NULL", key).Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	return row.Value, row.Revision, nil
}

func (s *SettingsStore) Upsert(key, value string) (int64, error) {
	var existing struct {
		ID       uint64
		Revision int64
	}
	err := s.db.Model(&AppSetting{}).Select("id, revision").Where("key = ?", key).Take(&existing).Error
	if err == gorm.ErrRecordNotFound {
		now := time.Now().UTC()
		setting := AppSetting{
			Key:       key,
			Value:     value,
			Revision:  1,
			UpdatedAt: &now,
		}
		if err := s.db.Create(&setting).Error; err != nil {
			return 0, fmt.Errorf("settings create: %w", err)
		}
		return 1, nil
	}
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	newRevision := existing.Revision + 1
	result := s.db.Model(&AppSetting{}).
		Where("key = ? AND revision = ?", key, existing.Revision).
		Updates(map[string]interface{}{
			"value":      value,
			"revision":   newRevision,
			"deleted_at": nil,
			"updated_at": now,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("settings upsert: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return 0, fmt.Errorf("settings upsert conflict for key %s", key)
	}
	return newRevision, nil
}

func (s *SettingsStore) SetWithCAS(key, value string, baseRevision int64) (int64, error) {
	if baseRevision == 0 {
		var existing struct {
			ID       uint64
			Revision int64
		}
		err := s.db.Model(&AppSetting{}).Select("id, revision").Where("key = ?", key).Take(&existing).Error
		if err == gorm.ErrRecordNotFound {
			now := time.Now().UTC()
			setting := AppSetting{
				Key:       key,
				Value:     value,
				Revision:  1,
				UpdatedAt: &now,
			}
			if err := s.db.Create(&setting).Error; err != nil {
				return 0, fmt.Errorf("settings create: %w", err)
			}
			return 1, nil
		}
		if err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("conflict: expected new but exists with revision %d", existing.Revision)
	}

	result := s.db.Model(&AppSetting{}).
		Where("key = ? AND revision = ? AND deleted_at IS NULL", key, baseRevision).
		Updates(map[string]interface{}{
			"value":      value,
			"revision":   baseRevision + 1,
			"updated_at": time.Now().UTC(),
		})

	if result.Error != nil {
		return 0, fmt.Errorf("settings update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return 0, fmt.Errorf("conflict: revision mismatch for key %s at revision %d", key, baseRevision)
	}
	return baseRevision + 1, nil
}

func (s *SettingsStore) DeleteWithTombstone(key string, baseRevision int64) (int64, error) {
	var existing struct {
		ID        uint64
		Revision  int64
		DeletedAt gorm.DeletedAt
	}
	err := s.db.Model(&AppSetting{}).Select("id, revision, deleted_at").Where("key = ?", key).Take(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return 0, fmt.Errorf("key not found")
	}
	if err != nil {
		return 0, err
	}
	if existing.DeletedAt.Valid {
		return existing.Revision, nil
	}
	newRevision := existing.Revision + 1
	if baseRevision != 0 && baseRevision != existing.Revision {
		return 0, fmt.Errorf("conflict: expected revision %d but got %d", baseRevision, existing.Revision)
	}
	now := time.Now().UTC()
	result := s.db.Model(&AppSetting{}).
		Where("key = ? AND revision = ?", key, existing.Revision).
		Updates(map[string]interface{}{
			"deleted_at": gorm.DeletedAt{Time: now, Valid: true},
			"value":      "",
			"revision":   newRevision,
			"updated_at": now,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("settings delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return 0, fmt.Errorf("conflict: revision mismatch during delete for key %s", key)
	}
	return newRevision, nil
}

func (s *service) getAppSetting(key string) string {
	store := NewSettingsStore(s.db)
	val, _, err := store.Get(key)
	if err != nil {
		return ""
	}
	return val
}

func (s *service) setAppSetting(key, val string) {
	store := NewSettingsStore(s.db)
	if _, err := store.Upsert(key, val); err != nil {
		log.Printf("system settings: persist %q failed: %v", key, err)
	}
}

func (s *service) setAppSettingWithRevision(key, val string, baseRevision int64) (int64, error) {
	store := NewSettingsStore(s.db)
	return store.SetWithCAS(key, val, baseRevision)
}

func (s *service) deleteAppSetting(key string, baseRevision int64) (int64, error) {
	store := NewSettingsStore(s.db)
	return store.DeleteWithTombstone(key, baseRevision)
}
