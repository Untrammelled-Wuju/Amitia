package migration

func DesktopPetInstallationDeviceScopeIndexMigration() Migration {
	return Migration{
		Version: "20261023004",
		Name:    "drop_device_agnostic_desktop_pet_installation_unique_index",
		Up: func(s *Step) error {
			s.Execute("DROP INDEX IF EXISTS idx_dpinst_package_version")
			s.CreateIndex("idx_dpinst_package_version", "desktop_pet_installations", []string{"package_id", "package_version"}, false)
			return nil
		},
	}
}
