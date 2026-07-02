package migration

func BackupMigration() Migration {
	return Migration{
		Version: "202607010001",
		Name:    "create_backup_records_table",
		Up: func(step *Step) error {
			step.CreateTable("CREATE TABLE IF NOT EXISTS backup_records (id TEXT PRIMARY KEY, backup_path TEXT NOT NULL DEFAULT '', backup_size INTEGER DEFAULT 0, checksum TEXT DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', started_at TEXT DEFAULT '', finished_at TEXT DEFAULT '', error_message TEXT DEFAULT '')")
			step.CreateTable("CREATE TABLE IF NOT EXISTS backup_contents (id TEXT PRIMARY KEY, backup_id TEXT NOT NULL DEFAULT '', table_name TEXT NOT NULL DEFAULT '', row_count INTEGER DEFAULT 0, checksum TEXT DEFAULT '')")
			return nil
		},
	}
}
