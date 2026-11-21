package devicemesh

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
	"github.com/u-ai/backend/internal/devicemesh/proof"
)

var schemaStmts = []string{
	meshaudit.Schema,
	coordination.PolicySchema,
	coordination.ResourceSchema,
	coordination.InboxSchema,
	coordination.OutboxSchema,
	coordination.ProviderSchema,
	coordination.RequestRegistrySchema,
	coordination.CapabilityGrantsSchema,
	coordination.CapabilityExposureSchema,
	coordination.CapabilityExposureStateSchema,
	coordination.CoreConsoleSchema,
	coordination.MemoryJobsSchema,
	coordination.ContinuityJobsSchema,
	coordination.RemoteAuthoritySchema,
	coordination.SourceAuthoritySchema,
	coordination.CancelledAuthoritySchema,
	coordination.DeliveryFailuresSchema,
	proof.IdentitySchema,
	proof.NonceSchema,
	pairing.ApprovalPolicySchema,
	pairing.ApprovalSchema,
	`CREATE TABLE IF NOT EXISTS kernel_devices (
		device_id TEXT PRIMARY KEY,
		space_id TEXT NOT NULL,
		platform TEXT NOT NULL DEFAULT '',
		label TEXT NOT NULL DEFAULT '',
		reported_name TEXT NOT NULL DEFAULT '',
		pending_name TEXT NOT NULL DEFAULT '',
		name_revision INTEGER NOT NULL DEFAULT 0,
		trust_state TEXT NOT NULL DEFAULT 'pending',
		created_at TEXT NOT NULL,
		trusted_at TEXT,
		last_seen_at TEXT NOT NULL,
		revision INTEGER NOT NULL DEFAULT 1
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_kernel_devices_user_device ON kernel_devices(space_id, device_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_devices_space_id ON kernel_devices(space_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_devices_trust_state ON kernel_devices(trust_state)`,

	`CREATE TABLE IF NOT EXISTS kernel_device_mesh_bootstrap_tickets (
		ticket_id TEXT PRIMARY KEY,
		ticket_hash TEXT NOT NULL,
		space_id TEXT NOT NULL,
		device_id TEXT NOT NULL,
		runtime_id TEXT NOT NULL,
		platform TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'active',
		expires_at TEXT NOT NULL,
		consumed_at TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_device_mesh_bt_hash ON kernel_device_mesh_bootstrap_tickets(ticket_hash)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_device_mesh_bt_user ON kernel_device_mesh_bootstrap_tickets(space_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_device_mesh_bt_device ON kernel_device_mesh_bootstrap_tickets(device_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_device_mesh_bt_status ON kernel_device_mesh_bootstrap_tickets(status)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_device_mesh_bt_expires ON kernel_device_mesh_bootstrap_tickets(expires_at)`,

	`CREATE TABLE IF NOT EXISTS kernel_device_runtime_credentials (
		credential_id TEXT PRIMARY KEY,
		credential_hash TEXT NOT NULL,
		space_id TEXT NOT NULL,
		device_id TEXT NOT NULL,
		runtime_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		last_used_at TEXT NOT NULL,
		revoked_at TEXT,
		revision INTEGER NOT NULL DEFAULT 1
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_device_rc_hash ON kernel_device_runtime_credentials(credential_hash)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_device_rc_space ON kernel_device_runtime_credentials(space_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_device_rc_device ON kernel_device_runtime_credentials(device_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_device_rc_runtime ON kernel_device_runtime_credentials(space_id, device_id, runtime_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_device_rc_status ON kernel_device_runtime_credentials(status)`,
}

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	for _, stmt := range schemaStmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("devicemesh: ensure schema: %w", err)
		}
	}
	if err := ensureDeviceNameColumns(ctx, db); err != nil {
		return err
	}
	return ensureCoordinationRoleColumns(ctx, db)
}

var coordinationRoleColumnStatements = []string{
	`ALTER TABLE kernel_device_coordination ADD COLUMN center_role TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE kernel_device_coordination ADD COLUMN local_role TEXT NOT NULL DEFAULT ''`,
}

func ensureCoordinationRoleColumns(ctx context.Context, db *sql.DB) error {
	for _, stmt := range coordinationRoleColumnStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
				continue
			}
			return fmt.Errorf("devicemesh: ensure coordination role columns: %w", err)
		}
	}
	return nil
}

var deviceNameColumnStatements = []string{
	`ALTER TABLE kernel_devices ADD COLUMN reported_name TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE kernel_devices ADD COLUMN pending_name TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE kernel_devices ADD COLUMN name_revision INTEGER NOT NULL DEFAULT 0`,
}

func ensureDeviceNameColumns(ctx context.Context, db *sql.DB) error {
	for _, stmt := range deviceNameColumnStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
				continue
			}
			return fmt.Errorf("devicemesh: ensure device name columns: %w", err)
		}
	}
	return nil
}
