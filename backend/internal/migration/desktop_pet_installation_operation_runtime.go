package migration

func DesktopPetInstallationOperationRuntimeMigration() Migration {
	return Migration{
		Version: "20261023001",
		Name:    "add_runtime_id_to_desktop_pet_installation_operations",
		Up: func(s *Step) error {
			s.AddColumn("desktop_pet_installation_operations", "runtime_id", "TEXT NOT NULL DEFAULT ''")
			s.CreateIndex("idx_dpinstop_runtime", "desktop_pet_installation_operations", []string{"runtime_id"}, false)
			return nil
		},
	}
}
