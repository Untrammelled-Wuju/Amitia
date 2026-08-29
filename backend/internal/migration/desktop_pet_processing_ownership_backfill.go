// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

// DesktopPetProcessingOwnershipBackfillMigration repairs processing-task rows
// created before user_id/character_id became first-class processing columns.
// It is intentionally forward-only instead of modifying the historical atomic
// commit migration, preserving checksums on already-upgraded installations.
func DesktopPetProcessingOwnershipBackfillMigration() Migration {
	return Migration{
		Version: "202608300001",
		Name:    "backfill_desktop_pet_processing_ownership",
		Up: func(s *Step) error {
			s.Execute(`UPDATE desktop_pet_processing_tasks
SET user_id = COALESCE((
  SELECT gt.user_id
  FROM desktop_pet_generation_tasks AS gt
  WHERE gt.id = desktop_pet_processing_tasks.generation_task_id
), '')
WHERE (user_id IS NULL OR user_id = '')
  AND EXISTS (
    SELECT 1
    FROM desktop_pet_generation_tasks AS gt
    WHERE gt.id = desktop_pet_processing_tasks.generation_task_id
      AND gt.user_id IS NOT NULL
      AND gt.user_id <> ''
  )`)

			s.Execute(`UPDATE desktop_pet_processing_tasks
SET character_id = COALESCE((
  SELECT gt.character_id
  FROM desktop_pet_generation_tasks AS gt
  WHERE gt.id = desktop_pet_processing_tasks.generation_task_id
), '')
WHERE (character_id IS NULL OR character_id = '')
  AND EXISTS (
    SELECT 1
    FROM desktop_pet_generation_tasks AS gt
    WHERE gt.id = desktop_pet_processing_tasks.generation_task_id
      AND gt.character_id IS NOT NULL
      AND gt.character_id <> ''
  )`)
			return nil
		},
	}
}
