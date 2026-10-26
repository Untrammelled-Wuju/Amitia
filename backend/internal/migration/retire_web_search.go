package migration

func RetireWebSearchMigration() Migration {
	return Migration{
		Version: "20261026003",
		Name:    "retire_web_search",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS retired_secret_cleanup (
				secret_ref TEXT PRIMARY KEY,
				created_at DATETIME NOT NULL
			)`)
			if exists, err := s.TableExists("search_api_keys"); err != nil {
				return err
			} else if exists {
				s.Execute(`INSERT OR IGNORE INTO retired_secret_cleanup(secret_ref, created_at)
					SELECT api_key, CURRENT_TIMESTAMP FROM search_api_keys
					WHERE api_key LIKE 'secret://%' OR api_key LIKE 'mcp-secret://%'`)
			}
			if exists, err := s.TableExists("search_credential_cleanup"); err != nil {
				return err
			} else if exists {
				s.Execute(`INSERT OR IGNORE INTO retired_secret_cleanup(secret_ref, created_at)
					SELECT secret_ref, CURRENT_TIMESTAMP FROM search_credential_cleanup
					WHERE secret_ref LIKE 'secret://%' OR secret_ref LIKE 'mcp-secret://%'`)
			}
			for _, table := range []string{
				"web_turn_citation_evidence",
				"web_turn_citations",
				"web_evidence",
				"web_page_versions",
				"web_pages",
				"web_references",
				"search_credential_cleanup",
				"search_api_keys",
			} {
				s.Execute("DROP TABLE IF EXISTS " + table)
			}
			return nil
		},
	}
}
