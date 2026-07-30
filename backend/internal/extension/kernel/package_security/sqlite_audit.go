package package_security

import (
	"context"
	"database/sql"
	"time"
)

type SQLiteAuditWriter struct {
	db *sql.DB
}

func NewSQLiteAuditWriter(db *sql.DB) *SQLiteAuditWriter {
	return &SQLiteAuditWriter{db: db}
}

func (w *SQLiteAuditWriter) WriteAuditEvent(ctx context.Context, event ResourceAuditEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_, err := w.db.ExecContext(ctx, `INSERT INTO extension_package_security_audit (
		event_id, event_type, package_id, version, publisher_id, report_id, staging_id,
		snapshot_id, operation_id, details, success, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.EventID, event.EventType,
		event.PackageID, event.Version, event.PublisherID, event.ReportID, event.StagingID,
		event.SnapshotID, event.OperationID, event.Details, boolToInteger(event.Success),
		event.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func boolToInteger(value bool) int {
	if value {
		return 1
	}
	return 0
}

var _ AuditWriter = (*SQLiteAuditWriter)(nil)
