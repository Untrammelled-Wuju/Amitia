package migration

func DesktopPetInstallationOperationIdempotencyMigration() Migration {
	return Migration{
		Version: "20260818002",
		Name:    "desktop_pet_installation_operation_idempotency_unique",
		Up: func(s *Step) error {
			s.Execute(`UPDATE desktop_pet_installation_operations AS o
SET idempotency_key = idempotency_key || ':legacy:' || id
WHERE idempotency_key <> ''
  AND EXISTS (
    SELECT 1
    FROM desktop_pet_installation_operations AS older
    WHERE older.user_id = o.user_id
      AND older.device_id = o.device_id
      AND older.operation_type = o.operation_type
      AND older.idempotency_key = o.idempotency_key
      AND older.rowid < o.rowid
  )`)
			s.Execute(`CREATE UNIQUE INDEX IF NOT EXISTS idx_dpinstop_idempotency_unique
ON desktop_pet_installation_operations(user_id, device_id, operation_type, idempotency_key)
WHERE idempotency_key <> ''`)
			return nil
		},
	}
}
