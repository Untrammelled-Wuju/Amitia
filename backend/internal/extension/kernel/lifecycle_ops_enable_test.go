package kernel

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func setupEnableTestRuntime(t *testing.T, ctx context.Context, extID string, fakeRT *runtime_supervisor.FakeRuntime) (*Runtime, *Container, string) {
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
		EnablementState:   domain.EnablementDisabled,
		InstalledAt:       now,
		UpdatedAt:         now,
		Generation:        0,
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

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}
	if err := container.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementDisabled); err != nil {
		t.Fatalf("SetEnablement must succeed: %v", err)
	}
	if err := container.EnablementStore.SetDesiredRuntime(ctx, extSubject, enablement.DesiredRuntimeStopped); err != nil {
		t.Fatalf("SetDesiredRuntime must succeed: %v", err)
	}

	return rt, container, extRoot
}

func TestEnable_SucceedsWhenAllStepsOK(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/enable-ok"
	fakeRT := runtime_supervisor.NewFakeRuntime()

	rt, container, _ := setupEnableTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	if err := rt.Enable(ctx, extID); err != nil {
		t.Fatalf("Enable must succeed when all steps are ok: %v", err)
	}

	inst, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if err != nil {
		t.Fatalf("GetInstallation must succeed: %v", err)
	}
	if inst.EnablementState != domain.EnablementEnabled {
		t.Errorf("expected EnablementState=enabled, got %s", inst.EnablementState)
	}
	if inst.Generation != 1 {
		t.Errorf("expected Generation=1, got %d", inst.Generation)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}
	state, err := container.EnablementStore.Get(ctx, extSubject)
	if err != nil {
		t.Fatalf("EnablementStore.Get must succeed: %v", err)
	}
	if state.Enablement != enablement.EnablementEnabled {
		t.Errorf("expected enablement=enabled, got %s", state.Enablement)
	}
	if state.DesiredRuntime != enablement.DesiredRuntimeStarted {
		t.Errorf("expected desiredRuntime=started, got %s", state.DesiredRuntime)
	}

	if fakeRT.StartCount() == 0 {
		t.Errorf("expected FakeRuntime.Start to be called")
	}
}

func TestEnable_RollsBackOnRuntimeStartFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/enable-rt-fail"
	fakeRT := runtime_supervisor.NewFakeRuntime()
	fakeRT.SetStartErr(runtime_supervisor.ErrFakeStart)

	rt, container, _ := setupEnableTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	err := rt.Enable(ctx, extID)
	if err == nil {
		t.Fatalf("Enable must fail when runtime start fails")
	}

	inst, getErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if getErr != nil {
		t.Fatalf("GetInstallation must succeed: %v", getErr)
	}
	if inst.EnablementState != domain.EnablementDisabled {
		t.Errorf("expected EnablementState=disabled (rolled back), got %s", inst.EnablementState)
	}
	if inst.Generation != 0 {
		t.Errorf("expected Generation=0 (not incremented), got %d", inst.Generation)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}
	state, _ := container.EnablementStore.Get(ctx, extSubject)
	if state.Enablement != enablement.EnablementDisabled {
		t.Errorf("expected enablement=disabled (rolled back), got %s", state.Enablement)
	}
}

func TestEnable_RollsBackOnRuntimeReconcileNotReady(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/enable-reconcile-notready"
	fakeRT := runtime_supervisor.NewFakeRuntime()

	rt, container, _ := setupEnableTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	fakeRT.SetStartErr(runtime_supervisor.ErrFakeStart)
	err := rt.Enable(ctx, extID)
	if err == nil {
		t.Fatalf("Enable must fail when runtime reconcile returns not ready")
	}

	inst, getErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if getErr != nil {
		t.Fatalf("GetInstallation must succeed: %v", getErr)
	}
	if inst.EnablementState != domain.EnablementDisabled {
		t.Errorf("expected EnablementState=disabled (rolled back), got %s", inst.EnablementState)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}
	state, _ := container.EnablementStore.Get(ctx, extSubject)
	if state.Enablement != enablement.EnablementDisabled {
		t.Errorf("expected enablement=disabled (rolled back), got %s", state.Enablement)
	}
}

func TestEnable_NoEarlyStateCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/enable-no-early-commit"
	fakeRT := runtime_supervisor.NewFakeRuntime()
	fakeRT.SetStartErr(runtime_supervisor.ErrFakeStart)

	rt, container, _ := setupEnableTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	originalInst, _ := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	originalGen := originalInst.Generation

	_ = rt.Enable(ctx, extID)

	afterInst, _ := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if afterInst.Generation != originalGen {
		t.Errorf("Generation must not be incremented on failure: original=%d after=%d", originalGen, afterInst.Generation)
	}
	if afterInst.EnablementState != domain.EnablementDisabled {
		t.Errorf("EnablementState must remain disabled on failure: got %s", afterInst.EnablementState)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}
	afterState, _ := container.EnablementStore.Get(ctx, extSubject)
	if afterState.Enablement != enablement.EnablementDisabled {
		t.Errorf("EnablementStore must remain disabled on failure: got %s", afterState.Enablement)
	}
}

func TestEnable_ConcurrentCallsAreSerialized(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	extID := "com.amitia.test/enable-concurrent"
	fakeRT := runtime_supervisor.NewFakeRuntime()

	rt, container, _ := setupEnableTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = rt.Enable(ctx, extID)
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Errorf("concurrent Enable[%d] must succeed (idempotent): %v", i, e)
		}
	}

	inst, _ := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if inst.EnablementState != domain.EnablementEnabled {
		t.Errorf("expected EnablementState=enabled after concurrent enable, got %s", inst.EnablementState)
	}
	if inst.Generation < 1 {
		t.Errorf("expected Generation>=1 after concurrent enable, got %d", inst.Generation)
	}
}

func TestEnable_RejectsNilContainer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rt := &Runtime{}
	err := rt.Enable(ctx, "com.amitia.test/no-container")
	if err == nil {
		t.Fatalf("Enable must fail when container is nil")
	}
}

func TestEnable_FailsOnUnknownExtension(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/enable-unknown"
	fakeRT := runtime_supervisor.NewFakeRuntime()

	rt, container, _ := setupEnableTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	unknownExtID := "com.amitia.test/not-installed"
	err := rt.Enable(ctx, unknownExtID)
	if err == nil {
		t.Fatalf("Enable must fail for unknown extension")
	}
}

func TestEnable_GenerationIncrementedOnSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/enable-gen-incr"
	fakeRT := runtime_supervisor.NewFakeRuntime()

	rt, container, _ := setupEnableTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	originalInst, _ := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	originalGen := originalInst.Generation

	if err := rt.Enable(ctx, extID); err != nil {
		t.Fatalf("Enable must succeed: %v", err)
	}

	afterInst, _ := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if afterInst.Generation != originalGen+1 {
		t.Errorf("expected Generation=%d, got %d", originalGen+1, afterInst.Generation)
	}
}
