// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type canonicalCreateRequest struct {
	CharacterID string
	MemoryType  MemoryType
	Source      string
	Scope       string

	Key   string
	Value string

	Importance int
	Confidence int

	ExpiresAt *string

	EntityID   string
	EntityType string

	SourceMsgID  string
	SourceConvID string

	VerifiedStatus string

	SensitivityLevel      string
	AllowProactiveMention bool
	RequiresConfirmation  bool

	DerivationKey string

	OperationID string
	EventType   string
	EventReason string

	Derivations []MemoryDerivationInput
}

type canonicalUpdateRequest struct {
	Updates map[string]interface{}

	OperationID string
	EventType   string
	EventReason string

	ExpectedVersion *int
}

type canonicalDeleteRequest struct {
	OperationID string
	EventReason string

	HardDelete bool
}

type MemoryDerivationInput struct {
	InputMemoryID    string
	InputVersion     int
	InputSnapshotHash string
	DerivationKind   string
	Ordinal          int
}

type MemoryEventRecord struct {
	ID string

	MemoryID string
	Version  int

	EventType string

	OperationID  string
	SnapshotHash string
	EventReason  string

	Key        string
	Value      string
	MemoryType string

	Importance int
	Confidence int

	Source      string
	CharacterID string

	CreatedAt string
}

