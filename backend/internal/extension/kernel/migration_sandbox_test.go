package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func newMigrationSandboxTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func makeMigrationCallRequest(extID string, input map[string]any) host_api.CallRequest {
	inputJSON, _ := json.Marshal(input)
	return host_api.CallRequest{
		CallID: "test-call",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			ExtensionID: domain.ExtensionID(extID),
			ModuleID:    "test-module",
		},
		Method:  host_api.MethodMigrationSQLExecute,
		Version: 1,
		Input:   inputJSON,
	}
}

func makeMigrationQueryCallRequest(extID string, input map[string]any) host_api.CallRequest {
	req := makeMigrationCallRequest(extID, input)
	req.Method = host_api.MethodMigrationSQLQuery
	return req
}

func TestMigrationSQLExecuteHandler_CreateExtTable(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.my-ext"

	req := makeMigrationCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("CREATE TABLE ext_%s_settings (id INTEGER PRIMARY KEY, key TEXT, value TEXT);",
			migration.NormalizeExtensionID(extID)),
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error.Message)
	}

	var output map[string]any
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatal(err)
	}
	if output["ok"] != true {
		t.Errorf("expected ok=true, got %v", output["ok"])
	}
	if output["statementsExecuted"].(float64) != 1 {
		t.Errorf("expected 1 statement, got %v", output["statementsExecuted"])
	}

	var tableName string
	prefix := migration.ExtensionNamespacePrefix(extID)
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name LIKE ?", prefix+"%").
		Scan(&tableName)
	if err != nil {
		t.Fatalf("table not found: %v", err)
	}
	expectedName := prefix + "settings"
	if tableName != expectedName {
		t.Errorf("expected table %s, got %s", expectedName, tableName)
	}
}

func TestMigrationSQLExecuteHandler_InsertAndQuery(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "org.test.extension"

	nsPrefix := migration.ExtensionNamespacePrefix(extID)
	createSQL := fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY, name TEXT);", nsPrefix)
	insertSQL := fmt.Sprintf("INSERT INTO %sdata (name) VALUES ('alice'), ('bob');", nsPrefix)

	req := makeMigrationCallRequest(extID, map[string]any{
		"sql": createSQL + "\n" + insertSQL,
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error.Message)
	}

	output := parseResultOutput(t, result.Output)
	if output["rowsAffected"].(float64) != 2 {
		t.Errorf("expected 2 rows affected, got %v", output["rowsAffected"])
	}

	var count int
	err = db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %sdata", nsPrefix)).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows, got %d", count)
	}
}

func TestMigrationSQLExecuteHandler_RejectHostTable(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	_, _ = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")

	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.evil.attacker"

	req := makeMigrationCallRequest(extID, map[string]any{
		"sql": "INSERT INTO users (name) VALUES ('hacked');",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure for host table access")
	}
	if result.Error.Code != PackageErrCodeMigrationNamespaceViolation {
		t.Errorf("expected error code %s, got %s", PackageErrCodeMigrationNamespaceViolation, result.Error.Code)
	}
}

func TestMigrationSQLExecuteHandler_RejectOtherExtensionTable(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	otherExtID := "org.other.extension"
	otherPrefix := migration.ExtensionNamespacePrefix(otherExtID)
	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY, secret TEXT);", otherPrefix))

	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.evil.attacker"

	req := makeMigrationCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("INSERT INTO %sdata (secret) VALUES ('stolen');", otherPrefix),
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure for cross-extension access")
	}
	if result.Error.Code != PackageErrCodeMigrationNamespaceViolation {
		t.Errorf("expected namespace violation, got %s", result.Error.Code)
	}
}

func TestMigrationSQLExecuteHandler_RejectSystemTable(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.test"

	tests := []string{
		"SELECT * FROM sqlite_master;",
		"SELECT * FROM sqlite_sequence;",
		"INSERT INTO sqlite_sequence VALUES ('test', 1);",
	}

	for _, sqlStmt := range tests {
		req := makeMigrationCallRequest(extID, map[string]any{"sql": sqlStmt})
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", sqlStmt, err)
		}
		if result.Status != host_api.StatusFailed {
			t.Errorf("expected failure for %q", sqlStmt)
		}
		if result.Error.Code != PackageErrCodeMigrationNamespaceViolation {
			t.Errorf("expected namespace violation for %q, got %s", sqlStmt, result.Error.Code)
		}
	}
}

