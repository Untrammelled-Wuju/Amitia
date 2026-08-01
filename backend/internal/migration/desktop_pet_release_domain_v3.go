package migration

func DesktopPetReleaseDomainV3Migration() Migration {
	return Migration{
		Version: "202608010001",
		Name:    "release_domain_fix_constraints_and_fields",
		Up: func(s *Step) error {
			if err := fixReleaseBuildSnapshotFields(s); err != nil {
				return err
			}
			if err := addEvaluationSetHash(s); err != nil {
				return err
			}
			if err := addMissingReleaseColumns(s); err != nil {
				return err
			}
			if err := fixLegacyMappingOwnership(s); err != nil {
				return err
			}
			if err := addLifecycleRevisionFields(s); err != nil {
				return err
			}
			return nil
		},
	}
}

func fixReleaseBuildSnapshotFields(s *Step) error {
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "active_revision_set_json", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "quality_gate_json", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "required_actions_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "excluded_actions_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "action_snapshots_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "preview_snapshot_json", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "package_contract_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "build_profile_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "build_profile_version", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "build_config_json", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "input_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "snapshot_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "active_revision_set_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_build_snapshots", "evaluation_set_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	s.Execute("DROP INDEX IF EXISTS idx_drbs_input_hash")
	s.CreateIndex("idx_drbs_input_hash", "desktop_pet_release_build_snapshots", []string{"input_hash"}, true)
	s.CreateIndex("idx_drbs_snapshot_hash", "desktop_pet_release_build_snapshots", []string{"snapshot_hash"}, true)
	return nil
}

func addEvaluationSetHash(s *Step) error {
	if err := s.AddColumn("desktop_pet_release_publish_journals", "snapshot_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_publish_journals", "snapshot_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_publish_journals", "workspace_storage_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_publish_journals", "staging_storage_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_publish_journals", "published_storage_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_publish_journals", "archive_storage_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_publish_journals", "archive_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_publish_journals", "file_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_publish_journals", "total_bytes", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_publish_journals", "archive_bytes", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_release_publish_journals", "completed_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func addMissingReleaseColumns(s *Step) error {
	if err := s.AddColumn("desktop_pet_package_releases", "evaluation_set_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "archive_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "archive_bytes", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "lifecycle_revision", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "integrity_revision", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "archived_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "revoked_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "revocation_reason", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "legacy_package_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "legacy_version", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return nil
}

func fixLegacyMappingOwnership(s *Step) error {
	if err := s.AddColumn("desktop_pet_legacy_package_mappings", "owner_user_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_legacy_package_mappings", "source_manifest_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_legacy_package_mappings", "migration_operation_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	s.Execute("CREATE UNIQUE INDEX IF NOT EXISTS uq_dlpm_owner_legacy ON desktop_pet_legacy_package_mappings(owner_user_id, legacy_package_id)")
	return nil
}

func addLifecycleRevisionFields(s *Step) error {
	if err := s.AddColumn("desktop_pet_release_build_operations", "execution_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}
