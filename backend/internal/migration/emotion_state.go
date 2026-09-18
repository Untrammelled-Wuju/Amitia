package migration

func EmotionStateMigration() Migration {
	return Migration{
		Version: "20260913002",
		Name:    "create_emotion_states",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS emotion_states (
				user_id TEXT NOT NULL DEFAULT '',
				character_id TEXT NOT NULL DEFAULT '',
				user_affect_json TEXT NOT NULL DEFAULT '{}',
				relationship_emotion_json TEXT NOT NULL DEFAULT '{}',
				signals_json TEXT NOT NULL DEFAULT '{}',
				baseline_json TEXT NOT NULL DEFAULT '{}',
				version INTEGER NOT NULL DEFAULT 0,
				updated_at DATETIME NOT NULL,
				PRIMARY KEY (user_id, character_id)
			)`)
			s.CreateIndex("idx_emotion_states_character", "emotion_states", []string{"character_id"}, false)
			return nil
		},
	}
}
