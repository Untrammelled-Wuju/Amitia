package migration

import (
	_ "embed"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

//go:embed baseline.sql
var baselineSQL string

const coreSchemaBaselineVersion = 1

func ApplyBaseline(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is required")
	}
	initialized, err := coreSchemaInitialized(db)
	if err != nil {
		return fmt.Errorf("check database baseline state: %w", err)
	}
	if initialized {
		if err := ensureRolePurgeJobsTable(db); err != nil {
			return err
		}
		if err := ensureDesktopPetInstallationOperationColumns(db); err != nil {
			return err
		}
		return ensureDesktopPetInstallationIndexes(db)
	}
	statements := splitSQLStatements(baselineSQL)
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, prefix := range []string{"CREATE TABLE ", "CREATE INDEX ", "CREATE UNIQUE INDEX "} {
			for i, statement := range statements {
				if !strings.HasPrefix(strings.ToUpper(statement), prefix) {
					continue
				}
				if err := tx.Exec(statement).Error; err != nil {
					return fmt.Errorf("execute baseline statement %d failed: %w", i+1, err)
				}
			}
		}
		if err := tx.Exec(`INSERT INTO core_schema_baseline(singleton, schema_version, created_at) VALUES(1, ?, CURRENT_TIMESTAMP)`, coreSchemaBaselineVersion).Error; err != nil {
			return fmt.Errorf("record schema baseline failed: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return ensureDesktopPetInstallationOperationColumns(db)
}

func ensureRolePurgeJobsTable(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS role_data_purge_jobs (
	role_id TEXT PRIMARY KEY,
	state TEXT NOT NULL DEFAULT 'pending',
	attempts INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
)`).Error; err != nil {
		return fmt.Errorf("create role data purge jobs table: %w", err)
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_role_purge_due ON role_data_purge_jobs(state, next_attempt_at)`).Error; err != nil {
		return fmt.Errorf("create role data purge jobs index: %w", err)
	}
	return nil
}

func ensureDesktopPetInstallationIndexes(db *gorm.DB) error {
	var indexes []string
	if err := db.Raw(`SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = 'desktop_pet_installations'
		AND name IN ('uq_dpinst_live_space_device_pet', 'uq_dpinst_user_device_pet', 'idx_dpinst_package_version')`).Scan(&indexes).Error; err != nil {
		return fmt.Errorf("inspect desktop pet installation indexes: %w", err)
	}
	if len(indexes) == 1 && indexes[0] == "uq_dpinst_live_space_device_pet" {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, statement := range []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS uq_dpinst_live_space_device_pet ON desktop_pet_installations(space_id, device_id, pet_id) WHERE lifecycle_state <> 'uninstalled'",
			"DROP INDEX IF EXISTS uq_dpinst_user_device_pet",
			"DROP INDEX IF EXISTS idx_dpinst_package_version",
		} {
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("apply desktop pet installation indexes: %w", err)
			}
		}
		return nil
	})
}