func TestMigrationSQLExecuteHandler_RejectForbiddenCommands(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.test"

	tests := []string{
		"ATTACH DATABASE '/tmp/evil.db' AS evil;",
		"DETACH DATABASE evil;",
		"VACUUM INTO '/tmp/evil.db';",
		"PRAGMA writable_schema = 1;",
	}

	for _, sqlStmt := range tests {
		req := makeMigrationCallRequest(extID, map[string]any{"sql": sqlStmt})
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", sqlStmt, err)
		}
		if result.Status != host_api.StatusFailed {
			t.Errorf("expected failure for %q", sqlStmt)
		}
		if result.Error.Code != PackageErrCodeMigrationNamespaceViolation {
			t.Errorf("expected namespace violation for %q, got %s: %s", sqlStmt, result.Error.Code, result.Error.Message)
		}
	}
}

func TestMigrationSQLExecuteHandler_EmptyExtensionID(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)

	req := makeMigrationCallRequest("", map[string]any{"sql": "CREATE TABLE test (id INTEGER);"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure")
	}
	if result.Error.Code != host_api.ErrorCodeInputInvalid {
		t.Errorf("expected input_invalid, got %s", result.Error.Code)
	}
}

func TestMigrationSQLExecuteHandler_EmptySQL(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)

	tests := []string{"", "   ", "\n\n\t"}

	for _, sqlStmt := range tests {
		req := makeMigrationCallRequest("com.example.test", map[string]any{"sql": sqlStmt})
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Status != host_api.StatusFailed {
			t.Fatal("expected failure for empty SQL")
		}
		if result.Error.Code != host_api.ErrorCodeInputInvalid {
			t.Errorf("expected input_invalid, got %s", result.Error.Code)
		}
	}
}

func TestMigrationSQLExecuteHandler_InvalidJSON(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)

	req := host_api.CallRequest{
		CallID: "test-call",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			ExtensionID: domain.ExtensionID("com.example.test"),
		},
		Input: json.RawMessage(`{invalid json`),
	}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure")
	}
	if result.Error.Code != host_api.ErrorCodeInputInvalid {
		t.Errorf("expected input_invalid, got %s", result.Error.Code)
	}
}

func TestMigrationSQLExecuteHandler_AlterTable(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	createReq := makeMigrationCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("CREATE TABLE %sitems (id INTEGER PRIMARY KEY, name TEXT);", nsPrefix),
	})
	if result, _ := handler(context.Background(), createReq); result.Status != host_api.StatusSuccess {
		t.Fatalf("create failed: %s", result.Error.Message)
	}

	alterReq := makeMigrationCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("ALTER TABLE %sitems ADD COLUMN description TEXT;", nsPrefix),
	})
	result, err := handler(context.Background(), alterReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("alter failed: %s", result.Error.Message)
	}
}

func TestMigrationSQLExecuteHandler_DropTable(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	createReq := makeMigrationCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("CREATE TABLE %stemp (id INTEGER PRIMARY KEY);", nsPrefix),
	})
	if result, _ := handler(context.Background(), createReq); result.Status != host_api.StatusSuccess {
		t.Fatalf("create failed: %s", result.Error.Message)
	}

	dropReq := makeMigrationCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("DROP TABLE %stemp;", nsPrefix),
	})
	result, err := handler(context.Background(), dropReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("drop failed: %s", result.Error.Message)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name LIKE ?", nsPrefix+"%").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 tables after drop, got %d", count)
	}
}

func TestMigrationSQLExecuteHandler_CreateIndex(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	sqlStmt := fmt.Sprintf("CREATE TABLE %sitems (id INTEGER PRIMARY KEY, name TEXT); CREATE INDEX %sidx_name ON %sitems(name);",
		nsPrefix, nsPrefix, nsPrefix)

	req := makeMigrationCallRequest(extID, map[string]any{"sql": sqlStmt})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error.Message)
	}
}

