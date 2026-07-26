package lifecycle_manager

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func newTestManager(t *testing.T) (*Manager, *domain.InMemoryDefinitionRepository, *domain.InMemoryInstallationRepository, *domain.InMemoryRuntimeRepository, *InMemoryAuditWriter) {
	defRepo := domain.NewInMemoryDefinitionRepository()
	instRepo := domain.NewInMemoryInstallationRepository()
	rtRepo := domain.NewInMemoryRuntimeRepository()
	loader := NewDefaultStateLoader(defRepo, instRepo, rtRepo)
	preflight := NewDefaultPreflightChecker()
	executor := NewDefaultExecutor(instRepo, defRepo, rtRepo)
	audit := NewInMemoryAuditWriter()
	mgr := NewManager(loader, preflight, executor, audit)
	return mgr, defRepo, instRepo, rtRepo, audit
}

func setupExtension(t *testing.T, defRepo *domain.InMemoryDefinitionRepository, extID string) domain.SemanticVersion {
	v, _ := domain.ParseVersion("1.0.0")
	def := domain.ExtensionDefinition{
		ID:              domain.ExtensionID(extID),
		Version:         v,
		ManifestVersion: 2,
		Name:            domain.LocalizedText{Default: "Test"},
		Modules: []domain.ModuleDefinition{
			{ID: "main", ExtensionID: domain.ExtensionID(extID), Type: domain.ModuleTypeBuiltin},
		},
	}
	if err := defRepo.PutExtension(context.Background(), def); err != nil {
		t.Fatalf("PutExtension: %v", err)
	}
	return v
}

func TestManagerInstall(t *testing.T) {
	mgr, defRepo, instRepo, _, audit := newTestManager(t)
	v := setupExtension(t, defRepo, "com.example/test")
	cmd := LifecycleCommand{
		Kind:          CmdInstall,
		ExtensionID:   "com.example/test",
		TargetVersion: v,
		PackageID:     "pkg1",
	}
	result, err := mgr.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}
	inst, err := instRepo.GetInstallation(context.Background(), "com.example/test")
	if err != nil {
		t.Fatalf("GetInstallation: %v", err)
	}
	if inst.InstalledVersion.Compare(v) != 0 {
		t.Errorf("expected version %s, got %s", v.String(), inst.InstalledVersion.String())
	}
	if len(audit.Events()) == 0 {
		t.Errorf("expected audit events")
	}
}

func TestManagerEnableDisable(t *testing.T) {
	mgr, defRepo, instRepo, _, _ := newTestManager(t)
	v := setupExtension(t, defRepo, "com.example/test")
	_, _ = mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdInstall, ExtensionID: "com.example/test", TargetVersion: v, PackageID: "pkg1",
	})
	_, err := mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdEnable, ExtensionID: "com.example/test",
	})
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	inst, _ := instRepo.GetInstallation(context.Background(), "com.example/test")
	if inst.EnablementState != domain.EnablementEnabled {
		t.Errorf("expected enabled, got %s", inst.EnablementState)
	}
	_, err = mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdDisable, ExtensionID: "com.example/test",
	})
	if err != nil {
		t.Fatalf("Disable: %v", err)
	}
	inst, _ = instRepo.GetInstallation(context.Background(), "com.example/test")
	if inst.EnablementState != domain.EnablementDisabled {
		t.Errorf("expected disabled, got %s", inst.EnablementState)
	}
}

func TestManagerUninstall(t *testing.T) {
	mgr, defRepo, instRepo, _, _ := newTestManager(t)
	v := setupExtension(t, defRepo, "com.example/test")
	_, _ = mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdInstall, ExtensionID: "com.example/test", TargetVersion: v, PackageID: "pkg1",
	})
	_, err := mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdUninstall, ExtensionID: "com.example/test",
	})
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	_, err = instRepo.GetInstallation(context.Background(), "com.example/test")
	if err == nil {
		t.Errorf("expected error after uninstall")
	}
}

func TestManagerDryRun(t *testing.T) {
	mgr, defRepo, instRepo, _, _ := newTestManager(t)
	v := setupExtension(t, defRepo, "com.example/test")
	cmd := LifecycleCommand{
		Kind:          CmdInstall,
		ExtensionID:   "com.example/test",
		TargetVersion: v,
		PackageID:     "pkg1",
		DryRun:        true,
	}
	result, err := mgr.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != "dry_run" {
		t.Errorf("expected dry_run, got %s", result.Status)
	}
	_, err = instRepo.GetInstallation(context.Background(), "com.example/test")
	if err == nil {
		t.Errorf("dry run should not persist")
	}
}

