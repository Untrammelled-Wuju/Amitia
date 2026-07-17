package migration

func ExtensionPackagesMigration() Migration {
	return Migration{
		Version: "202607170009",
		Name:    "add_local_extension_package_management",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_package_import_sessions (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL,
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
format TEXT NOT NULL,
package_hash TEXT NOT NULL,
status TEXT NOT NULL,
preview_json TEXT NOT NULL DEFAULT '{}',
package_blob BLOB NOT NULL,
file_name TEXT NOT NULL DEFAULT '',
expires_at TEXT NOT NULL,
consumed_at TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_package_installations (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL DEFAULT '',
extension_version TEXT NOT NULL DEFAULT '',
operation TEXT NOT NULL,
source TEXT NOT NULL DEFAULT '',
package_hash TEXT NOT NULL DEFAULT '',
signature_status TEXT NOT NULL DEFAULT '',
signer_fingerprint TEXT NOT NULL DEFAULT '',
previous_version TEXT NOT NULL DEFAULT '',
target_version TEXT NOT NULL DEFAULT '',
user_id TEXT NOT NULL,
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL,
error_code TEXT NOT NULL DEFAULT '',
trace_id TEXT NOT NULL,
created_at TEXT NOT NULL,
completed_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_package_signers (
id TEXT PRIMARY KEY,
fingerprint TEXT NOT NULL UNIQUE,
public_key TEXT NOT NULL,
algorithm TEXT NOT NULL,
display_name TEXT NOT NULL DEFAULT '',
trusted INTEGER NOT NULL DEFAULT 0,
trusted_at TEXT NOT NULL DEFAULT '',
revoked_at TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_version_dependencies (
extension_id TEXT NOT NULL,
extension_version TEXT NOT NULL,
dependency_id TEXT NOT NULL,
version_constraint TEXT NOT NULL DEFAULT '',
required INTEGER NOT NULL DEFAULT 1,
created_at TEXT NOT NULL,
UNIQUE(extension_id, extension_version, dependency_id)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_package_exports (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL,
extension_id TEXT NOT NULL,
file_name TEXT NOT NULL,
mime TEXT NOT NULL,
package_hash TEXT NOT NULL,
content_blob BLOB NOT NULL,
expires_at TEXT NOT NULL,
created_at TEXT NOT NULL,
downloaded_at TEXT NOT NULL DEFAULT ''
)`)
			columns := []struct{ table, name, definition string }{
				{"extensions", "owner_user_id", "TEXT NOT NULL DEFAULT ''"},
				{"extensions", "scope_type", "TEXT NOT NULL DEFAULT 'global'"},
				{"extensions", "scope_id", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "artifact_id", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "artifact_hash", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "package_hash", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "source", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "signature_status", "TEXT NOT NULL DEFAULT 'unsigned'"},
				{"extension_versions", "signer_fingerprint", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "compatibility_status", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "capabilities_json", "TEXT NOT NULL DEFAULT '[]'"},
				{"extension_versions", "installed_by", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "validation_status", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "test_status", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "archived_at", "TEXT NOT NULL DEFAULT ''"},
				{"extension_versions", "package_blob", "BLOB"},
				{"extension_configs", "archived_at", "TEXT NOT NULL DEFAULT ''"},
			}
			for _, column := range columns {
				if err := s.AddColumn(column.table, column.name, column.definition); err != nil {
					return err
				}
			}
			indexes := []struct {
				name, table string
				columns     []string
				unique      bool
			}{
				{"idx_package_sessions_hash", "extension_package_import_sessions", []string{"user_id", "package_hash", "status"}, false},
				{"idx_package_sessions_expiry", "extension_package_import_sessions", []string{"expires_at", "status"}, false},
				{"idx_package_operations_extension", "extension_package_installations", []string{"extension_id", "created_at"}, false},
				{"idx_package_operations_trace", "extension_package_installations", []string{"trace_id"}, false},
				{"idx_package_dependencies_reverse", "extension_version_dependencies", []string{"dependency_id", "extension_id"}, false},
				{"idx_package_exports_expiry", "extension_package_exports", []string{"user_id", "expires_at"}, false},
			}
			for _, index := range indexes {
				if err := s.CreateIndex(index.name, index.table, index.columns, index.unique); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
