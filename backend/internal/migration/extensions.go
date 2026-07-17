package migration

func ExtensionsMigration() Migration {
	return Migration{
		Version: "202607170001",
		Name:    "add_skill_runtime_tables",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extensions (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL UNIQUE,
kind TEXT NOT NULL DEFAULT 'Skill',
name TEXT NOT NULL DEFAULT '',
current_version TEXT NOT NULL DEFAULT '',
source TEXT NOT NULL DEFAULT '',
enabled INTEGER NOT NULL DEFAULT 0,
manifest_json TEXT NOT NULL DEFAULT '{}',
normalized_manifest_json TEXT NOT NULL DEFAULT '{}',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
archived_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_versions (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
version TEXT NOT NULL,
manifest_json TEXT NOT NULL DEFAULT '{}',
checksum TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, version)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_capability_grants (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
capability TEXT NOT NULL,
decision TEXT NOT NULL DEFAULT 'deny',
scope_type TEXT NOT NULL DEFAULT 'global',
scope_id TEXT NOT NULL DEFAULT '',
expires_at TEXT NOT NULL DEFAULT '',
consumed_at TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, capability, scope_type, scope_id)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_configs (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
scope_type TEXT NOT NULL DEFAULT 'global',
scope_id TEXT NOT NULL DEFAULT '',
config_json TEXT NOT NULL DEFAULT '{}',
config_version INTEGER NOT NULL DEFAULT 1,
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, scope_type, scope_id)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_runs (
run_id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
extension_version TEXT NOT NULL DEFAULT '',
skill_id TEXT NOT NULL,
user_id TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
conversation_id TEXT NOT NULL DEFAULT '',
channel TEXT NOT NULL DEFAULT '',
trigger TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
input_summary TEXT NOT NULL DEFAULT '{}',
output_summary TEXT NOT NULL DEFAULT '{}',
side_effects_json TEXT NOT NULL DEFAULT '[]',
idempotency_key TEXT NOT NULL DEFAULT '',
started_at TEXT NOT NULL DEFAULT '',
finished_at TEXT NOT NULL DEFAULT '',
duration_ms INTEGER NOT NULL DEFAULT 0,
error_code TEXT NOT NULL DEFAULT '',
error_detail TEXT NOT NULL DEFAULT '',
trace_id TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
UNIQUE(skill_id, character_id, conversation_id, idempotency_key)
)`)
			indexes := []struct {
				name    string
				table   string
				columns []string
			}{
				{name: "idx_extensions_extension_id", table: "extensions", columns: []string{"extension_id"}},
				{name: "idx_extension_runs_skill", table: "extension_runs", columns: []string{"skill_id"}},
				{name: "idx_extension_runs_status", table: "extension_runs", columns: []string{"status"}},
				{name: "idx_extension_runs_created", table: "extension_runs", columns: []string{"created_at"}},
				{name: "idx_extension_runs_trace", table: "extension_runs", columns: []string{"trace_id"}},
				{name: "idx_extension_runs_character_conversation", table: "extension_runs", columns: []string{"character_id", "conversation_id"}},
				{name: "idx_extension_grants_lookup", table: "extension_capability_grants", columns: []string{"extension_id", "capability", "scope_type", "scope_id"}},
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
