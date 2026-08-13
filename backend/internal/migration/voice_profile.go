// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func VoiceProfileMigration() Migration {
	return Migration{
		Version: "202608130004",
		Name:    "add_voice_profiles_and_wake_configs",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS voice_profiles (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL DEFAULT '',
				asr_config_id INTEGER DEFAULT 0,
				tts_config_id INTEGER DEFAULT 0,
				realtime_provider_id TEXT DEFAULT '',
				wake_config_id TEXT DEFAULT '',
				vad_preset TEXT DEFAULT 'default',
				interrupt_policy TEXT DEFAULT 'immediate',
				privacy_mode TEXT DEFAULT 'standard',
				is_default INTEGER DEFAULT 0,
				created_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT ''
			)`)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS wake_configs (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL DEFAULT '',
				enabled INTEGER DEFAULT 0,
				backend TEXT DEFAULT 'software',
				model_resource_uri TEXT DEFAULT '',
				phrases TEXT DEFAULT '',
				threshold REAL DEFAULT 0.05,
				cooldown_ms INTEGER DEFAULT 2000,
				created_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT ''
			)`)

			s.CreateIndex("idx_voice_profiles_is_default", "voice_profiles", []string{"is_default"}, false)
			return nil
		},
	}
}
