// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ChangeLogStore interface {
	Append(record *ChangeRecord) error
	AppendTx(tx *gorm.DB, record *ChangeRecord) error
	ClaimMutationTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) (bool, *MutationClaim, error)
	CommitClaimTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) error
	RollbackClaimTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) error
	GetClaimTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) (*MutationClaim, error)
	ListAfter(userID string, scope CursorScope, cursor Sequence, limit int, entityType EntityType) ([]ChangeRecord, error)
	Pull(userID string, scope CursorScope, cursor Sequence, limit int, entityType EntityType) ([]ChangeRecord, Sequence, bool, error)
	GetLatestSequence() (Sequence, error)
	GetByMutationID(mutationID MutationID) (*ChangeRecord, error)
	GetByMutationIDAndUser(mutationID MutationID, userID string) (*ChangeRecord, error)
	GetByMutationIDAndUserTx(tx *gorm.DB, mutationID MutationID, userID string) (*ChangeRecord, error)
	Count() (int64, error)
}

type ChangeRecorder interface {
	RecordChange(tx *gorm.DB, entityType EntityType, entityID EntityID, op OperationType, revision int64, mutationID MutationID, userID string, scope CursorScope, payload []byte) (*ChangeRecord, error)
}

type sqliteChangeLogStore struct {
	db *gorm.DB
}

func NewChangeLogStore(db *gorm.DB) ChangeLogStore {
	return &sqliteChangeLogStore{db: db}
}

func (s *sqliteChangeLogStore) Append(record *ChangeRecord) error {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.Checksum == "" {
		record.Checksum = computeChecksum(record.Payload)
	}
	return s.db.Create(record).Error
}

func (s *sqliteChangeLogStore) AppendTx(tx *gorm.DB, record *ChangeRecord) error {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.Checksum == "" {
		record.Checksum = computeChecksum(record.Payload)
	}
	return tx.Create(record).Error
}

func (s *sqliteChangeLogStore) ClaimMutationTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) (bool, *MutationClaim, error) {
	var existing MutationClaim
	err := tx.Where("user_id = ? AND scope = ? AND mutation_id = ?", userID, scope, mutationID).Take(&existing).Error
	if err == nil {
		return false, &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return false, nil, err
	}

	claim := &MutationClaim{
		UserID:     userID,
		Scope:      scope,
		MutationID: mutationID,
		Status:     MutationClaimStatusPending,
		CreatedAt:  time.Now().UTC(),
	}

	if err := tx.Create(claim).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate") {
			var conflict MutationClaim
			if tx.Where("user_id = ? AND scope = ? AND mutation_id = ?", userID, scope, mutationID).Take(&conflict).Error == nil {
				return false, &conflict, nil
			}
		}
		return false, nil, fmt.Errorf("claim mutation: create: %w", err)
	}
	return true, claim, nil
}

func (s *sqliteChangeLogStore) CommitClaimTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) error {
	return tx.Model(&MutationClaim{}).
		Where("user_id = ? AND scope = ? AND mutation_id = ? AND status = ?", userID, scope, mutationID, MutationClaimStatusPending).
		Update("status", MutationClaimStatusCommitted).Error
}

func (s *sqliteChangeLogStore) RollbackClaimTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) error {
	return tx.Model(&MutationClaim{}).
		Where("user_id = ? AND scope = ? AND mutation_id = ? AND status = ?", userID, scope, mutationID, MutationClaimStatusPending).
		Update("status", MutationClaimStatusRolledBack).Error
}

func (s *sqliteChangeLogStore) GetClaimTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) (*MutationClaim, error) {
	var claim MutationClaim
	err := tx.Where("user_id = ? AND scope = ? AND mutation_id = ?", userID, scope, mutationID).Take(&claim).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &claim, nil
}

func (s *sqliteChangeLogStore) ListAfter(userID string, scope CursorScope, cursor Sequence, limit int, entityType EntityType) ([]ChangeRecord, error) {
	var records []ChangeRecord
	query := s.db.Where("seq > ?", cursor).Order("seq ASC").Limit(limit)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if scope != "" {
		query = query.Where("scope = ?", scope)
	}
	if entityType != "" {
		query = query.Where("entity_type = ?", entityType)
	}
	err := query.Find(&records).Error
	return records, err
}

