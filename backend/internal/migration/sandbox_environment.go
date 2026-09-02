package migration

func DesktopPetCutoverBaselineBackfillMigration() Migration {
	return Migration{
		Version: "20260903001",
		Name:    "backfill_desktop_pet_cutover_baseline_records",
		Up: func(s *Step) error {
			s.Execute(`INSERT OR IGNORE INTO desktop_pet_migration_operations (id, kind, status, started_at, updated_at, completed_at, error, metadata) VALUES ('baseline-desktop-pet-v2', 'desktop-pet-v2-cutover', 'completed', '2026-09-03T00:00:00Z', '2026-09-03T00:00:00Z', '2026-09-03T00:00:00Z', '', '{"planId":"desktop-pet-v2-cutover","sourceVersion":"baseline","targetVersion":"v2","checkpoint":"baseline","processedCount":0,"conflictCount":0}')`)
			s.Execute(`INSERT OR IGNORE INTO desktop_pet_read_cutovers (id, operation_id, step_name, cutover_at, verified) VALUES ('baseline-desktop-pet-v2-read', 'baseline-desktop-pet-v2', 'v2_read_path', '2026-09-03T00:00:00Z', 1)`)
			s.Execute(`INSERT OR IGNORE INTO desktop_pet_write_cutovers (id, operation_id, step_name, cutover_at, verified) VALUES ('baseline-desktop-pet-v2-write-installation', 'baseline-desktop-pet-v2', 'installation', '2026-09-03T00:00:00Z', 1)`)
			s.Execute(`INSERT OR IGNORE INTO desktop_pet_write_cutovers (id, operation_id, step_name, cutover_at, verified) VALUES ('baseline-desktop-pet-v2-write-editing', 'baseline-desktop-pet-v2', 'editing', '2026-09-03T00:00:00Z', 1)`)
			return nil
		},
	}
}

func SandboxEnvironmentMigration() Migration {
	return Migration{
		Version: "20260902002",
		Name:    "create_sandbox_environment_variables_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS sandbox_environment_variables (
				scope_id TEXT NOT NULL,
				name TEXT NOT NULL,
				value TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL DEFAULT '',
				updated_at DATETIME NOT NULL DEFAULT '',
				PRIMARY KEY (scope_id, name)
			)`)
			s.CreateIndex("idx_sandbox_environment_scope", "sandbox_environment_variables", []string{"scope_id"}, false)
			return nil
		},
	}
}
