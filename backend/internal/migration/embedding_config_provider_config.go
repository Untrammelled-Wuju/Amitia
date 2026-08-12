// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func EmbeddingConfigProviderConfigMigration() Migration {
	return Migration{
		Version: "202608130003",
		Name:    "add_embedding_configs_provider_config_json",
		Up: func(s *Step) error {
			s.AddColumn("embedding_configs", "provider_config_json", "TEXT DEFAULT ''")
			return nil
		},
	}
}