func TestMigrationSQLExecuteHandler_TransactionRollback(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	createReq := makeMigrationCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY, val TEXT);", nsPrefix),
	})
	if result, _ := handler(context.Background(), createReq); result.Status != host_api.StatusSuccess {
		t.Fatalf("create failed: %s", result.Error.Message)
	}

	badReq := makeMigrationCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("INSERT INTO %sdata (val) VALUES ('ok'); INSERT INTO nonexistent_table VALUES (1);", nsPrefix),
	})
	result, err := handler(context.Background(), badReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure")
	}

	var count int
	err = db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %sdata", nsPrefix)).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}

func TestMigrationSQLExecuteHandler_SchemaHashDiffers(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	req := makeMigrationCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY);", nsPrefix),
	})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success: %s", result.Error.Message)
	}

	output := parseResultOutput(t, result.Output)
	beforeHash := output["beforeSchemaHash"].(string)
	afterHash := output["afterSchemaHash"].(string)
	if beforeHash == afterHash {
		t.Error("expected different schema hashes before and after table creation")
	}
}

func TestMigrationSQLQueryHandler_SelectFromExtTable(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY, name TEXT);", nsPrefix))
	_, _ = db.Exec(fmt.Sprintf("INSERT INTO %sdata (name) VALUES ('alice'), ('bob'), ('charlie');", nsPrefix))

	handler := createMigrationSQLQueryHandler(db)
	req := makeMigrationQueryCallRequest(extID, map[string]any{
		"sql":   fmt.Sprintf("SELECT * FROM %sdata ORDER BY id;", nsPrefix),
		"limit": 10,
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success: %s", result.Error.Message)
	}

	output := parseResultOutput(t, result.Output)
	rows := output["rows"].([]any)
	if len(rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(rows))
	}
}

func TestMigrationSQLQueryHandler_LimitEnforced(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY);", nsPrefix))
	for i := 0; i < 50; i++ {
		_, _ = db.Exec(fmt.Sprintf("INSERT INTO %sdata (id) VALUES (%d);", nsPrefix, i))
	}

	handler := createMigrationSQLQueryHandler(db)
	req := makeMigrationQueryCallRequest(extID, map[string]any{
		"sql":   fmt.Sprintf("SELECT * FROM %sdata;", nsPrefix),
		"limit": 5,
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success: %s", result.Error.Message)
	}

	output := parseResultOutput(t, result.Output)
	rows := output["rows"].([]any)
	if len(rows) != 5 {
		t.Errorf("expected 5 rows with limit, got %d", len(rows))
	}
}

func TestMigrationSQLQueryHandler_DefaultLimit(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY);", nsPrefix))
	for i := 0; i < 1200; i++ {
		_, _ = db.Exec(fmt.Sprintf("INSERT INTO %sdata (id) VALUES (%d);", nsPrefix, i))
	}

	handler := createMigrationSQLQueryHandler(db)
	req := makeMigrationQueryCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("SELECT * FROM %sdata;", nsPrefix),
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success: %s", result.Error.Message)
	}

	output := parseResultOutput(t, result.Output)
	rows := output["rows"].([]any)
	if len(rows) != maxMigrationQueryRows {
		t.Errorf("expected %d rows with default limit, got %d", maxMigrationQueryRows, len(rows))
	}
}

func TestMigrationSQLQueryHandler_RejectNonSelect(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLQueryHandler(db)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	tests := []string{
		fmt.Sprintf("INSERT INTO %sdata VALUES (1);", nsPrefix),
		fmt.Sprintf("UPDATE %sdata SET id = 2;", nsPrefix),
		fmt.Sprintf("DELETE FROM %sdata;", nsPrefix),
		fmt.Sprintf("DROP TABLE %sdata;", nsPrefix),
	}

	for _, sqlStmt := range tests {
		req := makeMigrationQueryCallRequest(extID, map[string]any{"sql": sqlStmt})
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", sqlStmt, err)
		}
		if result.Status != host_api.StatusFailed {
			t.Errorf("expected failure for %q", sqlStmt)
		}
		if result.Error.Code != PackageErrCodeMigrationSandboxViolation {
			t.Errorf("expected sandbox violation for %q, got %s", sqlStmt, result.Error.Code)
		}
	}
}

