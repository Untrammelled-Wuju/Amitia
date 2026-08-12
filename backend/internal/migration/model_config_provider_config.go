// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func ModelConfigProviderConfigMigration() Migration {
	return Migration{
		Version: "202608130002",
		Name:    "add_provider_config_json",
		Up: func(s *Step) error {
			return s.AddColumn("model_configs", "provider_config_json", "TEXT DEFAULT '{}'")
		},
	}
}
