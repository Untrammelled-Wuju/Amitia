// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func TtsClonedVoicesMigration() Migration {
	return Migration{
		Version: "20260907007",
		Name:    "create_tts_cloned_voices_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS tts_cloned_voices (
				user_id TEXT NOT NULL,
				speaker_id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				tts_config_id INTEGER NOT NULL DEFAULT 0,
				language INTEGER NOT NULL DEFAULT 0,
				status TEXT NOT NULL DEFAULT 'ready',
				created_at TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT ''
			)`)
			if err := s.CreateIndex("idx_tts_cloned_voices_user_created", "tts_cloned_voices", []string{"user_id", "created_at"}, false); err != nil {
				return err
			}
			// Preserve character-bound clone voices created before clone metadata
			// became a first-class Core resource. A provider speaker id is global,
			// so only unambiguous single-owner legacy ids are backfilled; ambiguous
			// ids remain unclaimed and are intentionally blocked in cloud mode.
			s.Execute(`INSERT OR IGNORE INTO tts_cloned_voices (
				user_id, speaker_id, name, tts_config_id, language, status, created_at, updated_at
			)
			SELECT
				COALESCE(NULLIF(TRIM(MAX(user_id)), ''), 'default'),
				TRIM(custom_voice_id),
				'历史复刻音色',
				CASE
					WHEN COUNT(DISTINCT COALESCE(NULLIF(TRIM(voice_config_id), ''), '0')) = 1
					THEN CAST(COALESCE(NULLIF(TRIM(MAX(voice_config_id)), ''), '0') AS INTEGER)
					ELSE 0
				END,
				0,
				'ready',
				COALESCE(MAX(created_at), ''),
				COALESCE(MAX(updated_at), '')
			FROM characters
			WHERE LOWER(TRIM(COALESCE(voice_mode, ''))) = 'clone'
				AND TRIM(COALESCE(custom_voice_id, '')) <> ''
			GROUP BY TRIM(custom_voice_id)
			HAVING COUNT(DISTINCT COALESCE(NULLIF(TRIM(user_id), ''), 'default')) = 1`)
			return nil
		},
	}
}
