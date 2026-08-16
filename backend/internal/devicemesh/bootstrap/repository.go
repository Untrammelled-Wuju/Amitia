package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, ticket *BootstrapTicket) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO kernel_device_mesh_bootstrap_tickets (
			ticket_id, ticket_hash, user_id, device_id, runtime_id, platform,
			status, expires_at, consumed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ticket.TicketID, ticket.TicketHash,
		ticket.UserID.String(), ticket.DeviceID.String(), ticket.RuntimeID.String(),
		ticket.Platform.String(), ticket.Status,
		ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
		formatTimePtr(ticket.ConsumedAt),
		ticket.CreatedAt.UTC().Format(time.RFC3339Nano),
		ticket.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("bootstrap: create ticket: %w", err)
	}
	return nil
}

func (r *Repository) GetByHash(ctx context.Context, hash string) (*BootstrapTicket, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT ticket_id, ticket_hash, user_id, device_id, runtime_id, platform,
			status, expires_at, consumed_at, created_at, updated_at
		FROM kernel_device_mesh_bootstrap_tickets WHERE ticket_hash = ?`,
		hash,
	)
	return scanTicket(row)
}

func (r *Repository) GetByID(ctx context.Context, ticketID string) (*BootstrapTicket, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT ticket_id, ticket_hash, user_id, device_id, runtime_id, platform,
			status, expires_at, consumed_at, created_at, updated_at
		FROM kernel_device_mesh_bootstrap_tickets WHERE ticket_id = ?`,
		ticketID,
	)
	return scanTicket(row)
}

func (r *Repository) Consume(ctx context.Context, hash string, consumedAt time.Time) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("bootstrap: begin tx: %w", err)
	}
	defer tx.Rollback()

	ok, err := r.ConsumeTx(ctx, tx, hash, consumedAt)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("bootstrap: commit: %w", err)
	}
	return true, nil
}

func (r *Repository) ConsumeTx(ctx context.Context, tx *sql.Tx, hash string, consumedAt time.Time) (bool, error) {
	var ticketID string
	var status string
	var expiresStr string
	err := tx.QueryRowContext(ctx,
		`SELECT ticket_id, status, expires_at FROM kernel_device_mesh_bootstrap_tickets
		WHERE ticket_hash = ?`,
		hash,
	).Scan(&ticketID, &status, &expiresStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("bootstrap: query ticket: %w", err)
	}

	if status != string(TicketActive) {
		return false, nil
	}

	expiresAt, err := time.Parse(time.RFC3339Nano, expiresStr)
	if err != nil {
		return false, fmt.Errorf("bootstrap: parse expires: %w", err)
	}
	if consumedAt.After(expiresAt) {
		return false, nil
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE kernel_device_mesh_bootstrap_tickets
		SET status = ?, consumed_at = ?, updated_at = ?
		WHERE ticket_hash = ? AND status = ? AND expires_at > ?`,
		string(TicketConsumed),
		consumedAt.UTC().Format(time.RFC3339Nano),
		consumedAt.UTC().Format(time.RFC3339Nano),
		hash,
		string(TicketActive),
		consumedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return false, fmt.Errorf("bootstrap: consume ticket: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("bootstrap: rows affected: %w", err)
	}

	return rows == 1, nil
}

func (r *Repository) RevokeExpired(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE kernel_device_mesh_bootstrap_tickets
		SET status = ?, updated_at = ?
		WHERE status = ? AND expires_at <= ?`,
		string(TicketExpired),
		now.UTC().Format(time.RFC3339Nano),
		string(TicketActive),
		now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, fmt.Errorf("bootstrap: revoke expired: %w", err)
	}
	return res.RowsAffected()
}

type ticketScanner interface {
	Scan(dest ...any) error
}

func scanTicket(s ticketScanner) (*BootstrapTicket, error) {
	var t BootstrapTicket
	var userID, deviceID, runtimeID, platform, status string
	var expiresStr, createdStr, updatedStr string
	var consumedStr sql.NullString

	err := s.Scan(
		&t.TicketID, &t.TicketHash,
		&userID, &deviceID, &runtimeID, &platform,
		&status, &expiresStr, &consumedStr, &createdStr, &updatedStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("bootstrap: scan ticket: %w", err)
	}

	t.UserID = runtimeidentity.UserID(userID)
	t.DeviceID = runtimeidentity.DeviceID(deviceID)
	t.RuntimeID = runtimeidentity.RuntimeID(runtimeID)
	t.Platform = runtimeidentity.Platform(platform)
	t.Status = TicketStatus(status)

	t.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresStr)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: parse expires: %w", err)
	}
	t.CreatedAt, err = time.Parse(time.RFC3339Nano, createdStr)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: parse created: %w", err)
	}
	t.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedStr)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: parse updated: %w", err)
	}
	if consumedStr.Valid && consumedStr.String != "" {
		consumed, err := time.Parse(time.RFC3339Nano, consumedStr.String)
		if err != nil {
			return nil, fmt.Errorf("bootstrap: parse consumed: %w", err)
		}
		t.ConsumedAt = &consumed
	}

	return &t, nil
}

func formatTimePtr(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339Nano), Valid: true}
}
