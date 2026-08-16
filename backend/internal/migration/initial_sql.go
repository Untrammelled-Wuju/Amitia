package migration

import (
	"fmt"
	"os"
	"regexp"
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
			skip, err := shouldSkipStatement(tx, stmt)
			if err != nil {
				return fmt.Errorf("check schema state for statement %d failed: %w", i+1, err)
			}
			if skip {
				continue
			}
			if err := tx.Exec(stmt).Error; err != nil {
				if shouldRepairMessageSequenceBackfill(stmt, err) {
					if repairErr := repairMessageSequenceBackfill(tx); repairErr != nil {
						return fmt.Errorf("repair initial sql statement %d failed: %w", i+1, repairErr)
					}
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
			skip, err := shouldSkipStatement(tx, item.sql)
			if err != nil {
				return fmt.Errorf("check schema state for deferred statement %d failed: %w", item.number, err)
			}
			if skip {
				continue
			}
			if err := tx.Exec(item.sql).Error; err != nil {
				if shouldDeferInitialSQL(item.sql, err) {
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

var addColumnPattern = regexp.MustCompile(`(?i)^ALTER\s+TABLE\s+("[^"]+"|[` + "`" + `[^` + "`" + `]+` + "`" + `]|\w+)\s+ADD\s+(?:COLUMN\s+)?("[^"]+"|[` + "`" + `[^` + "`" + `]+` + "`" + `]|\w+)`)

var createIndexPattern = regexp.MustCompile(`(?i)^CREATE\s+(?:UNIQUE\s+)?INDEX\s+(IF\s+NOT\s+EXISTS\s+)?("[^"]+"|[` + "`" + `[^` + "`" + `]+` + "`" + `]|\w+)\s+ON\s+`)

func shouldSkipStatement(tx *gorm.DB, stmt string) (bool, error) {
	trimmed := strings.TrimSpace(stmt)
	if m := addColumnPattern.FindStringSubmatch(trimmed); m != nil {
		table := unquoteIdentifier(m[1])
		column := unquoteIdentifier(m[2])
		return shouldSkipAddColumn(tx, table, column)
	}
	if m := createIndexPattern.FindStringSubmatch(trimmed); m != nil {
		if strings.TrimSpace(strings.ToUpper(m[1])) != "" {
			return false, nil
		}
		indexName := unquoteIdentifier(m[2])
		return shouldSkipCreateIndex(tx, indexName)
	}
	return false, nil
}

func shouldSkipAddColumn(tx *gorm.DB, table, column string) (bool, error) {
	if !safeIdentifier(table) || !safeIdentifier(column) {
		return false, nil
	}
	tableExists, err := tableExistsInTx(tx, table)
	if err != nil {
		return false, err
	}
	if !tableExists {
		return true, nil
	}
	columnExists, err := columnExistsInTx(tx, table, column)
	if err != nil {
		return false, err
	}
	return columnExists, nil
}

func shouldSkipCreateIndex(tx *gorm.DB, indexName string) (bool, error) {
	if !safeIdentifier(indexName) {
		return false, nil
	}
	var count int64
	if err := tx.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", indexName).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func tableExistsInTx(tx *gorm.DB, name string) (bool, error) {
	var count int64
	if err := tx.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func columnExistsInTx(tx *gorm.DB, table, column string) (bool, error) {
	var rows []struct {
		Name string `gorm:"column:name"`
	}
	if err := tx.Raw("PRAGMA table_info(" + table + ")").Scan(&rows).Error; err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Name == column {
			return true, nil
		}
	}
	return false, nil
}

func unquoteIdentifier(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '`' && s[len(s)-1] == '`') {
			return s[1 : len(s)-1]
		}
	}
	return s
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

func shouldRepairMessageSequenceBackfill(stmt string, err error) bool {
	if err == nil {
		return false
	}
	normalizedStmt := strings.Join(strings.Fields(strings.ToLower(stmt)), " ")
	if !strings.HasPrefix(normalizedStmt, "update messages set sequence = (") {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed: messages.conversation_id, messages.sequence")
}

func repairMessageSequenceBackfill(db *gorm.DB) error {
	return db.Exec(`
WITH invalid AS (
	SELECT id, conversation_id, ROW_NUMBER() OVER (PARTITION BY conversation_id ORDER BY created_at, id) AS rn
	FROM messages
	WHERE sequence IS NULL OR sequence <= 0
),
maxseq AS (
	SELECT conversation_id, COALESCE(MAX(sequence), 0) AS base
	FROM messages
	WHERE sequence > 0
	GROUP BY conversation_id
)
UPDATE messages
SET sequence = COALESCE((SELECT base FROM maxseq WHERE maxseq.conversation_id = messages.conversation_id), 0) + (SELECT rn FROM invalid WHERE invalid.id = messages.id)
WHERE id IN (SELECT id FROM invalid)`).Error
}
