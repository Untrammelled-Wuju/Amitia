// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type BootstrapTicket struct {
	ID                string `gorm:"column:id;primaryKey" json:"id"`
	TicketHash        string `gorm:"column:ticket_hash;uniqueIndex" json:"-"`
	UserID            string `gorm:"column:user_id;index:idx_bt_user" json:"userId"`
	DeviceID          string `gorm:"column:device_id;index:idx_bt_device" json:"deviceId"`
	RuntimeID         string `gorm:"column:runtime_id;index:idx_bt_runtime" json:"runtimeId"`
	Status            string `gorm:"column:status;index:idx_bt_status" json:"status"`
	ExpiresAt         string `gorm:"column:expires_at" json:"expiresAt"`
	ConsumedAt        string `gorm:"column:consumed_at" json:"consumedAt"`
	ConsumedByRuntime string `gorm:"column:consumed_by_runtime" json:"consumedByRuntime"`
	Reason            string `gorm:"column:reason" json:"reason"`
	CreatedAt         string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         string `gorm:"column:updated_at" json:"updatedAt"`
}

func (BootstrapTicket) TableName() string { return "desktop_pet_runtime_bootstrap_tickets" }

const (
	BootstrapTicketStatusActive   = "active"
	BootstrapTicketStatusConsumed = "consumed"
	BootstrapTicketStatusExpired  = "expired"
	BootstrapTicketStatusRevoked  = "revoked"
)

var ErrTicketNotFound = errors.New("runtime bootstrap ticket not found")
var ErrTicketExpired = errors.New("runtime bootstrap ticket expired")
var ErrTicketConsumed = errors.New("runtime bootstrap ticket already consumed")
var ErrTicketRevoked = errors.New("runtime bootstrap ticket revoked")

type BootstrapTicketRepository struct {
	db *gorm.DB
}

func NewBootstrapTicketRepository(db *gorm.DB) *BootstrapTicketRepository {
	return &BootstrapTicketRepository{db: db}
}

func (r *BootstrapTicketRepository) ReadinessCheck(ctx context.Context) error {
	if r.db == nil {
		return errors.New("database is nil")
	}
	sqlDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("get underlying db: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

func generateTicketID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("ticket ID generation failed: %w", err)
	}
	return "rbt_" + hex.EncodeToString(b), nil
}

func hashTicket(ticket string) string {
	h := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(h[:])
}

func (r *BootstrapTicketRepository) Create(ctx context.Context, userID, deviceID, runtimeID string, ttl time.Duration) (rawTicket string, ticket *BootstrapTicket, err error) {
	rawTicket, err = generateRawTicket()
	if err != nil {
		return "", nil, err
	}
	ticketID, err := generateTicketID()
	if err != nil {
		return "", nil, err
	}
	now := time.Now()
	ticket = &BootstrapTicket{
		ID:         ticketID,
		TicketHash: hashTicket(rawTicket),
		UserID:     userID,
		DeviceID:   deviceID,
		RuntimeID:  runtimeID,
		Status:     BootstrapTicketStatusActive,
		ExpiresAt:  now.Add(ttl).UTC().Format(time.RFC3339),
		CreatedAt:  now.UTC().Format(time.RFC3339),
		UpdatedAt:  now.UTC().Format(time.RFC3339),
	}
	if err := r.db.WithContext(ctx).Create(ticket).Error; err != nil {
		return "", nil, err
	}
	return rawTicket, ticket, nil
}

