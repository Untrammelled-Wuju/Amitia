// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetTaskExecutionFieldsMigration() Migration {
	return Migration{
		Version: "202607240005",
		Name:    "add_desktop_pet_task_execution_fields",
		Up: func(s *Step) error {
			if err := s.AddColumn("desktop_pet_generation_tasks", "execution_id", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_generation_tasks", "worker_id", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_generation_tasks", "lease_expires_at", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_generation_tasks", "last_heartbeat_at", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_generation_tasks", "attempt_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_generation_tasks", "cancel_requested_at", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			return nil
		},
	}
}
