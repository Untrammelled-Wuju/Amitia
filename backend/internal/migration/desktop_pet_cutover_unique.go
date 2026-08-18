package migration

// DesktopPetCutoverUniqueMigration makes cutover records idempotent per
// operation/step. It also deduplicates any rows left by older retry behavior
// before creating the unique indexes.
func DesktopPetCutoverUniqueMigration() Migration {
	return Migration{
		Version: "20260818005",
		Name:    "desktop_pet_cutover_operation_step_unique",
		Up: func(s *Step) error {
			s.Execute(`DELETE FROM desktop_pet_read_cutovers
WHERE rowid NOT IN (
  SELECT MAX(rowid) FROM desktop_pet_read_cutovers GROUP BY operation_id, step_name
)`)
			s.Execute(`DELETE FROM desktop_pet_write_cutovers
WHERE rowid NOT IN (
  SELECT MAX(rowid) FROM desktop_pet_write_cutovers GROUP BY operation_id, step_name
)`)
			s.CreateIndex("idx_dprc_operation_step_unique", "desktop_pet_read_cutovers", []string{"operation_id", "step_name"}, true)
			s.CreateIndex("idx_dpwc_operation_step_unique", "desktop_pet_write_cutovers", []string{"operation_id", "step_name"}, true)
			return nil
		},
	}
}
