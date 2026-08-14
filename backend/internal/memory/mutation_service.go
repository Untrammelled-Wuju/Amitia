// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrMemoryVersionConflict = errors.New("memory version conflict")

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
}

type MemoryDerivationInput struct {
	InputMemoryID     string
	InputVersion      int
	InputSnapshotHash string
	DerivationKind    string
	Ordinal           int
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

type memorySemanticSnapshot struct {
	CharacterID string `json:"characterId"`

	MemoryType string `json:"memoryType"`
	Source     string `json:"source"`
	Scope      string `json:"scope"`

	Key   string `json:"key"`
	Value string `json:"value"`

	Importance int `json:"importance"`
	Confidence int `json:"confidence"`

	ExpiresAt *string `json:"expiresAt,omitempty"`

	EntityID   string `json:"entityId,omitempty"`
	EntityType string `json:"entityType,omitempty"`

	SourceMsgID  string `json:"sourceMsgId,omitempty"`
	SourceConvID string `json:"sourceConvId,omitempty"`

	VerifiedStatus string `json:"verifiedStatus"`

	SensitivityLevel string `json:"sensitivityLevel"`

	AllowProactiveMention bool `json:"allowProactiveMention"`

	RequiresConfirmation bool `json:"requiresConfirmation"`
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
			return ErrMemoryVersionConflict
		}

		newVersion := current.Version + 1

		updates := make(map[string]interface{})
		for k, v := range req.Updates {
			updates[k] = v
		}
		updates["version"] = newVersion

		res := tx.Model(&Memory{}).Where("id = ? AND version = ?", id, current.Version).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrMemoryVersionConflict
		}

		var updated Memory
		if err := tx.Where("id = ?", id).First(&updated).Error; err != nil {
			return err
		}

		snapshotHash := computeMemorySnapshotHashCanonical(&updated)

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

		res := tx.Model(&Memory{}).Where("id = ? AND version = ?", id, current.Version).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrMemoryVersionConflict
		}

		var updated Memory
		if err := tx.Where("id = ?", id).First(&updated).Error; err != nil {
			return err
		}

		snapshotHash := computeMemorySnapshotHashCanonical(&updated)

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
	snapshot := memorySemanticSnapshot{
		CharacterID:           m.CharacterID,
		MemoryType:            m.MemoryType,
		Source:                m.Source,
		Scope:                 m.Scope,
		Key:                   m.Key,
		Value:                 m.Value,
		Importance:            m.Importance,
		Confidence:            m.Confidence,
		ExpiresAt:             m.ExpiresAt,
		EntityID:              m.EntityID,
		EntityType:            m.EntityType,
		SourceMsgID:           m.SourceMsgID,
		SourceConvID:          m.SourceConvID,
		VerifiedStatus:        m.VerifiedStatus,
		SensitivityLevel:      m.SensitivityLevel,
		AllowProactiveMention: m.AllowProactiveMention,
		RequiresConfirmation:  m.RequiresConfirmation,
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}
