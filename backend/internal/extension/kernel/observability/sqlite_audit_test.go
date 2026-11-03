package observability

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
)

func TestSQLiteListAuditEventsFiltersApprovalAndReadsMetadata(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE kernel_observability_audit_events (
			audit_id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL DEFAULT '',
			operation_id TEXT NOT NULL DEFAULT '',
			invocation_id TEXT NOT NULL DEFAULT '',
			actor_type TEXT NOT NULL DEFAULT '',
			actor_id TEXT NOT NULL DEFAULT '',
			subject_type TEXT NOT NULL DEFAULT '',
			subject_id TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL DEFAULT '',
			decision TEXT NOT NULL DEFAULT '',
			risk_level TEXT NOT NULL DEFAULT '',
			scope_summary TEXT NOT NULL DEFAULT '',
			permission_ids_json TEXT NOT NULL DEFAULT '[]',
			target_type TEXT NOT NULL DEFAULT '',
			target_id TEXT NOT NULL DEFAULT '',
			result TEXT NOT NULL DEFAULT '',
			error_code TEXT NOT NULL DEFAULT '',
			grant_id TEXT NOT NULL DEFAULT '',
			approval_id TEXT NOT NULL DEFAULT '',
			snapshot_id TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			metadata_json TEXT NOT NULL DEFAULT '{}'
		)
	`); err != nil {
		t.Fatal(err)
	}
	store := NewSQLiteStorage(db)
	now := time.Now().UTC()
	if err := store.SaveAuditEvent(t.Context(), AuditEvent{
		AuditID: "audit-one", ActorType: ActorUser, ActorID: "device-a", SubjectType: SubjectTool,
		SubjectID: "terminal.write", Action: "approval.decide", Decision: "approved", Result: "resolved",
		ApprovalID: "approval-one", PermissionIDs: []string{"tool.write"}, CreatedAt: now,
		Metadata: map[string]any{"principalType": "local_ui", "authMethod": "desktop_session"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAuditEvent(t.Context(), AuditEvent{
		AuditID: "audit-two", ActorType: ActorUser, ActorID: "device-b", Action: "approval.decide",
		ApprovalID: "approval-two", CreatedAt: now.Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	events, _, err := store.ListAuditEvents(t.Context(), AuditFilter{ApprovalID: "approval-one", ListOptions: ListOptions{Limit: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ActorID != "device-a" || len(events[0].PermissionIDs) != 1 || events[0].Metadata["authMethod"] != "desktop_session" {
		t.Fatalf("unexpected audit events: %+v", events)
	}
}
