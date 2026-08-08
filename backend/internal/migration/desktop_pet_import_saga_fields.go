package migration

func DesktopPetImportSagaFieldsMigration() Migration {
	return Migration{
		Version: "202608090001",
		Name:    "add_desktop_pet_import_saga_fields",
		Up: func(s *Step) error {
			if err := s.AddColumn(
				"desktop_pet_release_publish_journals",
				"operation_kind",
				"TEXT NOT NULL DEFAULT 'build'",
			); err != nil {
				return err
			}

			if err := s.AddColumn(
				"desktop_pet_import_package_snapshots",
				"operation_id",
				"TEXT NOT NULL DEFAULT ''",
			); err != nil {
				return err
			}

			if err := s.AddColumn(
				"desktop_pet_import_package_snapshots",
				"status",
				"TEXT NOT NULL DEFAULT 'preparing'",
			); err != nil {
				return err
			}

			if err := s.AddColumn(
				"desktop_pet_import_package_snapshots",
				"last_error",
				"TEXT NOT NULL DEFAULT ''",
			); err != nil {
				return err
			}

			s.CreateIndex(
				"idx_drpj_kind_stage",
				"desktop_pet_release_publish_journals",
				[]string{
					"operation_kind",
					"stage",
				},
				false,
			)

			s.CreateIndex(
				"idx_dips_operation",
				"desktop_pet_import_package_snapshots",
				[]string{
					"operation_id",
				},
				false,
			)

			s.CreateIndex(
				"idx_dips_status",
				"desktop_pet_import_package_snapshots",
				[]string{
					"status",
				},
				false,
			)

			return nil
		},
	}
}