func TestMigrationSQLQueryHandler_RejectHostTable(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	_, _ = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")

	handler := createMigrationSQLQueryHandler(db)
	extID := "com.evil.attacker"

	req := makeMigrationQueryCallRequest(extID, map[string]any{
		"sql": "SELECT * FROM users;",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure for host table access")
	}
	if result.Error.Code != PackageErrCodeMigrationNamespaceViolation {
		t.Errorf("expected namespace violation, got %s", result.Error.Code)
	}
}

func TestMigrationSQLQueryHandler_RejectMultipleStatements(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)
	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY);", nsPrefix))

	handler := createMigrationSQLQueryHandler(db)
	req := makeMigrationQueryCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("SELECT * FROM %sdata; SELECT * FROM %sdata;", nsPrefix, nsPrefix),
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure for multiple statements")
	}
	if result.Error.Code != host_api.ErrorCodeInputInvalid {
		t.Errorf("expected input_invalid, got %s", result.Error.Code)
	}
}

func TestMigrationSQLQueryHandler_EmptyExtensionID(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLQueryHandler(db)

	req := makeMigrationQueryCallRequest("", map[string]any{"sql": "SELECT 1;"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure")
	}
	if result.Error.Code != host_api.ErrorCodeInputInvalid {
		t.Errorf("expected input_invalid, got %s", result.Error.Code)
	}
}

func TestMigrationSQLQueryHandler_EmptySQL(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLQueryHandler(db)

	req := makeMigrationQueryCallRequest("com.example.test", map[string]any{"sql": ""})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure for empty SQL")
	}
	if result.Error.Code != host_api.ErrorCodeInputInvalid {
		t.Errorf("expected input_invalid, got %s", result.Error.Code)
	}
}

func TestMigrationSQLQueryHandler_WithCTE(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY, val INTEGER);", nsPrefix))
	_, _ = db.Exec(fmt.Sprintf("INSERT INTO %sdata (val) VALUES (10), (20), (30);", nsPrefix))

	handler := createMigrationSQLQueryHandler(db)
	req := makeMigrationQueryCallRequest(extID, map[string]any{
		"sql": fmt.Sprintf("WITH %ssummary AS (SELECT SUM(val) as total FROM %sdata) SELECT total FROM %ssummary;", nsPrefix, nsPrefix, nsPrefix),
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success: %s", result.Error.Message)
	}

	output := parseResultOutput(t, result.Output)
	rows := output["rows"].([]any)
	if len(rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(rows))
	}
}

func TestSetupMigrationSandboxRoutes_Success(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	gateway := host_api.NewDefaultGateway()

	err := setupMigrationSandboxRoutes(gateway, MigrationSandboxDeps{DB: db})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	executeRoute, ok := gateway.QueryCapability(context.Background(), host_api.MethodMigrationSQLExecute)
	if !ok {
		t.Fatal("migration.sql.execute route not registered")
	}
	if executeRoute.RiskLevel != host_api.RiskHigh {
		t.Errorf("expected high risk, got %v", executeRoute.RiskLevel)
	}
	if executeRoute.SideEffectLevel != host_api.SideEffectWrite {
		t.Errorf("expected write side effect, got %v", executeRoute.SideEffectLevel)
	}

	queryRoute, ok := gateway.QueryCapability(context.Background(), host_api.MethodMigrationSQLQuery)
	if !ok {
		t.Fatal("migration.sql.query route not registered")
	}
	if queryRoute.RiskLevel != host_api.RiskLow {
		t.Errorf("expected low risk, got %v", queryRoute.RiskLevel)
	}
	if queryRoute.SideEffectLevel != host_api.SideEffectReadOnly {
		t.Errorf("expected read-only side effect, got %v", queryRoute.SideEffectLevel)
	}
}

func TestSetupMigrationSandboxRoutes_NilDB(t *testing.T) {
	gateway := host_api.NewDefaultGateway()

	err := setupMigrationSandboxRoutes(gateway, MigrationSandboxDeps{DB: nil})
	if err == nil {
		t.Fatal("expected error for nil DB")
	}
}

