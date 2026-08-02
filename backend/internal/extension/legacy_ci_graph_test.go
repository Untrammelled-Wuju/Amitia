package extension

import (
	"os"
	"path/filepath"
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
		"package_legacy_writer.go":    true,
		"package_legacy_lifecycle.go": true,
		"package_recovery.go":         true,
		"package_manager_test.go":     true,
		"package_baseline_test.go":    true,
		"legacy_ci_graph_test.go":     true,
		"legacy_final_zero_test.go":   true,
		"openapi_routes_test.go":      true,
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

func TestPackageServiceMetricsIncludeLegacyMonitoring(t *testing.T) {
	requiredMetrics := []string{
		"package_legacy_read_calls",
		"package_legacy_write_calls",
		"legacy_data_detected",
		"legacy_migration_required",
		"legacy_write_attempts",
	}
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "package_service.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, metric := range requiredMetrics {
		if !strings.Contains(source, metric) {
			t.Fatalf("package_service.go missing required legacy monitoring metric: %s", metric)
		}
	}
}

func TestPackageServiceInstallRequiresKernelProxy(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "package_installer.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if strings.Contains(source, "//go:build legacy_migration") {
		t.Fatal("package_installer.go must not have legacy_migration build tag")
	}
	if !strings.Contains(source, "s.kernel == nil") {
		t.Fatal("package_installer.go must guard Install with kernel nil check")
	}
}

func TestPackageServiceLifecycleRequiresKernelProxy(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "package_lifecycle.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if strings.Contains(source, "//go:build legacy_migration") {
		t.Fatal("package_lifecycle.go must not have legacy_migration build tag")
	}
	forbidden := []string{
		"rollbackLegacyPackage(",
		"uninstallLegacyPackage(",
		"installLegacyPackage(",
	}
	for _, method := range forbidden {
		if strings.Contains(source, method) {
			t.Fatalf("package_lifecycle.go contains legacy method call: %s", method)
		}
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
