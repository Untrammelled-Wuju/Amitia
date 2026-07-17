package migration

func ExtensionPackageRecoveryMigration() Migration {
	return Migration{
		Version: "202607170012",
		Name:    "add_extension_package_recovery_state",
		Up: func(s *Step) error {
			columns := []struct {
				name       string
				definition string
			}{
				{"artifact_status", "TEXT NOT NULL DEFAULT 'ready'"},
				{"activation_status", "TEXT NOT NULL DEFAULT 'inactive'"},
				{"operation_id", "TEXT NOT NULL DEFAULT ''"},
				{"failure_code", "TEXT NOT NULL DEFAULT ''"},
			}
			for _, column := range columns {
				if err := s.AddColumn("extension_versions", column.name, column.definition); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
