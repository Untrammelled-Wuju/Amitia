package migration

func DesktopPetCharacterUnbindMigration() Migration {
	return Migration{
		Version: "20260913001",
		Name:    "unbind_desktop_pet_from_character",
		Up: func(s *Step) error {
			for _, table := range []string{
				"desktop_pet_generation_tasks",
				"desktop_pet_processing_tasks",
				"desktop_pet_packages",
				"desktop_pet_installations",
				"desktop_pet_release_build_snapshots",
				"desktop_pet_action_revisions",
				"desktop_pet_action_active_revisions",
				"desktop_pet_frame_assets",
				"desktop_pet_edit_sessions",
				"desktop_pet_regeneration_jobs",
				"desktop_pet_edit_candidates",
				"desktop_pet_edit_draft_snapshots",
				"desktop_pet_edit_audit_logs",
				"desktop_pet_processing_source_manifests",
				"desktop_pet_quality_evaluations",
				"desktop_pet_reference_assets",
				"desktop_pet_action_streams",
				"desktop_pet_revision_bridge_journals",
				"desktop_pet_active_action_revision_bindings",
				"desktop_pet_quality_input_snapshots",
				"desktop_pet_quality_gate_snapshots",
			} {
				s.Execute("UPDATE " + table + " SET character_id = '' WHERE character_id <> ''")
			}
			s.Execute("UPDATE desktop_pet_identities SET source_character_id = '', binding_policy = 'unbound' WHERE source_character_id <> '' OR binding_policy <> 'unbound'")
			return nil
		},
	}
}
