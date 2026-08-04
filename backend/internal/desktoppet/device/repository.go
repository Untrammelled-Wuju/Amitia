// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package device

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/middleware/security"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Identity struct {
	ID                string
	UserID            string
	DeviceID          string
	DesktopInstanceID string
	Platform          string
	AppVersion        string
	Status            string
	FirstSeenAt       string
	LastSeenAt        string
	RevokedAt         string
}

func (Identity) TableName() string {
	return "desktop_pet_devices"
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
	now := time.Now().
		UTC().
		Format(time.RFC3339Nano)

	identity.FirstSeenAt = now
	identity.LastSeenAt = now
	identity.Status = "active"

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "device_id"},
			},
			DoUpdates: clause.Assignments(
				map[string]any{
					"desktop_instance_id":
						identity.DesktopInstanceID,
					"platform":
						identity.Platform,
					"app_version":
						identity.AppVersion,
					"status":
						"active",
					"last_seen_at":
						now,
					"revoked_at":
						"",
				},
			),
		}).
		Create(&identity).Error
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
