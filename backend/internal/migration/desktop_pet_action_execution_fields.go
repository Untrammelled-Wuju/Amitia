// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetActionExecutionFieldsMigration() Migration {
	return Migration{
		Version: "202607240006",
		Name:    "add_desktop_pet_action_execution_fields",
		Up: func(s *Step) error {
			if err := s.AddColumn("desktop_pet_generation_task_actions", "attempt_number", "INTEGER NOT NULL DEFAULT 1"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_generation_task_actions", "generation_spec_version", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_generation_task_actions", "current_attempt", "INTEGER NOT NULL DEFAULT 1"); err != nil {
				return err
			}
			return nil
		},
	}
}
