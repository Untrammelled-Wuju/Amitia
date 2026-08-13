package upgrade

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration"
	"github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/config"
)

type fakePluginRegistry struct {
	mu          sync.Mutex
	byExtension map[string][]domain.PluginDescriptor
	plugins     map[domain.PluginID]domain.PluginDescriptor
}

func newFakePluginRegistry() *fakePluginRegistry {
	return &fakePluginRegistry{
		byExtension: make(map[string][]domain.PluginDescriptor),
		plugins:     make(map[domain.PluginID]domain.PluginDescriptor),
	}
}

func (f *fakePluginRegistry) addPlugin(extID string, desc domain.PluginDescriptor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byExtension[extID] = append(f.byExtension[extID], desc)
	f.plugins[desc.ID] = desc
}

func (f *fakePluginRegistry) ListByExtension(ctx context.Context, extensionID string) ([]domain.PluginDescriptor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.PluginDescriptor, 0, len(f.byExtension[extensionID]))
	for _, d := range f.byExtension[extensionID] {
		result = append(result, d.Clone())
	}
	return result, nil
}

func (f *fakePluginRegistry) Get(ctx context.Context, pluginID domain.PluginID) (domain.PluginDescriptor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.plugins[pluginID]
	if !ok {
		return domain.PluginDescriptor{}, domain.NewHostError(domain.ErrNotFound, "not found")
	}
	return d.Clone(), nil
}

func (f *fakePluginRegistry) Snapshot() []domain.PluginDescriptor {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.PluginDescriptor, 0, len(f.plugins))
	for _, d := range f.plugins {
		result = append(result, d.Clone())
	}
	return result
}

func (f *fakePluginRegistry) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.plugins)
}

type fakeRuntimeManager struct {
	mu        sync.Mutex
	runtimes  map[domain.RuntimeInstanceID]*runtime.RuntimeInstanceRef
	stopCount int
	startCount int
	failStop  domain.RuntimeInstanceID
	failStart domain.RuntimeInstanceID
}

func newFakeRuntimeManager() *fakeRuntimeManager {
	return &fakeRuntimeManager{
		runtimes: make(map[domain.RuntimeInstanceID]*runtime.RuntimeInstanceRef),
	}
}

func (f *fakeRuntimeManager) addRuntime(id domain.RuntimeInstanceID, pluginID domain.PluginID, state domain.RuntimeState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runtimes[id] = &runtime.RuntimeInstanceRef{
		ID:       id,
		PluginID: pluginID,
		State:    state,
	}
}

func (f *fakeRuntimeManager) GetRuntime(runtimeID domain.RuntimeInstanceID) (*runtime.RuntimeInstanceRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runtimes[runtimeID]
	if !ok {
		return nil, errors.New("runtime not found")
	}
	return &runtime.RuntimeInstanceRef{ID: r.ID, PluginID: r.PluginID, State: r.State}, nil
}

func (f *fakeRuntimeManager) ListRuntimes() []*runtime.RuntimeInstanceRef {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]*runtime.RuntimeInstanceRef, 0, len(f.runtimes))
	for _, r := range f.runtimes {
		result = append(result, &runtime.RuntimeInstanceRef{ID: r.ID, PluginID: r.PluginID, State: r.State})
	}
	return result
}

func (f *fakeRuntimeManager) setState(runtimeID domain.RuntimeInstanceID, state domain.RuntimeState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.runtimes[runtimeID]; ok {
		r.State = state
	}
}

type fakeRuntimeExecutor struct {
	mu           sync.Mutex
	stopped      []domain.RuntimeInstanceID
	started      []domain.RuntimeInstanceID
	failStop     error
	failStart    error
	beforeStopFn func(id domain.RuntimeInstanceID)
	afterStartFn func(id domain.RuntimeInstanceID)
}

func newFakeRuntimeExecutor() *fakeRuntimeExecutor {
	return &fakeRuntimeExecutor{}
}

