package migration

func ExtensionAgentSkillsMigration() Migration {
	return Migration{
		Version: "202607170007",
		Name:    "add_agent_skills_native_compatibility",
		Up: func(s *Step) error {
			if err := s.AddColumn("extension_artifacts", "artifact_kind", "TEXT NOT NULL DEFAULT 'workflow'"); err != nil {
				return err
			}
			if err := s.AddColumn("extension_artifacts", "content_blob", "BLOB"); err != nil {
				return err
			}
			if err := s.AddColumn("extension_artifacts", "resource_index_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
				return err
			}
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_agent_skill_metadata (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL UNIQUE,
user_id TEXT NOT NULL,
name TEXT NOT NULL,
description TEXT NOT NULL,
license TEXT NOT NULL DEFAULT '',
compatibility TEXT NOT NULL DEFAULT '',
metadata_json TEXT NOT NULL DEFAULT '{}',
allowed_tools TEXT NOT NULL DEFAULT '',
display_name TEXT NOT NULL DEFAULT '',
short_description TEXT NOT NULL DEFAULT '',
default_prompt TEXT NOT NULL DEFAULT '',
openai_metadata_json TEXT NOT NULL DEFAULT '{}',
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
source TEXT NOT NULL,
compatibility_status TEXT NOT NULL,
compatibility_report_json TEXT NOT NULL DEFAULT '{}',
content_hash TEXT NOT NULL,
artifact_id TEXT NOT NULL,
raw_frontmatter_json TEXT NOT NULL DEFAULT '{}',
extra_frontmatter_json TEXT NOT NULL DEFAULT '{}',
resource_index_json TEXT NOT NULL DEFAULT '[]',
tool_mappings_json TEXT NOT NULL DEFAULT '[]',
scripts_present INTEGER NOT NULL DEFAULT 0,
scripts_required INTEGER NOT NULL DEFAULT 0,
enabled INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL,
removed_at TEXT NOT NULL DEFAULT '',
UNIQUE(user_id, name, scope_type, scope_id)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_agent_skill_activations (
id TEXT PRIMARY KEY,
activation_id TEXT NOT NULL UNIQUE,
extension_id TEXT NOT NULL,
user_id TEXT NOT NULL,
character_id TEXT NOT NULL DEFAULT '',
conversation_id TEXT NOT NULL DEFAULT '',
channel TEXT NOT NULL DEFAULT '',
trigger_type TEXT NOT NULL,
explicit INTEGER NOT NULL DEFAULT 0,
status TEXT NOT NULL,
loaded_tokens INTEGER NOT NULL DEFAULT 0,
resource_reads INTEGER NOT NULL DEFAULT 0,
resource_paths_json TEXT NOT NULL DEFAULT '[]',
trace_id TEXT NOT NULL DEFAULT '',
error_code TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL
)`)
			indexes := []struct {
				name, table string
				columns     []string
			}{{"idx_agent_skills_scope", "extension_agent_skill_metadata", []string{"user_id", "scope_type", "scope_id", "enabled"}}, {"idx_agent_skills_catalog", "extension_agent_skill_metadata", []string{"name", "compatibility_status", "enabled"}}, {"idx_agent_skills_hash", "extension_agent_skill_metadata", []string{"content_hash"}}, {"idx_agent_skill_activations_scope", "extension_agent_skill_activations", []string{"user_id", "character_id", "conversation_id", "created_at"}}, {"idx_agent_skill_activations_extension", "extension_agent_skill_activations", []string{"extension_id", "created_at"}}}
			for _, index := range indexes {
				if err := s.CreateIndex(index.name, index.table, index.columns, false); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func ExtensionAgentSkillTraceMigration() Migration {
	return Migration{
		Version: "202607170008",
		Name:    "complete_agent_skill_prompt_trace",
		Up: func(s *Step) error {
			columns := []struct{ name, definition string }{
				{"agent_skill_name", "TEXT NOT NULL DEFAULT ''"},
				{"source", "TEXT NOT NULL DEFAULT ''"},
				{"scope_type", "TEXT NOT NULL DEFAULT ''"},
				{"compatibility_status", "TEXT NOT NULL DEFAULT ''"},
				{"scripts_used", "INTEGER NOT NULL DEFAULT 0"},
				{"tool_mappings_json", "TEXT NOT NULL DEFAULT '[]'"},
				{"instruction_position", "TEXT NOT NULL DEFAULT ''"},
				{"token_limit_hit", "INTEGER NOT NULL DEFAULT 0"},
			}
			for _, column := range columns {
				if err := s.AddColumn("extension_agent_skill_activations", column.name, column.definition); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