func (r *BootstrapTicketRepository) ConsumeWithValidation(ctx context.Context, rawTicket, runtimeID, deviceID string) (*BootstrapTicket, error) {
	if rawTicket == "" {
		return nil, ErrTicketNotFound
	}
	hash := hashTicket(rawTicket)

	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback().Error
			panic(recovered)
		}
	}()

	var ticket BootstrapTicket
	if err := tx.Where("ticket_hash = ?", hash).Take(&ticket).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}

	switch ticket.Status {
	case BootstrapTicketStatusConsumed:
		tx.Rollback()
		return nil, ErrTicketConsumed
	case BootstrapTicketStatusRevoked:
		tx.Rollback()
		return nil, ErrTicketRevoked
	case BootstrapTicketStatusExpired:
		tx.Rollback()
		return nil, ErrTicketExpired
	}

	if ticket.ExpiresAt == "" {
		tx.Rollback()
		return nil, ErrTicketExpired
	}
	expires, err := time.Parse(time.RFC3339, ticket.ExpiresAt)
	if err != nil {
		if markErr := markTicketExpiredTx(tx, ticket.ID, "expires_at_corrupted"); markErr != nil {
			_ = tx.Rollback().Error
			return nil, markErr
		}
		if commitErr := tx.Commit().Error; commitErr != nil {
			return nil, commitErr
		}
		return nil, ErrTicketExpired
	}
	if time.Now().After(expires) {
		if markErr := markTicketExpiredTx(tx, ticket.ID, "expired"); markErr != nil {
			_ = tx.Rollback().Error
			return nil, markErr
		}
		if commitErr := tx.Commit().Error; commitErr != nil {
			return nil, commitErr
		}
		return nil, ErrTicketExpired
	}

	if deviceID != "" && ticket.DeviceID != "" && ticket.DeviceID != deviceID {
		tx.Rollback()
		return nil, ErrTicketRevoked
	}

	if ticket.RuntimeID != "" && ticket.RuntimeID != runtimeID {
		tx.Rollback()
		return nil, ErrTicketRevoked
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result := tx.Model(&BootstrapTicket{}).
		Where("id = ? AND status = ? AND expires_at > ?", ticket.ID, BootstrapTicketStatusActive, now).
		Updates(map[string]interface{}{
			"status":              BootstrapTicketStatusConsumed,
			"consumed_at":         now,
			"consumed_by_runtime": runtimeID,
			"updated_at":          now,
		})

	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		tx.Rollback()
		return nil, ErrTicketConsumed
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	ticket.Status = BootstrapTicketStatusConsumed
	ticket.ConsumedAt = now
	ticket.ConsumedByRuntime = runtimeID
	return &ticket, nil
}

func markTicketExpiredTx(tx *gorm.DB, ticketID, reason string) error {
	result := tx.Model(&BootstrapTicket{}).Where("id = ? AND status = ?", ticketID, BootstrapTicketStatusActive).Updates(map[string]interface{}{
		"status":     BootstrapTicketStatusExpired,
		"reason":     reason,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("runtime bootstrap ticket expiration CAS affected %d rows", result.RowsAffected)
	}
	return nil
}

func generateRawTicket() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "amt_rt_" + hex.EncodeToString(b), nil
}

func (r *BootstrapTicketRepository) Consume(ctx context.Context, rawTicket, runtimeID string) (*BootstrapTicket, error) {
	if rawTicket == "" {
		return nil, ErrTicketNotFound
	}
	hash := hashTicket(rawTicket)

	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback().Error
			panic(recovered)
		}
	}()

	var ticket BootstrapTicket
	if err := tx.Where("ticket_hash = ?", hash).Take(&ticket).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}

	switch ticket.Status {
	case BootstrapTicketStatusConsumed:
		tx.Rollback()
		return nil, ErrTicketConsumed
	case BootstrapTicketStatusRevoked:
		tx.Rollback()
		return nil, ErrTicketRevoked
	case BootstrapTicketStatusExpired:
		tx.Rollback()
		return nil, ErrTicketExpired
	}

	if ticket.ExpiresAt == "" {
		tx.Rollback()
		return nil, ErrTicketExpired
	}
	expires, err := time.Parse(time.RFC3339, ticket.ExpiresAt)
	if err != nil {
		if markErr := markTicketExpiredTx(tx, ticket.ID, "expires_at_corrupted"); markErr != nil {
			_ = tx.Rollback().Error
			return nil, markErr
		}
		if commitErr := tx.Commit().Error; commitErr != nil {
			return nil, commitErr
		}
		return nil, ErrTicketExpired
	}
	if time.Now().After(expires) {
		if markErr := markTicketExpiredTx(tx, ticket.ID, "expired"); markErr != nil {
			_ = tx.Rollback().Error
			return nil, markErr
		}
		if commitErr := tx.Commit().Error; commitErr != nil {
			return nil, commitErr
		}
		return nil, ErrTicketExpired
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result := tx.Model(&BootstrapTicket{}).
		Where("id = ? AND status = ? AND expires_at > ?", ticket.ID, BootstrapTicketStatusActive, now).
		Updates(map[string]interface{}{
			"status":              BootstrapTicketStatusConsumed,
			"consumed_at":         now,
			"consumed_by_runtime": runtimeID,
			"updated_at":          now,
		})

	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		tx.Rollback()
		return nil, ErrTicketConsumed
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	ticket.Status = BootstrapTicketStatusConsumed
	ticket.ConsumedAt = now
	ticket.ConsumedByRuntime = runtimeID
	return &ticket, nil
}

