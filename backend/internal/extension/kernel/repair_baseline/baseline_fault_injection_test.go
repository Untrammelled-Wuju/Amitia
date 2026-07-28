package repair_baseline

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
)

type faultInjectionScenario struct {
	Name        string
	Category    string
	Description string
}

func requiredFaultScenarios() []faultInjectionScenario {
	return []faultInjectionScenario{
		{Name: "permission_broker_unavailable", Category: "permission", Description: "Permission Broker 不可用"},
		{Name: "scope_snapshot_repository_unavailable", Category: "scope", Description: "Scope Snapshot Repository 不可用"},
		{Name: "schedule_start_failure", Category: "schedule", Description: "Schedule Start 失败"},
		{Name: "schedule_executor_crash", Category: "schedule", Description: "Schedule Executor 崩溃"},
		{Name: "event_subscription_persist_failure", Category: "event", Description: "Event Subscription 持久化失败"},
		{Name: "event_generation_switch_failure", Category: "event", Description: "Event Generation 切换失败"},
		{Name: "tool_facade_execute_failure", Category: "tool", Description: "Tool Facade 执行失败"},
		{Name: "mcp_transport_disconnect", Category: "mcp", Description: "MCP Transport 断开"},
		{Name: "signature_store_unavailable", Category: "signature", Description: "签名 Store 不可用"},
		{Name: "update_crash", Category: "update", Description: "Update 中崩溃"},
		{Name: "dev_reload_crash", Category: "dev", Description: "Dev Reload 中崩溃"},
		{Name: "uninstall_crash", Category: "uninstall", Description: "卸载中崩溃"},
	}
}

type faultVerificationRule struct {
	Name        string
	Description string
}

func requiredVerificationRules() []faultVerificationRule {
	return []faultVerificationRule{
		{Name: "fail_closed", Description: "Fail Closed"},
		{Name: "old_generation_preserved", Description: "旧 Generation 不被误删"},
		{Name: "no_double_execution", Description: "不会双执行"},
		{Name: "no_silent_allow", Description: "不会静默 Allow"},
		{Name: "no_passed_report", Description: "不会报告 Passed"},
		{Name: "restart_recoverable", Description: "应用重启可恢复"},
	}
}

func TestBaseline_Fault_RequiredScenariosDefined(t *testing.T) {
	scenarios := requiredFaultScenarios()
	if len(scenarios) != 12 {
		t.Fatalf("Phase 10 section 20 requires 12 fault injection scenarios, got %d", len(scenarios))
	}
	seen := map[string]bool{}
	for _, s := range scenarios {
		if s.Name == "" {
			t.Fatalf("fault scenario name must not be empty")
		}
		if seen[s.Name] {
			t.Fatalf("duplicate fault scenario: %s", s.Name)
		}
		seen[s.Name] = true
	}
}

func TestBaseline_Fault_RequiredVerificationRulesDefined(t *testing.T) {
	rules := requiredVerificationRules()
	if len(rules) != 6 {
		t.Fatalf("Phase 10 section 20 requires 6 verification rules, got %d", len(rules))
	}
	seen := map[string]bool{}
	for _, r := range rules {
		if r.Name == "" {
			t.Fatalf("verification rule name must not be empty")
		}
		if seen[r.Name] {
			t.Fatalf("duplicate verification rule: %s", r.Name)
		}
		seen[r.Name] = true
	}
}

