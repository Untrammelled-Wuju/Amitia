package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/javascript_main"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

var ErrPackageMigrationTypeUnsupported = errors.New("PACKAGE_MIGRATION_TYPE_UNSUPPORTED")

var forbiddenMigrationTableRegex = regexp.MustCompile(`(?i)\b(users|characters|messages|package_operations|package_installations|schema_migrations|extension_migration_definitions|extension_migration_operations|extension_migration_steps|extension_migration_checkpoints|extension_data_snapshots|extension_snapshot_entries)\b`)

var forbiddenSQLCommandRegex = regexp.MustCompile(`(?i)\b(ATTACH|DETACH|load_extension)\b|VACUUM\s+INTO|PRAGMA\s+writable_schema`)

var tableReferenceRegex = regexp.MustCompile(`(?i)\b(?:CREATE\s+TABLE|ALTER\s+TABLE|INSERT\s+INTO|UPDATE|DELETE\s+FROM|DROP\s+TABLE)\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?(?:["` + "`" + `\[])?([A-Za-z_][A-Za-z0-9_]*)`)

type migrationQueryExecutor interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

type packageMigrationRuntime struct {
	runtime       *Runtime
	pkg           *amitiax.Package
	stagingPath   string
	extensionID   string
	packageOpID   string
	files         map[string]amitiax.FileEntry
	defsPersisted map[string]bool
}

func newPackageMigrationRuntime(rt *Runtime, pkg *amitiax.Package, stagingPath, extensionID, packageOpID string) *packageMigrationRuntime {
	files := map[string]amitiax.FileEntry{}
	if pkg != nil {
		for _, file := range pkg.Files {
			files[strings.ReplaceAll(file.Path, "\\", "/")] = file
		}
	}
	return &packageMigrationRuntime{
		runtime:       rt,
		pkg:           pkg,
		stagingPath:   stagingPath,
		extensionID:   extensionID,
		packageOpID:   packageOpID,
		files:         files,
		defsPersisted: map[string]bool{},
	}
}

func (pmr *packageMigrationRuntime) persistDefinition(ctx context.Context, def migration.MigrationDefinition) error {
	if pmr.runtime.container == nil || pmr.runtime.container.MigrationRepository == nil {
		return nil
	}
	if pmr.defsPersisted[def.MigrationID] {
		return nil
	}
	if err := pmr.runtime.container.MigrationRepository.SaveMigrationDefinition(ctx, &def); err != nil {
		return fmt.Errorf("kernel: persist migration definition %s: %w", def.MigrationID, err)
	}
	pmr.defsPersisted[def.MigrationID] = true
	return nil
}

func (pmr *packageMigrationRuntime) readMigrationScript(def migration.MigrationDefinition) ([]byte, error) {
	if pmr.stagingPath == "" {
		return nil, fmt.Errorf("kernel: migration staging path unavailable for %s", def.Entry)
	}
	scriptPath := filepath.Join(pmr.stagingPath, strings.ReplaceAll(def.Entry, "\\", "/"))
	info, err := os.Stat(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("kernel: migration script not found: %s: %w", def.Entry, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("kernel: migration entry is directory: %s", def.Entry)
	}
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("kernel: read migration script %s: %w", def.Entry, err)
	}
	if len(scriptContent) == 0 {
		return nil, fmt.Errorf("kernel: migration script empty: %s", def.Entry)
	}
	return scriptContent, nil
}

func (pmr *packageMigrationRuntime) executeScript(ctx context.Context, step migration.ReversiblePlanStep, def migration.MigrationDefinition) (json.RawMessage, error) {
	scriptContent, err := pmr.readMigrationScript(def)
	if err != nil {
		return nil, err
	}
	scriptSum := sha256.Sum256(scriptContent)
	baseEvidence := map[string]any{
		"migrationId":    step.MigrationID,
		"direction":      step.Direction,
		"definitionHash": def.DefinitionHash,
		"entry":          def.Entry,
		"executedAt":     time.Now().UTC().Format(time.RFC3339Nano),
		"scriptSize":     len(scriptContent),
		"scriptHash":     hex.EncodeToString(scriptSum[:]),
		"runtimeType":    def.RuntimeType,
		"extensionId":    pmr.extensionID,
		"packageOpId":    pmr.packageOpID,
	}
	switch def.RuntimeType {
	case "sql":
		return pmr.executeSQLMigration(ctx, step, def, scriptContent, baseEvidence)
	case "javascript":
		return pmr.executeJavaScriptMigration(ctx, step, def, baseEvidence)
	default:
		return nil, fmt.Errorf("kernel: migration %s: runtime type %q: %w", def.MigrationID, def.RuntimeType, ErrPackageMigrationTypeUnsupported)
	}
}

