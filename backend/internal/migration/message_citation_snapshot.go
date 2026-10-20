package migration

// MessageCitationSnapshotMigration stores a compact, immutable citation view on
// assistant messages. The snapshot is intentionally independent from Knowledge
// secondary indexes so historical answers remain explainable after reindex,
// rollback or source lifecycle changes.
func MessageCitationSnapshotMigration() Migration {
	return Migration{
		Version: "20260927001",
		Name:    "add_message_citation_snapshot",
		Up: func(s *Step) error {
			return s.AddColumn("messages", "citation_snapshot_json", "TEXT NOT NULL DEFAULT '[]'")
		},
	}
}
