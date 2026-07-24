// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func ImageGenConfigEnabledMigration() Migration {
	return Migration{
		Version: "202607240003",
		Name:    "add_image_gen_configs_enabled_column",
		Up: func(s *Step) error {
			return s.AddColumn("image_gen_configs", "enabled", "INTEGER NOT NULL DEFAULT 1")
		},
	}
}