func (f *fakeRuntimeExecutor) StartRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failStart != nil {
		return f.failStart
	}
	f.started = append(f.started, runtimeID)
	if f.afterStartFn != nil {
		f.afterStartFn(runtimeID)
	}
	return nil
}

func (f *fakeRuntimeExecutor) StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failStop != nil {
		return f.failStop
	}
	f.stopped = append(f.stopped, runtimeID)
	if f.beforeStopFn != nil {
		f.beforeStopFn(runtimeID)
	}
	return nil
}

func (f *fakeRuntimeExecutor) StartServices(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceIDs []domain.ServiceID) error {
	return nil
}

func (f *fakeRuntimeExecutor) StopServices(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceIDs []domain.ServiceID) error {
	return nil
}

func (f *fakeRuntimeExecutor) CleanupRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	return nil
}

func (f *fakeRuntimeExecutor) SetResolveDefinition(fn runtime.DefinitionResolverFunc) {}

func (f *fakeRuntimeExecutor) setFailStop(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failStop = err
}

type fakeDefinitionReconciler struct {
	mu          sync.Mutex
	reconciled  []string
	failNext    bool
	failWithErr error
}

func (f *fakeDefinitionReconciler) ReconcileExtension(extensionID string) *service_definition.ReconcileReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reconciled = append(f.reconciled, extensionID)
	if f.failNext {
		return &service_definition.ReconcileReport{
			ExtensionID: extensionID,
			Errors:      []error{failWithErr},
		}
	}
	return &service_definition.ReconcileReport{ExtensionID: extensionID}
}

var failWithErr = errors.New("definition reconcile failure")

type fakeContributionReconciler struct {
	mu          sync.Mutex
	synced      []string
	failNext    bool
	failWithErr error
}

func (f *fakeContributionReconciler) SyncExtension(ctx context.Context, extensionID string) integration.SyncResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.synced = append(f.synced, extensionID)
	if f.failNext {
		return integration.SyncResult{Failed: 1, Errors: []error{errors.New("sync failure")}}
	}
	return integration.SyncResult{}
}

type fakeConfigValidator struct {
	mu          sync.Mutex
	validated   []string
	failNext    bool
}

func (f *fakeConfigValidator) Resolve(ctx context.Context, pluginID, runtimeID, serviceID string) (*config.ScopedConfig, []config.ValidationError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := pluginID + "/" + runtimeID
	f.validated = append(f.validated, key)
	if f.failNext {
		return nil, []config.ValidationError{{Key: "test", Message: "config error"}}
	}
	return &config.ScopedConfig{Revision: "test-rev-1"}, nil
}

type fakeKernelLifecycle struct {
	mu           sync.Mutex
	updated      []string
	failNext     bool
	updateCalls  int
	beforeUpdate func(extensionID string)
}

func (f *fakeKernelLifecycle) ExecuteUpdate(ctx context.Context, extensionID string, targetVersion string, operationID UpgradeOperationID) (*KernelUpdateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, extensionID)
	f.updateCalls++
	if f.beforeUpdate != nil {
		f.beforeUpdate(extensionID)
	}
	if f.failNext {
		return &KernelUpdateResult{Success: false, Reason: "test failure"}, nil
	}
	return &KernelUpdateResult{Success: true, NewVersion: targetVersion}, nil
}

func (f *fakeKernelLifecycle) requireUpdate(callCount int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.updateCalls == callCount
}

type fakeMigrationHook struct {
	mu           sync.Mutex
	executed     []MigrationContext
	result       MigrationResult
	err          error
}

func (f *fakeMigrationHook) ExecuteMigration(ctx context.Context, mc MigrationContext) (MigrationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.executed = append(f.executed, mc)
	if f.err != nil {
		return MigrationResultFailed, f.err
	}
	return f.result, nil
}

