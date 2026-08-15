// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type CursorStore interface {
	Get(deviceID string) (*SyncCursor, error)
	Save(cursor *SyncCursor) error
	UpdateApplied(deviceID string, seq Sequence) error
	UpdatePushed(deviceID string, seq Sequence) error
	ListByUser(userID string) ([]SyncCursor, error)
}

type sqliteCursorStore struct {
	db *gorm.DB
}

func NewCursorStore(db *gorm.DB) CursorStore {
	return &sqliteCursorStore{db: db}
}

func (s *sqliteCursorStore) Get(deviceID string) (*SyncCursor, error) {
	var cursor SyncCursor
	err := s.db.Where("device_id = ?", deviceID).First(&cursor).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &cursor, err
}

func (s *sqliteCursorStore) Save(cursor *SyncCursor) error {
	cursor.UpdatedAt = time.Now().UTC()
	existing, err := s.Get(cursor.DeviceID)
	if err != nil {
		return err
	}
	if existing == nil {
		return s.db.Create(cursor).Error
	}
	return s.db.Model(&SyncCursor{}).Where("device_id = ?", cursor.DeviceID).Updates(map[string]interface{}{
		"last_applied": cursor.LastApplied,
		"last_pushed":  cursor.LastPushed,
		"updated_at":   cursor.UpdatedAt,
	}).Error
}

func (s *sqliteCursorStore) UpdateApplied(deviceID string, seq Sequence) error {
	return s.db.Model(&SyncCursor{}).Where("device_id = ?", deviceID).Updates(map[string]interface{}{
		"last_applied": seq,
		"updated_at":   time.Now().UTC(),
	}).Error
}

func (s *sqliteCursorStore) UpdatePushed(deviceID string, seq Sequence) error {
	return s.db.Model(&SyncCursor{}).Where("device_id = ?", deviceID).Updates(map[string]interface{}{
		"last_pushed": seq,
		"updated_at":  time.Now().UTC(),
	}).Error
}

func (s *sqliteCursorStore) ListByUser(userID string) ([]SyncCursor, error) {
	var cursors []SyncCursor
	err := s.db.Where("user_id = ?", userID).Find(&cursors).Error
	return cursors, err
}

type CursorService struct {
	store CursorStore
}

func NewCursorService(store CursorStore) *CursorService {
	return &CursorService{store: store}
}

func (s *CursorService) GetOrCreate(deviceID, userID string, scope CursorScope) (*SyncCursor, error) {
	cursor, err := s.store.Get(deviceID)
	if err != nil {
		return nil, fmt.Errorf("cursor: get: %w", err)
	}
	if cursor != nil {
		return cursor, nil
	}

	cursor = &SyncCursor{
		DeviceID:    deviceID,
		UserID:      userID,
		Scope:       scope,
		LastApplied: 0,
		LastPushed:  0,
		UpdatedAt:   time.Now().UTC(),
	}
	if err := s.store.Save(cursor); err != nil {
		return nil, fmt.Errorf("cursor: create: %w", err)
	}
	return cursor, nil
}

func (s *CursorService) MarkApplied(deviceID string, seq Sequence) error {
	return s.store.UpdateApplied(deviceID, seq)
}

func (s *CursorService) MarkPushed(deviceID string, seq Sequence) error {
	return s.store.UpdatePushed(deviceID, seq)
}

func (s *CursorService) GetStatus(deviceID string, serverSeq Sequence) (*CursorStatus, error) {
	cursor, err := s.store.Get(deviceID)
	if err != nil {
		return nil, fmt.Errorf("cursor: get: %w", err)
	}
	if cursor == nil {
		return &CursorStatus{
			DeviceID:       deviceID,
			LastApplied:    0,
			LastPushed:     0,
			ServerSequence: serverSeq,
			Lag:            int64(serverSeq),
		}, nil
	}

	return &CursorStatus{
		DeviceID:       cursor.DeviceID,
		UserID:         cursor.UserID,
		LastApplied:    cursor.LastApplied,
		LastPushed:     cursor.LastPushed,
		ServerSequence: serverSeq,
		Lag:            int64(serverSeq) - int64(cursor.LastApplied),
		LastPullAt:     cursor.UpdatedAt,
	}, nil
}