func (s *service) createCanonicalMemory(req canonicalCreateRequest) (*Memory, error) {
	memoryType, ok := NormalizeMemoryType(string(req.MemoryType))
	if !ok {
		memoryType = MemoryTypeFact
	}

	importance := req.Importance
	if importance < 0 {
		importance = 0
	}
	if importance > 10 {
		importance = 10
	}

	confidence := req.Confidence
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 100 {
		confidence = 100
	}
	if confidence == 0 {
		confidence = 50
	}

	verifiedStatus := req.VerifiedStatus
	if verifiedStatus == "" {
		verifiedStatus = "unverified"
	}

	scope := req.Scope
	if scope == "" {
		scope = "character"
	}

	source := req.Source
	if source == "" {
		source = "manual"
	}

	operationID := req.OperationID
	if operationID == "" {
		operationID = uuid.New().String()
	}

	eventType := req.EventType
	if eventType == "" {
		eventType = "memory_created"
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	memoryID := uuid.New().String()
	snapshotHash := ""

	var derivations []MemoryDerivation
	for _, d := range req.Derivations {
		derivations = append(derivations, MemoryDerivation{
			ID:                uuid.New().String(),
			OutputMemoryID:    memoryID,
			InputMemoryID:     d.InputMemoryID,
			InputVersion:      d.InputVersion,
			InputSnapshotHash: d.InputSnapshotHash,
			DerivationKind:    d.DerivationKind,
			Ordinal:           d.Ordinal,
			OperationID:       operationID,
			CreatedAt:         now,
		})
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		m := &Memory{
			ID:                    memoryID,
			CharacterID:           req.CharacterID,
			MemoryType:            string(memoryType),
			Source:                source,
			Scope:                 scope,
			Key:                   req.Key,
			Value:                 req.Value,
			Importance:            importance,
			Confidence:            confidence,
			ExpiresAt:             req.ExpiresAt,
			EntityID:              req.EntityID,
			EntityType:            req.EntityType,
			SourceMsgID:           req.SourceMsgID,
			SourceConvID:          req.SourceConvID,
			VerifiedStatus:        verifiedStatus,
			SensitivityLevel:      req.SensitivityLevel,
			AllowProactiveMention: req.AllowProactiveMention,
			RequiresConfirmation:  req.RequiresConfirmation,
			Version:               1,
			DerivationKey:         req.DerivationKey,
		}

		if err := tx.Create(m).Error; err != nil {
			return err
		}

		snapshotHash = computeMemorySnapshotHashCanonical(m)

		event := MemoryEventRecord{
			ID:           uuid.New().String(),
			MemoryID:     memoryID,
			Version:      1,
			EventType:    eventType,
			OperationID:  operationID,
			SnapshotHash: snapshotHash,
			EventReason:  req.EventReason,
			Key:          req.Key,
			Value:        req.Value,
			MemoryType:   string(memoryType),
			Importance:   importance,
			Confidence:   confidence,
			Source:       source,
			CharacterID:  req.CharacterID,
			CreatedAt:    now,
		}

		if err := insertMemoryEvent(tx, event); err != nil {
			return err
		}

		for _, d := range derivations {
			if err := tx.Create(&d).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return s.repo.FindByID(memoryID)
}

func (s *service) updateCanonicalMemory(id string, req canonicalUpdateRequest) (*Memory, error) {
	operationID := req.OperationID
	if operationID == "" {
		operationID = uuid.New().String()
	}

	eventType := req.EventType
	if eventType == "" {
		eventType = "memory_updated"
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	var result *Memory

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var current Memory
		if err := tx.Where("id = ?", id).First(&current).Error; err != nil {
			return err
		}

		expectedVersion := 0
		if req.ExpectedVersion != nil {
			expectedVersion = *req.ExpectedVersion
		}
		if expectedVersion > 0 && current.Version != expectedVersion {
			return fmt.Errorf("version_conflict: expected %d, got %d", expectedVersion, current.Version)
		}

		newVersion := current.Version + 1

		updates := make(map[string]interface{})
		for k, v := range req.Updates {
			updates[k] = v
		}
		updates["version"] = newVersion

		if err := tx.Model(&Memory{}).Where("id = ? AND version = ?", id, current.Version).Updates(updates).Error; err != nil {
			return err
		}

		var updated Memory
		if err := tx.Where("id = ?", id).First(&updated).Error; err != nil {
			return err
		}

		snapshotHash := computeMemorySnapshotHash(&updated)

		event := MemoryEventRecord{
			ID:           uuid.New().String(),
			MemoryID:     id,
			Version:      newVersion,
			EventType:    eventType,
			OperationID:  operationID,
			SnapshotHash: snapshotHash,
			EventReason:  req.EventReason,
			Key:          updated.Key,
			Value:        updated.Value,
			MemoryType:   updated.MemoryType,
			Importance:   updated.Importance,
			Confidence:   updated.Confidence,
			Source:       updated.Source,
			CharacterID:  updated.CharacterID,
			CreatedAt:    now,
		}

		if err := insertMemoryEvent(tx, event); err != nil {
			return err
		}

		result = &updated
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *service) deleteCanonicalMemory(id string, req canonicalDeleteRequest) error {
	operationID := req.OperationID
	if operationID == "" {
		operationID = uuid.New().String()
	}

	eventType := "memory_deleted"
	now := time.Now().Format("2006-01-02 15:04:05")

	return s.db.Transaction(func(tx *gorm.DB) error {
		var current Memory
		if err := tx.Where("id = ?", id).First(&current).Error; err != nil {
			return err
		}

		newVersion := current.Version + 1

		updates := map[string]interface{}{
			"version":         newVersion,
			"verified_status": "tombstone",
		}

		if err := tx.Model(&Memory{}).Where("id = ? AND version = ?", id, current.Version).Updates(updates).Error; err != nil {
			return err
		}

		var updated Memory
		if err := tx.Where("id = ?", id).First(&updated).Error; err != nil {
			return err
		}

		snapshotHash := computeMemorySnapshotHash(&updated)

		event := MemoryEventRecord{
			ID:           uuid.New().String(),
			MemoryID:     id,
			Version:      newVersion,
			EventType:    eventType,
			OperationID:  operationID,
			SnapshotHash: snapshotHash,
			EventReason:  req.EventReason,
			Key:          updated.Key,
			Value:        updated.Value,
			MemoryType:   updated.MemoryType,
			Importance:   updated.Importance,
			Confidence:   updated.Confidence,
			Source:       updated.Source,
			CharacterID:  updated.CharacterID,
			CreatedAt:    now,
		}

		return insertMemoryEvent(tx, event)
	})
}

func insertMemoryEvent(tx *gorm.DB, event MemoryEventRecord) error {
	return tx.Exec(
		`INSERT INTO memory_events (id, memory_id, event_type, key, value, memory_type, importance, source, character_id, created_at, version, operation_id, snapshot_hash, event_reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.MemoryID, event.EventType, event.Key, event.Value, event.MemoryType, event.Importance, event.Source, event.CharacterID, event.CreatedAt, event.Version, event.OperationID, event.SnapshotHash, event.EventReason,
	).Error
}

func computeMemorySnapshotHashCanonical(m *Memory) string {
	if m == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(m.Key))
	sb.WriteString("|")
	sb.WriteString(strings.TrimSpace(m.Value))
	sb.WriteString("|")
	sb.WriteString(strings.TrimSpace(m.MemoryType))
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf("%d", m.Importance))
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf("%d", m.Confidence))
	sb.WriteString("|")
	sb.WriteString(strings.TrimSpace(m.Scope))

	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:])
}