func setupTestCoordinator(reg *fakePluginRegistry, rtMgr *fakeRuntimeManager, rtExec *fakeRuntimeExecutor) (*UpgradeCoordinator, *fakeDefinitionReconciler, *fakeContributionReconciler, *fakeConfigValidator, *fakeKernelLifecycle) {
	defRec := &fakeDefinitionReconciler{}
	contribRec := &fakeContributionReconciler{}
	cfgVal := &fakeConfigValidator{}
	kernel := &fakeKernelLifecycle{}

	c := NewUpgradeCoordinator(
		reg,
		rtMgr,
		rtExec,
		defRec,
		contribRec,
		cfgVal,
		kernel,
	)
	return c, defRec, contribRec, cfgVal, kernel
}

func TestUpgrade_NoActiveRuntime(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{
		ID:          "plugin-a",
		ExtensionID: "ext-1",
		Name:        "Plugin A",
		Version:     "1.0.0",
	})

	rtMgr := newFakeRuntimeManager()
	rtExec := newFakeRuntimeExecutor()
	c, defRec, contribRec, _, kernel := setupTestCoordinator(reg, rtMgr, rtExec)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected result.Success=true")
	}
	if len(result.QuiescedRuntimes) != 0 {
		t.Errorf("expected 0 quiesced runtimes, got %d", len(result.QuiescedRuntimes))
	}
	if len(result.ResumedRuntimes) != 0 {
		t.Errorf("expected 0 resumed runtimes, got %d", len(result.ResumedRuntimes))
	}
	if len(kernel.updated) != 1 || kernel.updated[0] != "ext-1" {
		t.Errorf("expected kernel update to be called once for ext-1, got %v", kernel.updated)
	}
	if len(defRec.reconciled) != 1 {
		t.Fatalf("expected definition reconcile to be called once, got %d", len(defRec.reconciled))
	}
	if len(contribRec.synced) != 1 {
		t.Fatalf("expected contribution sync to be called once, got %d", len(contribRec.synced))
	}
	if len(rtExec.stopped) != 0 {
		t.Errorf("expected 0 stops, got %d", len(rtExec.stopped))
	}
}

func TestUpgrade_OneActiveRuntime(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{
		ID:          "plugin-a",
		ExtensionID: "ext-1",
		Name:        "Plugin A",
		Version:     "1.0.0",
	})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, kernel := setupTestCoordinator(reg, rtMgr, rtExec)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected result.Success=true")
	}
	if len(result.QuiescedRuntimes) != 1 || result.QuiescedRuntimes[0] != "rt-1" {
		t.Errorf("expected quiesced=[rt-1], got %v", result.QuiescedRuntimes)
	}
	if len(result.ResumedRuntimes) != 1 || result.ResumedRuntimes[0] != "rt-1" {
		t.Errorf("expected resumed=[rt-1], got %v", result.ResumedRuntimes)
	}
	if len(kernel.updated) != 1 {
		t.Errorf("expected 1 kernel update, got %d", len(kernel.updated))
	}

	stoppedBeforeUpdate := false
	for _, s := range rtExec.stopped {
		if s == "rt-1" {
			stoppedBeforeUpdate = true
		}
	}
	if !stoppedBeforeUpdate {
		t.Error("expected rt-1 to be stopped before kernel update")
	}
}

func TestUpgrade_MultipleRuntimes(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{
		ID:          "plugin-a",
		ExtensionID: "ext-1",
		Name:        "Plugin A",
		Version:     "1.0.0",
	})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateRunning)
	rtMgr.addRuntime("rt-2", "plugin-a", domain.RuntimeStateRunning)
	rtMgr.addRuntime("rt-3", "plugin-a", domain.RuntimeStateDegraded)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, _ := setupTestCoordinator(reg, rtMgr, rtExec)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if len(result.QuiescedRuntimes) != 3 {
		t.Fatalf("expected 3 quiesced runtimes, got %d", len(result.QuiescedRuntimes))
	}
	if len(result.ResumedRuntimes) != 3 {
		t.Fatalf("expected 3 resumed runtimes, got %d", len(result.ResumedRuntimes))
	}
	if len(rtExec.stopped) != 3 {
		t.Fatalf("expected 3 stops, got %d", len(rtExec.stopped))
	}
	if len(rtExec.started) != 3 {
		t.Fatalf("expected 3 starts, got %d", len(rtExec.started))
	}
}

