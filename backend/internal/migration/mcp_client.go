package migration

func MCPClientMigration() Migration {
	return Migration{
		Version: "202607180003",
		Name:    "add_mcp_client_tables",
		Up: func(s *Step) error {
			tables := []string{
				`CREATE TABLE IF NOT EXISTS mcp_servers (id TEXT PRIMARY KEY, name TEXT NOT NULL, display_name TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', transport TEXT NOT NULL, endpoint TEXT NOT NULL DEFAULT '', command TEXT NOT NULL DEFAULT '', args_json TEXT NOT NULL DEFAULT '[]', work_dir TEXT NOT NULL DEFAULT '', protocol_version TEXT NOT NULL DEFAULT '', server_info_json TEXT NOT NULL DEFAULT '{}', capabilities_json TEXT NOT NULL DEFAULT '{}', instructions TEXT NOT NULL DEFAULT '', auth_type TEXT NOT NULL DEFAULT 'none', enabled INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL DEFAULT 'draft', source TEXT NOT NULL DEFAULT 'manual', normalized_identity TEXT NOT NULL UNIQUE, configuration_hash TEXT NOT NULL DEFAULT '', last_connected_at TEXT NOT NULL DEFAULT '', last_error_code TEXT NOT NULL DEFAULT '', last_error_message TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '')`,
				`CREATE TABLE IF NOT EXISTS mcp_server_scope_bindings (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, scope_type TEXT NOT NULL, scope_id TEXT NOT NULL DEFAULT '', enabled INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '', UNIQUE(server_id, scope_type, scope_id))`,
				`CREATE TABLE IF NOT EXISTS mcp_server_credentials (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, credential_type TEXT NOT NULL, secret_reference TEXT NOT NULL, expires_at TEXT NOT NULL DEFAULT '', scopes_json TEXT NOT NULL DEFAULT '[]', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '')`,
				`CREATE TABLE IF NOT EXISTS mcp_server_capabilities (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, capability TEXT NOT NULL, configuration_json TEXT NOT NULL DEFAULT '{}', enabled INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '', UNIQUE(server_id, capability))`,
				`CREATE TABLE IF NOT EXISTS mcp_tools (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, remote_name TEXT NOT NULL, skill_id TEXT NOT NULL UNIQUE, title TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', input_schema_json TEXT NOT NULL DEFAULT '{}', output_schema_json TEXT NOT NULL DEFAULT '{}', annotations_json TEXT NOT NULL DEFAULT '{}', execution_json TEXT NOT NULL DEFAULT '{}', capability_hints_json TEXT NOT NULL DEFAULT '[]', risk_level TEXT NOT NULL DEFAULT 'high', enabled INTEGER NOT NULL DEFAULT 0, hash TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '', UNIQUE(server_id, remote_name))`,
				`CREATE TABLE IF NOT EXISTS mcp_resources (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, uri TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', title TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', mime_type TEXT NOT NULL DEFAULT '', enabled INTEGER NOT NULL DEFAULT 0, hash TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '', UNIQUE(server_id, uri))`,
				`CREATE TABLE IF NOT EXISTS mcp_resource_templates (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, uri_template TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', title TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', mime_type TEXT NOT NULL DEFAULT '', enabled INTEGER NOT NULL DEFAULT 0, hash TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '', UNIQUE(server_id, uri_template))`,
				`CREATE TABLE IF NOT EXISTS mcp_prompts (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, remote_name TEXT NOT NULL, title TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', arguments_json TEXT NOT NULL DEFAULT '[]', enabled INTEGER NOT NULL DEFAULT 0, hash TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '', UNIQUE(server_id, remote_name))`,
				`CREATE TABLE IF NOT EXISTS mcp_dependency_links (id TEXT PRIMARY KEY, agent_skill_extension_id TEXT NOT NULL, server_id TEXT NOT NULL, dependency_name TEXT NOT NULL, required INTEGER NOT NULL DEFAULT 1, install_status TEXT NOT NULL DEFAULT 'missing', binding_status TEXT NOT NULL DEFAULT 'missing', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '', UNIQUE(agent_skill_extension_id, server_id, dependency_name))`,
				`CREATE TABLE IF NOT EXISTS mcp_operations (id TEXT PRIMARY KEY, type TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending', server_id TEXT NOT NULL DEFAULT '', agent_skill_id TEXT NOT NULL DEFAULT '', scope_type TEXT NOT NULL DEFAULT '', scope_id TEXT NOT NULL DEFAULT '', plan_json TEXT NOT NULL DEFAULT '{}', result_json TEXT NOT NULL DEFAULT '{}', error_code TEXT NOT NULL DEFAULT '', error_message TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '')`,
				`CREATE TABLE IF NOT EXISTS mcp_oauth_sessions (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, state_hash TEXT NOT NULL UNIQUE, code_verifier_reference TEXT NOT NULL, redirect_uri TEXT NOT NULL, requested_scopes_json TEXT NOT NULL DEFAULT '[]', status TEXT NOT NULL DEFAULT 'pending', expires_at TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '')`,
				`CREATE TABLE IF NOT EXISTS mcp_tasks (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, remote_task_id TEXT NOT NULL, character_id TEXT NOT NULL DEFAULT '', run_id TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'working', status_message TEXT NOT NULL DEFAULT '', result_json TEXT NOT NULL DEFAULT '{}', expires_at TEXT NOT NULL DEFAULT '', last_updated_at TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '', UNIQUE(server_id, remote_task_id))`,
				`CREATE TABLE IF NOT EXISTS mcp_audit_logs (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, operation TEXT NOT NULL, tool_name TEXT NOT NULL DEFAULT '', character_id TEXT NOT NULL DEFAULT '', conversation_id TEXT NOT NULL DEFAULT '', channel TEXT NOT NULL DEFAULT '', trace_id TEXT NOT NULL DEFAULT '', operation_id TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '', duration_ms INTEGER NOT NULL DEFAULT 0, error_code TEXT NOT NULL DEFAULT '', summary_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL DEFAULT '')`,
			}
			for _, table := range tables {
				s.CreateTable(table)
			}
			indexes := []struct {
				name    string
				table   string
				columns []string
			}{
				{name: "idx_mcp_servers_status", table: "mcp_servers", columns: []string{"status", "enabled"}},
				{name: "idx_mcp_bindings_scope", table: "mcp_server_scope_bindings", columns: []string{"scope_type", "scope_id", "enabled"}},
				{name: "idx_mcp_credentials_server", table: "mcp_server_credentials", columns: []string{"server_id"}},
				{name: "idx_mcp_tools_server_enabled", table: "mcp_tools", columns: []string{"server_id", "enabled"}},
				{name: "idx_mcp_resources_server", table: "mcp_resources", columns: []string{"server_id"}},
				{name: "idx_mcp_prompts_server", table: "mcp_prompts", columns: []string{"server_id"}},
				{name: "idx_mcp_dependencies_skill", table: "mcp_dependency_links", columns: []string{"agent_skill_extension_id"}},
				{name: "idx_mcp_operations_status", table: "mcp_operations", columns: []string{"status", "created_at"}},
				{name: "idx_mcp_tasks_expiry", table: "mcp_tasks", columns: []string{"status", "expires_at"}},
				{name: "idx_mcp_audit_server_created", table: "mcp_audit_logs", columns: []string{"server_id", "created_at"}},
			}
			for _, index := range indexes {
				if err := s.CreateIndex(index.name, index.table, index.columns, false); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
