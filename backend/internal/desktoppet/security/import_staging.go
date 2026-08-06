// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type StagingStatus string

const (
	StagingStatusUploading   StagingStatus = "uploading"
	StagingStatusQuarantined StagingStatus = "quarantined"
	StagingStatusInspecting  StagingStatus = "inspecting"
	StagingStatusReady       StagingStatus = "ready"
	StagingStatusConsuming   StagingStatus = "consuming"
	StagingStatusConsumed    StagingStatus = "consumed"
	StagingStatusFailed      StagingStatus = "failed"
	StagingStatusRejected    StagingStatus = "rejected"
	StagingStatusExpired     StagingStatus = "expired"
)

type ImportStaging struct {
	ID                   string        `gorm:"column:id;primaryKey" json:"id"`
	OwnerUserID          string        `gorm:"column:owner_user_id" json:"ownerUserId"`
	SourceFilename       string        `gorm:"column:source_filename" json:"sourceFilename"`
	SourceType           string        `gorm:"column:source_type" json:"sourceType"`
	SourceContentHash    string        `gorm:"column:source_content_hash" json:"sourceContentHash"`
	SourceBytes          int64         `gorm:"column:source_bytes" json:"sourceBytes"`
	RootKind             string        `gorm:"column:root_kind" json:"rootKind"`
	StorageKey           string        `gorm:"column:storage_key" json:"storageKey"`
	Status               StagingStatus `gorm:"column:status" json:"status"`
	QuarantinePath       string        `gorm:"column:quarantine_path" json:"quarantinePath"`
	InventoryHash        string        `gorm:"column:inventory_hash" json:"inventoryHash"`
	InventoryJSON        string        `gorm:"column:inventory_json" json:"inventoryJson"`
	StateRevision        int64         `gorm:"column:state_revision" json:"stateRevision"`
	CreatedAt            string        `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt            string        `gorm:"column:updated_at" json:"updatedAt"`
	ExpiresAt            string        `gorm:"column:expires_at" json:"expiresAt"`
	ConsumptionStartedAt string        `gorm:"column:consumption_started_at" json:"consumptionStartedAt"`
	ConsumedAt           string        `gorm:"column:consumed_at" json:"consumedAt,omitempty"`
	FailedAt             string        `gorm:"column:failed_at" json:"failedAt,omitempty"`
	FailureReason        string        `gorm:"column:failure_reason" json:"failureReason,omitempty"`
	RejectedReason       string        `gorm:"column:rejected_reason" json:"rejectedReason"`
	CorrelationID        string        `gorm:"column:correlation_id" json:"correlationId,omitempty"`
}

func NewImportStaging(ownerUserID, sourceFilename, sourceType string) (*ImportStaging, error) {
	id, err := generateStagingID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate staging ID: %w", err)
	}
	now := time.Now().UTC()
	expires := now.Add(2 * time.Hour)
	return &ImportStaging{
		ID:             id,
		OwnerUserID:    ownerUserID,
		SourceFilename: sourceFilename,
		SourceType:     sourceType,
		Status:         StagingStatusUploading,
		CreatedAt:      now.Format(time.RFC3339Nano),
		ExpiresAt:      expires.Format(time.RFC3339Nano),
	}, nil
}

func generateStagingID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "stg_" + hex.EncodeToString(b), nil
}

func (s *ImportStaging) MarkQuarantined(storageKey string) error {
	if s.Status != StagingStatusUploading {
		return fmt.Errorf("invalid status transition from %s to %s", s.Status, StagingStatusQuarantined)
	}
	s.Status = StagingStatusQuarantined
	s.StorageKey = storageKey
	return nil
}

func (s *ImportStaging) MarkInspecting() error {
	if s.Status != StagingStatusQuarantined {
		return fmt.Errorf("invalid status transition from %s to %s", s.Status, StagingStatusInspecting)
	}
	s.Status = StagingStatusInspecting
	return nil
}

func (s *ImportStaging) MarkReady(inventoryHash string) error {
	if s.Status != StagingStatusInspecting {
		return fmt.Errorf("invalid status transition from %s to %s", s.Status, StagingStatusReady)
	}
	s.Status = StagingStatusReady
	s.InventoryHash = inventoryHash
	return nil
}

func (s *ImportStaging) BeginConsumption() error {
	if s.Status != StagingStatusReady {
		return fmt.Errorf("invalid status transition from %s to %s", s.Status, StagingStatusConsuming)
	}
	if s.IsExpired() {
		s.Status = StagingStatusExpired
		return ErrStagingExpired
	}
	s.Status = StagingStatusConsuming
	s.ConsumedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return nil
}

func (s *ImportStaging) CompleteConsumption() error {
	if s.Status != StagingStatusConsuming {
		return fmt.Errorf("invalid status transition from %s to %s", s.Status, StagingStatusConsumed)
	}
	s.Status = StagingStatusConsumed
	return nil
}

func (ImportStaging) TableName() string { return "desktop_pet_import_stagings" }

func (s *ImportStaging) MarkRejected() {
	s.Status = StagingStatusRejected
}

func (s *ImportStaging) IsExpired() bool {
	if s.ExpiresAt == "" {
		return true
	}
	expires, err := time.Parse(time.RFC3339Nano, s.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().After(expires)
}

func (s *ImportStaging) CanAccess(userID string) error {
	if s.OwnerUserID != userID {
		return ErrStagingCrossUser
	}
	if s.Status == StagingStatusConsumed {
		return ErrStagingConsumed
	}
	if s.IsExpired() {
		return ErrStagingExpired
	}
	return nil
}

func SanitizeUploadName(name string) (string, error) {
	if name == "" {
		return "", errors.New("empty upload name")
	}
	if len(name) > 255 {
		return "", errors.New("upload name too long")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("empty upload name")
	}
	if name == "." || name == ".." {
		return "", errors.New("invalid upload name")
	}
	for _, c := range name {
		if c == '/' || c == '\\' || c == '\x00' || c == ':' || c == '*' || c == '?' || c == '"' || c == '<' || c == '>' || c == '|' {
			return "", fmt.Errorf("invalid character in upload name: %U", c)
		}
		if c < 0x20 || c == 0x7F {
			return "", fmt.Errorf("invalid character in upload name: %U", c)
		}
	}
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return "", errors.New("invalid upload name format")
	}
	base := name
	if idx := strings.Index(base, "."); idx > 0 {
		base = base[:idx]
	}
	if isReservedDeviceName(base) {
		return "", errors.New("reserved device name")
	}
	return name, nil
}

func isReservedDeviceName(name string) bool {
	reserved := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
	upper := strings.ToUpper(name)
	for _, r := range reserved {
		if upper == r {
			return true
		}
	}
	return false
}
