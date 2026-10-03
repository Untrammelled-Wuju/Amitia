package devicemesh

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

const legacyAuthorityCutover = "business-to-kernel-v1"

type meshStateTable struct {
	name    string
	columns []string
}

var legacyMeshTables = []meshStateTable{
	{"kernel_device_mesh_bootstrap_tickets", []string{"ticket_id", "ticket_hash", "space_id", "device_id", "runtime_id", "platform", "status", "expires_at", "consumed_at", "created_at", "updated_at"}},
	{"kernel_device_runtime_credentials", []string{"credential_id", "credential_hash", "space_id", "device_id", "runtime_id", "status", "created_at", "expires_at", "last_used_at", "revoked_at", "revision"}},
	{"kernel_device_pairing_offers", []string{"offer_id", "offer_hash", "space_id", "created_by_device_id", "status", "expires_at", "consumed_at", "created_at"}},
}

func ImportLegacyState(ctx context.Context, legacy, canonical *sql.DB) error {
	if legacy == nil || canonical == nil {
		return errors.New("device mesh cutover: both databases are required")
	}
	if legacy == canonical {
		return nil
	}
	var completed int
	if err := canonical.QueryRowContext(ctx, `SELECT COUNT(1) FROM kernel_device_mesh_cutovers WHERE cutover_id=?`, legacyAuthorityCutover).Scan(&completed); err != nil {
		return err
	}
	if completed > 0 {
		return nil
	}
	source, err := legacy.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer source.Rollback()
	target, err := canonical.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer target.Rollback()
	result, err := target.ExecContext(ctx, `INSERT OR IGNORE INTO kernel_device_mesh_cutovers (cutover_id, completed_at) VALUES (?, ?)`, legacyAuthorityCutover, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if inserted == 0 {
		return nil
	}
	for _, table := range legacyMeshTables {
		if err := importMeshTable(ctx, source, target, table); err != nil {
			return err
		}
	}
	return target.Commit()
}

func importMeshTable(ctx context.Context, source, target *sql.Tx, table meshStateTable) error {
	var exists int
	if err := source.QueryRowContext(ctx, `SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name=?`, table.name).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	columns := strings.Join(table.columns, ",")
	rows, err := source.QueryContext(ctx, "SELECT "+columns+" FROM "+table.name)
	if err != nil {
		return err
	}
	defer rows.Close()
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(table.columns)), ",")
	for rows.Next() {
		values := make([]any, len(table.columns))
		pointers := make([]any, len(values))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return err
		}
		if _, err := target.ExecContext(ctx, "INSERT OR IGNORE INTO "+table.name+" ("+columns+") VALUES ("+placeholders+")", values...); err != nil {
			return err
		}
		stored := make([]any, len(values))
		for i := range stored {
			pointers[i] = &stored[i]
		}
		if err := target.QueryRowContext(ctx, "SELECT "+columns+" FROM "+table.name+" WHERE "+table.columns[0]+"=?", values[0]).Scan(pointers...); err != nil {
			return err
		}
		if !reflect.DeepEqual(values, stored) {
			return fmt.Errorf("device mesh cutover: conflicting state in %s", table.name)
		}
	}
	return rows.Err()
}