func TestManagerConcurrentLock(t *testing.T) {
	mgr, defRepo, _, _, _ := newTestManager(t)
	v := setupExtension(t, defRepo, "com.example/test")
	cmd1 := LifecycleCommand{
		Kind: CmdInstall, ExtensionID: "com.example/test", TargetVersion: v, PackageID: "pkg1",
		RequestID: "req1",
	}
	if err := mgr.acquireLock(cmd1); err != nil {
		t.Fatalf("acquireLock 1: %v", err)
	}
	cmd2 := LifecycleCommand{
		Kind: CmdInstall, ExtensionID: "com.example/test", TargetVersion: v, PackageID: "pkg1",
		RequestID: "req2",
	}
	if err := mgr.acquireLock(cmd2); err == nil {
		t.Errorf("expected concurrent lock error")
	}
	mgr.releaseLock(cmd1)
	if err := mgr.acquireLock(cmd2); err != nil {
		t.Errorf("expected lock released: %v", err)
	}
	mgr.releaseLock(cmd2)
}

func TestManagerPreflightBlocking(t *testing.T) {
	mgr, _, _, _, _ := newTestManager(t)
	_, err := mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdEnable, ExtensionID: "com.example/notinstalled",
	})
	if err == nil {
		t.Errorf("expected blocking issue")
	}
}

func TestManagerUpdate(t *testing.T) {
	mgr, defRepo, instRepo, _, _ := newTestManager(t)
	v1 := setupExtension(t, defRepo, "com.example/test")
	_, _ = mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdInstall, ExtensionID: "com.example/test", TargetVersion: v1, PackageID: "pkg1",
	})
	v2, _ := domain.ParseVersion("1.1.0")
	def2 := domain.ExtensionDefinition{
		ID:              "com.example/test",
		Version:         v2,
		ManifestVersion: 2,
		Name:            domain.LocalizedText{Default: "Test v2"},
		Modules: []domain.ModuleDefinition{
			{ID: "main", ExtensionID: "com.example/test", Type: domain.ModuleTypeBuiltin},
		},
	}
	_ = defRepo.PutExtension(context.Background(), def2)
	_, err := mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdUpdate, ExtensionID: "com.example/test", TargetVersion: v2, PackageID: "pkg2",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	inst, _ := instRepo.GetInstallation(context.Background(), "com.example/test")
	if inst.InstalledVersion.Compare(v2) != 0 {
		t.Errorf("expected version %s, got %s", v2.String(), inst.InstalledVersion.String())
	}
}

func TestManagerPlan(t *testing.T) {
	mgr, defRepo, _, _, _ := newTestManager(t)
	v := setupExtension(t, defRepo, "com.example/test")
	plan, err := mgr.Plan(context.Background(), LifecycleCommand{
		Kind: CmdInstall, ExtensionID: "com.example/test", TargetVersion: v, PackageID: "pkg1",
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Errorf("expected steps")
	}
	if plan.AuditSummary == "" {
		t.Errorf("expected audit summary")
	}
}

func TestManagerEnableModule(t *testing.T) {
	mgr, defRepo, _, _, _ := newTestManager(t)
	v := setupExtension(t, defRepo, "com.example/test")
	_, _ = mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdInstall, ExtensionID: "com.example/test", TargetVersion: v, PackageID: "pkg1",
	})
	_, err := mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdEnableModule, ExtensionID: "com.example/test", ModuleID: "main",
	})
	if err != nil {
		t.Fatalf("EnableModule: %v", err)
	}
}

func TestManagerEnableModuleMissing(t *testing.T) {
	mgr, defRepo, _, _, _ := newTestManager(t)
	v := setupExtension(t, defRepo, "com.example/test")
	_, _ = mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdInstall, ExtensionID: "com.example/test", TargetVersion: v, PackageID: "pkg1",
	})
	_, err := mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdEnableModule, ExtensionID: "com.example/test", ModuleID: "missing",
	})
	if err == nil {
		t.Errorf("expected error for missing module")
	}
}

func TestExecutorStepCompensation(t *testing.T) {
	mgr, defRepo, _, _, _ := newTestManager(t)
	v := setupExtension(t, defRepo, "com.example/test")
	cmd := LifecycleCommand{
		Kind: CmdInstall, ExtensionID: "com.example/test", TargetVersion: v,
	}
	_, err := mgr.Execute(context.Background(), cmd)
	if err == nil {
		t.Errorf("expected error for missing package id")
	}
}

func TestManagerCompletedTime(t *testing.T) {
	mgr, defRepo, _, _, _ := newTestManager(t)
	v := setupExtension(t, defRepo, "com.example/test")
	result, err := mgr.Execute(context.Background(), LifecycleCommand{
		Kind: CmdInstall, ExtensionID: "com.example/test", TargetVersion: v, PackageID: "pkg1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.CompletedAt == nil {
		t.Errorf("expected completed at")
	}
	if result.CompletedAt.Before(result.StartedAt) {
		t.Errorf("completed before started")
	}
	_ = time.Now()
}
