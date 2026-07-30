package kernel

import (
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
	statements := splitSQLStatements(string(scriptContent))
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
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return nil, fmt.Errorf("kernel: migration sql begin immediate: %w", err)
	}
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
		postResult, postErr := migration.NewPreconditionValidator().ValidatePostconditions(ctx, def.Postcondition)
		if postErr != nil {
			return nil, fmt.Errorf("kernel: migration sql postcondition validate %s: %w", def.MigrationID, postErr)
		}
		if !postResult.Passed {
			return nil, fmt.Errorf("kernel: migration sql postconditions failed for %s: %v", def.MigrationID, postResult.Errors)
		}
	}
	baseEvidence["executionMode"] = "sql_executed"
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
		postResult, postErr := migration.NewPreconditionValidator().ValidatePostconditions(ctx, def.Postcondition)
		if postErr != nil {
			return nil, fmt.Errorf("kernel: migration javascript postcondition validate %s: %w", def.MigrationID, postErr)
		}
		if !postResult.Passed {
			return nil, fmt.Errorf("kernel: migration javascript postconditions failed for %s: %v", def.MigrationID, postResult.Errors)
		}
	}
	resultJSON, _ := json.Marshal(invokeResult)
	baseEvidence["executionMode"] = "javascript_executed"
	baseEvidence["function"] = functionName
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

func splitSQLStatements(content string) []string {
	rawStatements := strings.Split(content, ";")
	var statements []string
	for _, raw := range rawStatements {
		lines := strings.Split(raw, "\n")
		var kept []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "--") {
				continue
			}
			kept = append(kept, line)
		}
		stmt := strings.TrimSpace(strings.Join(kept, "\n"))
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}
	return statements
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
