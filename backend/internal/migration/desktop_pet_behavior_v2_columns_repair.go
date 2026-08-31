package migration

// DesktopPetBehaviorV2ColumnsRepairMigration is a forward-only repair for
// installations that may have recorded the historical V2 migration as applied
// even though one or more AddColumn probes failed and were ignored by older
// code. Re-running the idempotent AddColumn set repairs missing columns without
// changing the historical migration version or checksum.
func DesktopPetBehaviorV2ColumnsRepairMigration() Migration {
	return Migration{
		Version: "202608310003",
		Name:    "repair_desktop_pet_behavior_v2_columns",
		Up: func(s *Step) error {
			legacy := DesktopPetBehaviorV2ColumnsMigration()
			if legacy.Up == nil {
				return nil
			}
			return legacy.Up(s)
		},
	}
}
