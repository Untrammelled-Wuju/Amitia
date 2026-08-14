// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
// Package device provides the DesktopPet domain-specific device ownership/binding projection.
//
// NOTE: This repository is a COMPATIBILITY PROJECTION retained for DesktopPet installation
// and security ownership checks. It is NOT the authoritative device/runtime presence registry.
// Current device/runtime presence is owned by the shared kernel (host_registry.Registry).
//
// Callers must NOT use this repository for:
//   - Provider routing decisions
//   - Device selection for runtime operations
//   - Runtime session selection
//
// DesktopPet device ownership facts should be derived from the Extension Kernel when possible.
package device

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Identity represents a DesktopPet device ownership record.
// It is a domain-specific projection used for compatibility with DesktopPet installation
// and ownership verification. It does not represent current online/offline presence.
//
// For current device/runtime presence, use host_registry.Registry instead.
type Identity struct {
	ID                string
	UserID            runtimeidentity.UserID
	DeviceID          runtimeidentity.DeviceID
	DesktopInstanceID string
	Platform          runtimeidentity.Platform
	AppVersion        string
	Status            string
	FirstSeenAt       string
	LastSeenAt        string
	RevokedAt         string
}

func (Identity) TableName() string {
	return "desktop_pet_devices"
}

func (i Identity) RuntimeIdentity() runtimeidentity.Identity {
	return runtimeidentity.Identity{
		UserID:   i.UserID,
		DeviceID: i.DeviceID,
	}
}

// Repository provides access to DesktopPet device ownership records.
// This is a domain-specific repository for DesktopPet compatibility only.
// The authoritative device/runtime presence registry is host_registry.Registry.
type Repository struct {
	db *gorm.DB
}

func NewRepository(
	db *gorm.DB,
) *Repository {
	return &Repository{db: db}
}

// RegisterOrTouch updates the DesktopPet device ownership record for compatibility.
// This is a domain projection operation only. It does NOT update the shared kernel
// device/runtime presence registry (host_registry.Registry).
func (
	r *Repository,
) RegisterOrTouch(
	ctx context.Context,
	identity Identity,
) error {
	if r == nil || r.db == nil {
		return errors.New(
			"device repository is unavailable",
		)
	}

	identity.UserID = runtimeidentity.ParseUserID(string(identity.UserID))
	identity.DeviceID = runtimeidentity.ParseDeviceID(string(identity.DeviceID))
	identity.DesktopInstanceID = strings.TrimSpace(string(identity.DesktopInstanceID))

	if identity.UserID == "" ||
		identity.DeviceID == "" ||
		identity.DesktopInstanceID == "" {
		return errors.New(
			"device identity fields are required",
		)
	}

	if identity.ID == "" {
		identity.ID =
			"device_identity_" +
				uuid.NewString()
	}

	now := time.Now().
		UTC().
		Format(
			time.RFC3339Nano,
		)

	identity.FirstSeenAt = now
	identity.LastSeenAt = now
	identity.Status = "active"

	result := r.db.WithContext(ctx).
		Clauses(
			clause.OnConflict{
				Columns: []clause.Column{
					{
						Name: "user_id",
					},
					{
						Name: "device_id",
					},
				},
				DoUpdates: clause.Assignments(
					map[string]any{
						"desktop_instance_id": identity.DesktopInstanceID,
						"platform":            identity.Platform,
						"app_version":         identity.AppVersion,
						"status":              "active",
						"last_seen_at":        now,
						"revoked_at":          "",
					},
				),
			},
		).
		Create(&identity)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

// RequireOwned verifies DesktopPet domain device ownership for security compatibility.
// This check does NOT indicate whether the device is currently online or whether
// a runtime session is available. Use host_registry.Registry for presence checks
// and deviceruntime.Service for runtime session status.
func (
	r *Repository,
) RequireOwned(
	ctx context.Context,
	userID string,
	deviceID string,
) error {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&Identity{}).
		Where(
			"user_id = ? AND device_id = ? "+
				"AND status = 'active' "+
				"AND revoked_at = ''",
			userID,
			deviceID,
		).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return security.ErrNotFound
	}
	return nil
}

var _ = security.ErrNotFound
