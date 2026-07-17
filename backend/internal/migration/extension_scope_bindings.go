package migration

func ExtensionScopeBindingsMigration() Migration {
	return Migration{
		Version: "202607170010",
		Name:    "add_extension_scope_bindings",
		Up: func(s *Step) error {
			if err := s.AddColumn("extensions", "scope_type", "TEXT NOT NULL DEFAULT 'global'"); err != nil {
				return err
			}
			if err := s.AddColumn("extensions", "scope_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_scope_bindings (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
enabled INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL,
UNIQUE(extension_id, scope_type, scope_id)
)`)
			if err := s.CreateIndex("idx_extension_scope_bindings_scope", "extension_scope_bindings", []string{"scope_type", "scope_id", "enabled"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_extension_scope_bindings_extension", "extension_scope_bindings", []string{"extension_id", "scope_type", "scope_id"}, true); err != nil {
				return err
			}
			s.Execute(`INSERT OR IGNORE INTO extension_scope_bindings (id, extension_id, scope_type, scope_id, enabled, created_at, updated_at)
SELECT lower(hex(randomblob(16))), extension_id, 'global', '', enabled, created_at, updated_at FROM extensions WHERE COALESCE(scope_type, 'global') <> 'character'`)
			s.Execute(`INSERT OR IGNORE INTO extension_scope_bindings (id, extension_id, scope_type, scope_id, enabled, created_at, updated_at)
SELECT lower(hex(randomblob(16))), extension_id, 'character', scope_id, enabled, created_at, updated_at FROM extensions WHERE scope_type = 'character' AND scope_id <> ''`)
			return nil
		},
	}
}