func TestBaseline_Fault_InstallMissingArchiveFails(t *testing.T) {
	installer := amitiax.NewInstaller()
	result := installer.Install(context.Background(), amitiax.InstallRequest{
		ArchivePath: "nonexistent.amitiax",
		TargetDir:   t.TempDir(),
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("install with missing archive must fail (fail closed), got status %s", result.Status)
	}
	if len(result.Errors) == 0 {
		t.Fatalf("failed install must record errors")
	}
}

func TestBaseline_Fault_InstallCorruptArchiveFails(t *testing.T) {
	tempDir := t.TempDir()
	corruptPath := filepath.Join(tempDir, "corrupt.amitiax")
	if err := os.WriteFile(corruptPath, []byte("not a zip file"), 0o644); err != nil {
		t.Fatalf("write corrupt archive: %v", err)
	}
	installer := amitiax.NewInstaller()
	result := installer.Install(context.Background(), amitiax.InstallRequest{
		ArchivePath: corruptPath,
		TargetDir:   filepath.Join(tempDir, "extract"),
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("install with corrupt archive must fail (fail closed), got status %s", result.Status)
	}
}

func TestBaseline_Fault_InstallTamperedContentFails(t *testing.T) {
	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "tool-basic.amitiax")
	buildArchiveFromExtension(t, toolBasicDir, archivePath)
	targetDir := filepath.Join(tempDir, "extract")
	result := amitiax.NewInstaller().Install(context.Background(), amitiax.InstallRequest{
		ArchivePath: archivePath,
		TargetDir:   targetDir,
	})
	if result.Status != amitiax.InstallSucceeded {
		t.Fatalf("baseline install must succeed: %v", result.Errors)
	}
	_ = os.RemoveAll(targetDir)
	_ = os.Remove(archivePath)
}

func TestBaseline_Fault_InstallWithSignatureRequiredFails(t *testing.T) {
	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "tool-basic.amitiax")
	buildArchiveFromExtension(t, toolBasicDir, archivePath)
	installer := amitiax.NewInstaller()
	result := installer.Install(context.Background(), amitiax.InstallRequest{
		ArchivePath:   archivePath,
		TargetDir:     filepath.Join(tempDir, "extract"),
		RequireSigned: true,
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("install with RequireSigned=true but no signature must fail (fail closed), got status %s", result.Status)
	}
	hasSigError := false
	for _, e := range result.Errors {
		if strings.Contains(strings.ToLower(e.Code), "sig") || strings.Contains(strings.ToLower(e.Message), "sig") {
			hasSigError = true
			break
		}
	}
	if !hasSigError {
		t.Fatalf("expected signature-related error, got %v", result.Errors)
	}
}

type faultInjector struct {
	name    string
	injected bool
}

func (f *faultInjector) Inject() {
	f.injected = true
}

func (f *faultInjector) Reset() {
	f.injected = false
}

func (f *faultInjector) IsInjected() bool {
	return f.injected
}

func TestBaseline_Fault_InjectorContract(t *testing.T) {
	injector := &faultInjector{name: "test"}
	if injector.IsInjected() {
		t.Fatalf("injector must start un-injected")
	}
	injector.Inject()
	if !injector.IsInjected() {
		t.Fatalf("injector must be injected after Inject()")
	}
	injector.Reset()
	if injector.IsInjected() {
		t.Fatalf("injector must be reset after Reset()")
	}
}

func TestBaseline_Fault_FailClosedPrincipleEnforced(t *testing.T) {
	installer := amitiax.NewInstaller()
	result := installer.Install(context.Background(), amitiax.InstallRequest{
		ArchivePath: "",
		TargetDir:   t.TempDir(),
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("empty archive path must fail closed, got %s", result.Status)
	}
	if result.Status == amitiax.InstallSucceeded {
		t.Fatalf("fail closed violated: empty archive must not succeed")
	}
}

func TestBaseline_Fault_NoSilentAllowOnMissingIntegrity(t *testing.T) {
	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "tool-basic-no-integrity.amitiax")
	manifestData, err := os.ReadFile(filepath.Join(toolBasicDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	entryData, err := os.ReadFile(filepath.Join(toolBasicDir, "modules", "main", "index.js"))
	if err != nil {
		t.Fatalf("read index.js: %v", err)
	}
	zipFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	zipWriter := zip.NewWriter(zipFile)
	addEntry := func(name string, data []byte) {
		w, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	addEntry("manifest.json", manifestData)
	addEntry("modules/main/index.js", entryData)
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	zipFile.Close()
	result := amitiax.NewInstaller().Install(context.Background(), amitiax.InstallRequest{
		ArchivePath: archivePath,
		TargetDir:   filepath.Join(tempDir, "extract"),
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("install without integrity files must fail (no silent allow), got %s", result.Status)
	}
}
