package migration

func ExtensionOwnedResourcesMigration() Migration {
	return Migration{
		Version: "202607170011",
		Name:    "add_extension_owned_resources",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_owned_resources (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
extension_version TEXT NOT NULL DEFAULT '',
resource_type TEXT NOT NULL,
resource_id TEXT NOT NULL,
owner_scope_type TEXT NOT NULL DEFAULT 'global',
owner_scope_id TEXT NOT NULL DEFAULT '',
source_run_id TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'active',
cleanup_attempts INTEGER NOT NULL DEFAULT 0,
last_error TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL,
cleaned_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, resource_type, resource_id)
)`)
			if err := s.CreateIndex("idx_extension_owned_resources_owner", "extension_owned_resources", []string{"extension_id", "owner_scope_type", "owner_scope_id", "status"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_extension_owned_resources_resource", "extension_owned_resources", []string{"resource_type", "resource_id"}, false); err != nil {
				return err
			}
			columns := []struct {
				name       string
				definition string
			}{
				{"source_extension_id", "TEXT NOT NULL DEFAULT ''"},
				{"source_extension_version", "TEXT NOT NULL DEFAULT ''"},
				{"source_run_id", "TEXT NOT NULL DEFAULT ''"},
				{"owner_scope_type", "TEXT NOT NULL DEFAULT 'global'"},
				{"owner_scope_id", "TEXT NOT NULL DEFAULT ''"},
			}
			for _, column := range columns {
				if err := s.AddColumn("schedules", column.name, column.definition); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
