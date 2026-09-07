package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LegacyMetadataTables lists opaque compatibility metadata tables from pre-canonical
// builds. No production reader consumes these tables after the canonical cutover.
// Their rows are therefore preserved losslessly in the canonical archive before the
// legacy write surface is cleared.
var LegacyMetadataTables = []string{
	"legacy_mcp_metadata",
	"legacy_plugin_metadata",
	"legacy_skill_metadata",
	"legacy_memory_import_state",
	"legacy_backup_metadata",
	"legacy_tool_binding_aliases",
}

// LegacyMetadataArchive keeps the original legacy row payload so a cutover never
// discards data whose historical schema is not available to the current binary.
type LegacyMetadataArchive struct {
	ID          string `gorm:"column:id;primaryKey"`
	OperationID string `gorm:"column:operation_id;index"`
	SourceTable string `gorm:"column:source_table;index"`
	PayloadJSON string `gorm:"column:payload_json;type:text"`
	CreatedAt   string `gorm:"column:created_at"`
}

func (LegacyMetadataArchive) TableName() string { return "cutover_legacy_metadata_archive" }

// ArchiveLegacyMetadata performs the only safe canonical migration possible for
// the opaque legacy metadata tables: copy every row into a canonical, versioned
// archive inside the same database transaction, verify the copy count, and only
// then clear the legacy table. Canonical services already own their live state;
// this archive is retained for rollback/recovery and future semantic import tools.
func ArchiveLegacyMetadata(ctx context.Context, db *gorm.DB, operationID string) (map[string]int64, error) {
	if db == nil {
		return nil, fmt.Errorf("legacy metadata migration: database not available")
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return nil, fmt.Errorf("legacy metadata migration: operation ID is required")
	}
	if err := db.WithContext(ctx).AutoMigrate(&LegacyMetadataArchive{}); err != nil {
		return nil, fmt.Errorf("legacy metadata migration: create archive table: %w", err)
	}

	migrated := make(map[string]int64)
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, table := range LegacyMetadataTables {
			if !tx.Migrator().HasTable(table) {
				continue
			}
			var count int64
			if err := tx.Table(table).Count(&count).Error; err != nil {
				return fmt.Errorf("legacy metadata migration: count %s: %w", table, err)
			}
			if count == 0 {
				continue
			}

			var rows []map[string]interface{}
			if err := tx.Table(table).Find(&rows).Error; err != nil {
				return fmt.Errorf("legacy metadata migration: read %s: %w", table, err)
			}
			if int64(len(rows)) != count {
				return fmt.Errorf("legacy metadata migration: row-count mismatch for %s: read=%d counted=%d", table, len(rows), count)
			}

			archives := make([]LegacyMetadataArchive, 0, len(rows))
			now := time.Now().UTC().Format(time.RFC3339Nano)
			for _, row := range rows {
				payload, err := json.Marshal(row)
				if err != nil {
					return fmt.Errorf("legacy metadata migration: encode %s row: %w", table, err)
				}
				archives = append(archives, LegacyMetadataArchive{
					ID:          uuid.NewString(),
					OperationID: operationID,
					SourceTable: table,
					PayloadJSON: string(payload),
					CreatedAt:   now,
				})
			}
			if len(archives) > 0 {
				if err := tx.Create(&archives).Error; err != nil {
					return fmt.Errorf("legacy metadata migration: archive %s: %w", table, err)
				}
			}
			// table comes from the fixed whitelist above, not external input.
			if err := tx.Exec("DELETE FROM " + table).Error; err != nil {
				return fmt.Errorf("legacy metadata migration: clear %s: %w", table, err)
			}
			var remaining int64
			if err := tx.Table(table).Count(&remaining).Error; err != nil {
				return fmt.Errorf("legacy metadata migration: verify %s: %w", table, err)
			}
			if remaining != 0 {
				return fmt.Errorf("legacy metadata migration: %s still contains %d rows after archive", table, remaining)
			}
			migrated[table] = count
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return migrated, nil
}
