// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func ProviderApiTypeMigration() Migration {
	return Migration{
		Version: "202607300002",
		Name:    "add_api_type_to_model_config_tables",
		Up: func(s *Step) error {
			if err := s.AddColumn("tts_configs", "api_type", "TEXT DEFAULT 'volcengine'"); err != nil {
				return err
			}
			if err := s.AddColumn("asr_configs", "api_type", "TEXT DEFAULT 'volcengine'"); err != nil {
				return err
			}
			if err := s.AddColumn("vision_configs", "api_type", "TEXT DEFAULT 'volcengine'"); err != nil {
				return err
			}
			if err := s.AddColumn("embedding_configs", "api_type", "TEXT DEFAULT 'volcengine'"); err != nil {
				return err
			}
			if err := s.AddColumn("image_gen_configs", "api_type", "TEXT DEFAULT 'seedream'"); err != nil {
				return err
			}
			return nil
		},
	}
}