func ensureDesktopPetInstallationOperationColumns(db *gorm.DB) error {
	columns := []struct {
		table string
		name  string
		ddl   string
	}{
		{table: "desktop_pet_installation_operations", name: "runtime_id", ddl: "ALTER TABLE desktop_pet_installation_operations ADD COLUMN runtime_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_operations", name: "source_release_id", ddl: "ALTER TABLE desktop_pet_installation_operations ADD COLUMN source_release_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_operations", name: "execution_id", ddl: "ALTER TABLE desktop_pet_installation_operations ADD COLUMN execution_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_operations", name: "heartbeat_at", ddl: "ALTER TABLE desktop_pet_installation_operations ADD COLUMN heartbeat_at TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_operations", name: "desired_revision", ddl: "ALTER TABLE desktop_pet_installation_operations ADD COLUMN desired_revision INTEGER NOT NULL DEFAULT 0"},
		{table: "desktop_pet_installation_commit_journals", name: "runtime_id", ddl: "ALTER TABLE desktop_pet_installation_commit_journals ADD COLUMN runtime_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_commit_journals", name: "pet_id", ddl: "ALTER TABLE desktop_pet_installation_commit_journals ADD COLUMN pet_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_commit_journals", name: "source_release_id", ddl: "ALTER TABLE desktop_pet_installation_commit_journals ADD COLUMN source_release_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_commit_journals", name: "stage", ddl: "ALTER TABLE desktop_pet_installation_commit_journals ADD COLUMN stage TEXT NOT NULL DEFAULT 'operation_created'"},
		{table: "desktop_pet_installation_commit_journals", name: "status", ddl: "ALTER TABLE desktop_pet_installation_commit_journals ADD COLUMN status TEXT NOT NULL DEFAULT 'active'"},
		{table: "desktop_pet_installation_commit_journals", name: "execution_id", ddl: "ALTER TABLE desktop_pet_installation_commit_journals ADD COLUMN execution_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_commit_journals", name: "expected_old_status", ddl: "ALTER TABLE desktop_pet_installation_commit_journals ADD COLUMN expected_old_status TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_commit_journals", name: "rollback_path_key", ddl: "ALTER TABLE desktop_pet_installation_commit_journals ADD COLUMN rollback_path_key TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_switch_journals", name: "runtime_id", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN runtime_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_switch_journals", name: "old_binding_revision", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN old_binding_revision INTEGER NOT NULL DEFAULT 0"},
		{table: "desktop_pet_installation_switch_journals", name: "new_binding_revision", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN new_binding_revision INTEGER NOT NULL DEFAULT 0"},
		{table: "desktop_pet_installation_switch_journals", name: "old_release_id", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN old_release_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_switch_journals", name: "new_release_id", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN new_release_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_switch_journals", name: "stage", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN stage TEXT NOT NULL DEFAULT 'created'"},
		{table: "desktop_pet_installation_switch_journals", name: "status", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN status TEXT NOT NULL DEFAULT 'active'"},
		{table: "desktop_pet_installation_switch_journals", name: "execution_id", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN execution_id TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_switch_journals", name: "expected_old_stage", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN expected_old_stage TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_switch_journals", name: "error_code", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN error_code TEXT NOT NULL DEFAULT ''"},
		{table: "desktop_pet_installation_switch_journals", name: "error_message", ddl: "ALTER TABLE desktop_pet_installation_switch_journals ADD COLUMN error_message TEXT NOT NULL DEFAULT ''"},
	}
	for _, column := range columns {
		if !db.Migrator().HasTable(column.table) || db.Migrator().HasColumn(column.table, column.name) {
			continue
		}
		if err := db.Exec(column.ddl).Error; err != nil {
			return fmt.Errorf("add desktop pet operation column %s: %w", column.name, err)
		}
	}
	return nil
}

func splitSQLStatements(raw string) []string {
	statements := make([]string, 0)
	var current strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(trimmed)
		current.WriteString(" ")
		if strings.HasSuffix(trimmed, ";") {
			statement := strings.TrimSpace(strings.TrimSuffix(current.String(), ";"))
			current.Reset()
			if statement != "" {
				statements = append(statements, statement)
			}
		}
	}
	statement := strings.TrimSpace(current.String())
	if statement != "" {
		statements = append(statements, statement)
	}
	return statements
}

func IsNewDatabase(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func coreSchemaInitialized(db *gorm.DB) (bool, error) {
	isNew, err := IsNewDatabase(db)
	if err != nil {
		return false, err
	}
	if isNew {
		return false, nil
	}
	var version int
	if err := db.Raw("SELECT schema_version FROM core_schema_baseline WHERE singleton = 1").Scan(&version).Error; err != nil {
		return false, fmt.Errorf("existing database does not match the current core schema baseline: %w", err)
	}
	if version != coreSchemaBaselineVersion {
		return false, fmt.Errorf("unsupported core schema baseline version %d", version)
	}
	return true, nil
}
