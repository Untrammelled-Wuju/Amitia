package migration

func ExtensionArtifactRecoveryMigration() Migration {
	return Migration{
		Version: "202607170013",
		Name:    "add_extension_artifact_recovery_state",
		Up: func(s *Step) error {
			if err := s.AddColumn("extension_artifacts", "artifact_status", "TEXT NOT NULL DEFAULT 'active'"); err != nil {
				return err
			}
			return s.AddColumn("extension_artifacts", "operation_id", "TEXT NOT NULL DEFAULT ''")
		},
	}
}
