package migration

func MCPDuplicateRegistrationsMigration() Migration {
	return Migration{
		Version: "202607300001",
		Name:    "add_mcp_duplicate_tool_registrations_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS mcp_duplicate_tool_registrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tool_id TEXT NOT NULL,
    server_id TEXT NOT NULL,
    owner TEXT NOT NULL DEFAULT '',
    generation INTEGER NOT NULL DEFAULT 0,
    detected_at TEXT NOT NULL,
    resolved INTEGER NOT NULL DEFAULT 0
)`)
			return s.CreateIndex("idx_mcp_dup_unresolved", "mcp_duplicate_tool_registrations", []string{"resolved", "server_id"}, false)
		},
	}
}
