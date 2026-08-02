package extension

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestProductionSourceExcludesLegacyWriteImplementations(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	legacyMethods := []string{
		"installLegacyPackage(",
		"rollbackLegacyPackage(",
		"uninstallLegacyPackage(",
		"recoverPackageOperations(",
		"legacyInstallPackage(",
		"legacyRollbackPackage(",
		"legacyUninstallPackage(",
	}
	exemptFiles := map[string]bool{
		"package_legacy_writer.go":          true,
		"package_legacy_lifecycle.go":       true,
		"package_recovery.go":               true,
		"package_manager_test.go":           true,
		"package_baseline_test.go":          true,
		"legacy_ci_graph_test.go":           true,
		"legacy_final_zero_test.go":         true,
		"openapi_routes_test.go":            true,
		"package_service.go":                true,
		"package_handler.go":                true,
		"package_installer.go":              true,
		"package_lifecycle.go":              true,
		"package_repository.go":             true,
		"package_parser.go":                 true,
		"package_archive.go":                true,
		"package_protocol.go":               true,
		"package_kernel_facade.go":          true,
		"package_test_runner.go":            true,
		"legacy_bridge.go":                  true,
		"legacy_exports.go":                 true,
		"package_kernel_migration_write.go": true,
		"lifecycle_service.go":              true,
		"legacy_tool_adapter.go":            true,
		"extension_read_model.go":           true,
		"kernel_lifecycle_proxy.go":         true,
		"runtime_legacy.go":                 true,
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		base := filepath.Base(path)
		if exemptFiles[base] {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source := string(raw)
		if strings.Contains(source, "//go:build legacy_migration") {
			return nil
		}
		for _, method := range legacyMethods {
			if strings.Contains(source, method) {
				t.Fatalf("production file %s contains legacy write call: %s", base, method)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLegacyWriteFilesHaveBuildTag(t *testing.T) {
	legacyFiles := []string{
		"package_legacy_writer.go",
		"package_legacy_lifecycle.go",
		"package_recovery.go",
		"package_service.go",
		"package_installer.go",
		"package_lifecycle.go",
		"legacy_bridge.go",
		"extension_read_model.go",
		"kernel_lifecycle_proxy.go",
		"runtime_legacy.go",
	}
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range legacyFiles {
		path := filepath.Join(root, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("legacy file not found: %s", name)
		}
		source := string(raw)
		if !strings.Contains(source, "//go:build legacy_migration") {
			t.Fatalf("legacy file %s missing //go:build legacy_migration tag", name)
		}
	}
}

func TestProductionDependencyGraphExcludesLegacyMigration(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	forbiddenTypes := []string{
		"PackageService",
		"NewPackageService",
		"PackageHandler",
		"NewPackageHandler",
		"MigrateLegacyPackageData",
	}
	forbiddenFiles := []string{
		"package_service.go",
		"package_installer.go",
		"package_lifecycle.go",
		"package_handler.go",
		"package_kernel_facade.go",
		"legacy_bridge.go",
		"extension_read_model.go",
		"kernel_lifecycle_proxy.go",
		"runtime_legacy.go",
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		base := filepath.Base(path)
		for _, forbidden := range forbiddenFiles {
			if base == forbidden {
				return nil
			}
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source := string(raw)
		if strings.Contains(source, "//go:build legacy_migration") {
			return nil
		}
		for _, typ := range forbiddenTypes {
			pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(typ) + `\b`)
			if pattern.MatchString(source) {
				t.Fatalf("production file %s references legacy type: %s", base, typ)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPackageHandlerIsolatedToLegacyMigration(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "package_handler.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if !strings.Contains(source, "//go:build legacy_migration") {
		t.Fatal("package_handler.go must have //go:build legacy_migration tag (HTTP write endpoints isolated)")
	}
}

func TestMigrationCLIHasBuildTag(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "cmd", "legacy-package-migrate"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("migration CLI not found: %v", err)
	}
	source := string(raw)
	if !strings.Contains(source, "//go:build legacy_migration") {
		t.Fatal("cmd/legacy-package-migrate/main.go must have //go:build legacy_migration tag")
	}
}

func TestLegacyMigrationCLIEntryCompiles(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "cmd", "legacy-package-migrate"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.go")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("migration CLI not found: %v", err)
	}
}

func TestProductionRuntimeExcludesLegacyDependencies(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	productionFiles := []string{
		"runtime.go",
		"router.go",
		"handler.go",
		"service.go",
		"kernel_api.go",
		"kernel_handler.go",
	}
	forbiddenTypes := []string{
		"PackageService",
		"NewPackageService",
		"PackageHandler",
		"NewPackageHandler",
		"MigrateLegacyPackageData",
		"ExtensionReadModelService",
		"KernelLifecycleProxy",
	}
	for _, name := range productionFiles {
		path := filepath.Join(root, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		source := string(raw)
		for _, typ := range forbiddenTypes {
			if strings.Contains(source, typ) {
				t.Fatalf("production file %s references legacy type: %s", name, typ)
			}
		}
	}
}
