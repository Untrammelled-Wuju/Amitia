package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

func TestPackageMigrationMissingDeclaredScriptFails(t *testing.T) {
	for _, runtimeType := range []string{"config", "resource", "userdata"} {
		t.Run(runtimeType, func(t *testing.T) {
			runtime := newPackageMigrationRuntime(nil, nil, t.TempDir(), "example", "operation")
			_, err := runtime.executeScript(context.Background(), migration.ReversiblePlanStep{}, migration.MigrationDefinition{RuntimeType: runtimeType, Entry: "missing.json"})
			if err == nil || !strings.Contains(err.Error(), "missing.json") {
				t.Fatalf("expected original missing script error, got %v", err)
			}
		})
	}
}

func TestPackageMigrationRealSQLAndCompensation(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	extensionID := "com.example/migration-real"
	definitions := packageMigrationChain(extensionID, 1)
	packageMigrationDefinitionsAsSQL(definitions)
	manifest := packageMigrationManifest(extensionID, "1.1.0", definitions)
	preflight, err := NewPackageMigrationGuard(container.MigrationRepository).PreflightManifest(context.Background(), manifest, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	execution, err := runtime.executePackageUpdateMigrations(context.Background(), "real-operation", preflight, PackageRollbackPoint{RollbackPointID: "real-snapshot", ExtensionID: extensionID, SourceVersion: "1.0.0"}, packageMigrationPackage(manifest, definitions), packageMigrationSQLStagingDir(t, extensionID, definitions))
	if err != nil {
		t.Fatal(err)
	}
	table := fmt.Sprintf("%smig_%s", migration.ExtensionNamespacePrefix(extensionID), definitions[0].MigrationID)
	var count int
	if err := container.Store.DB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count); err != nil || count != 1 {
		t.Fatalf("forward migration did not create %s: count=%d error=%v", table, count, err)
	}
	if err := execution.guard.CompensateReverse(context.Background(), execution.request, execution.handler); err != nil {
		t.Fatal(err)
	}
	if err := container.Store.DB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count); err != nil || count != 0 {
		t.Fatalf("compensation did not remove %s: count=%d error=%v", table, count, err)
	}
}

func TestPackageMigrationPreconditionRequiresDatabase(t *testing.T) {
	runtime := newPackageMigrationRuntime(nil, nil, t.TempDir(), "example", "operation")
	_, err := runtime.handler()(context.Background(), migration.ReversiblePlanStep{}, migration.MigrationDefinition{MigrationID: "migration-1", Precondition: []migration.MigrationCondition{{Name: "required"}}})
	if err == nil || !strings.Contains(err.Error(), "precondition database unavailable") {
		t.Fatalf("expected precondition error, got %v", err)
	}
}

func TestPackageMigrationPreconditionPreventsMutation(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	extensionID := "com.example/precondition"
	definitions := packageMigrationChain(extensionID, 1)
	packageMigrationDefinitionsAsSQL(definitions)
	definition := definitions[0]
	definition.Precondition = []migration.MigrationCondition{{Name: "required table", Type: "table_exists", Expected: json.RawMessage(`{"table":"missing_required_table"}`)}}
	staging := packageMigrationSQLStagingDir(t, extensionID, definitions)
	executor := newPackageMigrationRuntime(runtime, nil, staging, extensionID, "operation")
	_, err := executor.handler()(context.Background(), migration.ReversiblePlanStep{MigrationID: definition.MigrationID}, definition)
	if err == nil || !strings.Contains(err.Error(), "missing_required_table") {
		t.Fatalf("expected original precondition failure, got %v", err)
	}
	var count int
	table := migration.ExtensionNamespacePrefix(extensionID) + "mig_" + definition.MigrationID
	if err := container.Store.DB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed precondition still mutated database: count=%d error=%v", count, err)
	}
}
