package extension

import (
	"encoding/json"
	"os"
	"os/exec"
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
		"legacy_ci_graph_test.go":   true,
		"legacy_final_zero_test.go": true,
		"openapi_routes_test.go":    true,
		"legacy_tool_adapter.go":    true,
		"legacy_tool_snapshot_test.go": true,
		"package_kernel_migration.go": true,
		"retired_legacy_routes.go":  true,
		"production_router_legacy_isolation_test.go": true,
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
		"production_router_legacy_isolation_test.go",
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
	moduleRoot := findGoModuleRoot(t)

	goExe, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go executable not found: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), executableName("legacy-package-migrate"))

	cmd := exec.Command(goExe, "build", "-trimpath", "-tags", "legacy_migration", "-o", outputPath, "./cmd/legacy-package-migrate")
	cmd.Dir = moduleRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("legacy migration CLI failed to compile: %v\n%s", err, string(output))
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("compiled CLI not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("compiled CLI is empty")
	}
}

func TestLegacyMigrationCLIExcludedFromDefaultBuild(t *testing.T) {
	moduleRoot := findGoModuleRoot(t)

	cmd := exec.Command("go", "list", "-e", "-json", "./cmd/legacy-package-migrate")
	cmd.Dir = moduleRoot
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(stderrBuf.String(), "build constraints exclude all Go files") {
			return
		}
		t.Fatalf("go list failed: %v\n%s", err, stderrBuf.String())
	}

	var pkg goListPackage
	if err := json.Unmarshal(output, &pkg); err != nil {
		t.Fatalf("parse go list: %v", err)
	}

	if pkg.Error != nil && strings.Contains(pkg.Error.Err, "build constraints exclude all Go files") {
		return
	}

	if len(pkg.GoFiles) > 0 {
		for _, f := range pkg.GoFiles {
			if f == "main.go" {
				t.Fatal("CLI main.go should be excluded from default build (missing legacy_migration tag)")
			}
		}
	}

	mainGoFound := false
	for _, f := range pkg.IgnoredGoFiles {
		if f == "main.go" {
			mainGoFound = true
			break
		}
	}
	if !mainGoFound {
		t.Fatal("CLI main.go should be in IgnoredGoFiles for default build")
	}
}

func TestLegacyMigrationCLIIncludedWithBuildTag(t *testing.T) {
	moduleRoot := findGoModuleRoot(t)

	cmd := exec.Command("go", "list", "-e", "-tags", "legacy_migration", "-json", "./cmd/legacy-package-migrate")
	cmd.Dir = moduleRoot
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list with tag failed: %v\n%s", err, stderrBuf.String())
	}

	var pkg goListPackage
	if err := json.Unmarshal(output, &pkg); err != nil {
		t.Fatalf("parse go list: %v", err)
	}

	mainGoFound := false
	for _, f := range pkg.GoFiles {
		if f == "main.go" {
			mainGoFound = true
			break
		}
	}
	if !mainGoFound {
		t.Fatal("CLI main.go should be in GoFiles when legacy_migration tag is set")
	}
}

func TestLegacyMigrationPackageCompiles(t *testing.T) {
	moduleRoot := findGoModuleRoot(t)

	cmd := exec.Command("go", "test", "-run", "^$", "-tags", "legacy_migration", "./internal/extension/package_legacy_migration")
	cmd.Dir = moduleRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("legacy migration package failed to compile: %v\n%s", err, string(output))
	}
}

func findGoModuleRoot(t *testing.T) string {
	t.Helper()

	cmd := exec.Command("go", "env", "GOMOD")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolve go.mod: %v", err)
	}

	goModPath := strings.TrimSpace(string(output))
	if goModPath == "" || goModPath == os.DevNull {
		t.Fatal("go.mod not found")
	}

	return filepath.Dir(goModPath)
}

func executableName(base string) string {
	if os.PathSeparator == '\\' || os.Getenv("GOOS") == "windows" {
		return base + ".exe"
	}
	return base
}

type goListPackage struct {
	GoFiles        []string `json:"GoFiles"`
	IgnoredGoFiles []string `json:"IgnoredGoFiles"`
	Error          *struct {
		Err string `json:"Err"`
	} `json:"Error"`
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
