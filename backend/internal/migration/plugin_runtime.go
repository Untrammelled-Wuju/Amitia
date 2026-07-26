// Deprecated: Legacy plugin runtime migration creating old schema tables
// (extension_states/events/schedules/plugin_runs/audits). Retained to keep
// historical schema intact during cutover; new writes go through
// extension/kernel/data_migration. Removal scheduled at Amitia extension
// refactor step 66 — see docs/amitiax/Amitia_扩展系统重构_第66步_删除旧PluginRuntime.md.

package migration

func PluginRuntimeMigration() Migration {
	return Migration{
		Version: "202607170002",
		Name:    "add_plugin_runtime_tables",
		Up: func(s *Step) error {
			for _, column := range []struct{ name, definition string }{
				{"lifecycle_status", "TEXT NOT NULL DEFAULT 'registered'"},
				{"health_status", "TEXT NOT NULL DEFAULT 'unknown'"},
				{"last_error_code", "TEXT NOT NULL DEFAULT ''"},
				{"last_error_at", "TEXT NOT NULL DEFAULT ''"},
				{"enabled_at", "TEXT NOT NULL DEFAULT ''"},
				{"disabled_at", "TEXT NOT NULL DEFAULT ''"},
			} {
				if err := s.AddColumn("extensions", column.name, column.definition); err != nil {
					return err
				}
			}
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_states (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
scope_type TEXT NOT NULL DEFAULT 'global',
scope_id TEXT NOT NULL DEFAULT '',
schema_version TEXT NOT NULL,
revision INTEGER NOT NULL DEFAULT 1,
state_json TEXT NOT NULL DEFAULT '{}',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, scope_type, scope_id)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_state_revisions (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
schema_version TEXT NOT NULL,
revision INTEGER NOT NULL,
state_json TEXT NOT NULL DEFAULT '{}',
created_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, scope_type, scope_id, revision)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_events (
id TEXT PRIMARY KEY,
event_id TEXT NOT NULL UNIQUE,
source TEXT NOT NULL,
type TEXT NOT NULL,
subject TEXT NOT NULL DEFAULT '',
data_json TEXT NOT NULL DEFAULT '{}',
trace_id TEXT NOT NULL DEFAULT '',
correlation_id TEXT NOT NULL DEFAULT '',
causation_id TEXT NOT NULL DEFAULT '',
depth INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_event_deliveries (
id TEXT PRIMARY KEY,
event_id TEXT NOT NULL,
plugin_id TEXT NOT NULL,
status TEXT NOT NULL DEFAULT 'pending',
attempts INTEGER NOT NULL DEFAULT 0,
next_attempt_at TEXT NOT NULL DEFAULT '',
last_error_code TEXT NOT NULL DEFAULT '',
last_error_detail TEXT NOT NULL DEFAULT '',
processed_at TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(event_id, plugin_id)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_schedules (
id TEXT PRIMARY KEY,
plugin_id TEXT NOT NULL,
schedule_id TEXT NOT NULL,
scope_type TEXT NOT NULL DEFAULT 'global',
scope_id TEXT NOT NULL DEFAULT '',
schedule_type TEXT NOT NULL,
expression TEXT NOT NULL,
timezone TEXT NOT NULL DEFAULT 'UTC',
payload_json TEXT NOT NULL DEFAULT '{}',
enabled INTEGER NOT NULL DEFAULT 1,
next_run_at TEXT NOT NULL DEFAULT '',
last_run_at TEXT NOT NULL DEFAULT '',
last_status TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(plugin_id, schedule_id, scope_type, scope_id)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_plugin_runs (
run_id TEXT PRIMARY KEY,
plugin_id TEXT NOT NULL,
plugin_version TEXT NOT NULL,
hook TEXT NOT NULL,
character_id TEXT NOT NULL DEFAULT '',
conversation_id TEXT NOT NULL DEFAULT '',
channel TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL,
duration_ms INTEGER NOT NULL DEFAULT 0,
error_code TEXT NOT NULL DEFAULT '',
trace_id TEXT NOT NULL DEFAULT '',
circuit_state TEXT NOT NULL DEFAULT 'closed',
created_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_audits (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
action TEXT NOT NULL,
scope_type TEXT NOT NULL DEFAULT 'global',
scope_id TEXT NOT NULL DEFAULT '',
detail_json TEXT NOT NULL DEFAULT '{}',
trace_id TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT ''
)`)
			indexes := []struct {
				name, table string
				columns     []string
			}{
				{"idx_extension_states_scope", "extension_states", []string{"extension_id", "scope_type", "scope_id"}},
				{"idx_extension_events_type_created", "extension_events", []string{"type", "created_at"}},
				{"idx_extension_deliveries_pending", "extension_event_deliveries", []string{"status", "next_attempt_at"}},
				{"idx_extension_deliveries_plugin", "extension_event_deliveries", []string{"plugin_id", "status"}},
				{"idx_extension_schedules_due", "extension_schedules", []string{"enabled", "next_run_at"}},
				{"idx_extension_plugin_runs_plugin", "extension_plugin_runs", []string{"plugin_id", "created_at"}},
				{"idx_extension_audits_extension", "extension_audits", []string{"extension_id", "created_at"}},
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
