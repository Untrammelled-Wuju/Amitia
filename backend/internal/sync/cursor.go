// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type CursorIdentity struct {
	UserID   string
	Scope    CursorScope
	DeviceID string
}

type CursorStore interface {
	Get(identity CursorIdentity) (*SyncCursor, error)
	Save(cursor *SyncCursor) error
	UpdateApplied(identity CursorIdentity, seq Sequence) error
	UpdatePushed(identity CursorIdentity, seq Sequence) error
	UpdatePushedTx(tx *gorm.DB, identity CursorIdentity, seq Sequence) error
	ListByUser(userID string) ([]SyncCursor, error)
}

type sqliteCursorStore struct {
	db *gorm.DB
}

func NewCursorStore(db *gorm.DB) CursorStore {
	return &sqliteCursorStore{db: db}
}

func (s *sqliteCursorStore) Get(identity CursorIdentity) (*SyncCursor, error) {
	var cursor SyncCursor
	err := s.db.Where("user_id = ? AND scope = ? AND device_id = ?", identity.UserID, identity.Scope, identity.DeviceID).First(&cursor).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &cursor, err
}

func (s *sqliteCursorStore) Save(cursor *SyncCursor) error {
	cursor.UpdatedAt = time.Now().UTC()
	existing, err := s.Get(CursorIdentity{UserID: cursor.UserID, Scope: cursor.Scope, DeviceID: cursor.DeviceID})
	if err != nil {
		return err
	}
	if existing == nil {
		return s.db.Create(cursor).Error
	}
	return s.db.Model(&SyncCursor{}).Where("user_id = ? AND scope = ? AND device_id = ?", cursor.UserID, cursor.Scope, cursor.DeviceID).Updates(map[string]interface{}{
		"last_applied": cursor.LastApplied,
		"last_pushed":  cursor.LastPushed,
		"updated_at":   cursor.UpdatedAt,
	}).Error
}

func (s *sqliteCursorStore) UpdateApplied(identity CursorIdentity, seq Sequence) error {
	return s.db.Exec(`INSERT INTO sync_cursors (device_id, user_id, scope, last_applied, last_pushed, updated_at)
		VALUES (?, ?, ?, ?, 0, ?)
		ON CONFLICT(user_id, scope, device_id) DO UPDATE SET
		last_applied = CASE WHEN sync_cursors.last_applied < excluded.last_applied THEN excluded.last_applied ELSE sync_cursors.last_applied END,
		updated_at = excluded.updated_at`, identity.DeviceID, identity.UserID, identity.Scope, seq, time.Now().UTC()).Error
}

func (s *sqliteCursorStore) UpdatePushed(identity CursorIdentity, seq Sequence) error {
	return s.db.Exec(`INSERT INTO sync_cursors (device_id, user_id, scope, last_applied, last_pushed, updated_at)
		VALUES (?, ?, ?, 0, ?, ?)
		ON CONFLICT(user_id, scope, device_id) DO UPDATE SET
		last_pushed = CASE WHEN sync_cursors.last_pushed < excluded.last_pushed THEN excluded.last_pushed ELSE sync_cursors.last_pushed END,
		updated_at = excluded.updated_at`, identity.DeviceID, identity.UserID, identity.Scope, seq, time.Now().UTC()).Error
}

func (s *sqliteCursorStore) UpdatePushedTx(tx *gorm.DB, identity CursorIdentity, seq Sequence) error {
	return tx.Exec(`INSERT INTO sync_cursors (device_id, user_id, scope, last_applied, last_pushed, updated_at)
		VALUES (?, ?, ?, 0, ?, ?)
		ON CONFLICT(user_id, scope, device_id) DO UPDATE SET
		last_pushed = CASE WHEN sync_cursors.last_pushed < excluded.last_pushed THEN excluded.last_pushed ELSE sync_cursors.last_pushed END,
		updated_at = excluded.updated_at`, identity.DeviceID, identity.UserID, identity.Scope, seq, time.Now().UTC()).Error
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

func (s *CursorService) GetOrCreate(identity CursorIdentity) (*SyncCursor, error) {
	cursor, err := s.store.Get(identity)
	if err != nil {
		return nil, fmt.Errorf("cursor: get: %w", err)
	}
	if cursor != nil {
		return cursor, nil
	}

	cursor = &SyncCursor{
		DeviceID:    identity.DeviceID,
		UserID:      identity.UserID,
		Scope:       identity.Scope,
		LastApplied: 0,
		LastPushed:  0,
		UpdatedAt:   time.Now().UTC(),
	}
	if err := s.store.Save(cursor); err != nil {
		return nil, fmt.Errorf("cursor: create: %w", err)
	}
	return cursor, nil
}

func (s *CursorService) MarkApplied(identity CursorIdentity, seq Sequence) error {
	return s.store.UpdateApplied(identity, seq)
}

func (s *CursorService) MarkPushed(identity CursorIdentity, seq Sequence) error {
	return s.store.UpdatePushed(identity, seq)
}

func (s *CursorService) MarkPushedTx(tx *gorm.DB, identity CursorIdentity, seq Sequence) error {
	return s.store.UpdatePushedTx(tx, identity, seq)
}

func (s *CursorService) GetStatus(identity CursorIdentity, serverSeq Sequence) (*CursorStatus, error) {
	cursor, err := s.store.Get(identity)
	if err != nil {
		return nil, fmt.Errorf("cursor: get: %w", err)
	}
	if cursor == nil {
		return &CursorStatus{
			DeviceID:       identity.DeviceID,
			UserID:         identity.UserID,
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
