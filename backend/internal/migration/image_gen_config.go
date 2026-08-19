package migration

func ImageGenConfigMigration() Migration {
	return Migration{
		Version:           "202607240001",
		Name:              "add_image_gen_configs_table",
		AcceptedChecksums: []string{"d2c40f847889eb44c771304084db13923a10c70319078129b73b1da0490fb6dd", "594ea579a3813b85c5e24d839d0233bce51621ec46aff50dd80c1286c2e82918"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS image_gen_configs (
id INTEGER PRIMARY KEY AUTOINCREMENT,
name TEXT NOT NULL,
api_key TEXT DEFAULT '',
model_name TEXT DEFAULT 'doubao-seedream-5-0',
base_url TEXT DEFAULT 'https://ark.cn-beijing.volces.com/api/v3',
is_active INTEGER DEFAULT 0,
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
)`)
			return nil
		},
	}
}
