package migration

import (
	"fmt"
	"strings"
)

var hostTables = map[string]bool{
	"users":                              true,
	"characters":                         true,
	"messages":                           true,
	"package_operations":                 true,
	"package_installations":              true,
	"schema_migrations":                  true,
	"extension_migration_definitions":    true,
	"extension_migration_operations":     true,
	"extension_migration_steps":          true,
	"extension_migration_checkpoints":    true,
	"extension_data_snapshots":           true,
	"extension_snapshot_entries":         true,
	"extension_kv_state":                 true,
	"extension_definitions":              true,
	"extension_installations":            true,
	"extension_modules":                  true,
	"extension_runtime_state":            true,
	"extension_tasks":                    true,
	"extension_schedules":                true,
	"extension_workflows":                true,
	"extension_contributions":            true,
	"extension_permissions":              true,
	"extension_permission_grants":        true,
	"extension_permission_requirements":  true,
	"extension_operations":               true,
	"extension_page_sessions":            true,
	"extension_host_api_audit":           true,
	"extension_wasm_modules":             true,
	"extension_resources":                true,
	"extension_scopes":                   true,
	"extension_ui_contributions":         true,
}

var sqliteSystemTables = map[string]bool{
	"sqlite_master":      true,
	"sqlite_sequence":    true,
	"sqlite_schema":      true,
	"sqlite_temp_master": true,
	"sqlite_temp_schema": true,
}

var sqliteSystemTablePrefixes = []string{"sqlite_stat"}

var forbiddenSQLKeywords = []string{
	"attach",
	"detach",
	"load_extension",
	"vacuum",
	"pragma writable_schema",
	"pragma schema_version",
	"pragma auto_vacuum",
	"pragma foreign_keys",
	"pragma journal_mode",
	"pragma wal_checkpoint",
	"pragma secure_delete",
	"pragma temp_store_directory",
	"pragma data_store_directory",
	"create trigger",
	"create view",
	"create virtual table",
	"create macro",
}

func NormalizeExtensionID(extensionID string) string {
	normalized := strings.ToLower(extensionID)
	var buf strings.Builder
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			buf.WriteRune(r)
		} else {
			buf.WriteRune('_')
		}
	}
	result := strings.Trim(buf.String(), "_")
	if result == "" {
		return "unknown"
	}
	return result
}

func ExtensionNamespacePrefix(extensionID string) string {
	return "ext_" + NormalizeExtensionID(extensionID) + "_"
}

func IsSystemTable(name string) bool {
	lower := strings.ToLower(name)
	if sqliteSystemTables[lower] {
		return true
	}
	for _, prefix := range sqliteSystemTablePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func IsHostTable(name string) bool {
	return hostTables[strings.ToLower(name)]
}

func checkForbiddenCommands(raw string) error {
	lower := strings.ToLower(strings.TrimSpace(raw))
	for _, kw := range forbiddenSQLKeywords {
		if strings.Contains(lower, kw) {
			return fmt.Errorf("kernel: migration sql uses forbidden command %q", kw)
		}
	}
	return nil
}

func ValidateStatement(stmt *SQLStatement, extensionID string) error {
	if stmt == nil {
		return fmt.Errorf("kernel: migration sql statement is nil")
	}
	if err := checkForbiddenCommands(stmt.Raw); err != nil {
		return err
	}
	if stmt.Type == SQLTypeOther {
		return nil
	}
	prefix := ExtensionNamespacePrefix(extensionID)
	for _, obj := range stmt.Objects {
		lower := strings.ToLower(obj.Name)
		if IsSystemTable(lower) {
			return fmt.Errorf("kernel: migration sql references system table %q", obj.Name)
		}
		if IsHostTable(lower) {
			return fmt.Errorf("kernel: migration sql references host table %q", obj.Name)
		}
		if !strings.HasPrefix(lower, "ext_") {
			return fmt.Errorf("kernel: migration sql object %q must use ext_ prefix", obj.Name)
		}
		if !strings.HasPrefix(lower, prefix) {
			return fmt.Errorf("kernel: migration sql object %q does not belong to namespace %q", obj.Name, prefix)
		}
	}
	return nil
}

func ValidateRawStatements(sqlContent string, extensionID string) ([]*SQLStatement, error) {
	stmts := splitSQLStatements(sqlContent)
	if len(stmts) == 0 {
		return nil, fmt.Errorf("kernel: migration sql has no executable statements")
	}
	var result []*SQLStatement
	for _, raw := range stmts {
		parsed, err := ParseStatement(raw)
		if err != nil {
			return nil, fmt.Errorf("kernel: migration sql parse error: %w", err)
		}
		if err := ValidateStatement(parsed, extensionID); err != nil {
			return nil, err
		}
		result = append(result, parsed)
	}
	return result, nil
}

func IsExtensionNamespaceTable(name string, extensionID string) bool {
	lower := strings.ToLower(name)
	if IsSystemTable(lower) {
		return false
	}
	if IsHostTable(lower) {
		return false
	}
	prefix := ExtensionNamespacePrefix(extensionID)
	return strings.HasPrefix(lower, prefix)
}
