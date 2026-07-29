package kernel

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func setupUninstallTestRuntime(t *testing.T, ctx context.Context, extID string, fakeRT *runtime_supervisor.FakeRuntime) (*Runtime, *Container, string) {
	t.Helper()
	tempDir := t.TempDir()
	extRoot := filepath.Join(tempDir, "extensions")

	container, err := NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}

	rt, err := NewRuntime(extRoot)
	if err != nil {
		t.Fatalf("NewRuntime must succeed: %v", err)
	}
	rt.SetContainer(container)

	defaultSupervisor, ok := container.RuntimeSupervisor.(*runtime_supervisor.DefaultSupervisor)
	if !ok {
		t.Fatalf("expected *DefaultSupervisor, got %T", container.RuntimeSupervisor)
	}
	if err := defaultSupervisor.RegisterFactory(runtime_supervisor.NewFakeFactory(domain.RuntimeTypeGo, fakeRT)); err != nil {
		t.Fatalf("RegisterFactory must succeed: %v", err)
	}

	version := domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}
	now := time.Now().UTC()
	inst := domain.ExtensionInstallation{
		InstallationID:    "inst-" + extID,
		ExtensionID:       domain.ExtensionID(extID),
		InstalledVersion:  version,
		PackageID:         "pkg-" + extID,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementEnabled,
		InstalledAt:       now,
		UpdatedAt:         now,
		Generation:        1,
	}
	if err := container.InstallationRepository.PutInstallation(ctx, inst); err != nil {
		t.Fatalf("PutInstallation must succeed: %v", err)
	}

	mod := domain.ModuleDefinition{
		ID:          "main",
		ExtensionID: domain.ExtensionID(extID),
		Name:        domain.LocalizedText{Default: "main"},
		Type:        domain.ModuleTypeService,
		Runtime: &domain.RuntimeDefinition{
			Type: domain.RuntimeTypeGo,
		},
	}
	if err := container.ModuleRepository.PutModule(ctx, mod); err != nil {
		t.Fatalf("PutModule must succeed: %v", err)
	}

	defID := runtime_supervisor.BuildRuntimeDefinitionID(extID, "main", domain.RuntimeTypeGo)
	result := container.RuntimeSupervisor.Reconcile(ctx, runtime_supervisor.ReconcileRequest{
		DefinitionID: defID,
		Desired:      runtime_supervisor.DesiredRunning,
		Spec: runtime_supervisor.InstanceSpec{
			DefinitionID: defID,
			ExtensionID:  domain.ExtensionID(extID),
			ModuleID:     "main",
			RuntimeType:  domain.RuntimeTypeGo,
			Generation:   1,
			Strategy:     runtime_supervisor.StrategySingletonPerModule,
		},
	})
	if result.Error != nil {
		t.Fatalf("Reconcile start must succeed: %v", result.Error)
	}

	return rt, container, extRoot
}

func TestUninstall_BlockedByRuntimeStopFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/uninstall-stop-fail"
	fakeRT := runtime_supervisor.NewFakeRuntime()
	fakeRT.SetStopErr(runtime_supervisor.ErrFakeStop)

	rt, container, _ := setupUninstallTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	err := rt.Uninstall(ctx, extID)
	if err == nil {
		t.Fatalf("Uninstall must fail when runtime stop fails")
	}

	inst, getErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if getErr != nil {
		t.Fatalf("installation record must still exist after stop failure: %v", getErr)
	}
	if inst.InstallationState != domain.InstallationStateUninstallFailed {
		t.Errorf("expected InstallationState=uninstall_failed, got %s", inst.InstallationState)
	}
	if inst.EnablementState != domain.EnablementRequiresRecovery {
		t.Errorf("expected EnablementState=requires_recovery, got %s", inst.EnablementState)
	}

	mods, _ := container.ModuleRepository.ListModules(ctx, domain.ExtensionID(extID))
	if len(mods) == 0 {
		t.Errorf("module records must still exist after stop failure")
	}
}

func TestResumeUninstall_AfterStopFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/resume-uninstall"
	fakeRT := runtime_supervisor.NewFakeRuntime()
	fakeRT.SetStopErr(runtime_supervisor.ErrFakeStop)

	rt, container, _ := setupUninstallTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	err := rt.Uninstall(ctx, extID)
	if err == nil {
		t.Fatalf("first Uninstall must fail")
	}

	inst, _ := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if inst.InstallationState != domain.InstallationStateUninstallFailed {
		t.Fatalf("expected uninstall_failed before resume, got %s", inst.InstallationState)
	}

	fakeRT.SetStopErr(nil)

	err = rt.ResumeUninstall(ctx, extID)
	if err != nil {
		t.Fatalf("ResumeUninstall must succeed after clearing stop error: %v", err)
	}

	_, getErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if getErr == nil {
		t.Fatalf("installation record must be deleted after successful resume uninstall")
	}
}

func TestResumeUninstall_RejectsNonFailedState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/resume-reject"
	fakeRT := runtime_supervisor.NewFakeRuntime()

	rt, container, _ := setupUninstallTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	err := rt.ResumeUninstall(ctx, extID)
	if err == nil {
		t.Fatalf("ResumeUninstall must reject non-failed state")
	}
}

func TestUninstall_SucceedsWhenStopSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/uninstall-ok"
	fakeRT := runtime_supervisor.NewFakeRuntime()

	rt, container, _ := setupUninstallTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	if err := rt.Uninstall(ctx, extID); err != nil {
		t.Fatalf("Uninstall must succeed when runtime stop succeeds: %v", err)
	}

	_, getErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if getErr == nil {
		t.Fatalf("installation record must be deleted after successful uninstall")
	}
}