func (s *sqliteChangeLogStore) Pull(userID string, scope CursorScope, cursor Sequence, limit int, entityType EntityType) ([]ChangeRecord, Sequence, bool, error) {
	records, err := s.ListAfter(userID, scope, cursor, limit+1, entityType)
	if err != nil {
		return nil, 0, false, err
	}
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	var nextCursor Sequence
	if len(records) > 0 {
		nextCursor = records[len(records)-1].Sequence
	} else {
		nextCursor = cursor
	}
	return records, nextCursor, hasMore, nil
}

func (s *sqliteChangeLogStore) GetLatestSequence() (Sequence, error) {
	var record ChangeRecord
	err := s.db.Order("seq DESC").First(&record).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return record.Sequence, err
}

func (s *sqliteChangeLogStore) GetByMutationID(mutationID MutationID) (*ChangeRecord, error) {
	var record ChangeRecord
	err := s.db.Where("mutation_id = ?", mutationID).First(&record).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &record, err
}

func (s *sqliteChangeLogStore) GetByMutationIDAndUser(mutationID MutationID, userID string) (*ChangeRecord, error) {
	var record ChangeRecord
	err := s.db.Where("mutation_id = ? AND user_id = ?", mutationID, userID).First(&record).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &record, err
}

func (s *sqliteChangeLogStore) GetByMutationIDAndUserTx(tx *gorm.DB, mutationID MutationID, userID string) (*ChangeRecord, error) {
	var record ChangeRecord
	err := tx.Where("mutation_id = ? AND user_id = ?", mutationID, userID).First(&record).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &record, err
}

func (s *sqliteChangeLogStore) Count() (int64, error) {
	var count int64
	err := s.db.Model(&ChangeRecord{}).Count(&count).Error
	return count, err
}

type SequenceGenerator interface {
	NextSequence() (Sequence, error)
	NextSequenceTx(tx *gorm.DB) (Sequence, error)
}

type sqliteSequenceGenerator struct {
	db *gorm.DB
}

func NewSequenceGenerator(db *gorm.DB) SequenceGenerator {
	return &sqliteSequenceGenerator{db: db}
}

func (g *sqliteSequenceGenerator) NextSequence() (Sequence, error) {
	var nextSeq int64
	err := g.db.Raw("UPDATE sync_sequence SET seq = seq + 1 RETURNING seq").Scan(&nextSeq).Error
	if err != nil {
		return g.fallbackNextSequence()
	}
	return Sequence(nextSeq), nil
}

func (g *sqliteSequenceGenerator) NextSequenceTx(tx *gorm.DB) (Sequence, error) {
	var nextSeq int64
	err := tx.Raw("UPDATE sync_sequence SET seq = seq + 1 RETURNING seq").Scan(&nextSeq).Error
	if err != nil {
		return g.fallbackNextSequenceTx(tx)
	}
	return Sequence(nextSeq), nil
}

func (g *sqliteSequenceGenerator) fallbackNextSequence() (Sequence, error) {
	var nextSeq int64
	err := g.db.Transaction(func(tx *gorm.DB) error {
		seq, err := g.fallbackNextSequenceTx(tx)
		if err != nil {
			return err
		}
		nextSeq = int64(seq)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return Sequence(nextSeq), nil
}

func (g *sqliteSequenceGenerator) fallbackNextSequenceTx(tx *gorm.DB) (Sequence, error) {
	var nextSeq int64
	if err := tx.Exec("INSERT INTO sync_sequence (id, seq) VALUES (1, 1) ON CONFLICT(id) DO UPDATE SET seq = seq + 1").Error; err != nil {
		return 0, err
	}
	if err := tx.Raw("SELECT seq FROM sync_sequence WHERE id = 1").Scan(&nextSeq).Error; err != nil {
		return 0, err
	}
	return Sequence(nextSeq), nil
}

func computeChecksum(payload []byte) string {
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

type ChangeLogService struct {
	store     ChangeLogStore
	sequences SequenceGenerator
}

func NewChangeLogService(store ChangeLogStore, sequences SequenceGenerator) *ChangeLogService {
	return &ChangeLogService{store: store, sequences: sequences}
}

func (s *ChangeLogService) RecordChange(tx *gorm.DB, entityType EntityType, entityID EntityID, op OperationType, revision int64, mutationID MutationID, userID string, scope CursorScope, payload []byte) (*ChangeRecord, error) {
	return s.AppendTx(tx, entityType, entityID, op, revision, mutationID, "", userID, scope, payload)
}

func (s *ChangeLogService) GetByMutationIDAndUserTx(tx *gorm.DB, mutationID MutationID, userID string) (*ChangeRecord, error) {
	return s.store.GetByMutationIDAndUserTx(tx, mutationID, userID)
}

func (s *ChangeLogService) ClaimMutationTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) (bool, *MutationClaim, error) {
	return s.store.ClaimMutationTx(tx, mutationID, userID, scope)
}

func (s *ChangeLogService) CommitClaimTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) error {
	return s.store.CommitClaimTx(tx, mutationID, userID, scope)
}

func (s *ChangeLogService) RollbackClaimTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) error {
	return s.store.RollbackClaimTx(tx, mutationID, userID, scope)
}

