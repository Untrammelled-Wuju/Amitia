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
		return nil
	}
	statements := splitSQLStatements(baselineSQL)
	return db.Transaction(func(tx *gorm.DB) error {
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
	})
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
