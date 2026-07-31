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
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/javascript_main"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/runtime"
)

var ErrPackageMigrationTypeUnsupported = errors.New("PACKAGE_MIGRATION_TYPE_UNSUPPORTED")

var forbiddenMigrationTableRegex = regexp.MustCompile(`(?i)\b(users|characters|messages|package_operations|package_installations|schema_migrations|extension_migration_definitions|extension_migration_operations|extension_migration_steps|extension_migration_checkpoints|extension_data_snapshots|extension_snapshot_entries|sqlite_master|sqlite_sequence|sqlite_schema)\b`)

var forbiddenSQLCommandRegex = regexp.MustCompile(`(?i)\b(ATTACH|DETACH|load_extension|CREATE\s+VIRTUAL\s+TABLE|CREATE\s+TRIGGER|CREATE\s+VIEW|CREATE\s+MACRO)\b|VACUUM\s+INTO|PRAGMA\s+writable_schema|PRAGMA\s+schema_version\s*=|PRAGMA\s+auto_vacuum\s*=`)

var tableReferenceRegex = regexp.MustCompile(`(?i)\b(?:CREATE\s+TABLE|ALTER\s+TABLE|INSERT\s+INTO|UPDATE|DELETE\s+FROM|DROP\s+TABLE|TRUNCATE\s+TABLE|REPLACE\s+INTO)\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?(?:["` + "`" + `\[])?([A-Za-z_][A-Za-z0-9_]*)`)