func (pmr *packageMigrationRuntime) executeSQLMigration(ctx context.Context, step migration.ReversiblePlanStep, def migration.MigrationDefinition, scriptContent []byte, baseEvidence map[string]any) (json.RawMessage, error) {
	if pmr.runtime.container == nil || pmr.runtime.container.Store == nil {
		return nil, fmt.Errorf("kernel: migration sql store unavailable for %s", def.MigrationID)
	}
	db := pmr.runtime.container.Store.DB()
	if db == nil {
		return nil, fmt.Errorf("kernel: migration sql db unavailable for %s", def.MigrationID)
	}
	statements := splitSQLStatementsSafe(string(scriptContent))
	if len(statements) == 0 {
		return nil, fmt.Errorf("kernel: migration sql has no executable statements: %s", def.Entry)
	}
	for _, stmt := range statements {
		if match := forbiddenMigrationTableRegex.FindString(stmt); match != "" {
			return nil, fmt.Errorf("kernel: migration sql references forbidden host table %q in %s", strings.ToLower(match), def.Entry)
		}
		if match := forbiddenSQLCommandRegex.FindString(stmt); match != "" {
			return nil, fmt.Errorf("kernel: migration sql uses forbidden command %q in %s", strings.ToUpper(match), def.Entry)
		}
		if err := validateExtNamespace(stmt); err != nil {
			return nil, fmt.Errorf("kernel: migration sql namespace violation in %s: %w", def.Entry, err)
		}
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("kernel: migration sql acquire connection: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
		_ = conn.Close()
	}()
	beforeHash, hashErr := pmr.computeExtSchemaHash(ctx, conn)
	if hashErr != nil {
		return nil, fmt.Errorf("kernel: migration sql before hash: %w", hashErr)
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return nil, fmt.Errorf("kernel: migration sql begin immediate: %w", err)
	}
	transactionID := fmt.Sprintf("tx-%s-%d", pmr.packageOpID, step.Order)
	var totalRowsAffected int64
	var statementsExecuted int
	for _, stmt := range statements {
		result, execErr := conn.ExecContext(ctx, stmt)
		if execErr != nil {
			return nil, fmt.Errorf("kernel: migration sql execute failed (%s): %w", def.Entry, execErr)
		}
		if rows, rowsErr := result.RowsAffected(); rowsErr == nil {
			totalRowsAffected += rows
		}
		statementsExecuted++
	}
	if len(def.Postcondition) > 0 {
		for _, cond := range def.Postcondition {
			if err := pmr.verifyPostconditionFromDB(ctx, conn, cond); err != nil {
				return nil, fmt.Errorf("kernel: migration sql postcondition failed for %s: %w", def.MigrationID, err)
			}
		}
	}
	afterHash, hashErr := pmr.computeExtSchemaHash(ctx, conn)
	if hashErr != nil {
		return nil, fmt.Errorf("kernel: migration sql after hash: %w", hashErr)
	}
	baseEvidence["executionMode"] = "sql_executed"
	baseEvidence["executor_type"] = "sql"
	baseEvidence["transaction_id"] = transactionID
	baseEvidence["affected_records"] = totalRowsAffected
	baseEvidence["before_hash"] = beforeHash
	baseEvidence["after_hash"] = afterHash
	baseEvidence["rowsAffected"] = totalRowsAffected
	baseEvidence["statementsExecuted"] = statementsExecuted
	evidenceJSON, marshalErr := json.Marshal(baseEvidence)
	if marshalErr != nil {
		return nil, fmt.Errorf("kernel: migration sql evidence marshal: %w", marshalErr)
	}
	if err := pmr.writeMigrationJournal(ctx, conn, step, def, evidenceJSON); err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return nil, fmt.Errorf("kernel: migration sql commit: %w", err)
	}
	committed = true
	return evidenceJSON, nil
}

