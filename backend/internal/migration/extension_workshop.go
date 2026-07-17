package migration

func ExtensionWorkshopMigration() Migration {
	return Migration{
		Version: "202607170003",
		Name:    "add_extension_workshop_tables",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_workshop_sessions (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL,
character_id TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'draft',
requirement TEXT NOT NULL,
current_revision INTEGER NOT NULL DEFAULT 0,
current_draft_id TEXT NOT NULL DEFAULT '',
validation_summary TEXT NOT NULL DEFAULT '{}',
risk_summary TEXT NOT NULL DEFAULT '{}',
test_summary TEXT NOT NULL DEFAULT '{}',
installed_skill_id TEXT NOT NULL DEFAULT '',
installed_version TEXT NOT NULL DEFAULT '',
permission_confirmation_json TEXT NOT NULL DEFAULT '{}',
permission_revision INTEGER NOT NULL DEFAULT 0,
permission_checksum TEXT NOT NULL DEFAULT '',
test_permission_confirmation_json TEXT NOT NULL DEFAULT '{}',
test_permission_revision INTEGER NOT NULL DEFAULT 0,
test_permission_checksum TEXT NOT NULL DEFAULT '',
lock_version INTEGER NOT NULL DEFAULT 1,
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL,
archived_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_workshop_revisions (
id TEXT PRIMARY KEY,
session_id TEXT NOT NULL,
revision INTEGER NOT NULL,
raw_model_output TEXT NOT NULL DEFAULT '{}',
plan_json TEXT NOT NULL DEFAULT '{}',
raw_draft_json TEXT NOT NULL,
normalized_draft_json TEXT NOT NULL,
manifest_json TEXT NOT NULL,
input_schema_json TEXT NOT NULL,
output_schema_json TEXT NOT NULL,
config_schema_json TEXT NOT NULL DEFAULT '{}',
workflow_json TEXT NOT NULL,
compiled_workflow_json TEXT NOT NULL,
capability_analysis_json TEXT NOT NULL DEFAULT '{}',
risk_analysis_json TEXT NOT NULL DEFAULT '{}',
validation_result_json TEXT NOT NULL DEFAULT '{}',
workflow_checksum TEXT NOT NULL,
model_provider TEXT NOT NULL DEFAULT '',
model_name TEXT NOT NULL DEFAULT '',
model_input_summary_json TEXT NOT NULL DEFAULT '{}',
model_output_summary_json TEXT NOT NULL DEFAULT '{}',
created_at TEXT NOT NULL,
UNIQUE(session_id, revision)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_workshop_test_runs (
id TEXT PRIMARY KEY,
test_run_id TEXT NOT NULL UNIQUE,
user_id TEXT NOT NULL,
character_id TEXT NOT NULL DEFAULT '',
session_id TEXT NOT NULL,
revision INTEGER NOT NULL,
workflow_checksum TEXT NOT NULL,
mode TEXT NOT NULL,
status TEXT NOT NULL,
input_summary TEXT NOT NULL DEFAULT '{}',
output_summary TEXT NOT NULL DEFAULT '{}',
step_results_json TEXT NOT NULL DEFAULT '[]',
assertion_results_json TEXT NOT NULL DEFAULT '[]',
side_effects_json TEXT NOT NULL DEFAULT '[]',
capabilities_json TEXT NOT NULL DEFAULT '[]',
warnings_json TEXT NOT NULL DEFAULT '[]',
error_code TEXT NOT NULL DEFAULT '',
error_detail TEXT NOT NULL DEFAULT '',
trace_id TEXT NOT NULL DEFAULT '',
started_at TEXT NOT NULL,
finished_at TEXT NOT NULL,
duration_ms INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_artifacts (
id TEXT PRIMARY KEY,
artifact_id TEXT NOT NULL UNIQUE,
extension_id TEXT NOT NULL,
extension_version TEXT NOT NULL,
source TEXT NOT NULL DEFAULT 'workshop',
session_id TEXT NOT NULL,
revision INTEGER NOT NULL,
manifest_json TEXT NOT NULL,
workflow_json TEXT NOT NULL,
schemas_json TEXT NOT NULL,
compiled_workflow_json TEXT NOT NULL,
tests_json TEXT NOT NULL DEFAULT '[]',
readme_text TEXT NOT NULL DEFAULT '',
checksum TEXT NOT NULL,
size_bytes INTEGER NOT NULL,
created_at TEXT NOT NULL,
archived_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, extension_version)
)`)
			indexes := []struct {
				name, table string
				columns     []string
			}{
				{"idx_workshop_sessions_owner", "extension_workshop_sessions", []string{"user_id", "character_id", "updated_at"}},
				{"idx_workshop_sessions_status", "extension_workshop_sessions", []string{"status"}},
				{"idx_workshop_revisions_session", "extension_workshop_revisions", []string{"session_id", "revision"}},
				{"idx_workshop_test_runs_session", "extension_workshop_test_runs", []string{"session_id", "revision", "created_at"}},
				{"idx_extension_artifacts_extension", "extension_artifacts", []string{"extension_id", "extension_version"}},
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

func ExtensionWorkshopPermissionScopesMigration() Migration {
	return Migration{
		Version: "202607170004",
		Name:    "split_workshop_test_and_production_permissions",
		Up: func(s *Step) error {
			if err := s.AddColumn("extension_workshop_sessions", "test_permission_confirmation_json", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
				return err
			}
			if err := s.AddColumn("extension_workshop_sessions", "test_permission_revision", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			return s.AddColumn("extension_workshop_sessions", "test_permission_checksum", "TEXT NOT NULL DEFAULT ''")
		},
	}
}

func ExtensionWorkshopPlannerMigration() Migration {
	return Migration{
		Version: "202607170005",
		Name:    "persist_workshop_structured_plan",
		Up: func(s *Step) error {
			return s.AddColumn("extension_workshop_revisions", "plan_json", "TEXT NOT NULL DEFAULT '{}'")
		},
	}
}

func ExtensionWorkshopGenerationSummaryMigration() Migration {
	return Migration{
		Version: "202607170006",
		Name:    "persist_workshop_generation_summaries",
		Up: func(s *Step) error {
			if err := s.AddColumn("extension_workshop_revisions", "model_input_summary_json", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
				return err
			}
			return s.AddColumn("extension_workshop_revisions", "model_output_summary_json", "TEXT NOT NULL DEFAULT '{}'")
		},
	}
}
