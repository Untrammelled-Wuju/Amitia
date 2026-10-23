package migration

func DesktopPetInstallationJournalColumnsMigration() Migration {
	return Migration{
		Version: "20261023003",
		Name:    "add_canonical_columns_to_desktop_pet_installation_journals",
		Up: func(s *Step) error {
			for _, column := range []struct {
				name       string
				definition string
			}{
				{"runtime_id", "TEXT NOT NULL DEFAULT ''"},
				{"pet_id", "TEXT NOT NULL DEFAULT ''"},
				{"source_release_id", "TEXT NOT NULL DEFAULT ''"},
				{"stage", "TEXT NOT NULL DEFAULT ''"},
				{"status", "TEXT NOT NULL DEFAULT ''"},
				{"execution_id", "TEXT NOT NULL DEFAULT ''"},
				{"expected_old_status", "TEXT NOT NULL DEFAULT ''"},
				{"rollback_path_key", "TEXT NOT NULL DEFAULT ''"},
			} {
				s.AddColumn("desktop_pet_installation_commit_journals", column.name, column.definition)
			}
			for _, column := range []struct {
				name       string
				definition string
			}{
				{"runtime_id", "TEXT NOT NULL DEFAULT ''"},
				{"old_binding_revision", "INTEGER NOT NULL DEFAULT 0"},
				{"new_binding_revision", "INTEGER NOT NULL DEFAULT 0"},
				{"old_release_id", "TEXT NOT NULL DEFAULT ''"},
				{"new_release_id", "TEXT NOT NULL DEFAULT ''"},
				{"stage", "TEXT NOT NULL DEFAULT ''"},
				{"status", "TEXT NOT NULL DEFAULT ''"},
				{"execution_id", "TEXT NOT NULL DEFAULT ''"},
				{"expected_old_stage", "TEXT NOT NULL DEFAULT ''"},
			} {
				s.AddColumn("desktop_pet_installation_switch_journals", column.name, column.definition)
			}
			s.CreateIndex("idx_dpicj_execution", "desktop_pet_installation_commit_journals", []string{"execution_id"}, false)
			s.CreateIndex("idx_dpisj_execution", "desktop_pet_installation_switch_journals", []string{"execution_id"}, false)
			return nil
		},
	}
}
