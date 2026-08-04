// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetRuntimeV2CommandForwardFixMigration() Migration {
	return Migration{
		Version: "202608040002",
		Name:    "align_runtime_commands_v2_with_model",
		Up: func(s *Step) error {
			columns := []struct {
				Name string
				Type string
			}{
				{"durability", "TEXT NOT NULL DEFAULT ''"},
				{"coalesce_key", "TEXT NOT NULL DEFAULT ''"},
				{"payload_json", "TEXT NOT NULL DEFAULT '{}'"},
				{"payload_schema_version", "INTEGER NOT NULL DEFAULT 0"},
				{"desired_revision", "INTEGER NOT NULL DEFAULT 0"},
				{"settings_revision", "INTEGER NOT NULL DEFAULT 0"},
				{"installation_id", "TEXT NOT NULL DEFAULT ''"},
				{"pet_id", "TEXT NOT NULL DEFAULT ''"},
				{"release_id", "TEXT NOT NULL DEFAULT ''"},
				{"expires_at", "TEXT NOT NULL DEFAULT ''"},
				{"last_attempt_id", "TEXT NOT NULL DEFAULT ''"},
				{"superseded_by_command_id", "TEXT NOT NULL DEFAULT ''"},
				{"created_at", "TEXT NOT NULL DEFAULT ''"},
			}

			for _, column := range columns {
				if err := s.AddColumn(
					"desktop_pet_runtime_commands_v2",
					column.Name,
					column.Type,
				); err != nil {
					return err
				}
			}

			s.Execute(`UPDATE desktop_pet_runtime_commands_v2
SET payload_json = CASE WHEN payload_json = '' THEN payload ELSE payload_json END,
    desired_revision = CASE WHEN desired_revision = 0 THEN revision ELSE desired_revision END,
    created_at = CASE WHEN created_at = '' THEN inserted_at ELSE created_at END,
    superseded_by_command_id = CASE WHEN superseded_by_command_id = '' THEN superseded_by ELSE superseded_by_command_id END`)

			if err := s.CreateIndex(
				"idx_rtcv2_user_device_runtime_status",
				"desktop_pet_runtime_commands_v2",
				[]string{
					"user_id",
					"device_id",
					"runtime_id",
					"status",
				},
				false,
			); err != nil {
				return err
			}

			return nil
		},
	}
}
