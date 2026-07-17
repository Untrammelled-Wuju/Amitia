package migration

func ExtensionScheduleOwnershipRepairMigration() Migration {
	return Migration{
		Version: "202607170015",
		Name:    "repair_extension_schedule_ownership",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS schedules (
id TEXT PRIMARY KEY,
title TEXT NOT NULL DEFAULT '',
description TEXT NOT NULL DEFAULT '',
due_time TEXT NOT NULL DEFAULT '',
repeat_mode TEXT NOT NULL DEFAULT 'none',
channel TEXT NOT NULL DEFAULT 'all',
status TEXT NOT NULL DEFAULT 'pending',
source_type TEXT NOT NULL DEFAULT 'user',
source_extension_id TEXT NOT NULL DEFAULT '',
source_extension_version TEXT NOT NULL DEFAULT '',
source_run_id TEXT NOT NULL DEFAULT '',
owner_scope_type TEXT NOT NULL DEFAULT 'global',
owner_scope_id TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
)`)
			columns := []struct {
				name       string
				definition string
			}{
				{"source_type", "TEXT NOT NULL DEFAULT 'user'"},
				{"source_extension_id", "TEXT NOT NULL DEFAULT ''"},
				{"source_extension_version", "TEXT NOT NULL DEFAULT ''"},
				{"source_run_id", "TEXT NOT NULL DEFAULT ''"},
				{"owner_scope_type", "TEXT NOT NULL DEFAULT 'global'"},
				{"owner_scope_id", "TEXT NOT NULL DEFAULT ''"},
			}
			for _, column := range columns {
				if err := s.AddColumn("schedules", column.name, column.definition); err != nil {
					return err
				}
			}
			if err := s.CreateIndex("idx_schedules_extension_owner", "schedules", []string{"source_extension_id", "owner_scope_type", "owner_scope_id", "status"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
