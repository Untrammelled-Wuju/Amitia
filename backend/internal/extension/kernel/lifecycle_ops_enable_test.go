package kernel

import (
	"context"
	"errors"
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

type flakyStateStore struct {
	inner               enablement.StateStore
	failOnSetEnablement int
	setEnablementCount  int
	failErr             error
}

func (f *flakyStateStore) Get(ctx context.Context, subject enablement.StateSubject) (enablement.SubjectState, error) {
	return f.inner.Get(ctx, subject)
}

func (f *flakyStateStore) SetEnablement(ctx context.Context, subject enablement.StateSubject, state enablement.EnablementState) error {
	f.setEnablementCount++
	if f.failOnSetEnablement > 0 && f.setEnablementCount == f.failOnSetEnablement {
		return f.failErr
	}
	return f.inner.SetEnablement(ctx, subject, state)
}

func (f *flakyStateStore) SetDesiredRuntime(ctx context.Context, subject enablement.StateSubject, state enablement.DesiredRuntimeState) error {
	return f.inner.SetDesiredRuntime(ctx, subject, state)
}

func (f *flakyStateStore) List(ctx context.Context, kind enablement.StateSubjectKind) ([]enablement.SubjectState, error) {
	return f.inner.List(ctx, kind)
}

func setupMultiModuleEnableTestRuntime(t *testing.T, ctx context.Context, extID string, goRT, mcpRT *runtime_supervisor.FakeRuntime) (*Runtime, *Container, string) {
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
	if err := defaultSupervisor.RegisterFactory(runtime_supervisor.NewFakeFactory(domain.RuntimeTypeGo, goRT)); err != nil {
		t.Fatalf("RegisterFactory Go must succeed: %v", err)
	}
	if err := defaultSupervisor.RegisterFactory(runtime_supervisor.NewFakeFactory(domain.RuntimeTypeMCP, mcpRT)); err != nil {
		t.Fatalf("RegisterFactory MCP must succeed: %v", err)
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

	modA := domain.ModuleDefinition{
		ID:          "moduleA",
		ExtensionID: domain.ExtensionID(extID),
		Name:        domain.LocalizedText{Default: "moduleA"},
		Type:        domain.ModuleTypeService,
		Runtime: &domain.RuntimeDefinition{
			Type: domain.RuntimeTypeGo,
		},
	}
	if err := container.ModuleRepository.PutModule(ctx, modA); err != nil {
		t.Fatalf("PutModule A must succeed: %v", err)
	}

	modB := domain.ModuleDefinition{
		ID:          "moduleB",
		ExtensionID: domain.ExtensionID(extID),
		Name:        domain.LocalizedText{Default: "moduleB"},
		Type:        domain.ModuleTypeService,
		Runtime: &domain.RuntimeDefinition{
			Type: domain.RuntimeTypeMCP,
		},
	}
	if err := container.ModuleRepository.PutModule(ctx, modB); err != nil {
		t.Fatalf("PutModule B must succeed: %v", err)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}
	if err := container.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementDisabled); err != nil {
		t.Fatalf("SetEnablement ext must succeed: %v", err)
	}
	if err := container.EnablementStore.SetDesiredRuntime(ctx, extSubject, enablement.DesiredRuntimeStopped); err != nil {
		t.Fatalf("SetDesiredRuntime ext must succeed: %v", err)
	}

	modASubject := enablement.StateSubject{Kind: enablement.SubjectModule, ID: "moduleA", ParentID: extID}
	if err := container.EnablementStore.SetEnablement(ctx, modASubject, enablement.EnablementDisabled); err != nil {
		t.Fatalf("SetEnablement modA must succeed: %v", err)
	}
	if err := container.EnablementStore.SetDesiredRuntime(ctx, modASubject, enablement.DesiredRuntimeStopped); err != nil {
		t.Fatalf("SetDesiredRuntime modA must succeed: %v", err)
	}

	modBSubject := enablement.StateSubject{Kind: enablement.SubjectModule, ID: "moduleB", ParentID: extID}
	if err := container.EnablementStore.SetEnablement(ctx, modBSubject, enablement.EnablementDisabled); err != nil {
		t.Fatalf("SetEnablement modB must succeed: %v", err)
	}
	if err := container.EnablementStore.SetDesiredRuntime(ctx, modBSubject, enablement.DesiredRuntimeStopped); err != nil {
		t.Fatalf("SetDesiredRuntime modB must succeed: %v", err)
	}

	return rt, container, extRoot
}

func TestEnable_MultiModuleRollbackOnSecondModuleFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/enable-multi-rollback"
	goRT := runtime_supervisor.NewFakeRuntime()
	mcpRT := runtime_supervisor.NewFakeRuntime()
	mcpRT.SetStartErr(runtime_supervisor.ErrFakeStart)

	rt, container, _ := setupMultiModuleEnableTestRuntime(t, ctx, extID, goRT, mcpRT)
	defer container.Close()

	err := rt.Enable(ctx, extID)
	if err == nil {
		t.Fatalf("Enable must fail when second module runtime fails")
	}

	inst, _ := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if inst.EnablementState != domain.EnablementDisabled {
		t.Errorf("expected EnablementState=disabled, got %s", inst.EnablementState)
	}
	if inst.Generation != 0 {
		t.Errorf("expected Generation=0, got %d", inst.Generation)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}
	extState, _ := container.EnablementStore.Get(ctx, extSubject)
	if extState.Enablement != enablement.EnablementDisabled {
		t.Errorf("expected extension enablement=disabled, got %s", extState.Enablement)
	}

	modASubject := enablement.StateSubject{Kind: enablement.SubjectModule, ID: "moduleA", ParentID: extID}
	modAState, _ := container.EnablementStore.Get(ctx, modASubject)
	if modAState.Enablement != enablement.EnablementDisabled {
		t.Errorf("expected moduleA enablement=disabled, got %s", modAState.Enablement)
	}

	modBSubject := enablement.StateSubject{Kind: enablement.SubjectModule, ID: "moduleB", ParentID: extID}
	modBState, _ := container.EnablementStore.Get(ctx, modBSubject)
	if modBState.Enablement != enablement.EnablementDisabled {
		t.Errorf("expected moduleB enablement=disabled, got %s", modBState.Enablement)
	}
}

func TestEnable_NoResidualRuntimeAfterRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/enable-no-residual"
	goRT := runtime_supervisor.NewFakeRuntime()
	mcpRT := runtime_supervisor.NewFakeRuntime()
	mcpRT.SetStartErr(runtime_supervisor.ErrFakeStart)

	rt, container, _ := setupMultiModuleEnableTestRuntime(t, ctx, extID, goRT, mcpRT)
	defer container.Close()

	_ = rt.Enable(ctx, extID)

	defIDA := runtime_supervisor.BuildRuntimeDefinitionID(extID, "moduleA", domain.RuntimeTypeGo)
	snapA := container.RuntimeSupervisor.Snapshot(ctx, defIDA)
	for _, inst := range snapA.Instances {
		if inst.Actual == runtime_supervisor.ActualReady || inst.Actual == runtime_supervisor.ActualStarting {
			t.Errorf("expected moduleA runtime instance %s to be stopped, got actual=%s", inst.InstanceID, inst.Actual)
		}
	}

	if goRT.StopCount() == 0 {
		t.Errorf("expected FakeRuntime.Stop to be called for moduleA after rollback")
	}
}

func TestEnable_RollsBackCommittedStateOnModuleEnablementFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/enable-rollback-committed"
	fakeRT := runtime_supervisor.NewFakeRuntime()

	rt, container, _ := setupEnableTestRuntime(t, ctx, extID, fakeRT)
	defer container.Close()

	originalStore := container.EnablementStore
	flakyStore := &flakyStateStore{
		inner:               originalStore,
		failOnSetEnablement: 2,
		failErr:             errors.New("injected: module enablement failure"),
	}
	container.EnablementStore = flakyStore
	rt.SetContainer(container)

	err := rt.Enable(ctx, extID)
	if err == nil {
		t.Fatalf("Enable must fail when module enablement write fails")
	}

	inst, _ := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if inst.EnablementState != domain.EnablementDisabled {
		t.Errorf("expected EnablementState=disabled (rolled back), got %s", inst.EnablementState)
	}
	if inst.Generation != 0 {
		t.Errorf("expected Generation=0 (rolled back), got %d", inst.Generation)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}
	extState, _ := container.EnablementStore.Get(ctx, extSubject)
	if extState.Enablement != enablement.EnablementDisabled {
		t.Errorf("expected extension enablement=disabled (rolled back), got %s", extState.Enablement)
	}
	if extState.DesiredRuntime != enablement.DesiredRuntimeStopped {
		t.Errorf("expected extension desiredRuntime=stopped (rolled back), got %s", extState.DesiredRuntime)
	}

	defID := runtime_supervisor.BuildRuntimeDefinitionID(extID, "main", domain.RuntimeTypeGo)
	snap := container.RuntimeSupervisor.Snapshot(ctx, defID)
	for _, inst := range snap.Instances {
		if inst.Actual == runtime_supervisor.ActualReady || inst.Actual == runtime_supervisor.ActualStarting {
			t.Errorf("expected runtime instance %s to be stopped after rollback, got actual=%s", inst.InstanceID, inst.Actual)
		}
	}

	if fakeRT.StopCount() == 0 {
		t.Errorf("expected FakeRuntime.Stop to be called after rollback of committed state")
	}
}