func (r *BootstrapTicketRepository) UpdateStatus(ctx context.Context, ticketID, status, reason string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if reason != "" {
		updates["reason"] = reason
	}
	return r.db.WithContext(ctx).Model(&BootstrapTicket{}).Where("id = ?", ticketID).Updates(updates).Error
}

func (r *BootstrapTicketRepository) ExpireOldTickets(ctx context.Context) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result := r.db.WithContext(ctx).
		Model(&BootstrapTicket{}).
		Where("status = ? AND expires_at < ?", BootstrapTicketStatusActive, now).
		Updates(map[string]interface{}{
			"status":     BootstrapTicketStatusExpired,
			"reason":     "auto-expired",
			"updated_at": now,
		})
	return result.RowsAffected, result.Error
}

func (r *BootstrapTicketRepository) GetByID(ctx context.Context, ticketID string) (*BootstrapTicket, error) {
	var ticket BootstrapTicket
	if err := r.db.WithContext(ctx).Where("id = ?", ticketID).Take(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	return &ticket, nil
}

func (r *BootstrapTicketRepository) RevokeUserTickets(ctx context.Context, userID string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result := r.db.WithContext(ctx).
		Model(&BootstrapTicket{}).
		Where("user_id = ? AND status = ?", userID, BootstrapTicketStatusActive).
		Updates(map[string]interface{}{
			"status":     BootstrapTicketStatusRevoked,
			"reason":     "user-revoked",
			"updated_at": now,
		})
	return result.RowsAffected, result.Error
}

func (r *BootstrapTicketRepository) RevokeDeviceTickets(ctx context.Context, userID, deviceID string) (int64, error) {
	if userID == "" || deviceID == "" {
		return 0, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result := r.db.WithContext(ctx).
		Model(&BootstrapTicket{}).
		Where("user_id = ? AND device_id = ? AND status = ?", userID, deviceID, BootstrapTicketStatusActive).
		Updates(map[string]interface{}{
			"status":     BootstrapTicketStatusRevoked,
			"reason":     "device-revoked",
			"updated_at": now,
		})
	return result.RowsAffected, result.Error
}

func (r *BootstrapTicketRepository) CleanupExpired(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).UTC().Format(time.RFC3339)
	result := r.db.WithContext(ctx).
		Where("status IN ? AND updated_at < ?", []string{BootstrapTicketStatusExpired, BootstrapTicketStatusConsumed, BootstrapTicketStatusRevoked}, cutoff).
		Delete(&BootstrapTicket{})
	return result.RowsAffected, result.Error
}

func (r *BootstrapTicketRepository) GetActiveByDevice(ctx context.Context, userID, deviceID string) (*BootstrapTicket, error) {
	var ticket BootstrapTicket
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND device_id = ? AND status = ?", userID, deviceID, BootstrapTicketStatusActive).
		Order("created_at DESC").
		Take(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	return &ticket, nil
}

func (r *BootstrapTicketRepository) Delete(ctx context.Context, ticketID string) error {
	return r.db.WithContext(ctx).Delete(&BootstrapTicket{}, "id = ?", ticketID).Error
}
