// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func TtsAsrBaseUrlMigration() Migration {
	return Migration{
		Version: "202607300003",
		Name:    "add_base_url_to_tts_asr_configs",
		Up: func(s *Step) error {
			if err := s.AddColumn("tts_configs", "base_url", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("asr_configs", "base_url", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			return nil
		},
	}
}
