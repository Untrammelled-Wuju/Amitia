package migration

func DesktopPetInstallationOperationCoordinatorColumnsMigration() Migration {
	return Migration{
		Version: "20261023002",
		Name:    "add_coordinator_columns_to_desktop_pet_installation_operations",
		Up: func(s *Step) error {
			s.AddColumn("desktop_pet_installation_operations", "source_release_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installation_operations", "execution_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installation_operations", "heartbeat_at", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installation_operations", "desired_revision", "INTEGER NOT NULL DEFAULT 0")
			s.CreateIndex("idx_dpinstop_execution", "desktop_pet_installation_operations", []string{"execution_id"}, false)
			return nil
		},
	}
}