func (pmr *packageMigrationRuntime) executeJavaScriptMigration(ctx context.Context, step migration.ReversiblePlanStep, def migration.MigrationDefinition, baseEvidence map[string]any) (json.RawMessage, error) {
	if pmr.runtime.container == nil || pmr.runtime.container.JSRuntimeFactory == nil {
		return nil, fmt.Errorf("kernel: migration javascript runtime unavailable for %s", def.MigrationID)
	}
	host, createErr := pmr.runtime.container.JSRuntimeFactory.Create(ctx, javascript_main.CreateHostRequest{
		ExtensionID:    pmr.extensionID,
		ModuleID:       def.ModuleID,
		Entry:          def.Entry,
		DefinitionHash: def.DefinitionHash,
	})
	if createErr != nil {
		return nil, fmt.Errorf("kernel: migration javascript create host %s: %w", def.MigrationID, createErr)
	}
	startResult := host.Start(ctx)
	if !startResult.Success {
		_ = host.Stop(ctx, "migration_start_failed")
		return nil, fmt.Errorf("kernel: migration javascript host start failed for %s: %s", def.MigrationID, startResult.Reason)
	}
	functionName := "up"
	if step.Direction == migration.DirectionReverse {
		functionName = "down"
	}
	invokeInput := map[string]interface{}{
		"extensionId": pmr.extensionID,
		"moduleId":    def.ModuleID,
		"migrationId": def.MigrationID,
		"direction":   string(step.Direction),
		"fromVersion": step.FromVersion,
		"toVersion":   step.ToVersion,
	}
	invokeResult, invokeErr := host.Invoke(ctx, functionName, invokeInput)
	_ = host.Stop(ctx, "migration_complete")
	if invokeErr != nil {
		return nil, fmt.Errorf("kernel: migration javascript %s invoke failed for %s: %w", functionName, def.MigrationID, invokeErr)
	}
	if len(def.Postcondition) > 0 {
		var db *sql.DB
		if pmr.runtime.container != nil && pmr.runtime.container.Store != nil {
			db = pmr.runtime.container.Store.DB()
		}
		if db == nil {
			postResult, postErr := migration.NewPreconditionValidator().ValidatePostconditions(ctx, def.Postcondition)
			if postErr != nil {
				return nil, fmt.Errorf("kernel: migration javascript postcondition validate %s: %w", def.MigrationID, postErr)
			}
			if !postResult.Passed {
				return nil, fmt.Errorf("kernel: migration javascript postconditions failed for %s: %v", def.MigrationID, postResult.Errors)
			}
		} else {
			for _, cond := range def.Postcondition {
				if err := pmr.verifyPostconditionFromDB(ctx, db, cond); err != nil {
					return nil, fmt.Errorf("kernel: migration javascript postcondition failed for %s: %w", def.MigrationID, err)
				}
			}
		}
	}
	resultJSON, _ := json.Marshal(invokeResult)
	baseEvidence["executionMode"] = "javascript_executed"
	baseEvidence["executor_type"] = "javascript"
	baseEvidence["function"] = functionName
	baseEvidence["functionInvoked"] = functionName
	baseEvidence["invokeResult"] = json.RawMessage(resultJSON)
	baseEvidence["hostInstance"] = host.InstanceID()
	evidenceJSON, marshalErr := json.Marshal(baseEvidence)
	if marshalErr != nil {
		return nil, fmt.Errorf("kernel: migration javascript evidence marshal: %w", marshalErr)
	}
	return evidenceJSON, nil
}

func (pmr *packageMigrationRuntime) writeMigrationJournal(ctx context.Context, conn *sql.Conn, step migration.ReversiblePlanStep, def migration.MigrationDefinition, evidence json.RawMessage) error {
	operationID := "migration-" + pmr.packageOpID
	stepID := step.Order
	recordID := fmt.Sprintf("%s-%d", operationID, stepID)
	outputHash := hashEvidenceBytes(evidence)
	now := time.Now().UTC()
	_, err := conn.ExecContext(ctx, `
		INSERT INTO extension_migration_steps
			(id, operation_id, step_id, migration_id, status, input_hash, output_hash, checkpoint_id,
			 started_at, finished_at, error_code, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, NULL, NULL)
		ON CONFLICT(operation_id, step_id) DO UPDATE SET
			migration_id = excluded.migration_id, status = excluded.status,
			output_hash = excluded.output_hash, finished_at = excluded.finished_at
	`, recordID, operationID, stepID, step.MigrationID, "succeeded", def.DefinitionHash, outputHash, now, now)
	if err != nil {
		return fmt.Errorf("kernel: migration journal write: %w", err)
	}
	return nil
}

