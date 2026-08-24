package resource

import (
	"context"
	"sync"
	"testing"
)

type fakeReader struct {
	mu       sync.RWMutex
	runtimes map[string]runtimeEntry
	services map[string]serviceEntry
	disabled map[string]bool
}

type runtimeEntry struct {
	pluginID    string
	extensionID string
	state       string
}

type serviceEntry struct {
	pluginID    string
	extensionID string
}

func newFakeReader() *fakeReader {
	return &fakeReader{
		runtimes: make(map[string]runtimeEntry),
		services: make(map[string]serviceEntry),
		disabled: make(map[string]bool),
	}
}

func (f *fakeReader) AddRuntime(runtimeID, pluginID, extensionID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runtimes[runtimeID] = runtimeEntry{pluginID: pluginID, extensionID: extensionID, state: "ready"}
}

func (f *fakeReader) AddService(runtimeID, serviceID, pluginID, extensionID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.services[runtimeID+"/"+serviceID] = serviceEntry{pluginID: pluginID, extensionID: extensionID}
}

func (f *fakeReader) SetDisabled(extensionID string, disabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disabled[extensionID] = disabled
}

func (f *fakeReader) ResolveRuntime(runtimeID string) (pluginID string, extensionID string, state string, err error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.runtimes[runtimeID]
	if !ok {
		return "", "", "", ErrRuntimeNotFound
	}
	return e.pluginID, e.extensionID, e.state, nil
}

func (f *fakeReader) ResolveService(runtimeID, serviceID string) (pluginID string, extensionID string, state string, err error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.services[runtimeID+"/"+serviceID]
	if !ok {
		return "", "", "", ErrServiceNotFound
	}
	return e.pluginID, e.extensionID, "", nil
}

func (f *fakeReader) ExtensionEnabled(extensionID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return !f.disabled[extensionID]
}

func (f *fakeReader) CurrentGeneration(runtimeID string) (int64, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if _, ok := f.runtimes[runtimeID]; !ok {
		return 0, ErrRuntimeNotFound
	}
	return 1, nil
}

func (f *fakeReader) RuntimeIDsByExtension(extensionID string) []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var out []string
	for id, entry := range f.runtimes {
		if entry.extensionID == extensionID {
			out = append(out, id)
		}
	}
	return out
}

type fakePending struct {
	mu    sync.Mutex
	count int
	limit int
}

func (p *fakePending) Count() int { p.mu.Lock(); defer p.mu.Unlock(); return p.count }
func (p *fakePending) CountByPeer(runtimeID, serviceID string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count
}
func (p *fakePending) LimitPerPeer() int { return p.limit }
func (p *fakePending) LimitGlobal() int  { return p.limit * 10 }

type fakeBinary struct {
	mu        sync.Mutex
	count     int
	limit     int
	byteLimit int64
}

func (b *fakeBinary) CountActive() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

func (b *fakeBinary) LimitActive() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.limit
}

func (b *fakeBinary) ActiveBytes() int64 { return int64(b.CountActive()) * 64 }
func (b *fakeBinary) LimitActiveBytes() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.byteLimit
}

type fakeGovernor struct {
	mu      sync.Mutex
	configs map[string]ServiceResourceLimitsSet
}

func (g *fakeGovernor) ConfigureResourceLimits(runtimeID string, limits ServiceResourceLimitsSet) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.configs == nil {
		g.configs = make(map[string]ServiceResourceLimitsSet)
	}
	g.configs[runtimeID] = limits
	return nil
}

func newTestAdapter() (*ResourceAdmissionAdapter, *fakeReader, *fakePending, *fakeBinary, *fakeGovernor) {
	reader := newFakeReader()
	pending := &fakePending{limit: 10}
	fbinary := &fakeBinary{limit: 5, byteLimit: 5 * 1024 * 1024}
	resourceGov := &fakeGovernor{}
	mapper := NewSubjectMapper(reader)
	adapter := NewResourceAdmissionAdapter(mapper, pending, fbinary, resourceGov)
	return adapter, reader, pending, fbinary, resourceGov
}

func TestAcquireRuntimeStartup_ValidGranted(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "plugin-1", "ext-1")
	reader.AddService("rt-1", "svc-1", "plugin-1", "ext-1")

	revert, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "plugin-1", ServiceID: "svc-1", Generation: 1}, &RuntimeResourceProfile{MaxMemoryMB: 512, MaxCPUPercent: 50, MaxFileDescriptors: 1024})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revert == nil {
		t.Fatal("expected non-nil revert")
	}
	revert()
}

func TestAcquireRuntimeStartup_UnknownRuntime(t *testing.T) {
	adapter, _, _, _, _ := newTestAdapter()
	_, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "missing", PluginID: "p", ServiceID: "s", Generation: 1}, nil)
	if err != ErrRuntimeNotFound {
		t.Fatalf("expected ErrRuntimeNotFound, got %v", err)
	}
}

