// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type ChangeLogStore interface {
	Append(record *ChangeRecord) error
	ListAfter(cursor Sequence, limit int, entityType EntityType) ([]ChangeRecord, error)
	GetLatestSequence() (Sequence, error)
	GetByMutationID(mutationID MutationID) (*ChangeRecord, error)
	Count() (int64, error)
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

func (s *sqliteChangeLogStore) ListAfter(cursor Sequence, limit int, entityType EntityType) ([]ChangeRecord, error) {
	var records []ChangeRecord
	query := s.db.Where("seq > ?", cursor).Order("seq ASC").Limit(limit)
	if entityType != "" {
		query = query.Where("entity_type = ?", entityType)
	}
	err := query.Find(&records).Error
	return records, err
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

func (s *sqliteChangeLogStore) Count() (int64, error) {
	var count int64
	err := s.db.Model(&ChangeRecord{}).Count(&count).Error
	return count, err
}

type SequenceGenerator interface {
	NextSequence() (Sequence, error)
}

type sqliteSequenceGenerator struct {
	db *gorm.DB
}

func NewSequenceGenerator(db *gorm.DB) SequenceGenerator {
	return &sqliteSequenceGenerator{db: db}
}

func (g *sqliteSequenceGenerator) NextSequence() (Sequence, error) {
	var maxSeq int64
	err := g.db.Model(&ChangeRecord{}).Select("COALESCE(MAX(seq), 0)").Scan(&maxSeq).Error
	if err != nil {
		return 0, err
	}
	return Sequence(maxSeq + 1), nil
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

func (s *ChangeLogService) Append(entityType EntityType, entityID EntityID, op OperationType, revision int64, mutationID MutationID, originDevice string, payload []byte) (*ChangeRecord, error) {
	if mutationID != "" {
		existing, err := s.store.GetByMutationID(mutationID)
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

	record := &ChangeRecord{
		ChangeID:     ChangeID(fmt.Sprintf("ch_%d", seq)),
		Sequence:     seq,
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

func (s *ChangeLogService) Pull(cursor Sequence, limit int, entityType EntityType) ([]ChangeRecord, Sequence, bool, error) {
	records, err := s.store.ListAfter(cursor, limit+1, entityType)
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