var tableFromJoinRegex = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+(?:["` + "`" + `\[])?([A-Za-z_][A-Za-z0-9_]*)`)

var systemTableRegex = regexp.MustCompile(`(?i)^(sqlite_master|sqlite_sequence|sqlite_schema|sqlite_stat\d*|sqlite_temp_master|sqlite_temp_schema)$`)

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
	if def.RuntimeType == "config" || def.RuntimeType == "resource" || def.RuntimeType == "userdata" {
		var scriptContent []byte
		if def.Entry != "" {
			if content, err := pmr.readMigrationScript(def); err == nil {
				scriptContent = content
			}
		}
		baseEvidence := pmr.buildBaseEvidence(step, def, scriptContent)
		switch def.RuntimeType {
		case "config":
			return pmr.executeConfigMigration(ctx, step, def, baseEvidence, scriptContent)
		case "resource":
			return pmr.executeResourceMigration(ctx, step, def, baseEvidence, scriptContent)
		case "userdata":
			return pmr.executeUserDataMigration(ctx, step, def, baseEvidence, scriptContent)
		}
	}
	scriptContent, err := pmr.readMigrationScript(def)
	if err != nil {
		return nil, err
	}
	baseEvidence := pmr.buildBaseEvidence(step, def, scriptContent)
	switch def.RuntimeType {
	case "sql":
		return pmr.executeSQLMigration(ctx, step, def, scriptContent, baseEvidence)
	case "javascript":
		return pmr.executeJavaScriptMigration(ctx, step, def, baseEvidence)
	default:
		return nil, fmt.Errorf("kernel: migration %s: runtime type %q: %w", def.MigrationID, def.RuntimeType, ErrPackageMigrationTypeUnsupported)
	}
}

func (pmr *packageMigrationRuntime) buildBaseEvidence(step migration.ReversiblePlanStep, def migration.MigrationDefinition, scriptContent []byte) map[string]any {
	evidence := map[string]any{
		"migrationId":    step.MigrationID,
		"direction":      step.Direction,
		"definitionHash": def.DefinitionHash,
		"entry":          def.Entry,
		"executedAt":     time.Now().UTC().Format(time.RFC3339Nano),
		"runtimeType":    def.RuntimeType,
		"extensionId":    pmr.extensionID,
		"packageOpId":    pmr.packageOpID,
	}
	if scriptContent != nil {
		scriptSum := sha256.Sum256(scriptContent)
		evidence["scriptSize"] = len(scriptContent)
		evidence["scriptHash"] = hex.EncodeToString(scriptSum[:])
	}
	return evidence
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
	migrationResourceLimits := javascript_main.CreateHostRequest{
		ExtensionID:          pmr.extensionID,
		ModuleID:             def.ModuleID,
		Entry:                def.Entry,
		DefinitionHash:       def.DefinitionHash,
		AllowedContributions: []javascript_main.AllowedContribution{},
		ResourceLimits: runtime.ResourceLimits{
			MaxMemoryMB:        64,
			MaxConcurrentCalls: 1,
			MaxQueueDepth:      1,
			SingleCallTimeout:  "10s",
			LogRatePerSecond:   5,
			HostAPIRatePerSec:  3,
			MaxOpenHandles:     8,
			MaxMessageSizeKB:   128,
		},
		NetworkDisabled: true,
		Env: []string{
			"AMITIA_MIGRATION_SANDBOX=1",
			"AMITIA_NETWORK_DISABLED=1",
			"AMITIA_FS_DISABLED=1",
			"AMITIA_MIGRATION_RUNTIME=1",
			"AMITIA_HOST_API_ALLOWLIST=migration.*",
			"AMITIA_PROCESS_ISOLATION=strict",
			"NODE_OPTIONS=--no-experimental-fetch --disable-network-imports --no-experimental-global-navigator --no-experimental-global-customevent",
		},
	}
	host, createErr := pmr.runtime.container.JSRuntimeFactory.Create(ctx, migrationResourceLimits)
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

type MigrationExecutor interface {
	Prepare(ctx context.Context) error
	Execute(ctx context.Context) error
	Verify(ctx context.Context) error
	Compensate(ctx context.Context) error
}

type migrationExecutorBase struct {
	runtime       *Runtime
	extensionID   string
	step          migration.ReversiblePlanStep
	def           migration.MigrationDefinition
	scriptContent []byte
}

func (e *migrationExecutorBase) executeScriptSQL(ctx context.Context) (int, error) {
	if len(e.scriptContent) == 0 {
		return 0, nil
	}
	if e.runtime == nil || e.runtime.container == nil || e.runtime.container.Store == nil {
		return 0, fmt.Errorf("kernel: migration executor: store unavailable")
	}
	db := e.runtime.container.Store.DB()
	if db == nil {
		return 0, fmt.Errorf("kernel: migration executor: database unavailable")
	}
	statements := splitSQLStatementsSafe(string(e.scriptContent))
	if len(statements) == 0 {
		return 0, nil
	}
	for _, stmt := range statements {
		if match := forbiddenMigrationTableRegex.FindString(stmt); match != "" {
			return 0, fmt.Errorf("kernel: migration executor: sql references forbidden host table %q", strings.ToLower(match))
		}
		if match := forbiddenSQLCommandRegex.FindString(stmt); match != "" {
			return 0, fmt.Errorf("kernel: migration executor: sql uses forbidden command %q", strings.ToUpper(match))
		}
		if err := validateExtNamespace(stmt); err != nil {
			return 0, fmt.Errorf("kernel: migration executor: sql namespace violation: %w", err)
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("kernel: migration executor: begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	var executed int
	for _, stmt := range statements {
		if _, execErr := tx.ExecContext(ctx, stmt); execErr != nil {
			return 0, fmt.Errorf("kernel: migration executor: execute sql: %w", execErr)
		}
		executed++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("kernel: migration executor: commit: %w", err)
	}
	committed = true
	return executed, nil
}

type ConfigMigrationExecutor struct {
	migrationExecutorBase
	snapshotContributions     []domain.ContributionDefinition
	snapshotRequirements      []sqlite.PermissionRequirement
	snapshotGrants            []sqlite.PermissionGrant
	scriptStatementsExecuted  int
}

func (e *ConfigMigrationExecutor) Prepare(ctx context.Context) error {
	if e.runtime == nil || e.runtime.container == nil {
		return fmt.Errorf("kernel: config migration executor: container unavailable")
	}
	extID := domain.ExtensionID(e.extensionID)
	contribs, err := e.runtime.container.ContributionRepository.ListContributions(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: config migration prepare: list contributions: %w", err)
	}
	e.snapshotContributions = contribs
	reqs, err := e.runtime.container.PermissionRepository.ListRequirements(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: config migration prepare: list requirements: %w", err)
	}
	e.snapshotRequirements = reqs
	grants, err := e.runtime.container.PermissionRepository.ListGrants(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: config migration prepare: list grants: %w", err)
	}
	e.snapshotGrants = grants
	return nil
}

func (e *ConfigMigrationExecutor) Execute(ctx context.Context) error {
	if e.step.Direction == migration.DirectionReverse {
		return e.Compensate(ctx)
	}
	executed, err := e.executeScriptSQL(ctx)
	if err != nil {
		return fmt.Errorf("kernel: config migration execute script: %w", err)
	}
	e.scriptStatementsExecuted = executed
	return nil
}

func (e *ConfigMigrationExecutor) Verify(ctx context.Context) error {
	if e.runtime == nil || e.runtime.container == nil {
		return fmt.Errorf("kernel: config migration verify: container unavailable")
	}
	extID := domain.ExtensionID(e.extensionID)
	contribs, err := e.runtime.container.ContributionRepository.ListContributions(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: config migration verify: list contributions: %w", err)
	}
	if len(contribs) == 0 && len(e.snapshotContributions) > 0 {
		return fmt.Errorf("kernel: config migration verify: contributions missing after migration")
	}
	return nil
}

func (e *ConfigMigrationExecutor) Compensate(ctx context.Context) error {
	if e.runtime == nil || e.runtime.container == nil {
		return fmt.Errorf("kernel: config migration compensate: container unavailable")
	}
	extID := domain.ExtensionID(e.extensionID)
	if err := e.runtime.container.ContributionRepository.DeleteContributions(ctx, extID); err != nil {
		return fmt.Errorf("kernel: config migration compensate: delete contributions: %w", err)
	}
	for _, contrib := range e.snapshotContributions {
		if err := e.runtime.container.ContributionRepository.PutContribution(ctx, contrib); err != nil {
			return fmt.Errorf("kernel: config migration compensate: restore contribution %s: %w", contrib.ID, err)
		}
	}
	if err := e.runtime.container.PermissionRepository.DeleteRequirements(ctx, extID); err != nil {
		return fmt.Errorf("kernel: config migration compensate: delete requirements: %w", err)
	}
	for _, req := range e.snapshotRequirements {
		if err := e.runtime.container.PermissionRepository.PutRequirement(ctx, req); err != nil {
			return fmt.Errorf("kernel: config migration compensate: restore requirement %s: %w", req.PermissionName, err)
		}
	}
	currentGrants, err := e.runtime.container.PermissionRepository.ListGrants(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: config migration compensate: list grants: %w", err)
	}
	for _, grant := range currentGrants {
		if err := e.runtime.container.PermissionRepository.DeleteGrant(ctx, extID, grant.PermissionName); err != nil {
			return fmt.Errorf("kernel: config migration compensate: delete grant %s: %w", grant.PermissionName, err)
		}
	}
	for _, grant := range e.snapshotGrants {
		if err := e.runtime.container.PermissionRepository.PutGrant(ctx, grant); err != nil {
			return fmt.Errorf("kernel: config migration compensate: restore grant %s: %w", grant.PermissionName, err)
		}
	}
	return nil
}

type ResourceMigrationExecutor struct {
	migrationExecutorBase
	snapshotResources        []domain.ResourceOwnership
	scriptStatementsExecuted int
}

func (e *ResourceMigrationExecutor) Prepare(ctx context.Context) error {
	if e.runtime == nil || e.runtime.container == nil || e.runtime.container.ResourceRepository == nil {
		return fmt.Errorf("kernel: resource migration executor: resource repository unavailable")
	}
	resources, err := e.runtime.container.ResourceRepository.ListResources(ctx, domain.ExtensionID(e.extensionID))
	if err != nil {
		return fmt.Errorf("kernel: resource migration prepare: list resources: %w", err)
	}
	e.snapshotResources = resources
	return nil
}

func (e *ResourceMigrationExecutor) Execute(ctx context.Context) error {
	if e.step.Direction == migration.DirectionReverse {
		return e.Compensate(ctx)
	}
	executed, err := e.executeScriptSQL(ctx)
	if err != nil {
		return fmt.Errorf("kernel: resource migration execute script: %w", err)
	}
	e.scriptStatementsExecuted = executed
	return nil
}

func (e *ResourceMigrationExecutor) Verify(ctx context.Context) error {
	if e.runtime == nil || e.runtime.container == nil || e.runtime.container.ResourceRepository == nil {
		return fmt.Errorf("kernel: resource migration verify: resource repository unavailable")
	}
	resources, err := e.runtime.container.ResourceRepository.ListResources(ctx, domain.ExtensionID(e.extensionID))
	if err != nil {
		return fmt.Errorf("kernel: resource migration verify: list resources: %w", err)
	}
	if len(resources) == 0 && len(e.snapshotResources) > 0 {
		return fmt.Errorf("kernel: resource migration verify: resources missing after migration")
	}
	return nil
}

func (e *ResourceMigrationExecutor) Compensate(ctx context.Context) error {
	if e.runtime == nil || e.runtime.container == nil || e.runtime.container.ResourceRepository == nil {
		return fmt.Errorf("kernel: resource migration compensate: resource repository unavailable")
	}
	extID := domain.ExtensionID(e.extensionID)
	current, err := e.runtime.container.ResourceRepository.ListResources(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: resource migration compensate: list current: %w", err)
	}
	for _, resource := range current {
		if err := e.runtime.container.ResourceRepository.DeleteResource(ctx, resource.ResourceID); err != nil {
			return fmt.Errorf("kernel: resource migration compensate: delete resource %s: %w", resource.ResourceID, err)
		}
	}
	for _, resource := range e.snapshotResources {
		if err := e.runtime.container.ResourceRepository.PutResource(ctx, resource); err != nil {
			return fmt.Errorf("kernel: resource migration compensate: restore resource %s: %w", resource.ResourceID, err)
		}
	}
	return nil
}

type UserDataMigrationExecutor struct {
	migrationExecutorBase
	jsonlExports             map[string]string
	scriptStatementsExecuted int
}

func (e *UserDataMigrationExecutor) Prepare(ctx context.Context) error {
	if e.runtime == nil || e.runtime.container == nil || e.runtime.container.Store == nil {
		return fmt.Errorf("kernel: userdata migration executor: store unavailable")
	}
	e.jsonlExports = make(map[string]string)
	db := e.runtime.container.Store.DB()
	if db == nil {
		return fmt.Errorf("kernel: userdata migration prepare: database unavailable")
	}
	for _, dd := range e.def.DataDomains {
		if dd.Namespace == "" {
			continue
		}
		rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", dd.Namespace))
		if err != nil {
			continue
		}
		export, exportErr := exportRowsAsJSONL(rows)
		_ = rows.Close()
		if exportErr != nil {
			continue
		}
		e.jsonlExports[dd.Namespace] = export
	}
	return nil
}

func (e *UserDataMigrationExecutor) Execute(ctx context.Context) error {
	if e.step.Direction == migration.DirectionReverse {
		return e.Compensate(ctx)
	}
	executed, err := e.executeScriptSQL(ctx)
	if err != nil {
		return fmt.Errorf("kernel: userdata migration execute script: %w", err)
	}
	e.scriptStatementsExecuted = executed
	return nil
}

func (e *UserDataMigrationExecutor) Verify(ctx context.Context) error {
	if e.runtime == nil || e.runtime.container == nil || e.runtime.container.Store == nil {
		return fmt.Errorf("kernel: userdata migration verify: store unavailable")
	}
	db := e.runtime.container.Store.DB()
	if db == nil {
		return fmt.Errorf("kernel: userdata migration verify: database unavailable")
	}
	for _, dd := range e.def.DataDomains {
		if dd.Namespace == "" {
			continue
		}
		var count int
		if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", dd.Namespace)).Scan(&count); err != nil {
			return fmt.Errorf("kernel: userdata migration verify: count %s: %w", dd.Namespace, err)
		}
	}
	return nil
}

func (e *UserDataMigrationExecutor) Compensate(ctx context.Context) error {
	if e.runtime == nil || e.runtime.container == nil || e.runtime.container.Store == nil {
		return fmt.Errorf("kernel: userdata migration compensate: store unavailable")
	}
	db := e.runtime.container.Store.DB()
	if db == nil {
		return fmt.Errorf("kernel: userdata migration compensate: database unavailable")
	}
	for table, jsonlData := range e.jsonlExports {
		if jsonlData == "" {
			continue
		}
		records, err := parseJSONL(jsonlData)
		if err != nil {
			continue
		}
		if len(records) == 0 {
			continue
		}
		_, _ = db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table))
		columns := extractColumnsFromRecords(records[0])
		if len(columns) == 0 {
			continue
		}
		placeholders := make([]string, len(columns))
		for i := range columns {
			placeholders[i] = "?"
		}
		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
		for _, record := range records {
			values := make([]interface{}, len(columns))
			for i, col := range columns {
				values[i] = record[col]
			}
			_, _ = db.ExecContext(ctx, query, values...)
		}
	}
	return nil
}

func (pmr *packageMigrationRuntime) executeConfigMigration(ctx context.Context, step migration.ReversiblePlanStep, def migration.MigrationDefinition, baseEvidence map[string]any, scriptContent []byte) (json.RawMessage, error) {
	if pmr.runtime.container == nil || pmr.runtime.container.ContributionRepository == nil || pmr.runtime.container.PermissionRepository == nil {
		return nil, fmt.Errorf("kernel: migration config repositories unavailable for %s", def.MigrationID)
	}
	executor := &ConfigMigrationExecutor{
		migrationExecutorBase: migrationExecutorBase{
			runtime:       pmr.runtime,
			extensionID:   pmr.extensionID,
			step:          step,
			def:           def,
			scriptContent: scriptContent,
		},
	}
	if err := executor.Prepare(ctx); err != nil {
		return nil, fmt.Errorf("kernel: migration config prepare %s: %w", def.MigrationID, err)
	}
	if err := executor.Execute(ctx); err != nil {
		_ = executor.Compensate(ctx)
		return nil, fmt.Errorf("kernel: migration config execute %s: %w", def.MigrationID, err)
	}
	if err := executor.Verify(ctx); err != nil {
		_ = executor.Compensate(ctx)
		return nil, fmt.Errorf("kernel: migration config verify %s: %w", def.MigrationID, err)
	}
	baseEvidence["executionMode"] = "config_executed"
	baseEvidence["executor_type"] = "config"
	baseEvidence["snapshotContributions"] = len(executor.snapshotContributions)
	baseEvidence["snapshotRequirements"] = len(executor.snapshotRequirements)
	baseEvidence["snapshotGrants"] = len(executor.snapshotGrants)
	baseEvidence["scriptStatementsExecuted"] = executor.scriptStatementsExecuted
	evidenceJSON, marshalErr := json.Marshal(baseEvidence)
	if marshalErr != nil {
		return nil, fmt.Errorf("kernel: migration config evidence marshal: %w", marshalErr)
	}
	return evidenceJSON, nil
}

func (pmr *packageMigrationRuntime) executeResourceMigration(ctx context.Context, step migration.ReversiblePlanStep, def migration.MigrationDefinition, baseEvidence map[string]any, scriptContent []byte) (json.RawMessage, error) {
	if pmr.runtime.container == nil || pmr.runtime.container.ResourceRepository == nil {
		return nil, fmt.Errorf("kernel: migration resource repository unavailable for %s", def.MigrationID)
	}
	executor := &ResourceMigrationExecutor{
		migrationExecutorBase: migrationExecutorBase{
			runtime:       pmr.runtime,
			extensionID:   pmr.extensionID,
			step:          step,
			def:           def,
			scriptContent: scriptContent,
		},
	}
	if err := executor.Prepare(ctx); err != nil {
		return nil, fmt.Errorf("kernel: migration resource prepare %s: %w", def.MigrationID, err)
	}
	if err := executor.Execute(ctx); err != nil {
		_ = executor.Compensate(ctx)
		return nil, fmt.Errorf("kernel: migration resource execute %s: %w", def.MigrationID, err)
	}
	if err := executor.Verify(ctx); err != nil {
		_ = executor.Compensate(ctx)
		return nil, fmt.Errorf("kernel: migration resource verify %s: %w", def.MigrationID, err)
	}
	baseEvidence["executionMode"] = "resource_executed"
	baseEvidence["executor_type"] = "resource"
	baseEvidence["snapshotResources"] = len(executor.snapshotResources)
	baseEvidence["scriptStatementsExecuted"] = executor.scriptStatementsExecuted
	evidenceJSON, marshalErr := json.Marshal(baseEvidence)
	if marshalErr != nil {
		return nil, fmt.Errorf("kernel: migration resource evidence marshal: %w", marshalErr)
	}
	return evidenceJSON, nil
}

func (pmr *packageMigrationRuntime) executeUserDataMigration(ctx context.Context, step migration.ReversiblePlanStep, def migration.MigrationDefinition, baseEvidence map[string]any, scriptContent []byte) (json.RawMessage, error) {
	if pmr.runtime.container == nil || pmr.runtime.container.Store == nil {
		return nil, fmt.Errorf("kernel: migration userdata store unavailable for %s", def.MigrationID)
	}
	executor := &UserDataMigrationExecutor{
		migrationExecutorBase: migrationExecutorBase{
			runtime:       pmr.runtime,
			extensionID:   pmr.extensionID,
			step:          step,
			def:           def,
			scriptContent: scriptContent,
		},
	}
	if err := executor.Prepare(ctx); err != nil {
		return nil, fmt.Errorf("kernel: migration userdata prepare %s: %w", def.MigrationID, err)
	}
	if err := executor.Execute(ctx); err != nil {
		_ = executor.Compensate(ctx)
		return nil, fmt.Errorf("kernel: migration userdata execute %s: %w", def.MigrationID, err)
	}
	if err := executor.Verify(ctx); err != nil {
		_ = executor.Compensate(ctx)
		return nil, fmt.Errorf("kernel: migration userdata verify %s: %w", def.MigrationID, err)
	}
	baseEvidence["executionMode"] = "userdata_executed"
	baseEvidence["executor_type"] = "userdata"
	baseEvidence["jsonlExportTables"] = len(executor.jsonlExports)
	baseEvidence["scriptStatementsExecuted"] = executor.scriptStatementsExecuted
	evidenceJSON, marshalErr := json.Marshal(baseEvidence)
	if marshalErr != nil {
		return nil, fmt.Errorf("kernel: migration userdata evidence marshal: %w", marshalErr)
	}
	return evidenceJSON, nil
}

func exportRowsAsJSONL(rows *sql.Rows) (string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return "", err
		}
		record := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				record[col] = string(b)
			} else {
				record[col] = val
			}
		}
		data, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			return "", marshalErr
		}
		builder.Write(data)
		builder.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func parseJSONL(data string) ([]map[string]interface{}, error) {
	var records []map[string]interface{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

func extractColumnsFromRecords(record map[string]interface{}) []string {
	columns := make([]string, 0, len(record))
	for col := range record {
		columns = append(columns, col)
	}
	sort.Strings(columns)
	return columns
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
	joinMatches := tableFromJoinRegex.FindAllStringSubmatch(sanitized, -1)
	var refs []string
	seen := map[string]bool{}
	for _, m := range matches {
		if len(m) > 1 && m[1] != "" {
			key := strings.ToLower(m[1])
			if !seen[key] {
				seen[key] = true
				refs = append(refs, m[1])
			}
		}
	}
	for _, m := range joinMatches {
		if len(m) > 1 && m[1] != "" {
			key := strings.ToLower(m[1])
			if !seen[key] {
				seen[key] = true
				refs = append(refs, m[1])
			}
		}
	}
	return refs
}

func validateExtNamespace(stmt string) error {
	refs := extractTableReferences(stmt)
	for _, ref := range refs {
		lower := strings.ToLower(ref)
		if systemTableRegex.MatchString(lower) {
			return fmt.Errorf("kernel: migration table %q is a system table", ref)
		}
		if !strings.HasPrefix(lower, "ext_") {
			return fmt.Errorf("kernel: migration table %q must use ext_ prefix", ref)
		}
	}
	return nil
}

func isExtNamespaceTable(name string) bool {
	lower := strings.ToLower(name)
	if systemTableRegex.MatchString(lower) {
		return false
	}
	return strings.HasPrefix(lower, "ext_")
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