func TestAcquireRuntimeStartup_UnknownService(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "plugin-1", "ext-1")
	_, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "plugin-1", ServiceID: "svc-1", Generation: 1}, nil)
	if err != ErrServiceNotFound {
		t.Fatalf("expected ErrServiceNotFound, got %v", err)
	}
}

func TestAcquireRuntimeStartup_PluginMismatch(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "plugin-A", "ext-1")
	reader.AddService("rt-1", "svc-1", "plugin-A", "ext-1")

	_, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "plugin-B", ServiceID: "svc-1", Generation: 1}, nil)
	if err != ErrSubjectInvalid {
		t.Fatalf("expected ErrSubjectInvalid, got %v", err)
	}
}

func TestAcquireRuntimeStartup_ExtensionDisabled(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")
	reader.SetDisabled("ext-1", true)

	_, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)
	if err != ErrExtensionDisabled {
		t.Fatalf("expected ErrExtensionDisabled, got %v", err)
	}
}

func TestAcquireRuntimeStartup_NegativeMemoryProfileRejected(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")

	_, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, &RuntimeResourceProfile{MaxMemoryMB: -1})
	if err != ErrProfileInvalid {
		t.Fatalf("expected ErrProfileInvalid, got %v", err)
	}
}

func TestAcquireRuntimeStartup_CPURateClamped(t *testing.T) {
	adapter, reader, _, _, gov := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")

	_, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, &RuntimeResourceProfile{MaxCPUPercent: 500})
	if err != nil {
		t.Fatalf("should accept but clamp cpu, got %v", err)
	}
	gov.mu.Lock()
	cfg := gov.configs["rt-1"]
	gov.mu.Unlock()
	if cfg.MaxCPUPercent != 100 {
		t.Fatalf("expected CPU clamped to 100, got %d", cfg.MaxCPUPercent)
	}
}

func TestAcquireRuntimeStartup_HostShutdown(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")
	adapter.Shutdown()

	_, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)
	if err != ErrHostShutdown {
		t.Fatalf("expected ErrHostShutdown, got %v", err)
	}
}

func TestAcquireRuntimeStartup_RuntimeStopping(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")

	_, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)
	if err != nil {
		t.Fatalf("initial startup should succeed: %v", err)
	}
	adapter.MarkStopping("rt-1")

	_, err = adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)
	if err != ErrRuntimeStopping {
		t.Fatalf("expected ErrRuntimeStopping, got %v", err)
	}
}

func TestAcquireRPCPending_WithinLimit(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")

	_, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)
	if err != nil {
		t.Fatalf("startup: %v", err)
	}

	decision, release := adapter.AcquireRPCPending(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1})
	if !decision.Allowed {
		t.Fatalf("should allow, got decision %+v", decision)
	}
	release()
}

func TestAcquireRPCPending_UnknownRuntime(t *testing.T) {
	adapter, _, _, _, _ := newTestAdapter()
	decision, _ := adapter.AcquireRPCPending(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "missing", PluginID: "p", ServiceID: "s", Generation: 1})
	if decision.Allowed {
		t.Fatal("should deny for missing runtime")
	}
}

func TestAcquireRPCPending_HostShutdown(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")
	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)
	adapter.Shutdown()

	decision, _ := adapter.AcquireRPCPending(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1})
	if decision.Allowed {
		t.Fatal("shutdown should deny new pending")
	}
	decision, _ = adapter.AcquireRPCPending(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "new-rt", PluginID: "p", ServiceID: "s", Generation: 1})
	if decision.Allowed {
		t.Fatal("shutdown should deny ALL new pending")
	}
}

func TestAcquireBinaryObject_WithinLimit(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")

	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)

	decision, _ := adapter.AcquireBinaryObject(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, 1024)
	if !decision.Allowed {
		t.Fatalf("should allow: %+v", decision)
	}
}

func TestAcquireBinaryObject_HostShutdown(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")
	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)
	adapter.Shutdown()

	decision, _ := adapter.AcquireBinaryObject(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, 100)
	if decision.Allowed {
		t.Fatal("should deny after shutdown")
	}
}

func TestAcquireQueuePublish_HostShutdown(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")
	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)
	adapter.Shutdown()

	decision, _ := adapter.AcquireQueuePublish(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1})
	if decision.Allowed {
		t.Fatal("should deny queue publish after shutdown")
	}
}

func TestResetClearsState(t *testing.T) {
	adapter, reader, _, _, _ := newTestAdapter()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")
	adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)
	adapter.Shutdown()
	adapter.Reset()

	_, err := adapter.AcquireRuntimeStartup(context.Background(), RuntimeIdentitySubject{
		RuntimeID: "rt-1", PluginID: "p-1", ServiceID: "s-1", Generation: 1}, nil)
	if err != nil {
		t.Fatalf("reset should clear shutdown, got %v", err)
	}
}
