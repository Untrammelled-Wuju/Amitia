// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
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

type Repository struct {
	db *gorm.DB
}

func NewRepository(
	db *gorm.DB,
) *Repository {
	return &Repository{db: db}
}

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

	identity.UserID =
		strings.TrimSpace(
			identity.UserID,
		)
	identity.DeviceID =
		strings.TrimSpace(
			identity.DeviceID,
		)
	identity.DesktopInstanceID =
		strings.TrimSpace(
			identity.DesktopInstanceID,
		)

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
