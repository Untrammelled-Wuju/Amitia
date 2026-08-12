package migration

func BackupTablesUpgradeMigration() Migration {
	return Migration{
		Version:           "202608130001",
		Name:              "upgrade_backup_tables_for_dataportability",
		AcceptedChecksums: []string{"d9afae396f9c1a28c5cb8f8b4eaaff6f7970ebb1ec8e7b7a737e59596becd491"},
		Up: func(step *Step) error {
			step.AddColumn("backup_records", "purpose", "TEXT DEFAULT 'user'")
			step.AddColumn("backup_records", "format_version", "INTEGER DEFAULT 1")
			step.AddColumn("backup_records", "profile", "TEXT DEFAULT 'full'")
			step.AddColumn("backup_records", "scope", "TEXT DEFAULT 'all'")
			step.AddColumn("backup_records", "encrypted", "INTEGER DEFAULT 0")
			step.AddColumn("backup_records", "manifest_checksum", "TEXT DEFAULT ''")
			step.AddColumn("backup_records", "app_version", "TEXT DEFAULT ''")
			step.AddColumn("backup_records", "schema_fingerprint", "TEXT DEFAULT ''")
			step.Execute("CREATE INDEX IF NOT EXISTS idx_backup_records_purpose ON backup_records(purpose)")
			step.Execute("CREATE INDEX IF NOT EXISTS idx_backup_records_created ON backup_records(started_at)")

			step.AddColumn("backup_contents", "component_id", "TEXT DEFAULT ''")
			step.AddColumn("backup_contents", "kind", "TEXT DEFAULT ''")
			step.AddColumn("backup_contents", "logical_name", "TEXT DEFAULT ''")
			step.AddColumn("backup_contents", "size_bytes", "INTEGER DEFAULT 0")
			step.AddColumn("backup_contents", "item_count", "INTEGER DEFAULT 0")
			step.AddColumn("backup_contents", "required", "INTEGER DEFAULT 1")
			step.AddColumn("backup_contents", "source_of_truth", "INTEGER DEFAULT 0")
			step.AddColumn("backup_contents", "rebuildable", "INTEGER DEFAULT 0")

			step.Execute("CREATE INDEX IF NOT EXISTS idx_backup_contents_backup_id ON backup_contents(backup_id)")
			step.Execute("CREATE INDEX IF NOT EXISTS idx_backup_contents_component ON backup_contents(component_id)")
			return nil
		},
	}
}
