package migration

func TombstoneRebuildMigration() Migration {
 	return Migration{
 		Version: "202607010005",
		Name:    "create_tombstone_rebuild_tracker_table",
 		Up: func(step *Step) error {
			step.CreateTable("CREATE TABLE IF NOT EXISTS tombstone_rebuild_tracker (id TEXT PRIMARY KEY, target_id TEXT NOT NULL DEFAULT '', target_type TEXT NOT NULL DEFAULT '', rebuild_type TEXT NOT NULL DEFAULT 'full', status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER DEFAULT 0, last_error TEXT DEFAULT '', started_at TEXT DEFAULT '', finished_at TEXT DEFAULT '', created_at TEXT DEFAULT '')")
			return nil
 		},
 	}
 }