func (s *ChangeLogService) GetClaimTx(tx *gorm.DB, mutationID MutationID, userID string, scope CursorScope) (*MutationClaim, error) {
	return s.store.GetClaimTx(tx, mutationID, userID, scope)
}

func (s *ChangeLogService) Append(entityType EntityType, entityID EntityID, op OperationType, revision int64, mutationID MutationID, originDevice string, userID string, scope CursorScope, payload []byte) (*ChangeRecord, error) {
	if mutationID != "" {
		existing, err := s.store.GetByMutationIDAndUser(mutationID, userID)
		if err != nil {
			return nil, fmt.Errorf("changelog: check mutation: %w", err)
		}
		if existing != nil {
			return existing, nil
		}
	}

	seq, err := s.sequences.NextSequence()
	if err != nil {
		return nil, fmt.Errorf("changelog: next sequence: %w", err)
	}

	if scope == "" {
		scope = ScopeDevice
	}

	record := &ChangeRecord{
		ChangeID:     ChangeID(fmt.Sprintf("ch_%d", seq)),
		Sequence:     seq,
		UserID:       userID,
		Scope:        scope,
		EntityType:   entityType,
		EntityID:     entityID,
		Operation:    op,
		Revision:     revision,
		MutationID:   mutationID,
		OriginDevice: originDevice,
		Payload:      payload,
		CreatedAt:    time.Now().UTC(),
	}
	record.Checksum = computeChecksum(payload)

	if err := s.store.Append(record); err != nil {
		return nil, fmt.Errorf("changelog: append: %w", err)
	}
	return record, nil
}

func (s *ChangeLogService) AppendTx(tx *gorm.DB, entityType EntityType, entityID EntityID, op OperationType, revision int64, mutationID MutationID, originDevice string, userID string, scope CursorScope, payload []byte) (*ChangeRecord, error) {
	if mutationID != "" {
		existing, err := s.store.GetByMutationIDAndUserTx(tx, mutationID, userID)
		if err != nil {
			return nil, fmt.Errorf("changelog: check mutation: %w", err)
		}
		if existing != nil {
			return existing, nil
		}
	}

	seq, err := s.sequences.NextSequenceTx(tx)
	if err != nil {
		return nil, fmt.Errorf("changelog: next sequence: %w", err)
	}

	if scope == "" {
		scope = ScopeDevice
	}

	record := &ChangeRecord{
		ChangeID:     ChangeID(fmt.Sprintf("ch_%d", seq)),
		Sequence:     seq,
		UserID:       userID,
		Scope:        scope,
		EntityType:   entityType,
		EntityID:     entityID,
		Operation:    op,
		Revision:     revision,
		MutationID:   mutationID,
		OriginDevice: originDevice,
		Payload:      payload,
		CreatedAt:    time.Now().UTC(),
	}
	record.Checksum = computeChecksum(payload)

	if err := s.store.AppendTx(tx, record); err != nil {
		return nil, fmt.Errorf("changelog: append: %w", err)
	}
	return record, nil
}

func (s *ChangeLogService) Pull(userID string, scope CursorScope, cursor Sequence, limit int, entityType EntityType) ([]ChangeRecord, Sequence, bool, error) {
	records, err := s.store.ListAfter(userID, scope, cursor, limit+1, entityType)
	if err != nil {
		return nil, 0, false, fmt.Errorf("changelog: list: %w", err)
	}

	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}

	var nextCursor Sequence
	if len(records) > 0 {
		nextCursor = records[len(records)-1].Sequence
	} else {
		nextCursor = cursor
	}

	return records, nextCursor, hasMore, nil
}

func (s *ChangeLogService) GetServerSequence() (Sequence, error) {
	return s.store.GetLatestSequence()
}
