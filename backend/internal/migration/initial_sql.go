package migration

import (
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"
)

type deferredStatement struct {
	number int
	sql    string
}

func ApplyInitialSQLFile(db *gorm.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return ApplyInitialSQL(db, string(data))
}

func ApplyInitialSQL(db *gorm.DB, raw string) error {
	if db == nil {
		return fmt.Errorf("db is required")
	}
	statements := splitSQLStatements(raw)
	return db.Transaction(func(tx *gorm.DB) error {
		deferred := make([]deferredStatement, 0)
		for i, stmt := range statements {
			if err := tx.Exec(stmt).Error; err != nil {
				if isDuplicateColumnError(err) {
					continue
				}
				if shouldDeferInitialSQL(stmt, err) {
					deferred = append(deferred, deferredStatement{number: i + 1, sql: stmt})
					continue
				}
				return fmt.Errorf("execute initial sql statement %d failed: %w", i+1, err)
			}
		}
		for _, item := range deferred {
			if err := tx.Exec(item.sql).Error; err != nil {
				if isDuplicateColumnError(err) {
					continue
				}
				return fmt.Errorf("execute deferred initial sql statement %d failed after retry: %w", item.number, err)
			}
		}
		return nil
	})
}

func splitSQLStatements(raw string) []string {
	lines := strings.Split(raw, "\n")
	statements := make([]string, 0)
	var current strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(trimmed)
		current.WriteString(" ")
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(current.String())
			stmt = strings.TrimSuffix(stmt, ";")
			current.Reset()
			if stmt != "" {
				statements = append(statements, stmt)
			}
		}
	}
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}
	return statements
}

func isDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}

func shouldDeferInitialSQL(stmt string, err error) bool {
	if err == nil {
		return false
	}
	upperStmt := strings.ToUpper(strings.TrimSpace(stmt))
	if !strings.HasPrefix(upperStmt, "CREATE INDEX ") && !strings.HasPrefix(upperStmt, "CREATE UNIQUE INDEX ") {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such column") || strings.Contains(msg, "no such table")
}