func TestComputeMigrationSandboxSchemaHash_Empty(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	hash, err := computeMigrationSandboxSchemaHash(context.Background(), conn, "com.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if len(hash) < 10 {
		t.Errorf("expected hash with prefix, got %s", hash)
	}
}

func TestComputeMigrationSandboxSchemaHash_WithTables(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %sa (id INTEGER PRIMARY KEY);", nsPrefix))
	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %sb (id INTEGER PRIMARY KEY);", nsPrefix))

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	hash1, err := computeMigrationSandboxSchemaHash(context.Background(), conn, extID)
	conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %sc (id INTEGER PRIMARY KEY);", nsPrefix))

	conn2, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()

	hash2, err := computeMigrationSandboxSchemaHash(context.Background(), conn2, extID)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 == hash2 {
		t.Error("expected different hashes after schema change")
	}
}

func TestComputeMigrationSandboxSchemaHash_IsolationBetweenExtensions(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	extA := "com.example.extA"
	extB := "com.example.extB"
	prefixA := migration.ExtensionNamespacePrefix(extA)
	prefixB := migration.ExtensionNamespacePrefix(extB)

	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %stableA (id INTEGER PRIMARY KEY);", prefixA))
	_, _ = db.Exec(fmt.Sprintf("CREATE TABLE %stableB (id INTEGER PRIMARY KEY);", prefixB))

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	hashA, err := computeMigrationSandboxSchemaHash(context.Background(), conn, extA)
	conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	conn2, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()

	hashB, err := computeMigrationSandboxSchemaHash(context.Background(), conn2, extB)
	if err != nil {
		t.Fatal(err)
	}

	if hashA == hashB {
		t.Error("expected different hashes for different extensions")
	}
}

func TestMigrationSQLExecuteHandler_MultipleStatements(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	sqlStmt := fmt.Sprintf(`
		CREATE TABLE %susers (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE %ssettings (id INTEGER PRIMARY KEY, key TEXT UNIQUE, value TEXT);
		INSERT INTO %susers (name) VALUES ('admin');
		INSERT INTO %ssettings (key, value) VALUES ('theme', 'dark');
	`, nsPrefix, nsPrefix, nsPrefix, nsPrefix)

	req := makeMigrationCallRequest(extID, map[string]any{"sql": sqlStmt})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success: %s", result.Error.Message)
	}

	output := parseResultOutput(t, result.Output)
	if output["statementsExecuted"].(float64) != 4 {
		t.Errorf("expected 4 statements, got %v", output["statementsExecuted"])
	}
	if output["rowsAffected"].(float64) != 2 {
		t.Errorf("expected 2 rows affected, got %v", output["rowsAffected"])
	}
}

func TestMigrationSQLExecuteHandler_CreateView(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	sqlStmt := fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY, status TEXT); CREATE VIEW %sactive AS SELECT * FROM %sdata WHERE status = 'active';",
		nsPrefix, nsPrefix, nsPrefix)

	req := makeMigrationCallRequest(extID, map[string]any{"sql": sqlStmt})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure for CREATE VIEW")
	}
	if result.Error.Code != PackageErrCodeMigrationNamespaceViolation {
		t.Errorf("expected namespace violation, got %s: %s", result.Error.Code, result.Error.Message)
	}
}

func TestMigrationSQLExecuteHandler_CreateTrigger(t *testing.T) {
	db := newMigrationSandboxTestDB(t)
	handler := createMigrationSQLExecuteHandler(db)
	extID := "com.example.test"
	nsPrefix := migration.ExtensionNamespacePrefix(extID)

	sqlStmt := fmt.Sprintf("CREATE TABLE %sdata (id INTEGER PRIMARY KEY); CREATE TRIGGER %strg BEFORE INSERT ON %sdata BEGIN SELECT 1; END;",
		nsPrefix, nsPrefix, nsPrefix)

	req := makeMigrationCallRequest(extID, map[string]any{"sql": sqlStmt})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != host_api.StatusFailed {
		t.Fatal("expected failure for CREATE TRIGGER")
	}
	if result.Error.Code != PackageErrCodeMigrationNamespaceViolation {
		t.Errorf("expected namespace violation, got %s: %s", result.Error.Code, result.Error.Message)
	}
}

func parseResultOutput(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	return m
}
