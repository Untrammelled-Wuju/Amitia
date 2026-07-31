package migration

func DesktopPetPhase8GatesMigration() Migration {
	return Migration{
		Version: "202607310010",
		Name:    "add_desktop_pet_phase8_gate_and_artifact_fields",
		Up: func(s *Step) error {
			if err := s.AddColumn("desktop_pet_action_generation_attempts", "active_succeeded_attempt_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_generation_attempts", "active_primary_artifact_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}

			if err := s.AddColumn("desktop_pet_action_revisions", "source_processing_revision_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "quality_overall_score", "REAL"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "quality_ruleset_version", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "quality_source_content_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "quality_evaluated_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}

			if err := s.AddColumn("desktop_pet_quality_gate_results", "active_revision_set_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_quality_gate_results", "evaluation_set_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_quality_gate_results", "ruleset_version", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_quality_gate_results", "invalidated_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}

			if err := s.AddColumn("desktop_pet_packages", "migrated_release_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}

			if err := s.AddColumn("desktop_pet_installations", "legacy_package_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}

			return nil
		},
	}
}