func hashEvidenceBytes(evidence []byte) string {
	sum := sha256.Sum256(evidence)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func splitSQLStatementsSafe(content string) []string {
	var statements []string
	var buf strings.Builder
	runes := []rune(content)
	n := len(runes)
	i := 0
	singleQuote := false
	doubleQuote := false
	bracketQuote := false
	lineComment := false
	blockComment := false
	beginDepth := 0
	caseDepth := 0
	flush := func() {
		stmt := strings.TrimSpace(buf.String())
		if stmt == "" || isCommentOnly(stmt) {
			buf.Reset()
			return
		}
		statements = append(statements, stmt)
		buf.Reset()
	}
	for i < n {
		r := runes[i]
		if lineComment {
			buf.WriteRune(r)
			if r == '\n' {
				lineComment = false
			}
			i++
			continue
		}
		if blockComment {
			buf.WriteRune(r)
			if r == '*' && i+1 < n && runes[i+1] == '/' {
				buf.WriteRune('/')
				i += 2
				blockComment = false
				continue
			}
			i++
			continue
		}
		if singleQuote {
			buf.WriteRune(r)
			if r == '\'' {
				if i+1 < n && runes[i+1] == '\'' {
					buf.WriteRune('\'')
					i += 2
					continue
				}
				singleQuote = false
			}
			i++
			continue
		}
		if doubleQuote {
			buf.WriteRune(r)
			if r == '"' {
				if i+1 < n && runes[i+1] == '"' {
					buf.WriteRune('"')
					i += 2
					continue
				}
				doubleQuote = false
			}
			i++
			continue
		}
		if bracketQuote {
			buf.WriteRune(r)
			if r == ']' {
				bracketQuote = false
			}
			i++
			continue
		}
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			lineComment = true
			buf.WriteRune('-')
			buf.WriteRune('-')
			i += 2
			continue
		}
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			blockComment = true
			buf.WriteRune('/')
			buf.WriteRune('*')
			i += 2
			continue
		}
		if r == '\'' {
			singleQuote = true
			buf.WriteRune(r)
			i++
			continue
		}
		if r == '"' {
			doubleQuote = true
			buf.WriteRune(r)
			i++
			continue
		}
		if r == '[' {
			bracketQuote = true
			buf.WriteRune(r)
			i++
			continue
		}
		if r == ';' {
			if beginDepth > 0 {
				buf.WriteRune(';')
				i++
				continue
			}
			flush()
			i++
			continue
		}
		if isSQLWordChar(r) {
			word, wordLen := readSQLWord(runes, i)
			switch strings.ToUpper(word) {
			case "BEGIN":
				beginDepth++
			case "CASE":
				caseDepth++
			case "END":
				if caseDepth > 0 {
					caseDepth--
				} else if beginDepth > 0 {
					beginDepth--
				}
			}
			buf.WriteString(word)
			i += wordLen
			continue
		}
		buf.WriteRune(r)
		i++
	}
	flush()
	return statements
}

func isCommentOnly(stmt string) bool {
	runes := []rune(stmt)
	n := len(runes)
	i := 0
	for i < n {
		r := runes[i]
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			for i < n && runes[i] != '\n' {
				i++
			}
			continue
		}
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			i += 2
			for i+1 < n && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			i++
			continue
		}
		return false
	}
	return true
}

