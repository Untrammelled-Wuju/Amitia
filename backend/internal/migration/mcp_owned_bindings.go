package migration

func MCPOwnedBindingsMigration() Migration {
	return Migration{
		Version: "20261021004",
		Name:    "add_mcp_owned_binding_metadata",
		Up: func(s *Step) error {
			for _, column := range []struct {
				name       string
				definition string
			}{
				{"owner_kind", "TEXT NOT NULL DEFAULT 'manual'"},
				{"owner_extension_id", "TEXT NOT NULL DEFAULT ''"},
				{"owner_module_id", "TEXT NOT NULL DEFAULT ''"},
				{"owner_contribution_id", "TEXT NOT NULL DEFAULT ''"},
				{"runtime_placement", "TEXT NOT NULL DEFAULT ''"},
				{"desired_enabled", "INTEGER NOT NULL DEFAULT 0"},
				{"config_revision", "INTEGER NOT NULL DEFAULT 1"},
				{"package_generation", "INTEGER NOT NULL DEFAULT 0"},
			} {
				if err := s.AddColumn("mcp_servers", column.name, column.definition); err != nil {
					return err
				}
			}
			s.Execute(`UPDATE mcp_servers SET owner_kind = CASE WHEN source = 'acquisition' THEN 'acquisition' ELSE 'manual' END WHERE owner_kind = '' OR owner_kind = 'manual'`)
			s.Execute(`UPDATE mcp_servers SET desired_enabled = enabled`)
			s.Execute(`CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_servers_package_owner ON mcp_servers(owner_extension_id, owner_contribution_id) WHERE owner_kind = 'package' AND owner_extension_id <> '' AND owner_contribution_id <> ''`)
			return nil
		},
	}
}