func TestUpgrade_MultiplePluginsOneExtension(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-b", ExtensionID: "ext-1", Name: "B", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-a", "plugin-a", domain.RuntimeStateRunning)
	rtMgr.addRuntime("rt-b", "plugin-b", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, _ := setupTestCoordinator(reg, rtMgr, rtExec)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if len(result.AffectedPlugins) != 2 {
		t.Fatalf("expected 2 affected plugins, got %d", len(result.AffectedPlugins))
	}
	if len(result.QuiescedRuntimes) != 2 {
		t.Fatalf("expected 2 quiesced runtimes, got %d", len(result.QuiescedRuntimes))
	}
	if len(result.ResumedRuntimes) != 2 {
		t.Fatalf("expected 2 resumed runtimes, got %d", len(result.ResumedRuntimes))
	}
}

func TestUpgrade_UnrelatedExtensionIsolation(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})
	reg.addPlugin("ext-2", domain.PluginDescriptor{ID: "plugin-x", ExtensionID: "ext-2", Name: "X", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-a", "plugin-a", domain.RuntimeStateRunning)
	rtMgr.addRuntime("rt-x", "plugin-x", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, kernel := setupTestCoordinator(reg, rtMgr, rtExec)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(kernel.updated) != 1 || kernel.updated[0] != "ext-1" {
		t.Errorf("expected kernel update for ext-1 only, got %v", kernel.updated)
	}

	for _, stopped := range rtExec.stopped {
		if stopped == "rt-x" {
			t.Error("rt-x (unrelated extension) should not be stopped")
		}
	}
	for _, started := range rtExec.started {
		if started == "rt-x" {
			t.Error("rt-x (unrelated extension) should not be resumed")
		}
	}

	if _, err := rtMgr.GetRuntime("rt-x"); err != nil {
		t.Error("rt-x should still be in manager")
	}
	rtx, _ := rtMgr.GetRuntime("rt-x")
	if rtx.State != domain.RuntimeStateRunning {
		t.Error("rt-x state should remain running")
	}
}

func TestUpgrade_QuiesceBeforeKernelUpdate(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, kernel := setupTestCoordinator(reg, rtMgr, rtExec)

	var seqMu sync.Mutex
	var events []string

	kernel.beforeUpdate = func(extensionID string) {
		seqMu.Lock()
		defer seqMu.Unlock()
		events = append(events, "kernel-update")
	}
	rtExec.beforeStopFn = func(id domain.RuntimeInstanceID) {
		seqMu.Lock()
		defer seqMu.Unlock()
		events = append(events, "stop-"+string(id))
	}
	rtExec.afterStartFn = func(id domain.RuntimeInstanceID) {
		seqMu.Lock()
		defer seqMu.Unlock()
		events = append(events, "start-"+string(id))
	}

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	_, err := c.ExecuteUpgrade(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	seqMu.Lock()
	defer seqMu.Unlock()

	stopIdx := -1
	updateIdx := -1
	for i, e := range events {
		if e == "stop-rt-1" && stopIdx == -1 {
			stopIdx = i
		}
		if e == "kernel-update" && updateIdx == -1 {
			updateIdx = i
		}
	}
	if stopIdx == -1 {
		t.Fatal("expected stop event in sequence")
	}
	if updateIdx == -1 {
		t.Fatal("expected kernel-update event in sequence")
	}
	if stopIdx >= updateIdx {
		t.Errorf("expected stop (idx=%d) before kernel-update (idx=%d), events=%v", stopIdx, updateIdx, events)
	}
}

func TestUpgrade_QuiesceFailure_BlocksUpdate(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()
	rtExec.setFailStop(errors.New("stop failed"))

	c, _, _, _, kernel := setupTestCoordinator(reg, rtMgr, rtExec)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	_, err := c.ExecuteUpgrade(context.Background(), req)

	if err == nil {
		t.Fatal("expected error from quiesce failure")
	}
	if kernel.updateCalls != 0 {
		t.Fatalf("expected 0 kernel update calls (quiesce failed), got %d", kernel.updateCalls)
	}
}

func TestUpgrade_ConcurrentUpdateSameExtension(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtExec := newFakeRuntimeExecutor()
	c, _, _, _, _ := setupTestCoordinator(reg, rtMgr, rtExec)

	var wg sync.WaitGroup
	resultsCh := make(chan *UpgradeResult, 2)
	barrier := make(chan struct{})

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-barrier
			req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
			result, _ := c.ExecuteUpgrade(context.Background(), req)
			resultsCh <- result
		}()
	}

	close(barrier)
	wg.Wait()
	close(resultsCh)

	var results []*UpgradeResult
	for r := range resultsCh {
		results = append(results, r)
	}

	successCount := 0
	failCount := 0
	for _, r := range results {
		if r == nil {
			failCount++
			continue
		}
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	if successCount != 2 {
		t.Errorf("expected 2 success (no runtime, fast completion), got %d (failures=%d)", successCount, failCount)
	}
}

func TestUpgrade_ParallelDifferentExtensions(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})
	reg.addPlugin("ext-2", domain.PluginDescriptor{ID: "plugin-b", ExtensionID: "ext-2", Name: "B", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtExec := newFakeRuntimeExecutor()
	c, _, _, _, kernel := setupTestCoordinator(reg, rtMgr, rtExec)

	var wg sync.WaitGroup
	result1Ch := make(chan *UpgradeResult, 1)
	result2Ch := make(chan *UpgradeResult, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
		result, _ := c.ExecuteUpgrade(context.Background(), req)
		result1Ch <- result
	}()
	go func() {
		defer wg.Done()
		req := UpgradeRequest{ExtensionID: "ext-2", TargetVersion: "2.0.0"}
		result, _ := c.ExecuteUpgrade(context.Background(), req)
		result2Ch <- result
	}()
	wg.Wait()

	result1 := <-result1Ch
	result2 := <-result2Ch

	if !result1.Success {
		t.Errorf("expected ext-1 upgrade success, got error: %v", result1.Error)
	}
	if !result2.Success {
		t.Errorf("expected ext-2 upgrade success, got error: %v", result2.Error)
	}
	if len(kernel.updated) != 2 {
		t.Errorf("expected 2 kernel updates (parallel), got %d", len(kernel.updated))
	}
}

func TestUpgrade_MigrationHookExecuted(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, _ := setupTestCoordinator(reg, rtMgr, rtExec)

	hook := &fakeMigrationHook{result: MigrationResultSuccess}
	c.RegisterMigrationHook("ext-1", hook)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	_, err := c.ExecuteUpgrade(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hook.executed) != 1 {
		t.Fatalf("expected 1 migration hook execution, got %d", len(hook.executed))
	}
	if hook.executed[0].ExtensionID != "ext-1" {
		t.Errorf("expected extension ID ext-1, got %s", hook.executed[0].ExtensionID)
	}
	if hook.executed[0].ToVersion != "2.0.0" {
		t.Errorf("expected target version 2.0.0, got %s", hook.executed[0].ToVersion)
	}
}

func TestUpgrade_MigrationHookFailureBlocksResume(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, _ := setupTestCoordinator(reg, rtMgr, rtExec)

	hook := &fakeMigrationHook{result: MigrationResultFailed}
	c.RegisterMigrationHook("ext-1", hook)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err == nil {
		t.Fatal("expected error from migration hook failure")
	}
	if result.Success {
		t.Error("result.Success should be false after migration failure")
	}
	if len(rtExec.started) != 0 {
		t.Errorf("expected 0 runtime starts (migration failed), got %d", len(rtExec.started))
	}
}

func TestUpgrade_NoMigrationWhenNoHook(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, _ := setupTestCoordinator(reg, rtMgr, rtExec)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestUpgrade_DefinitionReconcileFailureBlocksResume(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, defRec, _, _, kernel := setupTestCoordinator(reg, rtMgr, rtExec)
	defRec.failNext = true
	defRec.failWithErr = errors.New("definition error")

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err == nil {
		t.Fatal("expected error from definition reconcile failure")
	}
	if result.Success {
		t.Error("result should not be successful")
	}
	if len(rtExec.started) != 0 {
		t.Errorf("expected 0 resumes (definition reconcile failed), got %d", len(rtExec.started))
	}
	_ = kernel
}

func TestUpgrade_WasStopped_NotResumed(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateStopped)
	rtMgr.addRuntime("rt-2", "plugin-a", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, _ := setupTestCoordinator(reg, rtMgr, rtExec)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ResumedRuntimes) != 1 || result.ResumedRuntimes[0] != "rt-2" {
		t.Errorf("expected only rt-2 resumed, got %v", result.ResumedRuntimes)
	}
}

func TestUpgrade_WasFailed_NotResumed(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateFailed)
	rtMgr.addRuntime("rt-2", "plugin-a", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, _ := setupTestCoordinator(reg, rtMgr, rtExec)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ResumedRuntimes) != 1 || result.ResumedRuntimes[0] != "rt-2" {
		t.Errorf("expected only rt-2 resumed (not failed rt-1), got %v", result.ResumedRuntimes)
	}
}

func TestUpgrade_GateIsolation(t *testing.T) {
	reg := newFakePluginRegistry()
	rtMgr := newFakeRuntimeManager()
	rtExec := newFakeRuntimeExecutor()
	c, _, _, _, _ := setupTestCoordinator(reg, rtMgr, rtExec)

	if c.IsExtensionUpgrading("ext-1") {
		t.Error("ext-1 should not be upgrading initially")
	}

	gate := NewUpgradeGate()
	err := gate.Acquire("ext-1", "op-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gate.IsUpgrading("ext-1") {
		t.Error("ext-1 should be upgrading after acquire")
	}

	err = gate.Acquire("ext-1", "op-2")
	if err != nil {
		t.Fatalf("unexpected error acquiring second op: %v", err)
	}
	if !gate.IsUpgrading("ext-1") {
		t.Error("ext-1 should still be upgrading with concurrent ops")
	}

	gate.Release("ext-1", "op-1")
	if !gate.IsUpgrading("ext-1") {
		t.Error("ext-1 should still be upgrading after releasing one op")
	}

	gate.Release("ext-1", "op-2")
	if gate.IsUpgrading("ext-1") {
		t.Error("ext-1 should not be upgrading after releasing both ops")
	}
}

func TestUpgrade_StagesCompleted(t *testing.T) {
	reg := newFakePluginRegistry()
	reg.addPlugin("ext-1", domain.PluginDescriptor{ID: "plugin-a", ExtensionID: "ext-1", Name: "A", Version: "1.0.0"})

	rtMgr := newFakeRuntimeManager()
	rtMgr.addRuntime("rt-1", "plugin-a", domain.RuntimeStateRunning)
	rtExec := newFakeRuntimeExecutor()

	c, _, _, _, _ := setupTestCoordinator(reg, rtMgr, rtExec)

	req := UpgradeRequest{ExtensionID: "ext-1", TargetVersion: "2.0.0"}
	result, err := c.ExecuteUpgrade(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stage != UpgradeStateCompleted {
		t.Errorf("expected final stage=completed, got %s", result.Stage)
	}
}
