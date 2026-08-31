package migration

// DesktopPetActionRevisionDataRepairMigration is a forward-only repair for
// databases where the historical action revision data migration returned
// success after silently skipping individual rows. The repaired legacy Up is
// idempotent and now fails atomically on unresolved or failed writes.
func DesktopPetActionRevisionDataRepairMigration() Migration {
	legacy := DesktopPetActionRevisionDataMigrateMigration()
	return Migration{
		Version: "202608310004",
		Name:    "repair_legacy_action_revision_data_to_stream",
		// The repair performs direct transactional data writes and therefore must
		// never be re-executed as part of applied-migration checksum validation.
		ChecksumUp: func(_ *Step) error { return nil },
		Up:         legacy.Up,
	}
}