func isSQLWordChar(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func readSQLWord(runes []rune, i int) (string, int) {
	start := i
	for i < len(runes) && isSQLWordChar(runes[i]) {
		i++
	}
	return string(runes[start:i]), i - start
}

func sanitizeSQLForAnalysis(stmt string) string {
	var buf strings.Builder
	runes := []rune(stmt)
	n := len(runes)
	i := 0
	for i < n {
		r := runes[i]
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			for i < n && runes[i] != '\n' {
				i++
			}
			continue
		}
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			i += 2
			for i+1 < n && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
			continue
		}
		if r == '\'' {
			i++
			for i < n {
				if runes[i] == '\'' {
					if i+1 < n && runes[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			buf.WriteByte(' ')
			continue
		}
		buf.WriteRune(r)
		i++
	}
	return buf.String()
}

func extractTableReferences(stmt string) []string {
	sanitized := sanitizeSQLForAnalysis(stmt)
	matches := tableReferenceRegex.FindAllStringSubmatch(sanitized, -1)
	var refs []string
	for _, m := range matches {
		if len(m) > 1 && m[1] != "" {
			refs = append(refs, m[1])
		}
	}
	return refs
}

func validateExtNamespace(stmt string) error {
	refs := extractTableReferences(stmt)
	for _, ref := range refs {
		if !strings.HasPrefix(strings.ToLower(ref), "ext_") {
			return fmt.Errorf("kernel: migration table %q must use ext_ prefix", ref)
		}
	}
	return nil
}

func (pmr *packageMigrationRuntime) computeExtSchemaHash(ctx context.Context, executor migrationQueryExecutor) (string, error) {
	rows, err := executor.QueryContext(ctx, "SELECT name, sql FROM sqlite_master WHERE type='table' AND name LIKE 'ext_%' ORDER BY name")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	h := sha256.New()
	for rows.Next() {
		var name, sqlText string
		if err := rows.Scan(&name, &sqlText); err != nil {
			return "", err
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write([]byte(sqlText))
		h.Write([]byte{0})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	return "sha256:" + hex.EncodeToString(sum), nil
}

func (pmr *packageMigrationRuntime) verifyPostconditionFromDB(ctx context.Context, executor migrationQueryExecutor, condition migration.MigrationCondition) error {
	switch strings.ToLower(condition.Type) {
	case "table_exists":
		var spec struct {
			Table string `json:"table"`
		}
		if err := json.Unmarshal(condition.Expected, &spec); err != nil {
			return fmt.Errorf("kernel: postcondition %s table_exists unmarshal: %w", condition.Name, err)
		}
		if spec.Table == "" {
			return fmt.Errorf("kernel: postcondition %s table_exists missing table", condition.Name)
		}
		var count int
		if err := executor.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", spec.Table).Scan(&count); err != nil {
			return fmt.Errorf("kernel: postcondition %s table_exists query: %w", condition.Name, err)
		}
		if count == 0 {
			return fmt.Errorf("kernel: postcondition %s table_exists: table %s not found", condition.Name, spec.Table)
		}
		return nil
	case "column_exists":
		var spec struct {
			Table  string `json:"table"`
			Column string `json:"column"`
		}
		if err := json.Unmarshal(condition.Expected, &spec); err != nil {
			return fmt.Errorf("kernel: postcondition %s column_exists unmarshal: %w", condition.Name, err)
		}
		if spec.Table == "" || spec.Column == "" {
			return fmt.Errorf("kernel: postcondition %s column_exists missing table or column", condition.Name)
		}
		var count int
		if err := executor.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", spec.Table, spec.Column).Scan(&count); err != nil {
			return fmt.Errorf("kernel: postcondition %s column_exists query: %w", condition.Name, err)
		}
		if count == 0 {
			return fmt.Errorf("kernel: postcondition %s column_exists: column %s not found in %s", condition.Name, spec.Column, spec.Table)
		}
		return nil
	case "schema_version":
		var spec struct {
			Version int64 `json:"version"`
		}
		if err := json.Unmarshal(condition.Expected, &spec); err != nil {
			return fmt.Errorf("kernel: postcondition %s schema_version unmarshal: %w", condition.Name, err)
		}
		var actual int64
		if err := executor.QueryRowContext(ctx, "PRAGMA user_version").Scan(&actual); err != nil {
			return fmt.Errorf("kernel: postcondition %s schema_version query: %w", condition.Name, err)
		}
		if actual != spec.Version {
			return fmt.Errorf("kernel: postcondition %s schema_version: expected %d, got %d", condition.Name, spec.Version, actual)
		}
		return nil
	default:
		if !bytes.Equal(condition.Expected, condition.Actual) {
			return fmt.Errorf("kernel: postcondition %s: expected %s, got %s", condition.Name, string(condition.Expected), string(condition.Actual))
		}
		return nil
	}
}

func (pmr *packageMigrationRuntime) handler() migration.ReversibleStepHandler {
	return func(ctx context.Context, step migration.ReversiblePlanStep, def migration.MigrationDefinition) (migration.ReversibleStepResult, error) {
		if err := ctx.Err(); err != nil {
			return migration.ReversibleStepResult{}, err
		}
		if err := pmr.persistDefinition(ctx, def); err != nil {
			return migration.ReversibleStepResult{}, err
		}
		evidence, err := pmr.executeScript(ctx, step, def)
		if err != nil {
			return migration.ReversibleStepResult{}, err
		}
		return migration.ReversibleStepResult{Evidence: evidence}, nil
	}
}
